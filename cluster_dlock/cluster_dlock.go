package cluster_dlock

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"sync"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// KEY_EX_TIME and KEY_WAIT_TIME are retained for source compatibility.
	// New code should configure durations with WithTTL and WithRetryBackoff.
	KEY_EX_TIME   int64 = 300
	KEY_WAIT_TIME int32 = 100
	KEY_CHECK_NUM int   = 3

	DefaultTTL           = 30 * time.Second
	DefaultMinRetryDelay = 10 * time.Millisecond
	DefaultMaxRetryDelay = 200 * time.Millisecond
)

var (
	ErrAlreadyLocked = errors.New("dlock instance already holds a lock")
	ErrNotLocked     = errors.New("dlock instance does not hold a lock")
	ErrLockLost      = errors.New("distributed lock ownership lost")
)

const unlockScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return 0
end
if value ~= ARGV[1] then
	return -1
end
return redis.call("DEL", KEYS[1])
`

const extendScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`

// RedisClient is the subset of Redis operations required by Dlock.
// *redis.Client, *redis.ClusterClient, and redispool.RedisNode all satisfy it.
type RedisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

var (
	_ RedisClient = (*redis.Client)(nil)
	_ RedisClient = (*redis.ClusterClient)(nil)
)

type options struct {
	ttl           time.Duration
	minRetryDelay time.Duration
	maxRetryDelay time.Duration
}

func defaultOptions() options {
	return options{
		ttl:           DefaultTTL,
		minRetryDelay: DefaultMinRetryDelay,
		maxRetryDelay: DefaultMaxRetryDelay,
	}
}

// Option configures a Dlock.
type Option func(*options)

// WithTTL configures the Redis lease duration. The value must be positive.
func WithTTL(ttl time.Duration) Option {
	return func(opts *options) {
		opts.ttl = ttl
	}
}

// WithRetryBackoff configures the minimum and maximum delay between attempts.
// Both values must be positive and maxDelay must not be less than minDelay.
func WithRetryBackoff(minDelay, maxDelay time.Duration) Option {
	return func(opts *options) {
		opts.minRetryDelay = minDelay
		opts.maxRetryDelay = maxDelay
	}
}

type Dlock struct {
	// Key, Ltime, and Rdb are retained for source compatibility. Callers must not
	// mutate Key or Rdb while an operation is in progress. Ltime is informational;
	// ownership is determined exclusively by the private random token.
	Key   string
	Ltime int64
	Rdb   RedisClient

	mu            sync.Mutex
	acquiring     bool
	token         string
	ttl           time.Duration
	minRetryDelay time.Duration
	maxRetryDelay time.Duration
}

// DlockNew creates a Redis-backed distributed lock. ctx is retained for source
// compatibility; operation contexts are supplied to Lock, TryLock, Extend, and
// Unlock.
func DlockNew(_ context.Context, rdb RedisClient, lkey string, option ...Option) *Dlock {
	opts := defaultOptions()
	for _, apply := range option {
		if apply != nil {
			apply(&opts)
		}
	}

	return &Dlock{
		Key:           lkey,
		Rdb:           rdb,
		ttl:           opts.ttl,
		minRetryDelay: opts.minRetryDelay,
		maxRetryDelay: opts.maxRetryDelay,
	}
}

// Lock waits until the lock is acquired or ctx is canceled. Contention is
// handled with bounded exponential backoff and jitter; it is not a busy spin.
func (dl *Dlock) Lock(ctx context.Context) error {
	if err := dl.validate(); err != nil {
		return err
	}
	if !dl.beginAcquire() {
		return ErrAlreadyLocked
	}
	completed := false
	defer func() {
		if !completed {
			dl.cancelAcquire()
		}
	}()

	token, err := newToken()
	if err != nil {
		return fmt.Errorf("generate lock token: %w", err)
	}

	backoff := dl.minRetryDelay
	for attempt := 1; ; attempt++ {
		ok, err := dl.Rdb.SetNX(ctx, dl.Key, token, dl.ttl).Result()
		if err != nil {
			return fmt.Errorf("acquire distributed lock %q: %w", dl.Key, err)
		}
		if ok {
			dl.finishAcquire(token)
			completed = true
			logger.Debug(ctx, "distributed lock acquired", "key", dl.Key, "attempt", attempt)
			return nil
		}

		wait := jitter(backoff)
		logger.Debug(ctx, "distributed lock contended", "key", dl.Key, "attempt", attempt, "retry_after", wait)
		if err := waitForRetry(ctx, wait); err != nil {
			return err
		}
		backoff = nextBackoff(backoff, dl.maxRetryDelay)
	}
}

