//go:build performance

package workpool_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/workpool"
)

const (
	workPoolBenchmarkBatchSize    = 1_024
	workPoolProfileProducers      = 8
	workPoolProfileOpsPerProducer = 10_000
	workPoolCPUIterations         = 4_096
)

var (
	workPoolBenchmarkSink     any
	workPoolBenchmarkUintSink atomic.Uint64
)

func benchmarkContext() context.Context {
	return context.WithValue(context.Background(), "request_id", "workpool-benchmark")
}

func benchmarkNoOpProcess(_ context.Context, req any) (any, error) {
	return req, nil
}

func benchmarkCPUProcess(_ context.Context, req any) (any, error) {
	value := req.(uint64) + 0x9e3779b97f4a7c15
	for i := 0; i < workPoolCPUIterations; i++ {
		value ^= value << 13
		value ^= value >> 7
		value ^= value << 17
	}
	return value, nil
}

func BenchmarkWorkPoolDirectProcess(b *testing.B) {
	for _, benchmark := range []struct {
		name    string
		process workpool.Process
	}{
		{name: "NoOp", process: benchmarkNoOpProcess},
		{name: "CPU4096", process: benchmarkCPUProcess},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			ctx := benchmarkContext()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ret, err := benchmark.process(ctx, uint64(i))
				if err != nil {
					b.Fatal(err)
				}
				workPoolBenchmarkSink = ret
			}
		})
	}
}

func BenchmarkGoroutinePerTask(b *testing.B) {
	for _, benchmark := range []struct {
		name    string
		process workpool.Process
	}{
		{name: "NoOp", process: benchmarkNoOpProcess},
		{name: "CPU4096", process: benchmarkCPUProcess},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			ctx := benchmarkContext()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := make(chan any, 1)
				go func(value uint64) {
					ret, _ := benchmark.process(ctx, value)
					result <- ret
				}(uint64(i))
				workPoolBenchmarkSink = <-result
			}
		})
	}
}

func BenchmarkWorkPoolBatch(b *testing.B) {
	logger.Gfilelog = nil
	for _, benchmark := range []struct {
		name    string
		process workpool.Process
	}{
		{name: "NoOp", process: benchmarkNoOpProcess},
		{name: "CPU4096", process: benchmarkCPUProcess},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			for _, workers := range []int{1, 2, 4, 8, 16} {
				b.Run("Workers"+strconv.Itoa(workers), func(b *testing.B) {
					benchmarkWorkPoolBatch(b, workers, benchmark.process)
				})
			}
		})
	}
}

func benchmarkWorkPoolBatch(b *testing.B, workers int, process workpool.Process) {
	ctx := benchmarkContext()
	wp := workpool.NewWorkPool(workers)
	wp.Run(ctx)
	tasks := make([]*workpool.Task, workers)

	b.ReportAllocs()
	b.ResetTimer()
	completed := 0
	for completed < b.N {
		batchSize := min(workers, b.N-completed)
		for i := 0; i < batchSize; i++ {
			task, err := wp.AddTask(ctx, uint64(completed+i), process)
			if err != nil {
				b.Fatal(err)
			}
			tasks[i] = task
		}
		for i := 0; i < batchSize; i++ {
			ret, err := tasks[i].WaitRet(context.Background())
			if err != nil {
				b.Fatal(err)
			}
			workPoolBenchmarkSink = ret
		}
		completed += batchSize
	}
	b.StopTimer()
	wp.Kill(context.Background())
	wp.Join(context.Background())
}

