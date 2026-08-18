package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type internalTCPConn struct {
	open       atomic.Bool
	closeCalls atomic.Int32
}

func (c *internalTCPConn) Open(context.Context) error {
	c.open.Store(true)
	return nil
}

func (c *internalTCPConn) Close(context.Context) {
	c.closeCalls.Add(1)
	c.open.Store(false)
}

func (c *internalTCPConn) IsOpen(context.Context) bool {
	return c.open.Load()
}

type internalTCPFactory struct {
	mu       sync.Mutex
	created  []*internalTCPConn
	tlsConfs []*tls.Config
}

func (f *internalTCPFactory) create(_ context.Context, _ string, _ int, _ int, tlsConf *tls.Config) TcpConnInter {
	conn := &internalTCPConn{}
	f.mu.Lock()
	f.created = append(f.created, conn)
	f.tlsConfs = append(f.tlsConfs, tlsConf)
	f.mu.Unlock()
	return conn
}

func (f *internalTCPFactory) connections() []*internalTCPConn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*internalTCPConn(nil), f.created...)
}

func newInternalTCPPool(t *testing.T, maxConns, maxIdle, maxWaiters int) (*GTcpPool[*internalTCPConn], *internalTCPFactory) {
	t.Helper()
	factory := &internalTCPFactory{}
	conf := &TcpPoolConfig[*internalTCPConn]{
		MaxConns:     maxConns,
		MaxIdleConns: maxIdle,
		MaxWaiters:   maxWaiters,
		Cfunc:        factory.create,
	}
	pool := &GTcpPool[*internalTCPConn]{}
	pool.GTcpPoolInitWithContext(context.Background(), "127.0.0.1", 9000, 1_000, conf)
	return pool, factory
}

