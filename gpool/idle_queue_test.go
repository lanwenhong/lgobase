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
}
