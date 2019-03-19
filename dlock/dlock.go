package dlock

import (
	"github.com/garyburd/redigo/redis"
	"github.com/lanwenhong/goqfpay/logger"
	"github.com/lanwenhong/goqfpay/redispool"
	"github.com/satori/go.uuid"
	"time"
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
}

func DlockNew(rrp *redispool.RedisPool, lkey string) (dlock *Dlock, err error) {
	dlock = new(Dlock)
	dlock.Key = lkey
	dlock.Rrp = rrp

	dlock.Flag = uuid.NewV4().String()
	err = nil

	return dlock, err
}

func (dl *Dlock) Lock() error {
	for {
		t := time.Now().UnixNano()
		//st := fmt.Sprintf("%d", t)
		logger.Debugf("%s try lock", dl.Flag)
		ret, err := dl.Rrp.Do("set", dl.Key, t, "nx", "px", KEY_EX_TIME)
		if err != nil {
			return err
		} else {
			st, _ := redis.String(ret, err)
			logger.Debugf("flag %s try lock st: %s", dl.Flag, st)
			if st == "OK" {
				//set lock start time
				dl.Ltime = t
				break
			} else {
				//sleep wait unlock
				logger.Debugf("%s wait", dl.Flag)
				time.Sleep(time.Duration(KEY_WAIT_TIME) * time.Millisecond)
			}
		}
	}
	return nil
}

func (dl *Dlock) Unlock() error {
	ret, err := dl.Rrp.Do("get", dl.Key)
	if err != nil {
		return err
	}
	set_t, _ := redis.Int64(ret, err)
	etime := time.Now().UnixNano()
	logger.Debugf("%s etime: %d lock_time: %d redis_time: %d", dl.Flag, etime, dl.Ltime, set_t)
	if etime-dl.Ltime > KEY_EX_TIME*1000*1000 && etime-set_t < KEY_EX_TIME*1000*1000 {
		logger.Debugf("%s timout and found new lock", dl.Flag)
		return nil
	} else if set_t == 0 {
		logger.Debugf("---%s timeout", dl.Flag)
		return nil
	}
	logger.Debugf("%s release", dl.Flag)
	_, err = dl.Rrp.Do("del", dl.Key)
	return err
}
