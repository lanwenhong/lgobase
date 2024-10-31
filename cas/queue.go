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

	p := (*Item)(q.Tail)
	oldp := p

	for p.next != nil {
		p = (*Item)(p.next)
	}
	for !atomic.CompareAndSwapPointer(&p.next, nil, unsafe.Pointer(node)) {
		for p.next != nil {
			p = (*Item)(p.next)
		}
	}
	atomic.CompareAndSwapPointer(&q.Tail, unsafe.Pointer(oldp), unsafe.Pointer(node))
	return true
}

func (q *CasQueue) PopFront(ctx context.Context) (interface{}, bool) {
	p := (*Item)(q.Head)
	if p.next == nil {
		return nil, true
	}
	for !atomic.CompareAndSwapPointer(&q.Head, unsafe.Pointer(p), unsafe.Pointer(p.next)) {
		p = (*Item)(q.Head)
		if p.next == nil {
			return nil, true
		}
	}
	return *((*interface{})(((*Item)(p.next)).V)), true
}

func (q *CasQueue) Print(ctx context.Context) int64 {
	p := (*Item)(q.Head)
	var i int64
	for p.next != nil {
		logger.Debugf(ctx, "%v", *((*interface{})((*Item)(p.next).V)))
		//fmt.Println(*((*interface{})((*Item)(p.next).V)))
		p = (*Item)(p.next)
		i++
	}
	return i
}