func BenchmarkWorkPoolParallelRoundTrip(b *testing.B) {
	logger.Gfilelog = nil
	for _, workers := range []int{1, runtime.GOMAXPROCS(0), runtime.GOMAXPROCS(0) * 4} {
		workers := workers
		b.Run("Workers"+strconv.Itoa(workers), func(b *testing.B) {
			ctx := benchmarkContext()
			wp := workpool.NewWorkPool(workers)
			wp.Run(ctx)
			var rejected atomic.Uint64

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					for {
						task, err := wp.AddTask(ctx, uint64(1), benchmarkNoOpProcess)
						if errors.Is(err, workpool.ErrPoolFull) {
							rejected.Add(1)
							runtime.Gosched()
							continue
						}
						if err != nil {
							b.Error(err)
							break
						}
						ret, waitErr := task.WaitRet(context.Background())
						if waitErr != nil {
							b.Error(waitErr)
						}
						if value, ok := ret.(uint64); ok {
							workPoolBenchmarkUintSink.Store(value)
						}
						break
					}
				}
			})
			b.StopTimer()
			b.ReportMetric(float64(rejected.Load())/float64(max(1, b.N)), "rejects/op")
			wp.Kill(context.Background())
			wp.Join(context.Background())
		})
	}
}

func BenchmarkWorkPoolAddTask(b *testing.B) {
	logger.Gfilelog = nil
	b.Run("AcceptedFixedRequestID", func(b *testing.B) {
		benchmarkAcceptedAddTask(b, benchmarkContext())
	})
	b.Run("AcceptedGeneratedRequestID", func(b *testing.B) {
		benchmarkAcceptedAddTask(b, context.Background())
	})
	b.Run("RejectedFull", func(b *testing.B) {
		ctx := benchmarkContext()
		wp := workpool.NewWorkPool(1)
		if _, err := wp.AddTask(ctx, nil, benchmarkNoOpProcess); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := wp.AddTask(ctx, nil, benchmarkNoOpProcess); !errors.Is(err, workpool.ErrPoolFull) {
				b.Fatalf("AddTask error = %v, want %v", err, workpool.ErrPoolFull)
			}
		}
		b.StopTimer()
		wp.Kill(context.Background())
	})
}

