package redispool

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClusterPool(ctx context.Context, addrs []string, PoolSize int, MinIdleConns int,
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
	})
	return rdb
}
