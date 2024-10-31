package cas

import (
	"context"
	"sync"
	"testing"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func TestQPush(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		ColorFull:    true,
		Stdout:       true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
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
	logger.Debugf(ctx, "%d", c)
}