func benchmarkAcceptedAddTask(b *testing.B, ctx context.Context) {
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	completed := 0
	for completed < b.N {
		batchSize := min(workPoolBenchmarkBatchSize, b.N-completed)
		wp := workpool.NewWorkPool(batchSize)
		b.StartTimer()
		for i := 0; i < batchSize; i++ {
			if _, err := wp.AddTask(ctx, nil, benchmarkNoOpProcess); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		wp.Kill(context.Background())
		completed += batchSize
	}
}

func BenchmarkWorkPoolWaitRet(b *testing.B) {
	logger.Gfilelog = nil
	b.Run("Completed", func(b *testing.B) {
		ctx := benchmarkContext()
		wp := workpool.NewWorkPool(1)
		wp.Run(ctx)
		task, err := wp.AddTask(ctx, "done", benchmarkNoOpProcess)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := task.WaitRet(context.Background()); err != nil {
			b.Fatal(err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ret, err := task.WaitRet(context.Background())
			if err != nil {
				b.Fatal(err)
			}
			workPoolBenchmarkSink = ret
		}
		b.StopTimer()
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	b.Run("CanceledWaitContext", func(b *testing.B) {
		wp := workpool.NewWorkPool(1)
		task, err := wp.AddTask(benchmarkContext(), nil, benchmarkNoOpProcess)
		if err != nil {
			b.Fatal(err)
		}
		waitCtx, cancel := context.WithCancel(context.Background())
		cancel()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := task.WaitRet(waitCtx); !errors.Is(err, context.Canceled) {
				b.Fatalf("WaitRet error = %v, want context canceled", err)
			}
		}
		b.StopTimer()
		wp.Kill(context.Background())
	})
}

func BenchmarkWorkPoolLifecycle(b *testing.B) {
	logger.Gfilelog = nil
	for _, workers := range []int{1, 8, 32} {
		workers := workers
		b.Run("Workers"+strconv.Itoa(workers), func(b *testing.B) {
			ctx := benchmarkContext()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				wp := workpool.NewWorkPool(workers)
				wp.Run(ctx)
				wp.Kill(context.Background())
				wp.Join(context.Background())
			}
		})
	}
}

type workPoolProfileResult struct {
	name       string
	workers    int
	operations int
	duration   time.Duration
	rejected   uint64
	latencies  []time.Duration
}

func TestWorkPoolLatencyProfile(t *testing.T) {
	logger.Gfilelog = nil
	for _, benchmark := range []struct {
		name    string
		process workpool.Process
	}{
		{name: "NoOp", process: benchmarkNoOpProcess},
		{name: "CPU4096", process: benchmarkCPUProcess},
	} {
		for _, workers := range []int{1, 4, 8} {
			result := profileWorkPool(t, benchmark.name, workers, benchmark.process)
			sort.Slice(result.latencies, func(i, j int) bool {
				return result.latencies[i] < result.latencies[j]
			})
			qps := float64(result.operations) / result.duration.Seconds()
			fmt.Printf(
				"WORKPOOL_PROFILE workload=%s workers=%d producers=%d operations=%d qps=%.0f rejects=%d rejects_per_op=%.4f p50=%s p95=%s p99=%s\n",
				result.name,
				result.workers,
				workPoolProfileProducers,
				result.operations,
				qps,
				result.rejected,
				float64(result.rejected)/float64(result.operations),
				profilePercentile(result.latencies, 0.50),
				profilePercentile(result.latencies, 0.95),
				profilePercentile(result.latencies, 0.99),
			)
		}
	}
}

func profileWorkPool(t *testing.T, name string, workers int, process workpool.Process) workPoolProfileResult {
	t.Helper()
	ctx := benchmarkContext()
	wp := workpool.NewWorkPool(workers)
	wp.Run(ctx)

	type producerResult struct {
		rejected  uint64
		latencies []time.Duration
	}
	results := make(chan producerResult, workPoolProfileProducers)
	start := make(chan struct{})
	var producers sync.WaitGroup
	producers.Add(workPoolProfileProducers)
	profileStarted := time.Now()
	for producer := 0; producer < workPoolProfileProducers; producer++ {
		producer := producer
		go func() {
			defer producers.Done()
			<-start
			local := producerResult{latencies: make([]time.Duration, 0, workPoolProfileOpsPerProducer)}
			for operation := 0; operation < workPoolProfileOpsPerProducer; operation++ {
				started := time.Now()
				var task *workpool.Task
				for {
					var err error
					task, err = wp.AddTask(ctx, uint64(producer*workPoolProfileOpsPerProducer+operation), process)
					if errors.Is(err, workpool.ErrPoolFull) {
						local.rejected++
						runtime.Gosched()
						continue
					}
					if err != nil {
						t.Errorf("AddTask: %v", err)
						return
					}
					break
				}
				if _, err := task.WaitRet(context.Background()); err != nil {
					t.Errorf("WaitRet: %v", err)
					return
				}
				local.latencies = append(local.latencies, time.Since(started))
			}
			results <- local
		}()
	}
	profileStarted = time.Now()
	close(start)
	producers.Wait()
	duration := time.Since(profileStarted)
	close(results)
	wp.Kill(context.Background())
	wp.Join(context.Background())

	result := workPoolProfileResult{
		name:       name,
		workers:    workers,
		operations: workPoolProfileProducers * workPoolProfileOpsPerProducer,
		duration:   duration,
		latencies:  make([]time.Duration, 0, workPoolProfileProducers*workPoolProfileOpsPerProducer),
	}
	for producer := range results {
		result.rejected += producer.rejected
		result.latencies = append(result.latencies, producer.latencies...)
	}
	if len(result.latencies) != result.operations {
		t.Fatalf("profile %s workers=%d collected %d latencies, want %d", name, workers, len(result.latencies), result.operations)
	}
	return result
}

func profilePercentile(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * percentile)
	return values[index]
}
