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

// QueueMode selects the task storage and overload behavior used by WorkPool.
type QueueMode uint8

const (
	// QueueModeCAS preserves the historical WorkPool behavior: submissions are
	// stored in a growable CAS queue and are accepted while memory is available.
	QueueModeCAS QueueMode = iota
	// QueueModeChannel uses a bounded channel and returns ErrPoolFull when the
	// configured queue capacity is exhausted.
	QueueModeChannel
)

// Options configures a WorkPool without changing the historical
// NewWorkPool(int) constructor.
type Options struct {
	PoolSize  int
	QueueMode QueueMode
	// QueueSize only applies to QueueModeChannel. Zero defaults to PoolSize.
	QueueSize int
}

type WorkPool struct {
	// TaskQ and Notify retain their historically exported shape and provide the
	// storage and wake-up path for QueueModeCAS.
	TaskQ     cas.Queue
	PoolSize  int32
	Notify    chan struct{}
	Wg        sync.WaitGroup
	Cancel    context.CancelFunc
	QueueMode QueueMode
	QueueSize int32

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

// NewWorkPool creates a pool backed by the historical growable CAS queue.
func NewWorkPool(poolSize int) *WorkPool {
	return NewWorkPoolWithOptions(Options{
		PoolSize:  poolSize,
		QueueMode: QueueModeCAS,
	})
}

// NewWorkPoolWithOptions creates a pool with an explicit queue strategy.
// QueueModeChannel is bounded by QueueSize, while QueueModeCAS accepts tasks
// while memory is available.
func NewWorkPoolWithOptions(options Options) *WorkPool {
	if options.PoolSize <= 0 {
		panic("workpool: pool size must be greater than zero")
	}
	if options.QueueMode != QueueModeCAS && options.QueueMode != QueueModeChannel {
		panic("workpool: unsupported queue mode")
	}

	queueSize := options.QueueSize
	if options.QueueMode == QueueModeCAS {
		if queueSize != 0 {
			panic("workpool: queue size is not supported in CAS mode")
		}
	} else {
		if queueSize == 0 {
			queueSize = options.PoolSize
		}
		if queueSize < 0 {
			panic("workpool: queue size must not be negative")
		}
	}

	poolCtx, cancel := context.WithCancel(context.Background())
	wp := &WorkPool{
		PoolSize:  int32(options.PoolSize),
		TaskQ:     cas.CreateCasQueue(),
		Notify:    make(chan struct{}, options.PoolSize),
		QueueMode: options.QueueMode,
		QueueSize: int32(queueSize),
		ctx:       poolCtx,
		cancel:    cancel,
	}
	if options.QueueMode == QueueModeChannel {
		wp.tasks = make(chan *Task, queueSize)
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
	if wp.QueueMode == QueueModeCAS {
		if !wp.TaskQ.PushBack(ctx, task) {
			atomic.AddInt32(&wp.parallel, -1)
			wp.mu.Unlock()
			logger.Warn(ctx, "submit work pool task failed", "reason", "queue_push_failed")
			return nil, ErrPoolFull
		}
		// Notify only wakes sleeping workers; once awake, CAS workers keep
		// draining the queue. A bounded notification channel therefore does not
		// bound the number of accepted tasks.
		select {
		case wp.Notify <- struct{}{}:
		default:
		}
		wp.mu.Unlock()
		logger.Debug(ctx, "work pool task queued", "queued", queued, "queue_mode", "cas")
		return task, nil
	}

	select {
	case wp.tasks <- task:
		wp.mu.Unlock()
		logger.Debug(ctx, "work pool task queued", "queued", queued, "queue_mode", "channel")
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
		if wp.QueueMode == QueueModeCAS {
			task, ok := wp.nextCASTask()
			if !ok {
				return
			}
			atomic.AddInt32(&wp.parallel, -1)
			if wp.isStopped() {
				task.complete(nil, ErrPoolClosed)
				return
			}
			wp.execute(workerCtx, task)
			continue
		}

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

func (wp *WorkPool) nextCASTask() (*Task, bool) {
	for {
		value, _ := wp.TaskQ.PopFront(wp.ctx)
		if value != nil {
			task, ok := value.(*Task)
			if !ok {
				logger.Error(wp.ctx, "work pool CAS queue contains invalid task")
				continue
			}
			return task, true
		}

		select {
		case <-wp.ctx.Done():
			return nil, false
		case <-wp.Notify:
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
		if wp.QueueMode == QueueModeCAS {
			for {
				value, _ := wp.TaskQ.PopFront(ctx)
				if value == nil {
					wp.mu.Unlock()
					logger.Debug(ctx, "work pool stopped")
					return
				}
				task, ok := value.(*Task)
				if !ok {
					continue
				}
				atomic.AddInt32(&wp.parallel, -1)
				task.complete(nil, ErrPoolClosed)
			}
		}

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
