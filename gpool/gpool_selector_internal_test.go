package gpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/selector"
)

type internalSelectorFactory struct {
	mu      sync.Mutex
	created []*internalTestConn
	addrs   []string
	ports   []int
}

func (f *internalSelectorFactory) create(_ context.Context, addr string, port int, _ int) (Conn[internalTestClient], error) {
	conn := &internalTestConn{}
	_ = conn.Open()
	f.mu.Lock()
	f.created = append(f.created, conn)
	f.addrs = append(f.addrs, addr)
	f.ports = append(f.ports, port)
	f.mu.Unlock()
	return conn, nil
}

func (f *internalSelectorFactory) connections() []*internalTestConn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*internalTestConn(nil), f.created...)
}

func newInternalRPCPoolSelector(t *testing.T, ping PingSvr) (*RpcPoolSelector[internalTestClient], *internalSelectorFactory) {
	t.Helper()
	factory := &internalSelectorFactory{}
	poolSelector, err := NewRpcPoolSelectorWithError(context.Background(), &GPoolConfig[internalTestClient]{
		Addrs:        "node-a:9001/1000,node-b:9002/1000,node-c:9003/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		MaxWaiters:   8,
		Cfunc:        factory.create,
		Ping:         ping,
		PingTicker:   60,
	})
	if err != nil {
		t.Fatalf("NewRpcPoolSelectorWithError() error = %v", err)
	}
	t.Cleanup(func() {
		if err := poolSelector.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return poolSelector, factory
}

func waitForRPCSelectorCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for RPC selector condition")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRPCPoolSelectorOwnsOnePoolPerEndpoint(t *testing.T) {
	poolSelector, factory := newInternalRPCPoolSelector(t, nil)
	if len(poolSelector.Slist) != 3 {
		t.Fatalf("pool count = %d, want 3", len(poolSelector.Slist))
	}
	if created := len(factory.connections()); created != 3 {
		t.Fatalf("prewarmed connection count = %d, want 3", created)
	}

	seenPools := make(map[*Gpool[internalTestClient]]struct{}, 3)
	for i, item := range poolSelector.Slist {
		server := item.(*RpcSvr[internalTestClient])
		seenPools[server.Gp] = struct{}{}
		if server.Gp.Addr != server.GetAddr() || server.Gp.Port != server.GetPort() {
			t.Fatalf("server %d pool endpoint = %s:%d, want %s:%d", i, server.Gp.Addr, server.Gp.Port, server.GetAddr(), server.GetPort())
		}
	}
	if len(seenPools) != 3 {
		t.Fatalf("distinct child pool count = %d, want 3", len(seenPools))
	}
}

func TestRPCPoolSelectorRoundRobin(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, nil)
	want := []string{"node-a", "node-b", "node-c", "node-a", "node-b", "node-c"}
	for i, wantAddr := range want {
		server := poolSelector.RoundRobin(context.Background()).(*RpcSvr[internalTestClient])
		if server.GetAddr() != wantAddr {
			t.Fatalf("RoundRobin(%d) address = %q, want %q", i, server.GetAddr(), wantAddr)
		}
	}
}

func TestRPCPoolSelectorRoundRobinSkipsInvalidNode(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, nil)
	poolSelector.Slist[1].(*RpcSvr[internalTestClient]).SetStat(selector.SVR_NOTVALID)
	atomic.StoreInt32(&poolSelector.Pos, 0)

	want := []string{"node-a", "node-c", "node-a", "node-c", "node-a", "node-c"}
	for i, wantAddr := range want {
		server := poolSelector.RoundRobin(context.Background()).(*RpcSvr[internalTestClient])
		if server.GetAddr() != wantAddr {
			t.Fatalf("RoundRobin(%d) address = %q, want %q", i, server.GetAddr(), wantAddr)
		}
	}
}

