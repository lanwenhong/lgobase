package gpool

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

type internalTestClient struct{}

type internalTestConn struct {
	open       atomic.Bool
	closeCalls atomic.Int32
}

func (c *internalTestConn) Init(string, int, int) error { return nil }

func (c *internalTestConn) Open() error {
	c.open.Store(true)
	return nil
}

func (c *internalTestConn) Close() error {
	c.closeCalls.Add(1)
	c.open.Store(false)
	return nil
}

func (c *internalTestConn) IsOpen() bool { return c.open.Load() }

func (c *internalTestConn) GetThrfitClient() *internalTestClient { return nil }

func (c *internalTestConn) SetTimeOut(context.Context, time.Duration) {}

func newInternalTestPool(maxConns, maxIdleConns int) *Gpool[internalTestClient] {
	return newInternalTestPoolWithLimits(maxConns, maxIdleConns, 256, 1_000)
}

func newInternalTestPoolWithLimits(maxConns, maxIdleConns, maxWaiters, timeout int) *Gpool[internalTestClient] {
	conf := &GPoolConfig[internalTestClient]{
		MaxConns:     maxConns,
		MaxIdleConns: maxIdleConns,
		MaxWaiters:   maxWaiters,
		Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
			conn := &internalTestConn{}
			_ = conn.Open()
			return conn, nil
		},
	}

	pool := &Gpool[internalTestClient]{}
	pool.GpoolInit2(context.Background(), "test", 0, timeout, conf)
	return pool
}

func assertInternalPoolState(t *testing.T, pool *Gpool[internalTestClient], idle, inUse int) {
	t.Helper()

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if got := pool.idle.Len(); got != idle {
		t.Fatalf("idle connections = %d, want %d", got, idle)
	}
	if pool.inUse != inUse {
		t.Fatalf("in-use connections = %d, want %d", pool.inUse, inUse)
	}
}

func TestGpoolTracksBorrowedConnectionsWithoutUseList(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(2, 1)
	assertInternalPoolState(t, pool, 1, 0)

	first, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertInternalPoolState(t, pool, 0, 1)

	second, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertInternalPoolState(t, pool, 0, 2)

	first.Close(ctx)
	assertInternalPoolState(t, pool, 1, 1)

	second.Close(ctx)
	assertInternalPoolState(t, pool, 1, 0)
}

func TestGpoolWaiterUsesReturnedConnectionWithoutUseList(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(1, 1)

	first, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	type getResult struct {
		conn *PoolConn[internalTestClient]
		err  error
	}
	result := make(chan getResult, 1)
	go func() {
		conn, getErr := pool.Get(ctx)
		result <- getResult{conn: conn, err: getErr}
	}()

	deadline := time.Now().Add(time.Second)
	for {
		pool.mutex.Lock()
		waits := pool.Waits
		pool.mutex.Unlock()
		if waits == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second Get did not enter the wait path")
		}
		runtime.Gosched()
	}

	first.Close(ctx)

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.conn == nil {
			t.Fatal("waiter received a nil connection")
		}
		got.conn.Close(ctx)
	case <-time.After(time.Second):
		t.Fatal("waiter was not notified after a connection was returned")
	}

	assertInternalPoolState(t, pool, 1, 0)
}

func TestGpoolInUseCountNeverExceedsConfiguredCapacityInSteadyState(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(8, 8)

	errCh := make(chan error, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			<-start
			conn, err := pool.Get(ctx)
			if err != nil {
				errCh <- err
				return
			}

			pool.mutex.Lock()
			inUse := pool.inUse
			pool.mutex.Unlock()
			if inUse < 1 || inUse > pool.MaxConns {
				errCh <- fmt.Errorf("in-use connections = %d, max = %d", inUse, pool.MaxConns)
				conn.Close(ctx)
				return
			}

			conn.Close(ctx)
			errCh <- nil
		}()
	}
	close(start)

	for i := 0; i < 8; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}

	assertInternalPoolState(t, pool, 8, 0)
}

