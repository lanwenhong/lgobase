package network

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/selector"
)

func newInternalTCPSelector(t *testing.T, ping TcpPingSvr) (*TcpPoolSelector[*internalTCPConn], *internalTCPFactory) {
	t.Helper()
	factory := &internalTCPFactory{}
	poolSelector, err := NewTcpPoolSelector(context.Background(), &TcpPoolConfig[*internalTCPConn]{
		Addrs:        "node-a:9001/1000,node-b:9002/1000,node-c:9003/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		MaxWaiters:   8,
		Cfunc:        factory.create,
		Ping:         ping,
		PingTicker:   60,
	})
	if err != nil {
		t.Fatalf("NewTcpPoolSelector() error = %v", err)
	}
	t.Cleanup(func() {
		if err := poolSelector.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return poolSelector, factory
}

func TestTCPPoolSelectorOwnsOnePoolPerEndpoint(t *testing.T) {
	poolSelector, factory := newInternalTCPSelector(t, nil)
	if len(poolSelector.Slist) != 3 {
		t.Fatalf("pool count = %d, want 3", len(poolSelector.Slist))
	}
	if created := len(factory.connections()); created != 3 {
		t.Fatalf("prewarmed connection count = %d, want 3", created)
	}

	seenPools := make(map[*GTcpPool[*internalTCPConn]]struct{}, 3)
	for i, item := range poolSelector.Slist {
		server := item.(*TcpRpcSvr[*internalTCPConn])
		seenPools[server.Gp] = struct{}{}
		if server.Gp.Addr != server.GetAddr() || server.Gp.Port != server.GetPort() {
			t.Fatalf("server %d pool endpoint = %s:%d, want %s:%d", i, server.Gp.Addr, server.Gp.Port, server.GetAddr(), server.GetPort())
		}
	}
	if len(seenPools) != 3 {
		t.Fatalf("distinct child pool count = %d, want 3", len(seenPools))
	}
}

func TestTCPPoolSelectorRoundRobin(t *testing.T) {
	poolSelector, _ := newInternalTCPSelector(t, nil)
	want := []string{"node-a", "node-b", "node-c", "node-a", "node-b", "node-c"}
	for i, wantAddr := range want {
		server := poolSelector.RoundRobin(context.Background()).(*TcpRpcSvr[*internalTCPConn])
		if server.GetAddr() != wantAddr {
			t.Fatalf("RoundRobin(%d) address = %q, want %q", i, server.GetAddr(), wantAddr)
		}
	}
}

func TestTCPPoolSelectorRoundRobinSkipsInvalidNode(t *testing.T) {
	poolSelector, _ := newInternalTCPSelector(t, nil)
	poolSelector.Slist[1].(*TcpRpcSvr[*internalTCPConn]).SetStat(selector.SVR_NOTVALID)
	atomic.StoreInt32(&poolSelector.Pos, 0)

	want := []string{"node-a", "node-c", "node-a", "node-c", "node-a", "node-c"}
	for i, wantAddr := range want {
		server := poolSelector.RoundRobin(context.Background()).(*TcpRpcSvr[*internalTCPConn])
		if server.GetAddr() != wantAddr {
			t.Fatalf("RoundRobin(%d) address = %q, want %q", i, server.GetAddr(), wantAddr)
		}
	}
}

func TestTCPPoolSelectorConcurrentRoundRobinIsBalanced(t *testing.T) {
	poolSelector, _ := newInternalTCPSelector(t, nil)
	const calls = 3000
	var counts [3]atomic.Int32
	var wg sync.WaitGroup
	wg.Add(calls)
	for i := 0; i < calls; i++ {
		go func() {
			defer wg.Done()
			server := poolSelector.RoundRobin(context.Background()).(*TcpRpcSvr[*internalTCPConn])
			counts[server.GetPort()-9001].Add(1)
		}()
	}
	wg.Wait()
	for i := range counts {
		if got := counts[i].Load(); got != calls/3 {
			t.Fatalf("node %d selection count = %d, want %d", i, got, calls/3)
		}
	}
}

func TestTCPPoolSelectorProcessErrorWithoutPingKeepsNodeValid(t *testing.T) {
	poolSelector, _ := newInternalTCPSelector(t, nil)
	wantErr := errors.New("request failed")
	if err := poolSelector.Process(context.Background(), func(interface{}) (string, error) {
		return "request", wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Process() error = %v, want %v", err, wantErr)
	}
	for i, item := range poolSelector.Slist {
		if got := item.(*TcpRpcSvr[*internalTCPConn]).GetValid(); got != selector.SVR_VALID {
			t.Fatalf("server %d validity = %d, want valid", i, got)
		}
	}
}

func TestTCPPoolSelectorHealthCheckRestoresQuarantinedNode(t *testing.T) {
	pinged := make(chan struct{}, 1)
	poolSelector, _ := newInternalTCPSelector(t, func(interface{}) (string, error) {
		select {
		case pinged <- struct{}{}:
		default:
		}
		return "ping", nil
	})
	wantErr := errors.New("transport failed")
	if err := poolSelector.Process(context.Background(), func(interface{}) (string, error) {
		return "request", wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("Process() error = %v, want %v", err, wantErr)
	}

	select {
	case <-pinged:
	case <-time.After(time.Second):
		t.Fatal("health check was not triggered after request failure")
	}
	waitForTCPPoolCondition(t, func() bool {
		return poolSelector.Slist[0].(*TcpRpcSvr[*internalTCPConn]).GetValid() == selector.SVR_VALID
	})
}

func TestTCPPoolSelectorCloseClosesEveryChildPool(t *testing.T) {
	poolSelector, factory := newInternalTCPSelector(t, nil)
	if err := poolSelector.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if selected := poolSelector.RoundRobin(context.Background()); selected != nil {
		t.Fatalf("RoundRobin() after Close = %v, want nil", selected)
	}
	for i, item := range poolSelector.Slist {
		server := item.(*TcpRpcSvr[*internalTCPConn])
		if conn, err := server.Gp.Get(context.Background()); conn != nil || !errors.Is(err, ErrTcpPoolClosed) {
			t.Fatalf("child pool %d Get() after Close = (%v, %v), want (nil, ErrTcpPoolClosed)", i, conn, err)
		}
	}
	for i, conn := range factory.connections() {
		if got := conn.closeCalls.Load(); got != 1 {
			t.Fatalf("connection %d close calls = %d, want 1", i, got)
		}
	}
}

func TestTCPPoolSelectorRejectsInvalidConfiguration(t *testing.T) {
	factory := &internalTCPFactory{}
	tests := []struct {
		name string
		conf *TcpPoolConfig[*internalTCPConn]
	}{
		{name: "nil config", conf: nil},
		{name: "nil factory", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:1/100", MaxConns: 1}},
		{name: "zero max connections", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:1/100", Cfunc: factory.create}},
		{name: "too many idle connections", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:1/100", MaxConns: 1, MaxIdleConns: 2, Cfunc: factory.create}},
		{name: "missing timeout", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:1", MaxConns: 1, Cfunc: factory.create}},
		{name: "invalid port", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:x/100", MaxConns: 1, Cfunc: factory.create}},
		{name: "invalid timeout", conf: &TcpPoolConfig[*internalTCPConn]{Addrs: "node:1/0", MaxConns: 1, Cfunc: factory.create}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewTcpPoolSelector(context.Background(), tt.conf); err == nil {
				t.Fatal("NewTcpPoolSelector() error = nil, want configuration error")
			}
		})
	}
}
