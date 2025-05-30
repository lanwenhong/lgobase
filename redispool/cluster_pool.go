package redispool

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CLUSTER_TYPE = "CLUSTER"
	MASTER_SLAVE = "MASTER_SLAVE"
)

type RedisMethod interface {
	*redis.ClusterClient | *redis.Client
	redis.Cmdable
	//redis.SortedSetCmdable
}

type RedisOP[T RedisMethod] struct {
	Rdb T
}

type RedisNode interface {
	redis.Cmdable
	redis.BitMapCmdable
}

type GredisConf struct {
	Username     string
	Passwd       string
	Addrs        []string
	PoolSize     int
	MinIdleConns int
	Db           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type GredisClient struct {
	useType       string
	ClusterClient *redis.ClusterClient
	Client        *redis.Client
}

func NewRedisOP[T RedisMethod](client T) *RedisOP[T] {
	return &RedisOP[T]{
		Rdb: client,
	}
}

func NewGredis(conf *GredisConf, use_type string) *GredisClient {
	grc := &GredisClient{
		useType: use_type,
	}
	if grc.useType == CLUSTER_TYPE {
		grc.ClusterClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        conf.Addrs,
			PoolSize:     conf.PoolSize,
			DialTimeout:  conf.DialTimeout,
			ReadTimeout:  conf.ReadTimeout,
			WriteTimeout: conf.WriteTimeout,
			Username:     conf.Username,
			Password:     conf.Passwd,
			MinIdleConns: conf.MinIdleConns,
		})
	} else if grc.useType == MASTER_SLAVE {
		grc.Client = redis.NewClient(&redis.Options{
			Addr:         conf.Addrs[0],
			PoolSize:     conf.PoolSize,
			DialTimeout:  conf.DialTimeout,
			ReadTimeout:  conf.ReadTimeout,
			WriteTimeout: conf.WriteTimeout,
			Username:     conf.Username,
			Password:     conf.Passwd,
			MinIdleConns: conf.MinIdleConns,
			DB:           conf.Db,
		})
	}
	return grc
}

func (grc *GredisClient) GetRedisClient() (RedisNode, error) {
	switch grc.useType {
	case CLUSTER_TYPE:
		return grc.ClusterClient, nil
	case MASTER_SLAVE:
		return grc.Client, nil
	default:
		return nil, fmt.Errorf("redis type '%s' is not supported", grc.useType)
	}
}

func NewClusterPool(ctx context.Context, username string, passwd string, addrs []string, PoolSize int, MinIdleConns int,
	DialTimeout time.Duration,
	ReadTimeout time.Duration,
	WriteTimeout time.Duration,
) *redis.ClusterClient {
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        addrs,
		PoolSize:     PoolSize,
		DialTimeout:  DialTimeout,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		Username:     username,
		Password:     passwd,
	})
	return rdb
}

func NewGrPool(ctx context.Context, username string, passwd string, db int,
	addr string,
	PoolSize int,
	MinIdleConns int,
	DialTimeout time.Duration,
	ReadTimeout time.Duration,
	WriteTimeout time.Duration,
) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     PoolSize,
		DialTimeout:  DialTimeout,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		Username:     username,
		Password:     passwd,
		DB:           db,
	})
	return rdb
}
