package gpool_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/gpool"
)

// benchmarkPoolClient is deliberately empty: these benchmarks measure pool
// bookkeeping only, without network or Thrift serialization overhead.
type benchmarkPoolClient struct{}

type benchmarkPoolConn struct {
	open atomic.Bool
}

func (c *benchmarkPoolConn) Init(string, int, int) error { return nil }

func (c *benchmarkPoolConn) Open() error {
	c.open.Store(true)
	return nil
}

func (c *benchmarkPoolConn) Close() error {
	c.open.Store(false)
	return nil
}

func (c *benchmarkPoolConn) IsOpen() bool { return c.open.Load() }

func (c *benchmarkPoolConn) GetThrfitClient() *benchmarkPoolClient { return nil }

func (c *benchmarkPoolConn) SetTimeOut(context.Context, time.Duration) {}

func newBenchmarkPool(maxConns int) *gpool.Gpool[benchmarkPoolClient] {
	conf := &gpool.GPoolConfig[benchmarkPoolClient]{
		MaxConns:     maxConns,
		MaxIdleConns: maxConns,
		MaxWaiters:   max(256, maxConns*4),
		Cfunc: func(context.Context, string, int, int) (gpool.Conn[benchmarkPoolClient], error) {
			conn := &benchmarkPoolConn{}
			_ = conn.Open()
			return conn, nil
		},
	}

	pool := &gpool.Gpool[benchmarkPoolClient]{}
	pool.GpoolInit2(context.Background(), "benchmark", 0, 30_000, conf)
	return pool
}

func BenchmarkGpoolGetClose(b *testing.B) {
	ctx := context.Background()

	b.Run("Serial", func(b *testing.B) {
		pool := newBenchmarkPool(1)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			conn, err := pool.Get(ctx)
			if err != nil {
				b.Fatal(err)
			}
			conn.Close(ctx)
		}
	})

	b.Run("ParallelUncontended", func(b *testing.B) {
		// Keep enough pre-created connections that this case measures mutex/queue
		// contention, not the wait/notification path.
		pool := newBenchmarkPool(max(256, runtime.GOMAXPROCS(0)*4))
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := pool.Get(ctx)
				if err != nil {
					b.Fatal(err)
				}
				conn.Close(ctx)
			}
		})
	})

	b.Run("ParallelContendedOneConn", func(b *testing.B) {
		// This intentionally forces callers through the FIFO waiter queue.
		pool := newBenchmarkPool(1)
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := pool.Get(ctx)
				if err != nil {
					b.Fatal(err)
				}
				conn.Close(ctx)
			}
		})
	})
}
