package workpool

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
	"github.com/lanwenhong/lgobase/workpool"
)

func TestConsumer(t *testing.T) {
	ctx := context.Background()
	wp := workpool.NewWorkPool(3)
	wp.Run(ctx)

	for i := 0; i < 10; i++ {
		ctx := context.WithValue(ctx, "trace_id", util.NewRequestID())
		task, err := wp.AddTask(ctx, 1, func(ctx context.Context, req any) (any, error) {
			a := req.(int)
			a += 1
			return a, nil
		})

		if err != nil {
			t.Fatal(err)
		}

		ret, _ := task.WaitRet(ctx)
		logger.Debugf(ctx, "ret: %v", ret)
	}

	wp.Kill(ctx)
	wp.Join(ctx)

}
