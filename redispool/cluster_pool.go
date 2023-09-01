package redispool

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

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
