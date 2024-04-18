package redispool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/selector"
)

type RedisPool struct {
	Sl       *selector.Selector
	Plist    []*redis.Pool
	PrintLog bool
}

func NewPoolWithUrl(timeout int, maxidle_conn int, maxactive_conn int, sl *selector.Selector, print bool) *RedisPool {
	rp := new(RedisPool)
	rp.Sl = sl
	rp.Plist = make([]*redis.Pool, len(sl.Slist))

	for i := 0; i < len(sl.Slist); i++ {
		r_addr := sl.Slist[i].(selector.BaseSvr)
		redis_url := r_addr.GetAddr()
		rp.Plist[i] = new(redis.Pool)
		rp.Plist[i].MaxIdle = maxidle_conn
		rp.Plist[i].MaxActive = maxactive_conn
		rp.Plist[i].IdleTimeout = (time.Duration(timeout) / 1000) * time.Second

		rp.Plist[i].Dial = func() (redis.Conn, error) {
			c, err := redis.DialURL(redis_url, redis.DialConnectTimeout(5000*time.Millisecond),
				redis.DialReadTimeout(time.Duration(timeout)*time.Millisecond), redis.DialWriteTimeout(time.Duration(timeout)*time.Millisecond))
			if err != nil {
				fmt.Println(err.Error())
			}
			return c, err
		}
		rp.Plist[i].TestOnBorrow = func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		}
	}
	rp.PrintLog = print
	return rp
}

func NewPool(db int, passwd string, maxidle_conn int, maxactive_conn int, sl *selector.Selector, print bool) *RedisPool {
	rp := new(RedisPool)
	rp.Sl = sl
	rp.Plist = make([]*redis.Pool, len(sl.Slist))
	var passwd_s string
	var db_s string

	passwd_s = fmt.Sprintf("%s:%s@", "", passwd)
	if db >= 0 {
		db_s = fmt.Sprintf("/%d", db)
	} else {
		db_s = ""
	}

	for i := 0; i < len(sl.Slist); i++ {
		r_addr := sl.Slist[i].(selector.BaseSvr)
		ip := r_addr.GetAddr()
		port := r_addr.GetPort()
		addr := fmt.Sprintf("%s:%d", ip, port)
		redis_url := "redis://" + passwd_s + addr + db_s
		fmt.Printf("redis addr: %s\n", redis_url)
		rp.Plist[i] = new(redis.Pool)
		rp.Plist[i].MaxIdle = maxidle_conn
		rp.Plist[i].MaxActive = maxactive_conn
		timeout := time.Duration(r_addr.GetTimeOut())
		rp.Plist[i].IdleTimeout = (timeout / 1000) * time.Second

		rp.Plist[i].Dial = func() (redis.Conn, error) {
			c, err := redis.DialURL(redis_url, redis.DialConnectTimeout(5000*time.Millisecond), redis.DialReadTimeout(timeout*time.Millisecond), redis.DialWriteTimeout(timeout*time.Millisecond))
			if err != nil {
				fmt.Println(err.Error())
			}
			return c, err
		}
		rp.Plist[i].TestOnBorrow = func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		}
	}
	rp.PrintLog = print
	return rp
}

func (pr *RedisPool) RedisPoolRoundRobin(ctx context.Context) *redis.Pool {
	addr := pr.Sl.RoundRobin().(selector.BaseSvr)
	logger.Debugf(ctx, "get addr: %s index:%d", addr.GetAddr(), pr.Sl.Pos)
	getpool := pr.Plist[pr.Sl.Pos]
	return getpool
}

func (pr *RedisPool) Do(ctx context.Context, commandName string, args ...interface{}) (reply interface{}, err error) {
	snow := time.Now()
	smicros := snow.UnixNano() / 1000
	addr := pr.Sl.RoundRobin()
	if addr == nil {
		/*if pr.PrintLog {
			logger.Infof(ctx, "server=redis|addr=%s:%d|cmd=%s|args=%s|time=%d", addr.GetAddr(), addr.GetPort(), commandName, args, time.Now().UnixNano()/1000-smicros)
		}*/
		return nil, errors.New("not valid server")
	}
	//logger.Debugf("get addr: %s index:%d", addr.GetAddr(), pr.Sl.Pos)
	getpool := pr.Plist[pr.Sl.Pos]
	r_addr := addr.(selector.BaseSvr)
	c := getpool.Get()
	defer c.Close()
	reply, err = c.Do(commandName, args...)
	if pr.PrintLog {
		logger.Infof(ctx, "server=redis|addr=%s:%d|cmd=%s|args=%v|time=%d", r_addr.GetAddr(), r_addr.GetPort(), commandName, args, time.Now().UnixNano()/1000-smicros)
	}
	return reply, err
}
