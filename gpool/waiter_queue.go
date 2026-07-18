package gpool

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPoolWaitQueueFull = errors.New("gpool waiter queue is full")
	ErrPoolWaitTimeout   = errors.New("gpool waiter timeout")
	ErrPoolClosed        = errors.New("gpool is closed")
)

type waiterState uint8

const (
	waiterQueued waiterState = iota
	waiterDelivered
	waiterCanceled
)

type waitResultKind uint8

const (
	waitResultConn waitResultKind = iota
	waitResultCreate
)

type waitResult[T any] struct {
	kind waitResultKind
	conn *PoolConn[T]
	err  error
}

type poolWaiter[T any] struct {
	prev   *poolWaiter[T]
	next   *poolWaiter[T]
	state  waiterState
	result chan waitResult[T]
}

type waiterQueue[T any] struct {
	head *poolWaiter[T]
	tail *poolWaiter[T]
	size int
}

func (q *waiterQueue[T]) Len() int {
	return q.size
}

func (q *waiterQueue[T]) PushBack(waiter *poolWaiter[T]) {
	waiter.prev = q.tail
	waiter.next = nil
	if q.tail == nil {
		q.head = waiter
	} else {
		q.tail.next = waiter
	}
	q.tail = waiter
	q.size++
}

func (q *waiterQueue[T]) PopFront() *poolWaiter[T] {
	waiter := q.head
	if waiter == nil {
		return nil
	}
	q.Remove(waiter)
	return waiter
}

func (q *waiterQueue[T]) Remove(waiter *poolWaiter[T]) {
	if waiter.prev == nil {
		q.head = waiter.next
	} else {
		waiter.prev.next = waiter.next
	}
	if waiter.next == nil {
		q.tail = waiter.prev
	} else {
		waiter.next.prev = waiter.prev
	}
	waiter.prev = nil
	waiter.next = nil
	q.size--
}

// deliverWaiterLocked transfers ownership of one result to a queued waiter.
// The result channel is buffered and receives exactly one value, so this send
// cannot block. Callers must hold gp.mutex.
func (gp *Gpool[T]) deliverWaiterLocked(waiter *poolWaiter[T], result waitResult[T]) {
	waiter.state = waiterDelivered
	gp.Waits = uint(gp.waiters.Len())
	waiter.result <- result
}

// dispatchCapacityWaitersLocked reserves every currently free connection slot
// for the oldest queued waiters. Callers must hold gp.mutex.
func (gp *Gpool[T]) dispatchCapacityWaitersLocked() {
	for !gp.closed && gp.waiters.Len() > 0 && gp.capacityUsedLocked() < gp.MaxConns {
		waiter := gp.waiters.PopFront()
		gp.creating++
		gp.deliverWaiterLocked(waiter, waitResult[T]{kind: waitResultCreate})
	}
}

// waitForConnectionLocked queues the caller and returns after a connection, a
// reserved create slot, cancellation, or timeout. It expects gp.mutex to be
// locked and always unlocks it before waiting or returning.
func (gp *Gpool[T]) waitForConnectionLocked(ctx context.Context) (*PoolConn[T], error) {
	if gp.closed {
		gp.mutex.Unlock()
		return nil, ErrPoolClosed
	}
	if gp.waiters.Len() >= gp.MaxWaiters {
		gp.mutex.Unlock()
		return nil, ErrPoolWaitQueueFull
	}

	waiter := &poolWaiter[T]{
		state:  waiterQueued,
		result: make(chan waitResult[T], 1),
	}
	gp.waiters.PushBack(waiter)
	gp.Waits = uint(gp.waiters.Len())
	waitTimeout := time.Duration(gp.TimeOut) * time.Millisecond

	// This also handles the case where older waiters exist but capacity became
	// available before the current caller acquired the mutex.
	gp.dispatchCapacityWaitersLocked()
	gp.mutex.Unlock()

	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	select {
	case result := <-waiter.result:
		return gp.handleWaitResult(ctx, result)
	case <-ctx.Done():
		return gp.cancelWaiter(ctx, waiter, ctx.Err())
	case <-timer.C:
		return gp.cancelWaiter(ctx, waiter, ErrPoolWaitTimeout)
	}
}

func (gp *Gpool[T]) cancelWaiter(ctx context.Context, waiter *poolWaiter[T], cancelErr error) (*PoolConn[T], error) {
	gp.mutex.Lock()
	if waiter.state == waiterQueued {
		gp.waiters.Remove(waiter)
		waiter.state = waiterCanceled
		gp.Waits = uint(gp.waiters.Len())
		gp.dispatchCapacityWaitersLocked()
		gp.mutex.Unlock()
		return nil, cancelErr
	}

	// Delivery won the race with cancellation. deliverWaiterLocked puts the
	// result into the buffered channel before releasing the mutex.
	result := <-waiter.result
	gp.mutex.Unlock()
	return gp.handleWaitResult(ctx, result)
}

func (gp *Gpool[T]) handleWaitResult(ctx context.Context, result waitResult[T]) (*PoolConn[T], error) {
	if result.err != nil {
		return nil, result.err
	}

	switch result.kind {
	case waitResultConn:
		return result.conn, nil
	case waitResultCreate:
		gp.mutex.Lock()
		if gp.closed {
			gp.creating--
			gp.finishCloseLocked()
			gp.mutex.Unlock()
			return nil, ErrPoolClosed
		}
		gp.mutex.Unlock()

		pc, err := gp.getConnFromNew(ctx)
		gp.mutex.Lock()
		gp.creating--
		if gp.closed {
			if pc != nil {
				gp.closeConnectionLocked(ctx, pc)
			} else {
				gp.finishCloseLocked()
			}
			gp.mutex.Unlock()
			return nil, ErrPoolClosed
		}
		if pc != nil {
			pc.state.Store(uint32(poolConnBorrowed))
			gp.inUse++
		} else {
			gp.dispatchCapacityWaitersLocked()
		}
		gp.mutex.Unlock()
		return pc, err
	default:
		return nil, errors.New("gpool invalid waiter result")
	}
}
