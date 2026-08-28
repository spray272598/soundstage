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
// Uses pipelining to batch GET commands for better performance.
func (c *RedisLikeCounter) ScanRooms(ctx context.Context, match string, fn func(roomID string) error) error {
	iter := c.rdb.RDB().Scan(ctx, 0, match, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		key := iter.Val()
		keys = append(keys, key)
	}
	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	// Use pipeline to batch GET commands
	pipe := c.rdb.RDB().Pipeline()
	cmds := make([]*goredis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return err
	}

	for i, key := range keys {
		idx := indexAfterColon(key)
		if idx < 0 {
			continue
		}
		roomID := key[idx:]
		if err := fn(roomID); err != nil {
			return err
		}
		_ = cmds[i] // suppress unused warning
	}
	return nil
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
