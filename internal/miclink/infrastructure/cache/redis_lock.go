package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// ErrLockHeld is returned when the lock is already taken by another caller.
var ErrLockHeld = errors.New("lock already held")

// RedisLocker is a best-effort distributed lock backed by Redis SET NX with a
// randomly generated token. Unlock only releases the lock if the token still
// matches, so a caller never frees a lock taken by someone else after expiry.
type RedisLocker struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewRedisLocker creates a new RedisLocker with the given lease TTL.
func NewRedisLocker(c *redis.Client, ttl time.Duration) *RedisLocker {
	return &RedisLocker{rdb: c, ttl: ttl}
}

// Lock acquires the named lock. On success it returns an unlock function.
func (l *RedisLocker) Lock(ctx context.Context, key string) (func(), error) {
	token := newToken()
	ok, err := l.rdb.RDB().SetNX(ctx, lockKey(key), token, l.ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLockHeld
	}
	return func() {
		// Use a Lua script so the delete only happens when the token matches.
		script := goredis.NewScript(`if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`)
		_ = script.Run(context.Background(), l.rdb.RDB(), []string{lockKey(key)}, token).Err()
	}, nil
}

func lockKey(key string) string { return "soundstage:lock:" + key }

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Compile-time check.
var _ domain.Locker = (*RedisLocker)(nil)