func TestGpoolIdleRingEvictsOldestConnection(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(3, 2)

	first, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	third, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	firstConn := first.Gc.(*internalTestConn)
	secondConn := second.Gc.(*internalTestConn)
	thirdConn := third.Gc.(*internalTestConn)

	first.Close(ctx)
	second.Close(ctx)
	third.Close(ctx)

	if got := firstConn.closeCalls.Load(); got != 1 {
		t.Fatalf("oldest idle connection close calls = %d, want 1", got)
	}
	if got := secondConn.closeCalls.Load(); got != 0 {
		t.Fatalf("second idle connection close calls = %d, want 0", got)
	}
	if got := thirdConn.closeCalls.Load(); got != 0 {
		t.Fatalf("newest idle connection close calls = %d, want 0", got)
	}

	gotSecond, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotThird, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gotSecond != second || gotThird != third {
		t.Fatalf("remaining FIFO order = [%p, %p], want [%p, %p]", gotSecond, gotThird, second, third)
	}
}

func TestGpoolIdleRingKeepsAllConnectionsWhenIdleEqualsMax(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(3, 3)

	borrowed := make([]*PoolConn[internalTestClient], 0, 3)
	for i := 0; i < 3; i++ {
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		borrowed = append(borrowed, conn)
	}
	for _, conn := range borrowed {
		conn.Close(ctx)
	}

	assertInternalPoolState(t, pool, 3, 0)
	for i, conn := range borrowed {
		if got := conn.Gc.(*internalTestConn).closeCalls.Load(); got != 0 {
			t.Fatalf("connection %d close calls = %d, want 0", i, got)
		}
	}
}

func TestGpoolMaxWaitersDefaultsAndExplicitConfig(t *testing.T) {
	createConn := func(context.Context, string, int, int) (Conn[internalTestClient], error) {
		conn := &internalTestConn{}
		_ = conn.Open()
		return conn, nil
	}

	legacy := &Gpool[internalTestClient]{}
	legacy.GpoolInit("test", 0, 100, 5, 0, 0, createConn, nil)
	if legacy.MaxWaiters != 5 {
		t.Fatalf("legacy MaxWaiters = %d, want 5", legacy.MaxWaiters)
	}

	defaultConfig := &GPoolConfig[internalTestClient]{
		MaxConns: 7,
		Cfunc:    createConn,
	}
	configured := &Gpool[internalTestClient]{}
	configured.GpoolInit2(context.Background(), "test", 0, 100, defaultConfig)
	if configured.MaxWaiters != 7 || defaultConfig.MaxWaiters != 7 {
		t.Fatalf("default MaxWaiters pool=%d config=%d, want both 7", configured.MaxWaiters, defaultConfig.MaxWaiters)
	}

	explicitConfig := &GPoolConfig[internalTestClient]{
		MaxConns:   7,
		MaxWaiters: 11,
		Cfunc:      createConn,
	}
	explicit := &Gpool[internalTestClient]{}
	explicit.GpoolInit2(context.Background(), "test", 0, 100, explicitConfig)
	if explicit.MaxWaiters != 11 {
		t.Fatalf("explicit MaxWaiters = %d, want 11", explicit.MaxWaiters)
	}
}

func TestGpoolWaiterQueueFull(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPoolWithLimits(1, 1, 1, 1_000)
	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	firstWaiter := make(chan getResultForTest, 1)
	go func() {
		conn, getErr := pool.Get(ctx)
		firstWaiter <- getResultForTest{conn: conn, err: getErr}
	}()
	waitForWaiterCount(t, pool, 1)

	if conn, getErr := pool.Get(ctx); !errors.Is(getErr, ErrPoolWaitQueueFull) || conn != nil {
		t.Fatalf("queue-full Get = (%p, %v), want (nil, %v)", conn, getErr, ErrPoolWaitQueueFull)
	}

	borrowed.Close(ctx)
	select {
	case got := <-firstWaiter:
		if got.err != nil {
			t.Fatal(got.err)
		}
		got.conn.Close(ctx)
	case <-time.After(time.Second):
		t.Fatal("first waiter did not receive the returned connection")
	}
}

