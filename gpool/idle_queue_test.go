package gpool

import "testing"

func TestIdleQueueFIFOAndWrapAround(t *testing.T) {
	queue := newIdleQueue[internalTestClient](3)
	first := &PoolConn[internalTestClient]{}
	second := &PoolConn[internalTestClient]{}
	third := &PoolConn[internalTestClient]{}
	fourth := &PoolConn[internalTestClient]{}

	if !queue.Push(first) || !queue.Push(second) || !queue.Push(third) {
		t.Fatal("push into queue with available capacity failed")
	}
	if queue.Push(fourth) {
		t.Fatal("push into full queue succeeded")
	}
	if got := queue.Pop(); got != first {
		t.Fatalf("first pop = %p, want %p", got, first)
	}
	if !queue.Push(fourth) {
		t.Fatal("push after head advanced failed")
	}

	for i, want := range []*PoolConn[internalTestClient]{second, third, fourth} {
		if got := queue.Pop(); got != want {
			t.Fatalf("pop %d = %p, want %p", i, got, want)
		}
	}
	if got := queue.Pop(); got != nil {
		t.Fatalf("pop from empty queue = %p, want nil", got)
	}
	if queue.Len() != 0 {
		t.Fatalf("queue length = %d, want 0", queue.Len())
	}
	for i, item := range queue.items {
		if item != nil {
			t.Fatalf("slot %d still retains connection %p", i, item)
		}
	}
}

func TestIdleQueueZeroCapacity(t *testing.T) {
	queue := newIdleQueue[internalTestClient](0)
	if queue.Cap() != 0 || queue.Len() != 0 {
		t.Fatalf("zero-capacity queue has cap=%d len=%d", queue.Cap(), queue.Len())
	}
	if queue.Push(&PoolConn[internalTestClient]{}) {
		t.Fatal("push into zero-capacity queue succeeded")
	}
	if got := queue.Pop(); got != nil {
		t.Fatalf("pop from zero-capacity queue = %p, want nil", got)
	}
	if got := queue.PopNewest(); got != nil {
		t.Fatalf("pop newest from zero-capacity queue = %p, want nil", got)
	}
}

func TestIdleQueueLIFOAndOldestEvictionAfterWrapAround(t *testing.T) {
	queue := newIdleQueue[internalTestClient](3)
	first := &PoolConn[internalTestClient]{}
	second := &PoolConn[internalTestClient]{}
	third := &PoolConn[internalTestClient]{}
	fourth := &PoolConn[internalTestClient]{}
	fifth := &PoolConn[internalTestClient]{}

	if !queue.Push(first) || !queue.Push(second) || !queue.Push(third) {
		t.Fatal("push into queue with available capacity failed")
	}
	if got := queue.Pop(); got != first {
		t.Fatalf("oldest pop = %p, want %p", got, first)
	}
	if !queue.Push(fourth) {
		t.Fatal("push after head advanced failed")
	}
	// The tail wrapped to zero. LIFO must still return the last item inserted.
	if got := queue.PopNewest(); got != fourth {
		t.Fatalf("newest pop after wrap = %p, want %p", got, fourth)
	}
	if !queue.Push(fifth) || queue.Push(first) {
		t.Fatal("queue did not enforce its capacity after a newest pop")
	}
	if got := queue.Pop(); got != second {
		t.Fatalf("oldest eviction after newest pop = %p, want %p", got, second)
	}
	for i, want := range []*PoolConn[internalTestClient]{fifth, third} {
		if got := queue.PopNewest(); got != want {
			t.Fatalf("newest pop %d = %p, want %p", i, got, want)
		}
	}
	if queue.Len() != 0 || queue.PopNewest() != nil || queue.Pop() != nil {
		t.Fatal("queue was not empty after draining")
	}
	for i, item := range queue.items {
		if item != nil {
			t.Fatalf("slot %d still retains connection %p", i, item)
		}
	}
	if !queue.Push(first) || queue.PopNewest() != first {
		t.Fatal("queue could not be reused after draining")
	}
}