func TestRPCPoolSelectorConcurrentRoundRobinIsBalanced(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, nil)
	const calls = 3000
	var counts [3]atomic.Int32
	var wg sync.WaitGroup
	wg.Add(calls)
	for i := 0; i < calls; i++ {
		go func() {
			defer wg.Done()
			server := poolSelector.RoundRobin(context.Background()).(*RpcSvr[internalTestClient])
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

func TestRPCPoolSelectorRoundRobinDoesNotAllocate(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, nil)
	allocs := testing.AllocsPerRun(1000, func() {
		if poolSelector.RoundRobin(context.Background()) == nil {
			panic("RoundRobin returned nil")
		}
	})
	if allocs != 0 {
		t.Fatalf("RoundRobin allocations = %v, want 0", allocs)
	}
}

func TestRPCPoolSelectorDefaultsPingInterval(t *testing.T) {
	factory := &internalSelectorFactory{}
	conf := &GPoolConfig[internalTestClient]{
		Addrs:        "node:9000/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        factory.create,
		Ping: func(interface{}) (string, error) {
			return "ping", nil
		},
	}
	poolSelector, err := NewRpcPoolSelectorWithError(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewRpcPoolSelectorWithError() error = %v", err)
	}
	defer poolSelector.Close(context.Background())
	if conf.PingTicker != int64(defaultRPCPingInterval/time.Second) {
		t.Fatalf("PingTicker = %d, want %d", conf.PingTicker, int64(defaultRPCPingInterval/time.Second))
	}
}

func TestRPCPoolSelectorRejectsInvalidConfiguration(t *testing.T) {
	factory := &internalSelectorFactory{}
	tests := []struct {
		name string
		conf *GPoolConfig[internalTestClient]
	}{
		{name: "nil config", conf: nil},
		{name: "nil factory", conf: &GPoolConfig[internalTestClient]{Addrs: "node:9000/1000", MaxConns: 1}},
		{name: "negative max connections", conf: &GPoolConfig[internalTestClient]{Addrs: "node:9000/1000", MaxConns: -1, Cfunc: factory.create}},
		{name: "negative idle connections", conf: &GPoolConfig[internalTestClient]{Addrs: "node:9000/1000", MaxConns: 1, MaxIdleConns: -1, Cfunc: factory.create}},
		{name: "missing timeout", conf: &GPoolConfig[internalTestClient]{Addrs: "node:9000", MaxConns: 1, MaxIdleConns: 1, Cfunc: factory.create}},
		{name: "invalid port", conf: &GPoolConfig[internalTestClient]{Addrs: "node:x/1000", MaxConns: 1, MaxIdleConns: 1, Cfunc: factory.create}},
		{name: "invalid timeout", conf: &GPoolConfig[internalTestClient]{Addrs: "node:9000/0", MaxConns: 1, MaxIdleConns: 1, Cfunc: factory.create}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewRpcPoolSelectorWithError(context.Background(), tt.conf); err == nil {
				t.Fatal("NewRpcPoolSelectorWithError() error = nil, want configuration error")
			}
		})
	}
}

func TestRPCPoolSelectorPreservesGPoolConfigPositionalLayout(t *testing.T) {
	factory := &internalSelectorFactory{}
	conf := GPoolConfig[internalTestClient]{
		"node:9000/1000", 1, 1, 1, 0, 0, 0,
		factory.create, nil, nil, 0, nil,
	}
	poolSelector, err := NewRpcPoolSelectorWithError(context.Background(), &conf)
	if err != nil {
		t.Fatalf("NewRpcPoolSelectorWithError() error = %v", err)
	}
	defer poolSelector.Close(context.Background())
}

func TestRPCPoolSelectorPreservesIdleClampBehavior(t *testing.T) {
	factory := &internalSelectorFactory{}
	conf := &GPoolConfig[internalTestClient]{
		Addrs:        "node:9000/1000",
		MaxConns:     1,
		MaxIdleConns: 2,
		Cfunc:        factory.create,
	}
	poolSelector := &RpcPoolSelector[internalTestClient]{}
	if err := poolSelector.RpcPoolInit(context.Background(), conf); err != nil {
		t.Fatalf("RpcPoolInit() error = %v", err)
	}
	defer poolSelector.Close(context.Background())
	server := poolSelector.Slist[0].(*RpcSvr[internalTestClient])
	if server.Gp.MaxIdleConns != 1 {
		t.Fatalf("child MaxIdleConns = %d, want clamp to 1", server.Gp.MaxIdleConns)
	}
	if conf.MaxIdleConns != 2 {
		t.Fatalf("config MaxIdleConns = %d, want historical value 2", conf.MaxIdleConns)
	}
}