func TestGpoolWaiterTimeoutMatchesConnectionTimeout(t *testing.T) {
	const timeout = 80
	ctx := context.Background()
	pool := newInternalTestPoolWithLimits(1, 1, 1, timeout)
	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	conn, getErr := pool.Get(ctx)
	elapsed := time.Since(start)
	if conn != nil || !errors.Is(getErr, ErrPoolWaitTimeout) {
		t.Fatalf("timed-out Get = (%p, %v), want (nil, %v)", conn, getErr, ErrPoolWaitTimeout)
	}
	if elapsed < 60*time.Millisecond || elapsed > time.Second {
		t.Fatalf("wait elapsed = %v, connection timeout = %dms", elapsed, timeout)
	}

	pool.mutex.Lock()
	waiters := pool.waiters.Len()
	pool.mutex.Unlock()
	if waiters != 0 {
		t.Fatalf("waiters after timeout = %d, want 0", waiters)
	}
	borrowed.Close(ctx)
}

func TestGpoolWaiterContextCancellation(t *testing.T) {
	pool := newInternalTestPoolWithLimits(1, 1, 1, 1_000)
	borrowed, err := pool.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, getErr := pool.Get(ctx)
	if conn != nil || !errors.Is(getErr, context.Canceled) {
		t.Fatalf("canceled Get = (%p, %v), want (nil, %v)", conn, getErr, context.Canceled)
	}

	pool.mutex.Lock()
	waiters := pool.waiters.Len()
	pool.mutex.Unlock()
	if waiters != 0 {
		t.Fatalf("waiters after cancellation = %d, want 0", waiters)
	}
	borrowed.Close(context.Background())
}

type getResultForTest struct {
	conn *PoolConn[internalTestClient]
	err  error
}

func waitForWaiterCount(t *testing.T, pool *Gpool[internalTestClient], want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		pool.mutex.Lock()
		got := pool.waiters.Len()
		pool.mutex.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiter count = %d, want %d", got, want)
		}
		runtime.Gosched()
	}
}

func TestGpoolWaitersReceiveConnectionsFIFO(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPoolWithLimits(1, 1, 3, 1_000)
	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	type orderedResult struct {
		id   int
		conn *PoolConn[internalTestClient]
		err  error
	}
	results := make(chan orderedResult, 3)
	release := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	done := make(chan struct{}, 3)

	for i := 0; i < 3; i++ {
		id := i
		go func() {
			conn, getErr := pool.Get(ctx)
			results <- orderedResult{id: id, conn: conn, err: getErr}
			if getErr == nil {
				<-release[id]
				conn.Close(ctx)
			}
			done <- struct{}{}
		}()
		waitForWaiterCount(t, pool, i+1)
	}

	borrowed.Close(ctx)
	for want := 0; want < 3; want++ {
		select {
		case got := <-results:
			if got.err != nil {
				t.Fatal(got.err)
			}
			if got.id != want {
				t.Fatalf("delivered waiter = %d, want %d", got.id, want)
			}
			if got.conn != borrowed {
				t.Fatalf("waiter %d received connection %p, want direct handoff of %p", got.id, got.conn, borrowed)
			}
			close(release[want])
		case <-time.After(time.Second):
			t.Fatalf("waiter %d was not delivered", want)
		}
	}

	for range 3 {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("waiter goroutine did not finish")
		}
	}
	assertInternalPoolState(t, pool, 1, 0)
}

func TestGpoolWaiterCancellationRacesWithDelivery(t *testing.T) {
	for iteration := 0; iteration < 200; iteration++ {
		pool := newInternalTestPoolWithLimits(1, 1, 1, 1_000)
		borrowed, err := pool.Get(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan getResultForTest, 1)
		go func() {
			conn, getErr := pool.Get(ctx)
			result <- getResultForTest{conn: conn, err: getErr}
		}()
		waitForWaiterCount(t, pool, 1)

		start := make(chan struct{})
		closeDone := make(chan struct{})
		go func() {
			<-start
			borrowed.Close(context.Background())
			close(closeDone)
		}()
		go func() {
			<-start
			cancel()
		}()
		close(start)

		select {
		case got := <-result:
			if got.err == nil {
				if got.conn == nil {
					t.Fatal("delivery won cancellation race but returned nil connection")
				}
				got.conn.Close(context.Background())
			} else if !errors.Is(got.err, context.Canceled) {
				t.Fatalf("cancellation race returned unexpected error: %v", got.err)
			}
		case <-time.After(time.Second):
			t.Fatal("cancellation/delivery race did not complete")
		}
		select {
		case <-closeDone:
		case <-time.After(time.Second):
			t.Fatal("returning connection did not complete")
		}

		pool.mutex.Lock()
		waiters := pool.waiters.Len()
		inUse := pool.inUse
		idle := pool.idle.Len()
		pool.mutex.Unlock()
		if waiters != 0 || inUse != 0 || idle != 1 {
			t.Fatalf("iteration %d final state: waiters=%d inUse=%d idle=%d", iteration, waiters, inUse, idle)
		}
	}
}

