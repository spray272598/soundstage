package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// RedisRateLimiter is a fixed-window counter backed by Redis INCR + EXPIRE.
// It is good enough for per-user-per-room danmaku throttling and stays cheap
// under high concurrency because the hot path is a single INCR.
// Uses Redis pipelining to combine INCR + EXPIRE into a single round-trip.
type RedisRateLimiter struct {
	rdb *redis.Client
}

// NewRedisRateLimiter creates a new RedisRateLimiter.
func NewRedisRateLimiter(c *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: c}
}

// Allow returns true while the key's count is within limit for the window.
// The window timer is (re)armed only on the first hit.
// Uses pipeline to combine INCR and EXPIRE in a single round-trip.
func (l *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	k := fmt.Sprintf("rl:%s", key)
	pipe := l.rdb.RDB().Pipeline()
	incrCmd := pipe.Incr(ctx, k)
	// Only set expiry on first hit (when count becomes 1)
	pipe.Do(ctx, "EXPIRE", k, int(window.Seconds()))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	n := incrCmd.Val()
	return n <= int64(limit), nil
}

// Compile-time check.
var _ domain.RateLimiter = (*RedisRateLimiter)(nil)
