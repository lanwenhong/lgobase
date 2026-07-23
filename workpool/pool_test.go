package workpool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/workpool"
)

func TestConsumer(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace")
	wp := workpool.NewWorkPool(3)
	wp.Run(ctx)

	for i := 0; i < 10; i++ {
		task, err := wp.AddTask(ctx, 1, func(ctx context.Context, req any) (any, error) {
			return req.(int) + 1, nil
		})
		if err != nil {
			t.Fatal(err)
		}

		ret, err := task.WaitRet(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if ret != 2 {
			t.Fatalf("unexpected result: %v", ret)
		}
	}

	wp.Kill(ctx)
	wp.Join(ctx)
}

func TestTaskReturnsProcessErrorAndContextValues(t *testing.T) {
	type contextKey string
	const key contextKey = "task-value"

	ctx := context.WithValue(context.Background(), key, "expected")
	wantErr := errors.New("process failed")
	wp := workpool.NewWorkPool(1)
	wp.Run(ctx)
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	task, err := wp.AddTask(ctx, nil, func(taskCtx context.Context, _ any) (any, error) {
		if got := taskCtx.Value(key); got != "expected" {
			t.Errorf("context value = %v, want expected", got)
		}
		return "partial", wantErr
	})
	if err != nil {
		t.Fatal(err)
	}

	ret, err := task.WaitRet(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if ret != "partial" {
		t.Fatalf("result = %v, want partial", ret)
	}
}

func TestWaitRetHonorsContext(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := task.WaitRet(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitRet error = %v, want deadline exceeded", err)
	}

	wp.Kill(context.Background())
	if _, err := task.WaitRet(context.Background()); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("pending task error = %v, want pool closed", err)
	}
}

func TestConcurrentSubmitRespectsQueueCapacity(t *testing.T) {
	const (
		poolSize    = 8
		submitCount = 128
	)

	wp := workpool.NewWorkPool(poolSize)
	start := make(chan struct{})
	acceptedTasks := make(chan *workpool.Task, submitCount)
	var (
		accepted atomic.Int32
		full     atomic.Int32
		wg       sync.WaitGroup
	)

	for i := 0; i < submitCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
				return nil, nil
			})
			switch {
			case err == nil:
				accepted.Add(1)
				acceptedTasks <- task
			case errors.Is(err, workpool.ErrPoolFull):
				full.Add(1)
			default:
				t.Errorf("unexpected AddTask error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(acceptedTasks)

	if got := accepted.Load(); got != poolSize {
		t.Fatalf("accepted tasks = %d, want %d", got, poolSize)
	}
	if got := full.Load(); got != submitCount-poolSize {
		t.Fatalf("full errors = %d, want %d", got, submitCount-poolSize)
	}

	wp.Kill(context.Background())
	for task := range acceptedTasks {
		if _, err := task.WaitRet(context.Background()); !errors.Is(err, workpool.ErrPoolClosed) {
			t.Errorf("pending task error = %v, want pool closed", err)
		}
	}
}

func TestRunningPoolRespectsWorkerAndQueueCapacity(t *testing.T) {
	const poolSize = 4

	wp := workpool.NewWorkPool(poolSize)
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	release := make(chan struct{})
	started := make(chan struct{}, poolSize)
	var (
		active    atomic.Int32
		maxActive atomic.Int32
	)
	process := func(ctx context.Context, _ any) (any, error) {
		current := active.Add(1)
		updateMaxInt32(&maxActive, current)
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		active.Add(-1)
		return current, nil
	}

	tasks := make([]*workpool.Task, 0, poolSize*2)
	for i := 0; i < poolSize; i++ {
		task, err := wp.AddTask(context.Background(), nil, process)
		if err != nil {
			t.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	for i := 0; i < poolSize; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker did not start task")
		}
	}

	// All workers are busy, so exactly poolSize additional tasks must fit in
	// the bounded queue.
	for i := 0; i < poolSize; i++ {
		task, err := wp.AddTask(context.Background(), nil, process)
		if err != nil {
			t.Fatalf("queue task %d: %v", i, err)
		}
		tasks = append(tasks, task)
	}
	if task, err := wp.AddTask(context.Background(), nil, process); task != nil || !errors.Is(err, workpool.ErrPoolFull) {
		t.Fatalf("overflow AddTask = (%v, %v), want (nil, %v)", task, err, workpool.ErrPoolFull)
	}

	if got := active.Load(); got != poolSize {
		t.Fatalf("active tasks = %d, want %d", got, poolSize)
	}
	if got := maxActive.Load(); got != poolSize {
		t.Fatalf("max active tasks = %d, want %d", got, poolSize)
	}

	close(release)
	for _, task := range tasks {
		if _, err := task.WaitRet(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := maxActive.Load(); got > poolSize {
		t.Fatalf("max active tasks = %d, exceeds pool size %d", got, poolSize)
	}
}

func TestTaskCompletionWakesAllWaiters(t *testing.T) {
	const waiterCount = 64

	wp := workpool.NewWorkPool(1)
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	release := make(chan struct{})
	task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		<-release
		return "result", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, waiterCount)
	ready := sync.WaitGroup{}
	ready.Add(waiterCount)
	for i := 0; i < waiterCount; i++ {
		go func() {
			ready.Done()
			ret, waitErr := task.WaitRet(context.Background())
			if waitErr == nil && ret != "result" {
				waitErr = errors.New("unexpected task result")
			}
			results <- waitErr
		}()
	}
	ready.Wait()
	close(release)

	for i := 0; i < waiterCount; i++ {
		select {
		case waitErr := <-results:
			if waitErr != nil {
				t.Fatal(waitErr)
			}
		case <-time.After(time.Second):
			t.Fatal("task completion did not wake every waiter")
		}
	}
}

func TestWaitRetTimeoutDoesNotCancelTask(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	started := make(chan struct{})
	finish := make(chan struct{})
	task, err := wp.AddTask(context.Background(), nil, func(ctx context.Context, _ any) (any, error) {
		close(started)
		select {
		case <-finish:
			return "finished", ctx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if _, err := task.WaitRet(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		cancel()
		t.Fatalf("first WaitRet error = %v, want deadline exceeded", err)
	}
	cancel()
	close(finish)

	ret, err := task.WaitRet(context.Background())
	if err != nil {
		t.Fatalf("completed task error = %v, want nil", err)
	}
	if ret != "finished" {
		t.Fatalf("completed task result = %v, want finished", ret)
	}
}

func TestTasksSubmittedBeforeRunExecuteFIFOWithOneWorker(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	var (
		mu    sync.Mutex
		order []int
	)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	process := func(ctx context.Context, req any) (any, error) {
		value := req.(int)
		mu.Lock()
		order = append(order, value)
		mu.Unlock()
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return value, nil
	}

	first, err := wp.AddTask(context.Background(), 0, process)
	if err != nil {
		t.Fatal(err)
	}
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})
	<-started
	second, err := wp.AddTask(context.Background(), 1, process)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	for _, task := range []*workpool.Task{first, second} {
		if _, err := task.WaitRet(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("execution order = %v, want [0 1]", order)
	}
	for i, got := range order {
		if got != i {
			t.Fatalf("execution order = %v, want [0 1]", order)
		}
	}
}

func TestTaskPanicDoesNotStopWorker(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	panicTask, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		panic("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := panicTask.WaitRet(context.Background()); !errors.Is(err, workpool.ErrTaskPanic) {
		t.Fatalf("panic task error = %v, want task panic", err)
	}

	nextTask, err := wp.AddTask(context.Background(), 41, func(_ context.Context, req any) (any, error) {
		return req.(int) + 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ret, err := nextTask.WaitRet(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ret != 42 {
		t.Fatalf("result = %v, want 42", ret)
	}
}

func TestKillCancelsRunningAndQueuedTasks(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	wp.Run(context.Background())

	started := make(chan struct{})
	running, err := wp.AddTask(context.Background(), nil, func(ctx context.Context, _ any) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	queued, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		t.Error("queued task should not execute after Kill")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wp.Kill(context.Background())
	wp.Kill(context.Background())
	wp.Join(context.Background())

	if _, err := running.WaitRet(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("running task error = %v, want context canceled", err)
	}
	if _, err := queued.WaitRet(context.Background()); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("queued task error = %v, want pool closed", err)
	}
	if _, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		return nil, nil
	}); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("AddTask after Kill error = %v, want pool closed", err)
	}
}

func TestRunContextCancellationStopsPool(t *testing.T) {
	runCtx, cancelRun := context.WithCancel(context.Background())
	wp := workpool.NewWorkPool(1)
	wp.Run(runCtx)

	started := make(chan struct{})
	running, err := wp.AddTask(context.Background(), nil, func(ctx context.Context, _ any) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	queued, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		return "unexpected", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	cancelRun()
	waitForJoin(t, wp)

	if _, err := running.WaitRet(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("running task error = %v, want context canceled", err)
	}
	if _, err := queued.WaitRet(context.Background()); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("queued task error = %v, want pool closed", err)
	}
	if _, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		return nil, nil
	}); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("AddTask after Run context cancellation = %v, want pool closed", err)
	}
}

func TestCancelBeforeRunCompletesPendingTask(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	var executed atomic.Bool
	task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		executed.Store(true)
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wp.Cancel()
	wp.Cancel()
	wp.Run(context.Background())
	wp.Join(context.Background())

	if _, err := task.WaitRet(context.Background()); !errors.Is(err, workpool.ErrPoolClosed) {
		t.Fatalf("pending task error = %v, want pool closed", err)
	}
	if executed.Load() {
		t.Fatal("task executed after Cancel before Run")
	}
}

func TestRunAndKillRaceCompletesPendingTask(t *testing.T) {
	for iteration := 0; iteration < 500; iteration++ {
		wp := workpool.NewWorkPool(1)
		task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
			return "done", nil
		})
		if err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var calls sync.WaitGroup
		calls.Add(2)
		go func() {
			defer calls.Done()
			<-start
			wp.Run(context.Background())
		}()
		go func() {
			defer calls.Done()
			<-start
			wp.Kill(context.Background())
		}()
		close(start)
		calls.Wait()
		waitForJoin(t, wp)

		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		ret, waitErr := task.WaitRet(waitCtx)
		cancel()
		if waitErr == nil {
			if ret != "done" {
				t.Fatalf("iteration %d result = %v, want done", iteration, ret)
			}
			continue
		}
		if !errors.Is(waitErr, workpool.ErrPoolClosed) && !errors.Is(waitErr, context.Canceled) {
			t.Fatalf("iteration %d task error = %v", iteration, waitErr)
		}
	}
}