func updateMaxInt32(maxValue *atomic.Int32, value int32) {
	for {
		current := maxValue.Load()
		if value <= current || maxValue.CompareAndSwap(current, value) {
			return
		}
	}
}

func TestGpoolCreatingConnectionsOccupyCapacity(t *testing.T) {
	const (
		maxConns = 2
		workers  = 16
	)

	var activeCreates atomic.Int32
	var maxActiveCreates atomic.Int32
	var createCalls atomic.Int32
	releaseCreates := make(chan struct{})

	conf := &GPoolConfig[internalTestClient]{
		MaxConns:     maxConns,
		MaxIdleConns: 0,
		MaxWaiters:   workers,
		Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
			createCalls.Add(1)
			active := activeCreates.Add(1)
			updateMaxInt32(&maxActiveCreates, active)
			<-releaseCreates
			activeCreates.Add(-1)

			conn := &internalTestConn{}
			_ = conn.Open()
			return conn, nil
		},
	}

	pool := &Gpool[internalTestClient]{}
	pool.GpoolInit2(context.Background(), "test", 0, 5_000, conf)

	start := make(chan struct{})
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			<-start
			conn, err := pool.Get(context.Background())
			if err == nil {
				conn.Close(context.Background())
			}
			results <- err
		}()
	}
	close(start)

	deadline := time.Now().Add(time.Second)
	for activeCreates.Load() != maxConns {
		if time.Now().After(deadline) {
			t.Fatalf("active creates = %d, want %d", activeCreates.Load(), maxConns)
		}
		runtime.Gosched()
	}

	pool.mutex.Lock()
	creating := pool.creating
	capacityUsed := pool.capacityUsedLocked()
	pool.mutex.Unlock()
	if creating != maxConns || capacityUsed != maxConns {
		t.Fatalf("creating = %d, capacity used = %d, want both %d", creating, capacityUsed, maxConns)
	}
	if got := maxActiveCreates.Load(); got > maxConns {
		t.Fatalf("concurrent creates = %d, max connections = %d", got, maxConns)
	}

	close(releaseCreates)
	for i := 0; i < workers; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}

	if got := maxActiveCreates.Load(); got > maxConns {
		t.Fatalf("concurrent creates = %d, max connections = %d", got, maxConns)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	if got := pool.capacityUsedLocked(); got != 0 {
		t.Fatalf("capacity used after all connections closed = %d, want 0", got)
	}
}

type blockingCloseConn struct {
	open         atomic.Bool
	closeStarted chan struct{}
	releaseClose chan struct{}
	startOnce    *sync.Once
}

func (c *blockingCloseConn) Init(string, int, int) error { return nil }

func (c *blockingCloseConn) Open() error {
	c.open.Store(true)
	return nil
}

func (c *blockingCloseConn) Close() error {
	c.startOnce.Do(func() { close(c.closeStarted) })
	<-c.releaseClose
	c.open.Store(false)
	return nil
}

func (c *blockingCloseConn) IsOpen() bool { return c.open.Load() }

func (c *blockingCloseConn) GetThrfitClient() *internalTestClient { return nil }

func (c *blockingCloseConn) SetTimeOut(context.Context, time.Duration) {}

