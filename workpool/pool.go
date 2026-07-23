package workpool

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

var (
	ErrPoolFull   = errors.New("work pool full")
	ErrPoolClosed = errors.New("work pool closed")
	ErrNilProcess = errors.New("work pool process is nil")
	ErrTaskPanic  = errors.New("work pool task panic")
)

type Process func(ctx context.Context, req any) (any, error)

type WorkPool struct {
	// TaskQ and Notify are retained for source compatibility. Scheduling is
	// performed by the internal bounded task channel.
	TaskQ    cas.Queue
	PoolSize int32
	Notify   chan struct{}
	Wg       sync.WaitGroup
	Cancel   context.CancelFunc

	parallel int32
	tasks    chan *Task
	ctx      context.Context
	cancel   context.CancelFunc

	mu       sync.Mutex
	started  bool
	stopped  bool
	stopOnce sync.Once
	stopRun  func() bool
}

type Task struct {
	TaskId string
	Req    any
	Ret    any
	Err    error
	Wait   chan struct{}

	ctx          context.Context
	process      Process
	completeOnce sync.Once
}

func NewWorkPool(poolSize int) *WorkPool {
	if poolSize <= 0 {
		panic("workpool: pool size must be greater than zero")
	}

	poolCtx, cancel := context.WithCancel(context.Background())
	wp := &WorkPool{
		PoolSize: int32(poolSize),
		TaskQ:    cas.CreateCasQueue(),
		Notify:   make(chan struct{}, poolSize),
		tasks:    make(chan *Task, poolSize),
		ctx:      poolCtx,
		cancel:   cancel,
	}
	// Keep direct calls to the historically exported Cancel field safe and
	// consistent with Kill.
	wp.Cancel = func() {
		wp.stop(context.Background())
	}
	return wp
}

func (task *Task) WaitRet(ctx context.Context) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-task.Wait:
		return task.Ret, task.Err
	default:
	}

	select {
	case <-task.Wait:
		return task.Ret, task.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (task *Task) complete(ret any, err error) {
	task.completeOnce.Do(func() {
		task.Ret = ret
		task.Err = err
		close(task.Wait)
	})
}

func (wp *WorkPool) AddTask(ctx context.Context, req any, process Process) (*Task, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if process == nil {
		return nil, ErrNilProcess
	}

	reqID := ""
	if value, ok := ctx.Value("request_id").(string); ok {
		reqID = value
	}
	if reqID == "" {
		reqID = util.NewRequestID()
	}

	task := &Task{
		TaskId: reqID,
		Req:    req,
		Wait:   make(chan struct{}),
		// Submitted work is asynchronous. Retain request-scoped values for
		// logging, but preserve the historical behavior where completion of the
		// submitting request does not cancel an accepted task.
		ctx:     context.WithoutCancel(ctx),
		process: process,
	}

	wp.mu.Lock()
	if wp.stopped {
		wp.mu.Unlock()
		logger.Warn(ctx, "submit work pool task failed", "reason", "pool_closed")
		return nil, ErrPoolClosed
	}

	// Increment before publishing the task so a worker can never decrement the
	// queued count before the submitter increments it.
	queued := atomic.AddInt32(&wp.parallel, 1)
	select {
	case wp.tasks <- task:
		wp.mu.Unlock()
		logger.Debug(ctx, "work pool task queued", "queued", queued)
		return task, nil
	default:
		atomic.AddInt32(&wp.parallel, -1)
		wp.mu.Unlock()
		logger.Warn(ctx, "submit work pool task failed", "reason", "pool_full")
		return nil, ErrPoolFull
	}
}

func (wp *WorkPool) execute(workerCtx context.Context, task *Task) {
	startTime := time.Now()
	taskCtx := task.ctx
	if taskCtx == nil {
		taskCtx = context.Background()
	}
	if taskCtx.Value("trace_id") == nil {
		taskCtx = context.WithValue(taskCtx, "trace_id", workerCtx.Value("trace_id"))
	}
	taskCtx = context.WithValue(taskCtx, "request_id", task.TaskId)
	taskCtx, cancel := context.WithCancel(taskCtx)
	stopPoolCancellation := context.AfterFunc(wp.ctx, cancel)
	defer func() {
		stopPoolCancellation()
		cancel()
	}()

	if err := taskCtx.Err(); err != nil {
		task.complete(nil, err)
		return
	}

	var (
		ret any
		err error
	)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%w: %v", ErrTaskPanic, recovered)
				logger.Error(taskCtx, "work pool task panicked", "panic", recovered, "stack", string(debug.Stack()))
			}
		}()
		ret, err = task.process(taskCtx, task.Req)
	}()

	task.complete(ret, err)
	if err != nil {
		logger.Warn(taskCtx, "execute work pool task failed", "err", err, "cost", time.Since(startTime))
		return
	}
	logger.Debug(taskCtx, "execute work pool task completed", "cost", time.Since(startTime))
}

func (wp *WorkPool) worker(workerCtx context.Context) {
	defer wp.Wg.Done()
	logger.Debug(workerCtx, "work pool worker started")
	defer logger.Debug(workerCtx, "work pool worker stopped")

	for {
		select {
		case <-wp.ctx.Done():
			return
		case task := <-wp.tasks:
			atomic.AddInt32(&wp.parallel, -1)
			if wp.isStopped() {
				task.complete(nil, ErrPoolClosed)
				return
			}
			wp.execute(workerCtx, task)
		}
	}
}

func (wp *WorkPool) isStopped() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.stopped
}

func (wp *WorkPool) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	wp.mu.Lock()
	if wp.started || wp.stopped {
		wp.mu.Unlock()
		return
	}
	wp.started = true
	workerCount := int(wp.PoolSize)
	wp.Wg.Add(workerCount)
	wp.stopRun = context.AfterFunc(ctx, func() {
		wp.stop(ctx)
	})
	wp.mu.Unlock()

	for i := 0; i < workerCount; i++ {
		workerCtx := context.WithValue(wp.ctx, "trace_id", util.NewProcessID())
		go wp.worker(workerCtx)
	}
}

func (wp *WorkPool) Join(ctx context.Context) {
	wp.Wg.Wait()
	if ctx == nil {
		ctx = context.Background()
	}
	logger.Debug(ctx, "wait for work pool workers")
}

func (wp *WorkPool) stop(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	wp.stopOnce.Do(func() {
		wp.mu.Lock()
		wp.stopped = true
		if wp.stopRun != nil {
			wp.stopRun()
		}
		wp.cancel()
		for {
			select {
			case task := <-wp.tasks:
				atomic.AddInt32(&wp.parallel, -1)
				task.complete(nil, ErrPoolClosed)
			default:
				wp.mu.Unlock()
				logger.Debug(ctx, "work pool stopped")
				return
			}
		}
	})
}

func (wp *WorkPool) Kill(ctx context.Context) {
	wp.stop(ctx)
}