// TryLock attempts to acquire the lock once without waiting.
func (dl *Dlock) TryLock(ctx context.Context) (bool, error) {
	if err := dl.validate(); err != nil {
		return false, err
	}
	if !dl.beginAcquire() {
		return false, ErrAlreadyLocked
	}
	completed := false
	defer func() {
		if !completed {
			dl.cancelAcquire()
		}
	}()

	token, err := newToken()
	if err != nil {
		return false, fmt.Errorf("generate lock token: %w", err)
	}

	ok, err := dl.Rdb.SetNX(ctx, dl.Key, token, dl.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("try distributed lock %q: %w", dl.Key, err)
	}
	if ok {
		dl.finishAcquire(token)
		completed = true
	}
	return ok, nil
}

// Extend renews the lease only when Redis still contains this instance's
// ownership token. A zero result means the lease expired or another owner has
// acquired the lock.
func (dl *Dlock) Extend(ctx context.Context) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if err := dl.validate(); err != nil {
		return err
	}
	if dl.token == "" {
		return ErrNotLocked
	}

	ret, err := dl.Rdb.Eval(ctx, extendScript, []string{dl.Key}, dl.token, dl.ttl.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("extend distributed lock %q: %w", dl.Key, err)
	}
	if ret != 1 {
		dl.clearAcquired()
		return ErrLockLost
	}
	return nil
}

// Unlock atomically verifies ownership and deletes the key with a Lua script.
// It never deletes a lock held by a different token.
func (dl *Dlock) Unlock(ctx context.Context) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if err := dl.validate(); err != nil {
		return err
	}
	if dl.token == "" {
		return ErrNotLocked
	}

	ret, err := dl.Rdb.Eval(ctx, unlockScript, []string{dl.Key}, dl.token).Int64()
	if err != nil {
		return fmt.Errorf("unlock distributed lock %q: %w", dl.Key, err)
	}

	dl.clearAcquired()
	switch ret {
	case 1:
		logger.Debug(ctx, "distributed lock released", "key", dl.Key)
		return nil
	case 0, -1:
		return ErrLockLost
	default:
		return fmt.Errorf("unlock distributed lock %q: unexpected script result %d", dl.Key, ret)
	}
}

func (dl *Dlock) validate() error {
	switch {
	case dl.Key == "":
		return errors.New("distributed lock key must not be empty")
	case dl.ttl <= 0:
		return errors.New("distributed lock TTL must be positive")
	case dl.ttl < time.Millisecond:
		return errors.New("distributed lock TTL must be at least one millisecond")
	case dl.minRetryDelay <= 0:
		return errors.New("distributed lock minimum retry delay must be positive")
	case dl.maxRetryDelay < dl.minRetryDelay:
		return errors.New("distributed lock maximum retry delay must not be less than minimum retry delay")
	default:
		return nil
	}
}

func (dl *Dlock) beginAcquire() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.acquiring || dl.token != "" {
		return false
	}
	dl.acquiring = true
	return true
}

func (dl *Dlock) cancelAcquire() {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.acquiring = false
}

func (dl *Dlock) finishAcquire(token string) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.acquiring = false
	dl.token = token
	dl.Ltime = time.Now().UnixNano()
}

func (dl *Dlock) clearAcquired() {
	dl.token = ""
	dl.Ltime = 0
}

func newToken() (string, error) {
	var token [16]byte
	if _, err := cryptorand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func jitter(backoff time.Duration) time.Duration {
	if backoff <= time.Nanosecond {
		return backoff
	}
	half := backoff / 2
	return half + time.Duration(mathrand.Int64N(int64(backoff-half)))
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
