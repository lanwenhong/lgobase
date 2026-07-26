package cas

import (
	"context"
	"sync/atomic"
	"unsafe"

	"github.com/lanwenhong/lgobase/logger"
)

type Queue interface {
	PushBack(ctx context.Context, val interface{}) bool
	PopFront(ctx context.Context) (interface{}, bool)
	Print(ctx context.Context) int64
}
type CasQueue struct {
	Head unsafe.Pointer
	Tail unsafe.Pointer
}

type Item struct {
	next unsafe.Pointer
	V    unsafe.Pointer
}

func CreateCasQueue() Queue {
	q := &CasQueue{}
	node := unsafe.Pointer(&Item{nil, nil})
	q.Head = node
	q.Tail = node
	return q
}

func (q *CasQueue) PushBack(ctx context.Context, val interface{}) bool {
	node := &Item{
		next: nil,
		V:    unsafe.Pointer(&val),
	}

	for {
		tail := atomic.LoadPointer(&q.Tail)
		tailItem := (*Item)(tail)
		next := atomic.LoadPointer(&tailItem.next)
		if tail != atomic.LoadPointer(&q.Tail) {
			continue
		}
		if next != nil {
			atomic.CompareAndSwapPointer(&q.Tail, tail, next)
			continue
		}
		if atomic.CompareAndSwapPointer(&tailItem.next, nil, unsafe.Pointer(node)) {
			atomic.CompareAndSwapPointer(&q.Tail, tail, unsafe.Pointer(node))
			return true
		}
	}
}

func (q *CasQueue) PopFront(ctx context.Context) (interface{}, bool) {
	for {
		head := atomic.LoadPointer(&q.Head)
		tail := atomic.LoadPointer(&q.Tail)
		headItem := (*Item)(head)
		next := atomic.LoadPointer(&headItem.next)
		if head != atomic.LoadPointer(&q.Head) {
			continue
		}
		if head == tail {
			if next == nil {
				return nil, true
			}
			atomic.CompareAndSwapPointer(&q.Tail, tail, next)
			continue
		}

		value := *((*interface{})((*Item)(next).V))
		if atomic.CompareAndSwapPointer(&q.Head, head, next) {
			return value, true
		}
	}
}

func (q *CasQueue) Print(ctx context.Context) int64 {
	p := (*Item)(atomic.LoadPointer(&q.Head))
	var i int64
	for {
		next := atomic.LoadPointer(&p.next)
		if next == nil {
			return i
		}
		logger.Debug(ctx, "queue item visited", "value", *((*interface{})((*Item)(next).V)))
		//fmt.Println(*((*interface{})((*Item)(p.next).V)))
		p = (*Item)(next)
		i++
	}
}