func TestRPCPoolSelectorPreservesConstructorDefaults(t *testing.T) {
	factory := &internalSelectorFactory{}
	conf := &GPoolConfig[internalTestClient]{
		Addrs:    "node:9000/1000",
		MaxConns: 1,
		Cfunc:    factory.create,
	}
	poolSelector := NewRpcPoolSelector(context.Background(), conf)
	defer poolSelector.Close(context.Background())
	if conf.MaxIdleConns != 100 {
		t.Fatalf("constructor config MaxIdleConns = %d, want historical default 100", conf.MaxIdleConns)
	}
	server := poolSelector.Slist[0].(*RpcSvr[internalTestClient])
	if server.Gp.MaxIdleConns != 1 {
		t.Fatalf("child MaxIdleConns = %d, want effective clamp to 1", server.Gp.MaxIdleConns)
	}
}

func TestRPCPoolSelectorPreservesNoServerErrorText(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, nil)
	for _, item := range poolSelector.Slist {
		item.(*RpcSvr[internalTestClient]).SetStat(selector.SVR_NOTVALID)
	}
	err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "unused", nil
	})
	if !errors.Is(err, ErrNoValidRPCServer) || err.Error() != "no server" {
		t.Fatalf("ThriftCall() error = %v, want ErrNoValidRPCServer with historical text", err)
	}
}

func TestRPCPoolSelectorSupportsRepeatedInitialization(t *testing.T) {
	firstFactory := &internalSelectorFactory{}
	poolSelector := &RpcPoolSelector[internalTestClient]{}
	firstConf := &GPoolConfig[internalTestClient]{
		Addrs:        "node-a:9001/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        firstFactory.create,
	}
	if err := poolSelector.RpcPoolInit(context.Background(), firstConf); err != nil {
		t.Fatalf("first RpcPoolInit() error = %v", err)
	}
	secondFactory := &internalSelectorFactory{}
	secondConf := &GPoolConfig[internalTestClient]{
		Addrs:        "node-b:9002/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        secondFactory.create,
	}
	if err := poolSelector.RpcPoolInit(context.Background(), secondConf); err != nil {
		t.Fatalf("second RpcPoolInit() error = %v", err)
	}
	defer poolSelector.Close(context.Background())
	server := poolSelector.RoundRobin(context.Background()).(*RpcSvr[internalTestClient])
	if server.GetAddr() != "node-b" {
		t.Fatalf("reinitialized server address = %q, want node-b", server.GetAddr())
	}
	connections := firstFactory.connections()
	if len(connections) != 1 || connections[0].closeCalls.Load() != 1 {
		t.Fatalf("old selector connections were not closed during reinitialization")
	}
}

func TestRPCPoolSelectorLegacyConstructorPreservesInitializationError(t *testing.T) {
	poolSelector := NewRpcPoolSelector[internalTestClient](context.Background(), nil)
	wantErr := poolSelector.initErr
	if wantErr == nil {
		t.Fatal("legacy constructor selector initErr = nil, want configuration error")
	}
	if err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "unused", nil
	}); !errors.Is(err, wantErr) {
		t.Fatalf("ThriftCall() error = %v, want %v", err, wantErr)
	}
}

func TestRPCPoolSelectorProtocolErrorPreservesHistoricalEjection(t *testing.T) {
	pinged := make(chan struct{}, 1)
	poolSelector, _ := newInternalRPCPoolSelector(t, func(interface{}) (string, error) {
		select {
		case pinged <- struct{}{}:
		default:
		}
		return "ping", nil
	})
	protocolErr := thrift.NewTProtocolException(errors.New("bad response"))
	if err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "request", protocolErr
	}); !errors.Is(err, protocolErr) {
		t.Fatalf("ThriftCall() error = %v, want %v", err, protocolErr)
	}
	select {
	case <-pinged:
	case <-time.After(time.Second):
		t.Fatal("protocol error did not trigger historical health recovery")
	}
}

