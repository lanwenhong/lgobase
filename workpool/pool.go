package workpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

type Process func(ctx context.Context, req any) (any, error)

type WorkPool struct {
	TaskQ    cas.Queue
	PoolSize int32
	parallel int32
	Notify   chan struct{}
	Wg       sync.WaitGroup
	Cancel   context.CancelFunc
}

type Task struct {
	TaskId  string
	Req     any
	Ret     any
	Wait    chan struct{}
	process Process
}

func NewWorkPool(poolSize int) *WorkPool {
	wp := &WorkPool{
		PoolSize: int32(poolSize),
		TaskQ:    cas.CreateCasQueue(),
		Notify:   make(chan struct{}, poolSize),
		Wg:       sync.WaitGroup{},
	}
	return wp
}

func (task *Task) WaitRet(ctx context.Context) (any, error) {
	<-task.Wait
	return task.Ret, nil
}

func (wp *WorkPool) AddTask(ctx context.Context, req any, process Process) (*Task, error) {
	if atomic.LoadInt32(&wp.parallel) == wp.PoolSize {
		logger.Warnf(ctx, "work pool full")
		return nil, errors.New("work pool full")
	}
	req_id := ""
	if m := ctx.Value("request_id"); m != nil {
		if value, ok := m.(string); ok {
			req_id = value
		}
	} else {
		req_id = util.NewRequestID()
	}
	task := &Task{
		TaskId:  req_id,
		Req:     req,
		Wait:    make(chan struct{}, 1),
		process: process,
	}
	logger.Debugf(ctx, "add task")
	wp.TaskQ.PushBack(ctx, task)
	wp.Notify <- struct{}{}
	atomic.AddInt32(&wp.parallel, 1)
	logger.Debugf(ctx, "after push parallel: %d", atomic.LoadInt32(&wp.parallel))
	return task, nil
}

func (wp *WorkPool) do(ctx context.Context) {
	for {
		t, _ := wp.TaskQ.PopFront(ctx)
		if t == nil {
			logger.Debugf(ctx, "get nil from queue")
			break
		}
		atomic.AddInt32(&wp.parallel, -1)
		task := t.(*Task)
		ctx = context.WithValue(ctx, "request_id", task.TaskId)
		logger.Debugf(ctx, "after pop parallel: %d", atomic.LoadInt32(&wp.parallel))
		var err error
		task.Ret, err = task.process(ctx, task.Req)
		if err != nil {
			logger.Warnf(ctx, "task execute err: %v", err)
		}
		logger.Debugf(ctx, "task ret: %v", task.Ret)
		task.Wait <- struct{}{}
		close(task.Wait)
	}
}

func (wp *WorkPool) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	wp.Cancel = cancel
	for i := 0; i < int(wp.PoolSize); i++ {
		wp.Wg.Add(1)
		go func(ctx context.Context) {
			ctx = context.WithValue(ctx, "trace_id", util.NewRequestID())
			//ctx = context.Background()
			defer wp.Wg.Done()
			for {
				select {
				case <-wp.Notify:
					wp.do(ctx)
				case <-ctx.Done():
					logger.Debugf(ctx, "process exit")
					return
				}
			}
		}(ctx)
	}
}

func (wp *WorkPool) Join(ctx context.Context) {
	wp.Wg.Wait()
	logger.Debugf(ctx, "wait all process")
}

func (wp *WorkPool) Kill(ctx context.Context) {
	wp.Cancel()
}
