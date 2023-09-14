package dlock

import (
	"context"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/redispool"
	uuid "github.com/satori/go.uuid"
)

const (
	KEY_EX_TIME   int64 = 300
	KEY_WAIT_TIME int32 = 100
	KEY_CHECK_NUM int   = 3
)

type Dlock struct {
	Key   string
	Ltime int64
	Rrp   *redispool.RedisPool
	Flag  string
	//Ctx   context.Context
}

func DlockNew(rrp *redispool.RedisPool, lkey string) (dlock *Dlock, err error) {
	dlock = new(Dlock)
	dlock.Key = lkey
	dlock.Rrp = rrp

	dlock.Flag = uuid.NewV4().String()
	//dlock.Ctx = ctx
	err = nil

	return dlock, err
}

func (dl *Dlock) Lock(ctx context.Context) error {
	for {
		t := time.Now().UnixNano()
		//st := fmt.Sprintf("%d", t)
		logger.Debugf(ctx, "%s try lock", dl.Flag)
		ret, err := dl.Rrp.Do(ctx, "set", dl.Key, t, "nx", "px", KEY_EX_TIME)
		if err != nil {
			return err
		} else {
			st, _ := redis.String(ret, err)
			logger.Debugf(ctx, "flag %s try lock st: %s", dl.Flag, st)
			if st == "OK" {
				//set lock start time
				dl.Ltime = t
				break
			} else {
				//sleep wait unlock
				logger.Debugf(ctx, "%s wait", dl.Flag)
				time.Sleep(time.Duration(KEY_WAIT_TIME) * time.Millisecond)
			}
		}
	}
	return nil
}

func (dl *Dlock) Unlock(ctx context.Context) error {
	ret, err := dl.Rrp.Do(ctx, "get", dl.Key)
	if err != nil {
		return err
	}
	set_t, _ := redis.Int64(ret, err)
	etime := time.Now().UnixNano()
	logger.Debugf(ctx, "%s etime: %d lock_time: %d redis_time: %d", dl.Flag, etime, dl.Ltime, set_t)
	if etime-dl.Ltime > KEY_EX_TIME*1000*1000 && etime-set_t < KEY_EX_TIME*1000*1000 {
		logger.Debugf(ctx, "%s timout and found new lock", dl.Flag)
		return nil
	} else if set_t == 0 {
		logger.Debugf(ctx, "---%s timeout", dl.Flag)
		return nil
	}
	logger.Debugf(ctx, "%s release", dl.Flag)
	_, err = dl.Rrp.Do(ctx, "del", dl.Key)
	return err
}