func TestGpoolClosingConnectionOccupiesCapacityUntilCloseReturns(t *testing.T) {
	var createCalls atomic.Int32
	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	closeStartOnce := &sync.Once{}

	conf := &GPoolConfig[internalTestClient]{
		MaxConns:     1,
		MaxIdleConns: 0,
		Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
			createCalls.Add(1)
			conn := &blockingCloseConn{
				closeStarted: closeStarted,
				releaseClose: releaseClose,
				startOnce:    closeStartOnce,
			}
			_ = conn.Open()
			return conn, nil
		},
	}

	pool := &Gpool[internalTestClient]{}
	pool.GpoolInit2(context.Background(), "test", 0, 5_000, conf)

	first, err := pool.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	firstCloseDone := make(chan struct{})
	go func() {
		first.Close(context.Background())
		close(firstCloseDone)
	}()

	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("physical Close did not start")
	}

	pool.mutex.Lock()
	closing := pool.closing
	capacityUsed := pool.capacityUsedLocked()
	pool.mutex.Unlock()
	if closing != 1 || capacityUsed != 1 {
		t.Fatalf("closing = %d, capacity used = %d, want both 1", closing, capacityUsed)
	}

	type closeCapacityResult struct {
		conn *PoolConn[internalTestClient]
		err  error
	}
	secondResult := make(chan closeCapacityResult, 1)
	go func() {
		conn, getErr := pool.Get(context.Background())
		secondResult <- closeCapacityResult{conn: conn, err: getErr}
	}()

	deadline := time.Now().Add(time.Second)
	for {
		pool.mutex.Lock()
		waits := pool.Waits
		pool.mutex.Unlock()
		if waits == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second Get did not wait for the closing connection")
		}
		runtime.Gosched()
	}
	if got := createCalls.Load(); got != 1 {
		t.Fatalf("create calls before Close returned = %d, want 1", got)
	}

	close(releaseClose)
	select {
	case <-firstCloseDone:
	case <-time.After(time.Second):
		t.Fatal("first PoolConn.Close did not return after physical Close completed")
	}

	select {
	case got := <-secondResult:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.conn == nil {
			t.Fatal("second Get returned a nil connection")
		}
		got.conn.Close(context.Background())
	case <-time.After(time.Second):
		t.Fatal("second Get was not resumed after physical Close completed")
	}

	if got := createCalls.Load(); got != 2 {
		t.Fatalf("create calls after Close returned = %d, want 2", got)
	}
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	if got := pool.capacityUsedLocked(); got != 0 {
		t.Fatalf("capacity used after final Close = %d, want 0", got)
	}
}

func TestPoolConnCloseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(1, 1)
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	physical := conn.Gc.(*internalTestConn)

	const closers = 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(closers)
	for range closers {
		go func() {
			defer wg.Done()
			<-start
			conn.Close(ctx)
		}()
	}
	close(start)
	wg.Wait()

	assertInternalPoolState(t, pool, 1, 0)
	if got := physical.closeCalls.Load(); got != 0 {
		t.Fatalf("physical close calls before pool shutdown = %d, want 0", got)
	}

	if err := pool.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if got := physical.closeCalls.Load(); got != 1 {
		t.Fatalf("physical close calls after pool shutdown = %d, want 1", got)
	}
}

func TestGpoolCloseRejectsGetsWakesWaitersAndWaitsForBorrowed(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPoolWithLimits(1, 1, 1, 1_000)
	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	physical := borrowed.Gc.(*internalTestConn)

	waiterResult := make(chan getResultForTest, 1)
	go func() {
		conn, getErr := pool.Get(ctx)
		waiterResult <- getResultForTest{conn: conn, err: getErr}
	}()
	waitForWaiterCount(t, pool, 1)

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- pool.Close(ctx)
	}()

	select {
	case got := <-waiterResult:
		if got.conn != nil || !errors.Is(got.err, ErrPoolClosed) {
			t.Fatalf("waiter after pool Close = (%p, %v), want (nil, %v)", got.conn, got.err, ErrPoolClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("pool Close did not wake queued waiter")
	}

	select {
	case err := <-closeResult:
		t.Fatalf("pool Close returned before borrowed connection was returned: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if conn, getErr := pool.Get(ctx); conn != nil || !errors.Is(getErr, ErrPoolClosed) {
		t.Fatalf("Get after pool Close = (%p, %v), want (nil, %v)", conn, getErr, ErrPoolClosed)
	}

	borrowed.Close(ctx)
	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("pool Close did not finish after borrowed connection was returned")
	}

	if got := physical.closeCalls.Load(); got != 1 {
		t.Fatalf("borrowed physical close calls = %d, want 1", got)
	}
	if err := pool.Close(ctx); err != nil {
		t.Fatalf("second pool Close: %v", err)
	}
}

func TestGpoolCloseContextTimeoutContinuesShutdown(t *testing.T) {
	pool := newInternalTestPool(1, 1)
	borrowed, err := pool.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := pool.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool Close with borrowed connection = %v, want %v", err, context.DeadlineExceeded)
	}

	borrowed.Close(context.Background())
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("waiting for continued shutdown: %v", err)
	}
}

