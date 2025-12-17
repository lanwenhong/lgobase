package workpool

import (
	"context"
	"sync"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

type Process func(ctx context.Context, req any) (any, error)

type WorkPool struct {
	TaskQ    cas.Queue
	PoolSize int
	parallel int
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
		PoolSize: poolSize,
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
	trace_id := ""
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			trace_id = value
		}
	} else {
		trace_id = util.NewRequestID()
	}
	task := &Task{
		TaskId:  trace_id,
		Req:     req,
		Wait:    make(chan struct{}, 1),
		process: process,
	}
	logger.Debugf(ctx, "add task")
	wp.TaskQ.PushBack(ctx, task)
	wp.Notify <- struct{}{}
	return task, nil
}

func (wp *WorkPool) do(ctx context.Context) {
	for {
		t, _ := wp.TaskQ.PopFront(ctx)
		if t == nil {
			//logger.Debugf(ctx, "get nil from queue")
			break
		}
		task := t.(*Task)
		ctx = context.WithValue(ctx, "trace_id", task.TaskId)
		var err error
		task.Ret, err = task.process(ctx, task.Req)
		if err != nil {
			logger.Warnf(ctx, "task execute err: %v", err)
		}
		task.Wait <- struct{}{}
		close(task.Wait)
	}
}

func (wp *WorkPool) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	wp.Cancel = cancel
	for i := 0; i < wp.PoolSize; i++ {
		wp.Wg.Add(1)
		go func(ctx context.Context) {
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
