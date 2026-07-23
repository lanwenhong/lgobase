package cas_test

import (
	"context"
	"sync"
	"testing"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func TestQPush(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	q := cas.CreateCasQueue()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < 100; i++ {
			q.PushBack(ctx, 1)
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			q.PushBack(ctx, 2)
		}
		wg.Done()
	}()
	wg.Wait()

	c := q.Print(ctx)
	logger.Debug(ctx, "CAS queue test count", "count", c)
}

/*func TestPushPop(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	use_list := cas.NewCASDoubleLinkedList()
	//free_list := cas.NewCASDoubleLinkedList()

	for i := 0; i < 1000000; i++ {
		use_list.PushFront(ctx, i)
	}
	//logger.Debugf(ctx, "len: %d", use_list.Length())
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			ctx := context.WithValue(context.Background(), "trace_id", uuid.New().String())
			defer wg.Done()
			for {
				e := use_list.PopFront(ctx)
				if e != nil {
					logger.Debug(ctx, "CAS queue test popped item", "item", e)
					//free_list.PushFront(ctx, e.Value)
				} else {
					logger.Debug(ctx, "CAS queue test found unexpected item", "item", e)
					break
				}
			}
		}()
	}
	wg.Wait()
}

func Test2Push(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	use_list := cas.NewCASDoubleLinkedList()

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			ctx := context.WithValue(context.Background(), "trace_id", uuid.New().String())
			defer wg.Done()
			for i := 0; i < 10; i++ {
				use_list.PushFront(ctx, i)
			}
		}()
	}

	wg.Wait()
	for {
		e := use_list.PopFront(ctx)
		if e != nil {
			logger.Debug(ctx, "CAS queue benchmark popped item", "item", e)
		} else {
			break
		}

	}
}*/
