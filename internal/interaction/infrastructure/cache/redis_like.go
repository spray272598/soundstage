package cache

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// RedisLikeCounter keeps a per-room like tally. Individual like events are
// never written to MySQL; the periodic flush job snapshots the count.
type RedisLikeCounter struct {
	rdb *redis.Client
}

// NewRedisLikeCounter creates a new RedisLikeCounter.
func NewRedisLikeCounter(c *redis.Client) *RedisLikeCounter {
	return &RedisLikeCounter{rdb: c}
}

func likeKey(roomID string) string {
	return fmt.Sprintf("like:%s", roomID)
}

// Incr increments the room like tally and returns the new value.
func (c *RedisLikeCounter) Incr(ctx context.Context, roomID string) (int64, error) {
	return c.rdb.RDB().Incr(ctx, likeKey(roomID)).Result()
}

// Get returns the current like tally (0 if none).
func (c *RedisLikeCounter) Get(ctx context.Context, roomID string) (int64, error) {
	n, err := c.rdb.RDB().Get(ctx, likeKey(roomID)).Int64()
	if err == goredis.Nil {
		return 0, nil
	}
	return n, err
}

// ScanRooms invokes fn for every room id that currently has a like counter.
// fn only needs the room id, so we avoid the extra GET round-trips that a
// pipeline of fetches would incur (the counter values are never consumed).
func (c *RedisLikeCounter) ScanRooms(ctx context.Context, match string, fn func(roomID string) error) error {
	iter := c.rdb.RDB().Scan(ctx, 0, match, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		idx := indexAfterColon(key)
		if idx < 0 {
			continue
		}
		if err := fn(key[idx:]); err != nil {
			return err
		}
	}
	return iter.Err()
}

func indexAfterColon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return i + 1
		}
	}
	return -1
}

// Compile-time check.
var _ domain.LikeCounter = (*RedisLikeCounter)(nil)
