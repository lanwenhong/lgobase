package cluster_dlock

import (
	"context"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/redis/go-redis/v9"
)

const (
	KEY_EX_TIME   int64 = 300
	KEY_WAIT_TIME int32 = 100
	KEY_CHECK_NUM int   = 3
)

type Dlock struct {
	Key   string
	Ltime int64
	Rdb   *redis.ClusterClient
}

func DlockNew(rdb *redis.ClusterClient, lkey string) *Dlock {
	return &Dlock{
		Key: lkey,
		Rdb: rdb,
	}
}

func (dl *Dlock) Lock(ctx context.Context) error {
	for {
		t := time.Now().UnixNano()
		//st := fmt.Sprintf("%d", t)
		logger.Debugf(ctx, "try lock")
		//ret, err := dl.Rrp.Do("set", dl.Key, t, "nx", "px", KEY_EX_TIME)
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

func (dl *Dlock) Unlock(ctx context.Context) error {
	ret, err := dl.Rdb.Get(ctx, dl.Key).Int64()
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
