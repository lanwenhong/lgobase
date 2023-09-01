package cluster_dlock

import (
	"context"
	"testing"
	"time"

	cdl "github.com/lanwenhong/lgobase/cluster_dlock"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/redispool"
	"github.com/lanwenhong/lgobase/util"
	"github.com/redis/go-redis/v9"
)

func TestDlock(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	dl := cdl.DlockNew(rdb, "lvhanikezi", true)

	dl.Lock(ctx)
	go func() {
		cctx := context.WithValue(ctx, "trace_id", util.NewRequestID())
		dl := cdl.DlockNew(rdb, "lvhanikezi", true)
		dl.Lock(cctx)
		dl.Unlock(cctx)
	}()
	dl.Unlock(ctx)
	time.Sleep(1 * time.Second)
}

func TestNDlock(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	dl := cdl.NDlockNew[*redis.ClusterClient](ctx, rdb, "ganjujingyi")
	//cdl.NDlockNew[*redis.ClusterClient](ctx, rdb, "lvhanikezi")

	dl.NLock(ctx)
	go func() {
		cctx := context.WithValue(ctx, "trace_id", util.NewRequestID())
		dl := cdl.NDlockNew[*redis.ClusterClient](ctx, rdb, "ganjujingyi")
		dl.NLock(cctx)
		dl.NUnlock(cctx)
	}()
	dl.NUnlock(ctx)
	time.Sleep(1 * time.Second)

}
