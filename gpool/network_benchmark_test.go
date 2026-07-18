//go:build performance

package gpool_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/example"
)

const (
	networkBenchmarkTimeout     = 30_000
	networkBenchmarkParallelism = 4
)

type networkBenchmarkService struct{}

func (*networkBenchmarkService) Add(_ context.Context, a, b int32) (int32, error) {
	return a + b, nil
}

func (*networkBenchmarkService) Echo(_ context.Context, req string) (*example.Myret, error) {
	return &example.Myret{Ret: req}, nil
}

type networkBenchmarkServer struct {
	server            *thrift.TSimpleServer
	serveErr          chan error
	host              string
	port              int
	previousStopDelay time.Duration
	previousLogger    *slog.Logger
}

func startNetworkBenchmarkServer() (*networkBenchmarkServer, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	serverSocket := thrift.NewTServerSocketFromAddrTimeout(addr, time.Duration(networkBenchmarkTimeout)*time.Millisecond)
	processor := example.NewExampleProcessor(&networkBenchmarkService{})
	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	server := thrift.NewTSimpleServer4(processor, serverSocket, transportFactory, protocolFactory)
	server.SetLogContext(context.Background())
	if err := server.Listen(); err != nil {
		return nil, err
	}

	actualAddr, ok := serverSocket.Addr().(*net.TCPAddr)
	if !ok {
		_ = server.Stop()
		return nil, fmt.Errorf("unexpected benchmark server address type %T", serverSocket.Addr())
	}

	benchServer := &networkBenchmarkServer{
		server:            server,
		serveErr:          make(chan error, 1),
		host:              "127.0.0.1",
		port:              actualAddr.Port,
		previousStopDelay: thrift.ServerStopTimeout,
		previousLogger:    slog.Default(),
	}
	// TSimpleServer only closes active server-side transports after this delay.
	// Keep benchmark cleanup bounded even if a client transport fails to wake a
	// server read immediately.
	thrift.ServerStopTimeout = 250 * time.Millisecond
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() {
		benchServer.serveErr <- server.AcceptLoop()
	}()
	return benchServer, nil
}

func (s *networkBenchmarkServer) Close() error {
	stopErr := s.server.Stop()
	serveErr := <-s.serveErr
	thrift.ServerStopTimeout = s.previousStopDelay
	slog.SetDefault(s.previousLogger)
	if stopErr != nil {
		return stopErr
	}
	return serveErr
}

type networkConnectionCounters struct {
	createAttempts atomic.Int64
	createSuccess  atomic.Int64
	closeCalls     atomic.Int64
	open           atomic.Int64
	maxOpen        atomic.Int64
}

func (c *networkConnectionCounters) observeOpen(value int64) {
	for {
		current := c.maxOpen.Load()
		if value <= current || c.maxOpen.CompareAndSwap(current, value) {
			return
		}
	}
}

type countedNetworkConn struct {
	*gpool.TConn[example.ExampleClient]
	counters *networkConnectionCounters
	closed   atomic.Bool
}

func (c *countedNetworkConn) Close() error {
	c.counters.closeCalls.Add(1)
	if c.closed.CompareAndSwap(false, true) {
		c.counters.open.Add(-1)
	}
	return c.TConn.Close()
}

func newCountedNetworkConn(
	ctx context.Context,
	host string,
	port int,
	timeout int,
	counters *networkConnectionCounters,
) (gpool.Conn[example.ExampleClient], error) {
	counters.createAttempts.Add(1)
	conn := gpool.NewTConn[example.ExampleClient](host, port, timeout, gpool.TH_PRO_FRAMED)
	if err := conn.Open(); err != nil {
		return nil, err
	}
	conn.NewThClient(example.NewExampleClientFactory)
	counters.createSuccess.Add(1)
	open := counters.open.Add(1)
	counters.observeOpen(open)
	return &countedNetworkConn{TConn: conn, counters: counters}, nil
}

