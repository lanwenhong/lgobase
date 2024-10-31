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