func TestRunAndCancelAreSafeToCallMoreThanOnce(t *testing.T) {
	wp := workpool.NewWorkPool(2)
	wp.Run(context.Background())
	wp.Run(context.Background())
	wp.Cancel()
	wp.Cancel()
	wp.Join(context.Background())
}

func TestConcurrentSubmitAndKillCompletesAcceptedTasks(t *testing.T) {
	const submitCount = 128

	for iteration := 0; iteration < 20; iteration++ {
		wp := workpool.NewWorkPool(8)
		wp.Run(context.Background())

		start := make(chan struct{})
		accepted := make(chan *workpool.Task, submitCount)
		var submitters sync.WaitGroup
		for i := 0; i < submitCount; i++ {
			submitters.Add(1)
			go func() {
				defer submitters.Done()
				<-start
				task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
					return "done", nil
				})
				if err == nil {
					accepted <- task
					return
				}
				if !errors.Is(err, workpool.ErrPoolFull) && !errors.Is(err, workpool.ErrPoolClosed) {
					t.Errorf("unexpected AddTask error: %v", err)
				}
			}()
		}

		close(start)
		wp.Kill(context.Background())
		submitters.Wait()
		close(accepted)
		wp.Join(context.Background())

		for task := range accepted {
			waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, err := task.WaitRet(waitCtx)
			cancel()
			if err != nil && !errors.Is(err, workpool.ErrPoolClosed) && !errors.Is(err, context.Canceled) {
				t.Errorf("accepted task error = %v, want nil, pool closed, or context canceled", err)
			}
		}
	}
}