func newNetworkBenchmarkPool(
	ctx context.Context,
	server *networkBenchmarkServer,
	maxConns int,
	maxIdleConns int,
	maxWaiters int,
	counters *networkConnectionCounters,
) *gpool.Gpool[example.ExampleClient] {
	conf := &gpool.GPoolConfig[example.ExampleClient]{
		MaxConns:     maxConns,
		MaxIdleConns: maxIdleConns,
		MaxWaiters:   maxWaiters,
		Cfunc: func(ctx context.Context, host string, port int, timeout int) (gpool.Conn[example.ExampleClient], error) {
			return newCountedNetworkConn(ctx, host, port, timeout, counters)
		},
	}
	pool := &gpool.Gpool[example.ExampleClient]{}
	pool.GpoolInit2(ctx, server.host, server.port, networkBenchmarkTimeout, conf)
	return pool
}

func callNetworkBenchmarkRPC(ctx context.Context, conn gpool.Conn[example.ExampleClient]) error {
	client := conn.GetThrfitClient()
	got, err := client.Add(ctx, 1, 2)
	if err != nil {
		return err
	}
	if got != 3 {
		return fmt.Errorf("Add result = %d, want 3", got)
	}
	return nil
}

func reportNetworkConnectionMetrics(b *testing.B, counters *networkConnectionCounters) {
	b.ReportMetric(float64(counters.createSuccess.Load())/float64(b.N), "conn-create/op")
	b.ReportMetric(float64(counters.closeCalls.Load())/float64(b.N), "conn-close/op")
	b.ReportMetric(float64(counters.maxOpen.Load()), "max-open")
}

func closeNetworkConnections(connections []gpool.Conn[example.ExampleClient]) error {
	var firstErr error
	for _, conn := range connections {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func BenchmarkGpoolNetworkRPC(b *testing.B) {
	server, err := startNetworkBenchmarkServer()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := server.Close(); err != nil {
			b.Errorf("stop benchmark server: %v", err)
		}
	})

	ctx := context.Background()
	parallelWorkers := runtime.GOMAXPROCS(0) * networkBenchmarkParallelism

	b.Run("Dedicated/Serial", func(b *testing.B) {
		counters := &networkConnectionCounters{}
		conn, err := newCountedNetworkConn(ctx, server.host, server.port, networkBenchmarkTimeout, counters)
		if err != nil {
			b.Fatal(err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if err := callNetworkBenchmarkRPC(ctx, conn); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		if err := conn.Close(); err != nil {
			b.Fatal(err)
		}
		reportNetworkConnectionMetrics(b, counters)
	})

	b.Run("Pool/Serial", func(b *testing.B) {
		counters := &networkConnectionCounters{}
		pool := newNetworkBenchmarkPool(ctx, server, 1, 1, 1, counters)

		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			conn, err := pool.Get(ctx)
			if err != nil {
				b.Fatal(err)
			}
			if err := callNetworkBenchmarkRPC(ctx, conn.Gc); err != nil {
				b.Fatal(err)
			}
			conn.Close(ctx)
		}
		b.StopTimer()
		if err := pool.Close(ctx); err != nil {
			b.Fatal(err)
		}
		reportNetworkConnectionMetrics(b, counters)
	})

	b.Run("Dedicated/Parallel", func(b *testing.B) {
		counters := &networkConnectionCounters{}
		connections := make([]gpool.Conn[example.ExampleClient], parallelWorkers)
		for i := range connections {
			conn, err := newCountedNetworkConn(ctx, server.host, server.port, networkBenchmarkTimeout, counters)
			if err != nil {
				b.Fatal(err)
			}
			connections[i] = conn
		}

		var workerIndex atomic.Int64
		b.SetParallelism(networkBenchmarkParallelism)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			index := int(workerIndex.Add(1) - 1)
			conn := connections[index]
			for pb.Next() {
				if err := callNetworkBenchmarkRPC(ctx, conn); err != nil {
					b.Error(err)
					return
				}
			}
		})
		b.StopTimer()
		if err := closeNetworkConnections(connections); err != nil {
			b.Fatal(err)
		}
		reportNetworkConnectionMetrics(b, counters)
	})

	poolParallelBenchmark := func(b *testing.B, maxConns int) {
		counters := &networkConnectionCounters{}
		pool := newNetworkBenchmarkPool(ctx, server, maxConns, maxConns, parallelWorkers*2, counters)

		b.SetParallelism(networkBenchmarkParallelism)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := pool.Get(ctx)
				if err != nil {
					b.Error(err)
					return
				}
				if err := callNetworkBenchmarkRPC(ctx, conn.Gc); err != nil {
					conn.Close(ctx)
					b.Error(err)
					return
				}
				conn.Close(ctx)
			}
		})
		b.StopTimer()
		if err := pool.Close(ctx); err != nil {
			b.Fatal(err)
		}
		reportNetworkConnectionMetrics(b, counters)
	}

	b.Run("Pool/ParallelAmple", func(b *testing.B) {
		poolParallelBenchmark(b, parallelWorkers)
	})
	b.Run("Pool/ParallelMax8", func(b *testing.B) {
		poolParallelBenchmark(b, min(8, parallelWorkers))
	})
	b.Run("Pool/ParallelMax1", func(b *testing.B) {
		poolParallelBenchmark(b, 1)
	})

}

