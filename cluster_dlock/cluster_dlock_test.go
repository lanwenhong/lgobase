package cluster_dlock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/redispool"
	"github.com/redis/go-redis/v9"
)

var _ RedisClient = (redispool.RedisNode)(nil)

type fakeRedis struct {
	mu        sync.Mutex
	now       time.Time
	key       string
	value     string
	expiresAt time.Time
	attempts  int
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{now: time.Unix(1, 0)}
}

func (r *fakeRedis) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewBoolResult(false, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked()
	r.attempts++
	if r.value != "" {
		return redis.NewBoolResult(false, nil)
	}

	token, ok := value.(string)
	if !ok {
		return redis.NewBoolResult(false, fmt.Errorf("unexpected lock value type %T", value))
	}
	r.key = key
	r.value = token
	r.expiresAt = r.now.Add(expiration)
	return redis.NewBoolResult(true, nil)
}

func (r *fakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if err := ctx.Err(); err != nil {
		return redis.NewCmdResult(nil, err)
	}
	if len(keys) != 1 || len(args) == 0 {
		return redis.NewCmdResult(nil, errors.New("invalid script arguments"))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked()

	token := fmt.Sprint(args[0])
	switch script {
	case unlockScript:
		if r.value == "" {
			return redis.NewCmdResult(int64(0), nil)
		}
		if r.value != token {
			return redis.NewCmdResult(int64(-1), nil)
		}
		r.key = ""
		r.value = ""
		r.expiresAt = time.Time{}
		return redis.NewCmdResult(int64(1), nil)

	case extendScript:
		if len(args) != 2 {
			return redis.NewCmdResult(nil, errors.New("invalid extend arguments"))
		}
		if r.value == "" || r.value != token {
			return redis.NewCmdResult(int64(0), nil)
		}
		ttlMillis, ok := args[1].(int64)
		if !ok {
			return redis.NewCmdResult(nil, fmt.Errorf("unexpected TTL type %T", args[1]))
		}
		r.expiresAt = r.now.Add(time.Duration(ttlMillis) * time.Millisecond)
		return redis.NewCmdResult(int64(1), nil)

	default:
		return redis.NewCmdResult(nil, errors.New("unknown script"))
	}
}

func (r *fakeRedis) advance(duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = r.now.Add(duration)
	r.expireLocked()
}

func (r *fakeRedis) currentValue() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked()
	return r.value
}

func (r *fakeRedis) attemptCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

func (r *fakeRedis) expireLocked() {
	if r.value != "" && !r.now.Before(r.expiresAt) {
		r.key = ""
		r.value = ""
		r.expiresAt = time.Time{}
	}
}

func TestExpiredOwnerCannotDeleteNewOwner(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	ownerA := DlockNew(ctx, rdb, "lock:test", WithTTL(100*time.Millisecond))
	ownerB := DlockNew(ctx, rdb, "lock:test", WithTTL(100*time.Millisecond))

	if err := ownerA.Lock(ctx); err != nil {
		t.Fatalf("owner A Lock() error = %v", err)
	}
	rdb.advance(101 * time.Millisecond)
	if err := ownerB.Lock(ctx); err != nil {
		t.Fatalf("owner B Lock() error = %v", err)
	}
	ownerBToken := rdb.currentValue()

	if err := ownerA.Unlock(ctx); !errors.Is(err, ErrLockLost) {
		t.Fatalf("expired owner Unlock() error = %v, want ErrLockLost", err)
	}
	if got := rdb.currentValue(); got != ownerBToken {
		t.Fatalf("expired owner deleted new lock: got token %q, want %q", got, ownerBToken)
	}
	if err := ownerB.Unlock(ctx); err != nil {
		t.Fatalf("owner B Unlock() error = %v", err)
	}
}

func TestRepeatedUnlockCannotDeleteNewOwner(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	ownerA := DlockNew(ctx, rdb, "lock:test")
	ownerB := DlockNew(ctx, rdb, "lock:test")

	if err := ownerA.Lock(ctx); err != nil {
		t.Fatalf("owner A Lock() error = %v", err)
	}
	if err := ownerA.Unlock(ctx); err != nil {
		t.Fatalf("owner A Unlock() error = %v", err)
	}
	if err := ownerB.Lock(ctx); err != nil {
		t.Fatalf("owner B Lock() error = %v", err)
	}
	ownerBToken := rdb.currentValue()

	if err := ownerA.Unlock(ctx); !errors.Is(err, ErrNotLocked) {
		t.Fatalf("repeated Unlock() error = %v, want ErrNotLocked", err)
	}
	if got := rdb.currentValue(); got != ownerBToken {
		t.Fatalf("repeated unlock deleted new lock: got token %q, want %q", got, ownerBToken)
	}
}

func TestLockCancellationInterruptsBackoff(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	owner := DlockNew(ctx, rdb, "lock:test")
	waiter := DlockNew(ctx, rdb, "lock:test", WithRetryBackoff(time.Second, time.Second))

	if err := owner.Lock(ctx); err != nil {
		t.Fatalf("owner Lock() error = %v", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := waiter.Lock(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter Lock() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("context cancellation took %v; backoff sleep was not interrupted", elapsed)
	}
	if attempts := rdb.attemptCount(); attempts != 2 {
		t.Fatalf("SetNX attempts = %d, want 2", attempts)
	}
}

func TestTryLockDoesNotWait(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	owner := DlockNew(ctx, rdb, "lock:test")
	waiter := DlockNew(ctx, rdb, "lock:test")

	if err := owner.Lock(ctx); err != nil {
		t.Fatalf("owner Lock() error = %v", err)
	}
	acquired, err := waiter.TryLock(ctx)
	if err != nil {
		t.Fatalf("TryLock() error = %v", err)
	}
	if acquired {
		t.Fatal("TryLock() acquired a contended lock")
	}
}

func TestExtendRenewsOnlyCurrentOwner(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	owner := DlockNew(ctx, rdb, "lock:test", WithTTL(100*time.Millisecond))
	waiter := DlockNew(ctx, rdb, "lock:test", WithTTL(100*time.Millisecond))

	if err := owner.Lock(ctx); err != nil {
		t.Fatalf("owner Lock() error = %v", err)
	}
	rdb.advance(75 * time.Millisecond)
	if err := owner.Extend(ctx); err != nil {
		t.Fatalf("owner Extend() error = %v", err)
	}

	rdb.advance(50 * time.Millisecond)
	if acquired, err := waiter.TryLock(ctx); err != nil || acquired {
		t.Fatalf("TryLock() after original expiry = (%v, %v), want (false, nil)", acquired, err)
	}

	rdb.advance(51 * time.Millisecond)
	if acquired, err := waiter.TryLock(ctx); err != nil || !acquired {
		t.Fatalf("TryLock() after extended expiry = (%v, %v), want (true, nil)", acquired, err)
	}
	if err := owner.Extend(ctx); !errors.Is(err, ErrLockLost) {
		t.Fatalf("stale owner Extend() error = %v, want ErrLockLost", err)
	}
}

func TestInvalidOptionsReturnError(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeRedis()
	dl := DlockNew(ctx, rdb, "lock:test", WithTTL(0))

	if err := dl.Lock(ctx); err == nil {
		t.Fatal("Lock() with zero TTL returned nil error")
	}
}
