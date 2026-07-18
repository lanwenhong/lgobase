package gpool

// idleQueue is a fixed-capacity FIFO ring for idle connections. It is not
// thread-safe; Gpool protects every operation with its mutex.
type idleQueue[T any] struct {
	items []*PoolConn[T]
	head  int
	tail  int
	size  int
}

func newIdleQueue[T any](capacity int) idleQueue[T] {
	if capacity < 0 {
		capacity = 0
	}
	return idleQueue[T]{
		items: make([]*PoolConn[T], capacity),
	}
}

func (q *idleQueue[T]) Len() int {
	return q.size
}

func (q *idleQueue[T]) Cap() int {
	return len(q.items)
}

func (q *idleQueue[T]) Push(pc *PoolConn[T]) bool {
	if pc == nil || q.size == len(q.items) {
		return false
	}

	q.items[q.tail] = pc
	q.tail++
	if q.tail == len(q.items) {
		q.tail = 0
	}
	q.size++
	return true
}

func (q *idleQueue[T]) Pop() *PoolConn[T] {
	if q.size == 0 {
		return nil
	}

	pc := q.items[q.head]
	q.items[q.head] = nil
	q.head++
	if q.head == len(q.items) {
		q.head = 0
	}
	q.size--
	return pc
}
