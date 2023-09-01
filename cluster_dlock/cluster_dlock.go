package cluster_dlock

import (
	"context"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/redis/go-redis/v9"
)

type RedisClients interface {
	*redis.ClusterClient | *redis.Client
	redis.Cmdable
}

type NDlock[T RedisClients] struct {
	Key   string
	Ltime int64
	Rdb   T
}

func NDlockNew[T RedisClients](ctx context.Context, rdb T, lkey string) *NDlock[T] {
	return &NDlock[T]{
		Key: lkey,
		Rdb: rdb,
	}
}

func (dl *NDlock[T]) NLock(ctx context.Context) error {
	for {
		t := time.Now().UnixNano()
		logger.Debugf(ctx, "try lock")
		ret, err := dl.Rdb.SetNX(ctx, dl.Key, t, time.Duration(KEY_EX_TIME)*time.Millisecond).Result()
		if err != nil {
			logger.Warn(ctx, err.Error())
			return err
		} else {
			if ret {
				//set lock start time
				dl.Ltime = t
				break
			} else {
				//sleep wait unlock
				logger.Debug(ctx, "wait")
				time.Sleep(time.Duration(KEY_WAIT_TIME) * time.Millisecond)
			}
		}
	}
	return nil
}

func (dl *NDlock[T]) NUnlock(ctx context.Context) error {
	var ret int64
	var err error
	ret, err = dl.Rdb.Get(ctx, dl.Key).Int64()
	if err != nil && err != redis.Nil {
		logger.Debug(ctx, err.Error())
		return err
	}
	if err == redis.Nil {
		logger.Debug(ctx, "timeout")
		return nil
	}

	etime := time.Now().UnixNano()
	logger.Debugf(ctx, "etime: %d lock_time: %d redis_time: %d", etime, dl.Ltime, ret)
	if etime-dl.Ltime > KEY_EX_TIME*1000*1000 && etime-ret < KEY_EX_TIME*1000*1000 {
		logger.Debugf(ctx, "timout and found new lock")
		return nil
	}
	ret, err = dl.Rdb.Del(ctx, dl.Key).Result()
	if err != nil {
		logger.Warn(ctx, err.Error())
		return err
	}
	logger.Debug(ctx, "unlock ret: %d", ret)
	return nil
}