func TestAddTaskRejectsNilProcess(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	if _, err := wp.AddTask(context.Background(), nil, nil); !errors.Is(err, workpool.ErrNilProcess) {
		t.Fatalf("nil process error = %v, want nil process", err)
	}
}

func TestAcceptedTaskOutlivesSubmittingContext(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "request_id", "request-1"))
	task, err := wp.AddTask(ctx, nil, func(taskCtx context.Context, _ any) (any, error) {
		return taskCtx.Value("request_id"), taskCtx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	ret, err := task.WaitRet(context.Background())
	if err != nil {
		t.Fatalf("task error = %v, want nil", err)
	}
	if ret != "request-1" {
		t.Fatalf("request ID = %v, want request-1", ret)
	}
}

func TestRequestIDIsPreservedOrGenerated(t *testing.T) {
	wp := workpool.NewWorkPool(3)

	testCases := []struct {
		name       string
		ctx        context.Context
		wantTaskID string
	}{
		{
			name:       "existing string",
			ctx:        context.WithValue(context.Background(), "request_id", "request-existing"),
			wantTaskID: "request-existing",
		},
		{
			name: "empty string generates ID",
			ctx:  context.WithValue(context.Background(), "request_id", ""),
		},
		{
			name: "non-string generates ID",
			ctx:  context.WithValue(context.Background(), "request_id", 123),
		},
	}

	tasks := make([]*workpool.Task, 0, len(testCases))
	for _, testCase := range testCases {
		task, err := wp.AddTask(testCase.ctx, nil, func(ctx context.Context, _ any) (any, error) {
			return ctx.Value("request_id"), nil
		})
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if testCase.wantTaskID != "" && task.TaskId != testCase.wantTaskID {
			t.Fatalf("%s: TaskId = %q, want %q", testCase.name, task.TaskId, testCase.wantTaskID)
		}
		if task.TaskId == "" {
			t.Fatalf("%s: TaskId is empty", testCase.name)
		}
		tasks = append(tasks, task)
	}

	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})
	for i, task := range tasks {
		ret, err := task.WaitRet(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", testCases[i].name, err)
		}
		if ret != task.TaskId {
			t.Fatalf("%s: process request_id = %v, TaskId = %q", testCases[i].name, ret, task.TaskId)
		}
	}
}

func TestCompletedTaskResultWinsCanceledWaitContext(t *testing.T) {
	wp := workpool.NewWorkPool(1)
	wp.Run(context.Background())
	t.Cleanup(func() {
		wp.Kill(context.Background())
		wp.Join(context.Background())
	})

	task, err := wp.AddTask(context.Background(), nil, func(context.Context, any) (any, error) {
		return "done", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.WaitRet(context.Background()); err != nil {
		t.Fatal(err)
	}

	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()
	ret, err := task.WaitRet(waitCtx)
	if err != nil {
		t.Fatalf("completed WaitRet error = %v, want nil", err)
	}
	if ret != "done" {
		t.Fatalf("completed WaitRet result = %v, want done", ret)
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

func waitForJoin(t *testing.T, wp *workpool.WorkPool) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wp.Join(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Join did not return")
	}
}

func TestNewWorkPoolRejectsInvalidSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewWorkPool(0) did not panic")
		}
	}()
	workpool.NewWorkPool(0)
}