func TestRPCPoolSelectorSetShouldEjectOverridesDefault(t *testing.T) {
	poolSelector, _ := newInternalRPCPoolSelector(t, func(interface{}) (string, error) {
		return "ping", nil
	})
	poolSelector.SetShouldEject(func(error) bool { return false })
	transportErr := thrift.NewTTransportException(thrift.NOT_OPEN, "connection lost")
	if err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "request", transportErr
	}); !errors.Is(err, transportErr) {
		t.Fatalf("ThriftCall() error = %v, want transport error", err)
	}
	if got := poolSelector.Slist[0].(*RpcSvr[internalTestClient]).GetValid(); got != selector.SVR_VALID {
		t.Fatalf("server validity = %d, want valid with custom policy", got)
	}
}

func TestRPCPoolSelectorWrappedTransportErrorTriggersHealthRecovery(t *testing.T) {
	pinged := make(chan struct{}, 1)
	poolSelector, _ := newInternalRPCPoolSelector(t, func(interface{}) (string, error) {
		select {
		case pinged <- struct{}{}:
		default:
		}
		return "ping", nil
	})
	transportErr := thrift.NewTTransportException(thrift.NOT_OPEN, "connection lost")
	wantErr := fmt.Errorf("wrapped transport error: %w", transportErr)
	if err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "request", wantErr
	}); !errors.Is(err, transportErr) {
		t.Fatalf("ThriftCall() error = %v, want wrapped transport error", err)
	}
	select {
	case <-pinged:
	case <-time.After(time.Second):
		t.Fatal("health check was not triggered for wrapped transport error")
	}
	waitForRPCSelectorCondition(t, func() bool {
		return poolSelector.Slist[0].(*RpcSvr[internalTestClient]).GetValid() == selector.SVR_VALID
	})
}

func TestRPCPoolSelectorFailureNotificationDoesNotBlock(t *testing.T) {
	factory := &internalSelectorFactory{}
	pingStarted := make(chan struct{})
	releasePing := make(chan struct{})
	defer close(releasePing)
	var pingOnce sync.Once
	poolSelector, err := NewRpcPoolSelectorWithError(context.Background(), &GPoolConfig[internalTestClient]{
		Addrs:        "node-a:9001/1000,node-b:9002/1000",
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        factory.create,
		PingTicker:   60,
		Ping: func(interface{}) (string, error) {
			pingOnce.Do(func() { close(pingStarted) })
			<-releasePing
			return "ping", nil
		},
	})
	if err != nil {
		t.Fatalf("NewRpcPoolSelectorWithError() error = %v", err)
	}
	t.Cleanup(func() {
		if err := poolSelector.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	transportErr := thrift.NewTTransportException(thrift.NOT_OPEN, "connection lost")
	fail := func(interface{}) (string, error) { return "request", transportErr }
	if err := poolSelector.ThriftCall(context.Background(), fail); !errors.Is(err, transportErr) {
		t.Fatalf("first ThriftCall() error = %v, want transport error", err)
	}
	select {
	case <-pingStarted:
	case <-time.After(time.Second):
		t.Fatal("first health check did not start")
	}

	result := make(chan error, 1)
	go func() { result <- poolSelector.ThriftCall(context.Background(), fail) }()
	select {
	case err := <-result:
		if !errors.Is(err, transportErr) {
			t.Fatalf("second ThriftCall() error = %v, want transport error", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("second ThriftCall blocked while health checker was busy")
	}
}

func TestRPCPoolSelectorCloseClosesEveryChildPool(t *testing.T) {
	poolSelector, factory := newInternalRPCPoolSelector(t, nil)
	if err := poolSelector.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if selected := poolSelector.RoundRobin(context.Background()); selected != nil {
		t.Fatalf("RoundRobin() after Close = %v, want nil", selected)
	}
	if err := poolSelector.ThriftCall(context.Background(), func(interface{}) (string, error) {
		return "unused", nil
	}); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("ThriftCall() after Close error = %v, want ErrPoolClosed", err)
	}
	for i, item := range poolSelector.Slist {
		server := item.(*RpcSvr[internalTestClient])
		if conn, err := server.Gp.Get(context.Background()); conn != nil || !errors.Is(err, ErrPoolClosed) {
			t.Fatalf("child pool %d Get() after Close = (%v, %v), want (nil, ErrPoolClosed)", i, conn, err)
		}
	}
	for i, conn := range factory.connections() {
		if got := conn.closeCalls.Load(); got != 1 {
			t.Fatalf("connection %d close calls = %d, want 1", i, got)
		}
	}
}
