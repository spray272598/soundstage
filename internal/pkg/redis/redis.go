package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis for application use.
type Client struct {
	rdb *redis.Client
}

// New creates a new Redis client.
func New(addr string, db int, poolSize int) *Client {
	return &Client{
		rdb: redis.NewClient(&redis.Options{
			Addr:     addr,
			DB:       db,
			PoolSize: poolSize,
		}),
	}
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the client.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// RDB returns the underlying go-redis client.
func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// Pipeline returns a new pipeline for batched commands.
func (c *Client) Pipeline() redis.Pipeliner {
	return c.rdb.Pipeline()
}

// Pipelined executes fn within a pipeline.
func (c *Client) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return c.rdb.Pipelined(ctx, fn)
}
