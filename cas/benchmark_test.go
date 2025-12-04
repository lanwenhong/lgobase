package cas

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/util"
)

func BenchmarkQParallel(b *testing.B) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	q := cas.CreateCasQueue()
	for n := 0; n < b.N; n++ {
		q.PushBack(ctx, 1)
	}
}

func Benchmark2QParallel(b *testing.B) {
	b.ReportAllocs()
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	q := cas.CreateCasQueue()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.PushBack(ctx, 1)
			//q.PopFront(ctx)
		}
	})
}

/*func BenchmarkCASDoubleLinkedList_PushFront(b *testing.B) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	list := cas.NewCASDoubleLinkedList()
	b.ResetTimer() // 重置计时器（忽略前面的初始化耗时）

	// 循环 b.N 次：Go 会自动调整 b.N 确保测试时间足够（默认 1 秒）
	for i := 0; i < b.N; i++ {
		list.PushFront(ctx, i)
	}
}*/

/*func BenchmarkCASDoubleLinkedList_PopPush(b *testing.B) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	use_list := cas.NewCASDoubleLinkedList()
	free_list := cas.NewCASDoubleLinkedList()

	for i := 0; i < 1000000; i++ {
		use_list.PushFront(ctx, i)
	}
	b.ResetTimer() // 重置计时器（忽略前面的初始化耗时）
	// 循环 b.N 次：Go 会自动调整 b.N 确保测试时间足够（默认 1 秒）
	for i := 0; i < b.N; i++ {
		e, _ := use_list.PopFront(ctx)
		//logger.Debugf(ctx, "e: %v", e)
		if e != nil {
			free_list.PushFront(ctx, e.Value)
		}
	}
}

func BenchmarkParallelCASDoubleLinkedList_PopPush(b *testing.B) {
	b.ReportAllocs()
	ctx := context.WithValue(context.Background(), "trace_id", util.GenUlid())
	use_list := cas.NewCASDoubleLinkedList()
	free_list := cas.NewCASDoubleLinkedList()
	for i := 0; i < 100000; i++ {
		use_list.PushFront(ctx, i)
	}

	b.ResetTimer() // 重置计时器（忽略前面的初始化耗时）
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e, _ := use_list.PopFront(ctx)
			if e != nil {
				free_list.PushFront(ctx, e.Value)
			}

		}
	})
}*/