func newInternalTCPRetryPool(t *testing.T, maxRetries int) (*GTcpPool[*internalTCPConn], *internalTCPFactory) {
	t.Helper()
	factory := &internalTCPFactory{}
	conf := &TcpPoolConfig[*internalTCPConn]{
		MaxConns:      1,
		MaxIdleConns:  1,
		MaxWaiters:    1,
		RPCMaxRetries: maxRetries,
		Cfunc:         factory.create,
	}
	pool := &GTcpPool[*internalTCPConn]{}
	pool.GTcpPoolInitWithContext(context.Background(), "127.0.0.1", 9000, 1_000, conf)
	t.Cleanup(func() {
		if err := pool.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return pool, factory
}

func TestTCPPoolIsDisconnectedError(t *testing.T) {
	pool := &GTcpPool[*internalTCPConn]{}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "EOF", err: io.EOF, want: true},
		{name: "unexpected EOF", err: io.ErrUnexpectedEOF, want: true},
		{name: "wrapped connection reset", err: fmt.Errorf("read failed: %w", syscall.ECONNRESET), want: true},
		{name: "broken pipe", err: syscall.EPIPE, want: true},
		{name: "context deadline", err: context.DeadlineExceeded, want: false},
		{name: "application error", err: errors.New("application error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pool.isDisconnectedError(tt.err); got != tt.want {
				t.Fatalf("isDisconnectedError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPPoolProcessRetryCount(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		wantCalls  int
	}{
		{name: "zero preserves no retry behavior", maxRetries: 0, wantCalls: 1},
		{name: "negative preserves no retry behavior", maxRetries: -1, wantCalls: 1},
		{name: "two retries make three attempts", maxRetries: 2, wantCalls: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, factory := newInternalTCPRetryPool(t, tt.maxRetries)
			calls := 0
			err := pool.Process(context.Background(), func(interface{}) (string, error) {
				calls++
				return "test", io.EOF
			})
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Process() error = %v, want io.EOF", err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("process calls = %d, want %d", calls, tt.wantCalls)
			}
			if got := len(factory.connections()); got != tt.wantCalls {
				t.Fatalf("created connections = %d, want %d", got, tt.wantCalls)
			}
			if pool.GetUseLen() != 0 || pool.GetFreeLen() != 0 {
				t.Fatalf("pool state = in_use:%d idle:%d, want 0:0", pool.GetUseLen(), pool.GetFreeLen())
			}
		})
	}
}

func TestTCPPoolProcessRetriesWrappedDisconnectAndSucceeds(t *testing.T) {
	pool, factory := newInternalTCPRetryPool(t, 1)
	calls := 0
	err := pool.Process(context.Background(), func(interface{}) (string, error) {
		calls++
		if calls == 1 {
			return "test", fmt.Errorf("wrapped disconnect: %w", io.EOF)
		}
		return "test", nil
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("process calls = %d, want 2", calls)
	}
	if got := len(factory.connections()); got != 2 {
		t.Fatalf("created connections = %d, want 2", got)
	}
}

func TestTCPPoolProcessDoesNotRetryOtherErrors(t *testing.T) {
	pool, _ := newInternalTCPRetryPool(t, 3)
	calls := 0
	wantErr := errors.New("application error")
	err := pool.Process(context.Background(), func(interface{}) (string, error) {
		calls++
		return "test", wantErr
	})
	if err != wantErr {
		t.Fatalf("Process() error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("process calls = %d, want 1", calls)
	}
}

func TestTCPPoolProcessStopsRetryingWhenContextEnds(t *testing.T) {
	pool, _ := newInternalTCPRetryPool(t, 3)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := pool.Process(ctx, func(interface{}) (string, error) {
		calls++
		cancel()
		return "test", io.EOF
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Process() error = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("process calls = %d, want 1", calls)
	}
}

func waitForTCPPoolCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for TCP pool condition")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTCPPoolHandsReturnedConnectionToOldestWaiter(t *testing.T) {
	pool, _ := newInternalTCPPool(t, 1, 0, 2)
	ctx := context.Background()
	first, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}

	type result struct {
		conn *PoolTcpConn[*internalTCPConn]
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		conn, getErr := pool.Get(ctx)
		resultCh <- result{conn: conn, err: getErr}
	}()
	waitForTCPPoolCondition(t, func() bool { return pool.WaiterCount() == 1 })

	firstRaw := first.Gc
	first.Close(ctx)
	got := <-resultCh
	if got.err != nil {
		t.Fatalf("waiter Get() error = %v", got.err)
	}
	if got.conn.Gc != firstRaw {
		t.Fatal("returned connection was not handed directly to the oldest waiter")
	}
	if pool.GetUseLen() != 1 || pool.GetFreeLen() != 0 {
		t.Fatalf("pool counts after handoff = in_use:%d idle:%d, want 1:0", pool.GetUseLen(), pool.GetFreeLen())
	}
	got.conn.Close(ctx)
	if err := pool.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTCPPoolWaitHonorsContextCancellation(t *testing.T) {
	pool, _ := newInternalTCPPool(t, 1, 0, 1)
	borrowed, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	conn, err := pool.Get(ctx)
	if conn != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled Get() = (%v, %v), want (nil, context deadline exceeded)", conn, err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("canceled Get() took %v", elapsed)
	}
	if pool.WaiterCount() != 0 {
		t.Fatalf("waiter count after cancellation = %d, want 0", pool.WaiterCount())
	}

	borrowed.Close(context.Background())
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTCPPoolEnforcesWaiterLimit(t *testing.T) {
	pool, _ := newInternalTCPPool(t, 1, 0, 1)
	borrowed, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	defer cancelWaiter()
	waiterDone := make(chan error, 1)
	go func() {
		conn, getErr := pool.Get(waiterCtx)
		if conn != nil {
			conn.Close(context.Background())
		}
		waiterDone <- getErr
	}()
	waitForTCPPoolCondition(t, func() bool { return pool.WaiterCount() == 1 })

	if conn, getErr := pool.Get(context.Background()); conn != nil || !errors.Is(getErr, ErrTcpPoolWaitQueueFull) {
		t.Fatalf("Get() beyond waiter limit = (%v, %v), want (nil, ErrTcpPoolWaitQueueFull)", conn, getErr)
	}
	cancelWaiter()
	if err := <-waiterDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued Get() error = %v, want context canceled", err)
	}

	borrowed.Close(context.Background())
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTCPPoolEnforcesIdleLimit(t *testing.T) {
	pool, factory := newInternalTCPPool(t, 3, 1, 3)
	ctx := context.Background()
	borrowed := make([]*PoolTcpConn[*internalTCPConn], 0, 3)
	for i := 0; i < 3; i++ {
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Get(%d) error = %v", i, err)
		}
		borrowed = append(borrowed, conn)
	}
	for _, conn := range borrowed {
		conn.Close(ctx)
	}

	if pool.GetFreeLen() != 1 || pool.GetUseLen() != 0 {
		t.Fatalf("pool counts after return = idle:%d in_use:%d, want 1:0", pool.GetFreeLen(), pool.GetUseLen())
	}
	closed := int32(0)
	for _, conn := range factory.connections() {
		closed += conn.closeCalls.Load()
	}
	if closed != 2 {
		t.Fatalf("connections closed by idle limit = %d, want 2", closed)
	}

	if err := pool.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTCPPoolCloseWaitsForBorrowedConnection(t *testing.T) {
	pool, _ := newInternalTCPPool(t, 1, 0, 1)
	borrowed, err := pool.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := pool.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close() error = %v, want context deadline exceeded", err)
	}
	if conn, getErr := pool.Get(context.Background()); conn != nil || !errors.Is(getErr, ErrTcpPoolClosed) {
		t.Fatalf("Get() after Close = (%v, %v), want (nil, ErrTcpPoolClosed)", conn, getErr)
	}

	borrowed.Close(context.Background())
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestTCPPoolPassesTLSConfigToFactory(t *testing.T) {
	tlsConf := &tls.Config{MinVersion: tls.VersionTLS12}
	factory := &internalTCPFactory{}
	conf := &TcpPoolConfig[*internalTCPConn]{
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        factory.create,
		TlsConf:      tlsConf,
	}
	pool := &GTcpPool[*internalTCPConn]{}
	pool.GTcpPoolInitWithContext(context.Background(), "127.0.0.1", 9000, 1_000, conf)

	factory.mu.Lock()
	if len(factory.tlsConfs) != 1 || factory.tlsConfs[0] != tlsConf {
		factory.mu.Unlock()
		t.Fatalf("TLS configs passed to factory = %v, want [%p]", factory.tlsConfs, tlsConf)
	}
	factory.mu.Unlock()
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
