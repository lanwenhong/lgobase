package redispool

import (
	"context"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/redispool"
	"github.com/lanwenhong/lgobase/util"
	"github.com/redis/go-redis/v9"
)

func TestClusterConfig(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "", "", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	pong, err := rdb.Ping(ctx).Result()
	t.Log(pong)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClusterLPush(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	n, err := rdb.LPush(ctx, "I love", "liushishi").Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	n, err = rdb.LPush(ctx, "I love", "jujingyi").Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)
	n, err = rdb.LPush(ctx, "I love", "chenduling").Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)
	tm, err := rdb.TTL(ctx, "I love").Result()
	t.Log(err)
	t.Log(tm)
	if tm < 1*time.Nanosecond {
		t.Log("xxx")
	}
}

func TestClusterLRange(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	ret, err := rdb.LRange(ctx, "I love", 0, 3).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestClusterZAdd(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)

	lls := []redis.Z{
		redis.Z{Score: 90.0, Member: "java"},
		redis.Z{Score: 80.0, Member: "go"},
		redis.Z{Score: 70.0, Member: "python"},
		redis.Z{Score: 60.0, Member: "php"},
		redis.Z{Score: 50.0, Member: "ruby"},
	}

	l1 := redis.Z{
		Score:  100,
		Member: "rust",
	}
	l2 := redis.Z{
		Score:  55.5,
		Member: "cpp",
	}
	l3 := redis.Z{
		Score:  56.1,
		Member: "c",
	}

	l4 := redis.Z{
		Score:  15.1,
		Member: "swift",
	}

	l5 := redis.Z{
		Score:  16.1,
		Member: "erlang",
	}
	n, err := rdb.ZAdd(ctx, "lan", lls...).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	n, err = rdb.ZAdd(ctx, "lan", l1).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	n, err = rdb.ZAdd(ctx, "lan", l2).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)
	n, err = rdb.ZAdd(ctx, "lan", l3).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	n, err = rdb.ZAddNX(ctx, "lan", l4).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	n, err = rdb.ZAddNX(ctx, "lan", l5).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n)

	x, err := rdb.ZScore(ctx, "lan", "c").Result()
	/*if err != nil {
		t.Fatal(err)
	}*/
	t.Log(err)
	if err == redis.Nil {
		t.Log("ffffffff")
		t.Log(x)
	}
}

func TestClusterZRangeByScore(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)

	opt := redis.ZRangeBy{
		Min:    "(1.1",
		Max:    "100",
		Offset: 0,
		Count:  10,
	}
	ret, err := rdb.ZRangeByScoreWithScores(ctx, "lan", &opt).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestClusterSet(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "", "", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)

	ret, err := rdb.SetEx(ctx, "lan1", "gangangan", 2*time.Second).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
	time.Sleep(3 * time.Second)

	ret, err = rdb.Get(ctx, "lan1").Result()
	if err != nil && err == redis.Nil {
		t.Log(err)
	} else {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestClusterSetNX(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "", "", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)
	ret, err := rdb.SetNX(ctx, "lan1", "gangangan", 10*time.Millisecond).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	time.Sleep(1 * time.Second)
	ret, err = rdb.SetNX(ctx, "lan1", "gangangan", 10*time.Millisecond).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

}

func TestRedisOP(t *testing.T) {
	addrs := []string{
		":9001",
		":9002",
		":9003",
		":9004",
		":9005",
		":9006",
	}
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewClusterPool(ctx, "dc", "Abc12345%", addrs, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)

	op := redispool.NewRedisOP[*redis.ClusterClient](rdb)
	pong, err := op.Rdb.Ping(ctx).Result()
	t.Log(pong)
	if err != nil {
		t.Fatal(err)
	}
}

func Test1RedisOP(t *testing.T) {
	addr := ":6379"
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	rdb := redispool.NewGrPool(ctx, "", "", 0, addr, 100, 30,
		10*time.Second,
		30*time.Second,
		30*time.Second,
	)

	op := redispool.NewRedisOP[*redis.Client](rdb)
	pong, err := op.Rdb.Ping(ctx).Result()
	t.Log(pong)
	if err != nil {
		t.Fatal(err)
	}

	c, err := op.Rdb.Get(ctx, "ffff").Int()
	if err != nil {
		t.Log(err)
	}
	t.Log(c)
}
