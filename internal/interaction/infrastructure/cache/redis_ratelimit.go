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
type RedisRateLimiter struct {
	rdb *redis.Client
}

// NewRedisRateLimiter creates a new RedisRateLimiter.
func NewRedisRateLimiter(c *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: c}
}

// Allow returns true while the key's count is within limit for the window.
// The window timer is (re)armed only on the first hit.
func (l *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	k := fmt.Sprintf("rl:%s", key)
	n, err := l.rdb.RDB().Incr(ctx, k).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := l.rdb.RDB().Expire(ctx, k, window).Err(); err != nil {
			return false, err
		}
	}
	return n <= int64(limit), nil
}

// Compile-time check.
var _ domain.RateLimiter = (*RedisRateLimiter)(nil)