// BenchmarkGpoolNetworkRPCDialEach must be run with a fixed iteration count,
// for example -benchtime=1000x. A duration-based benchmark can exhaust the
// host's ephemeral TCP ports because every iteration creates a TIME_WAIT
// socket.
func BenchmarkGpoolNetworkRPCDialEach(b *testing.B) {
	server, err := startNetworkBenchmarkServer()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := server.Close(); err != nil {
			b.Errorf("stop benchmark server: %v", err)
		}
	})

	ctx := context.Background()
	counters := &networkConnectionCounters{}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		conn, err := newCountedNetworkConn(ctx, server.host, server.port, networkBenchmarkTimeout, counters)
		if err != nil {
			b.Fatal(err)
		}
		if err := callNetworkBenchmarkRPC(ctx, conn); err != nil {
			_ = conn.Close()
			b.Fatal(err)
		}
		if err := conn.Close(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reportNetworkConnectionMetrics(b, counters)
}

type networkLatencySample struct {
	get   time.Duration
	rpc   time.Duration
	total time.Duration
}

type networkLatencyRequest func() (networkLatencySample, error)

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := int(float64(len(values)-1) * percentile)
	return values[index]
}

func runNetworkLatencyScenario(
	t *testing.T,
	name string,
	workers int,
	callsPerWorker int,
	newRequest func(worker int) (networkLatencyRequest, func(), error),
) {
	t.Helper()

	requests := make([]networkLatencyRequest, workers)
	cleanups := make([]func(), workers)
	for worker := range workers {
		request, cleanup, err := newRequest(worker)
		if err != nil {
			for _, previousCleanup := range cleanups[:worker] {
				previousCleanup()
			}
			t.Fatalf("%s setup worker %d: %v", name, worker, err)
		}
		requests[worker] = request
		cleanups[worker] = cleanup
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	// Warm each connection and generated client before collecting latency.
	for worker, request := range requests {
		if _, err := request(); err != nil {
			t.Fatalf("%s warmup worker %d: %v", name, worker, err)
		}
	}

	totalCalls := workers * callsPerWorker
	samples := make([]networkLatencySample, totalCalls)
	errCh := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	wallStart := time.Now()
	for worker := range workers {
		worker := worker
		go func() {
			defer wg.Done()
			<-start
			base := worker * callsPerWorker
			for call := range callsPerWorker {
				sample, err := requests[worker]()
				if err != nil {
					errCh <- err
					return
				}
				samples[base+call] = sample
			}
		}()
	}
	wallStart = time.Now()
	close(start)
	wg.Wait()
	elapsed := time.Since(wallStart)
	close(errCh)
	for err := range errCh {
		t.Fatalf("%s request: %v", name, err)
	}

	getValues := make([]time.Duration, totalCalls)
	rpcValues := make([]time.Duration, totalCalls)
	totalValues := make([]time.Duration, totalCalls)
	for i, sample := range samples {
		getValues[i] = sample.get
		rpcValues[i] = sample.rpc
		totalValues[i] = sample.total
	}
	qps := float64(totalCalls) / elapsed.Seconds()
	t.Logf(
		"%s: calls=%d concurrency=%d qps=%.0f total[p50=%v p95=%v p99=%v] get[p50=%v p95=%v p99=%v] rpc[p50=%v p95=%v p99=%v]",
		name,
		totalCalls,
		workers,
		qps,
		percentileDuration(totalValues, 0.50),
		percentileDuration(totalValues, 0.95),
		percentileDuration(totalValues, 0.99),
		percentileDuration(getValues, 0.50),
		percentileDuration(getValues, 0.95),
		percentileDuration(getValues, 0.99),
		percentileDuration(rpcValues, 0.50),
		percentileDuration(rpcValues, 0.95),
		percentileDuration(rpcValues, 0.99),
	)
}

func TestGpoolNetworkRPCLatency(t *testing.T) {
	server, err := startNetworkBenchmarkServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("stop benchmark server: %v", err)
		}
	}()

	const (
		workers        = 32
		callsPerWorker = 1_000
	)
	ctx := context.Background()

	t.Run("Dedicated", func(t *testing.T) {
		counters := &networkConnectionCounters{}
		runNetworkLatencyScenario(t, "dedicated", workers, callsPerWorker, func(int) (networkLatencyRequest, func(), error) {
			conn, err := newCountedNetworkConn(ctx, server.host, server.port, networkBenchmarkTimeout, counters)
			if err != nil {
				return nil, nil, err
			}
			request := func() (networkLatencySample, error) {
				start := time.Now()
				err := callNetworkBenchmarkRPC(ctx, conn)
				elapsed := time.Since(start)
				return networkLatencySample{rpc: elapsed, total: elapsed}, err
			}
			return request, func() { _ = conn.Close() }, nil
		})
	})

	poolScenario := func(t *testing.T, name string, maxConns int) {
		counters := &networkConnectionCounters{}
		pool := newNetworkBenchmarkPool(ctx, server, maxConns, maxConns, workers*2, counters)
		defer func() {
			if err := pool.Close(ctx); err != nil {
				t.Errorf("close %s pool: %v", name, err)
			}
		}()

		runNetworkLatencyScenario(t, name, workers, callsPerWorker, func(int) (networkLatencyRequest, func(), error) {
			request := func() (networkLatencySample, error) {
				totalStart := time.Now()
				conn, err := pool.Get(ctx)
				gotConn := time.Now()
				if err != nil {
					return networkLatencySample{}, err
				}
				rpcErr := callNetworkBenchmarkRPC(ctx, conn.Gc)
				rpcDone := time.Now()
				conn.Close(ctx)
				return networkLatencySample{
					get:   gotConn.Sub(totalStart),
					rpc:   rpcDone.Sub(gotConn),
					total: time.Since(totalStart),
				}, rpcErr
			}
			return request, func() {}, nil
		})
	}

	t.Run("PoolAmple", func(t *testing.T) {
		poolScenario(t, "pool-ample", workers)
	})
	t.Run("PoolMax8", func(t *testing.T) {
		poolScenario(t, "pool-max-8", 8)
	})
	t.Run("PoolMax1", func(t *testing.T) {
		poolScenario(t, "pool-max-1", 1)
	})
}

func TestGpoolNetworkRPCHarness(t *testing.T) {
	server, err := startNetworkBenchmarkServer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("stop benchmark server: %v", err)
		}
	}()

	ctx := context.Background()
	counters := &networkConnectionCounters{}
	pool := newNetworkBenchmarkPool(ctx, server, 2, 2, 16, counters)

	const workers = 8
	const callsPerWorker = 25
	start := make(chan struct{})
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			for range callsPerWorker {
				conn, err := pool.Get(ctx)
				if err != nil {
					errCh <- err
					return
				}
				rpcErr := callNetworkBenchmarkRPC(ctx, conn.Gc)
				conn.Close(ctx)
				if rpcErr != nil {
					errCh <- rpcErr
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	if err := pool.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if got := counters.createSuccess.Load(); got > 2 {
		t.Fatalf("created connections = %d, max = 2", got)
	}
	if got, want := counters.closeCalls.Load(), counters.createSuccess.Load(); got != want {
		t.Fatalf("physical close calls = %d, successful creates = %d", got, want)
	}
}