func TestGpoolCloseContextCanExpireWhileIdlePhysicalCloseBlocks(t *testing.T) {
	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	conf := &GPoolConfig[internalTestClient]{
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
			conn := &blockingCloseConn{
				closeStarted: closeStarted,
				releaseClose: releaseClose,
				startOnce:    &sync.Once{},
			}
			_ = conn.Open()
			return conn, nil
		},
	}
	pool := &Gpool[internalTestClient]{}
	pool.GpoolInit2(context.Background(), "test", 0, 1_000, conf)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := pool.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool Close with blocked idle physical close = %v, want %v", err, context.DeadlineExceeded)
	}
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("idle physical Close did not start")
	}

	close(releaseClose)
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("waiting for blocked idle physical close: %v", err)
	}
}

func TestGpoolCloseRejectsConnectionCreatedDuringShutdown(t *testing.T) {
	createStarted := make(chan struct{})
	releaseCreate := make(chan struct{})
	created := make(chan *internalTestConn, 1)
	var startOnce sync.Once

	conf := &GPoolConfig[internalTestClient]{
		MaxConns:     1,
		MaxIdleConns: 0,
		MaxWaiters:   1,
		Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
			startOnce.Do(func() { close(createStarted) })
			<-releaseCreate
			conn := &internalTestConn{}
			_ = conn.Open()
			created <- conn
			return conn, nil
		},
	}
	pool := &Gpool[internalTestClient]{}
	pool.GpoolInit2(context.Background(), "test", 0, 1_000, conf)

	getResult := make(chan getResultForTest, 1)
	go func() {
		conn, getErr := pool.Get(context.Background())
		getResult <- getResultForTest{conn: conn, err: getErr}
	}()
	select {
	case <-createStarted:
	case <-time.After(time.Second):
		t.Fatal("connection creation did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := pool.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool Close during creation = %v, want %v", err, context.DeadlineExceeded)
	}
	close(releaseCreate)

	select {
	case got := <-getResult:
		if got.conn != nil || !errors.Is(got.err, ErrPoolClosed) {
			t.Fatalf("Get completing during shutdown = (%p, %v), want (nil, %v)", got.conn, got.err, ErrPoolClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("Get did not finish after connection creation was released")
	}

	physical := <-created
	if err := pool.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := physical.closeCalls.Load(); got != 1 {
		t.Fatalf("connection created during shutdown close calls = %d, want 1", got)
	}
}

func TestTConnCloseReturnsUnderlyingTransportResult(t *testing.T) {
	framedTransport := thrift.NewTMemoryBuffer()
	framed := &TConn[internalTestClient]{
		Protocol: TH_PRO_FRAMED,
		Tft:      framedTransport,
	}
	if err := framed.Close(); err != nil {
		t.Fatalf("close framed transport: %v", err)
	}

	bufferedTransport := thrift.NewTBufferedTransport(thrift.NewTMemoryBuffer(), 128)
	buffered := &TConn[internalTestClient]{
		Protocol: TH_PRO_BUFFER,
		Tbt:      bufferedTransport,
	}
	if err := buffered.Close(); err != nil {
		t.Fatalf("close buffered transport: %v", err)
	}
}
