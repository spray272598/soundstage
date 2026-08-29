package redis

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spray272598/soundstage/internal/pkg/resilience"
)

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ResilientClient wraps the Redis client with retry and circuit breaker.
type ResilientClient struct {
	*Client
	retryPolicy *resilience.RetryPolicy
}

// ResilientConfig holds configuration for ResilientClient.
type ResilientConfig struct {
	RetryPolicy *resilience.RetryPolicy
}

// DefaultResilientConfig returns sensible defaults for Redis retry.
func DefaultResilientConfig() ResilientConfig {
	return ResilientConfig{
		RetryPolicy: &resilience.RetryPolicy{
			MaxRetries:  3,
			BaseDelay:   50 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Multiplier:  2.0,
			Jitter:      0.1,
			RetryableErrors: []error{
				redis.ErrClosed,
				redis.ErrPoolTimeout,
			},
			IsRetryable: func(err error) bool {
				// Retry on timeout, connection errors, and pool timeout
				if errors.Is(err, context.DeadlineExceeded) {
					return true
				}
				var redisErr redis.Error
				if errors.As(err, &redisErr) {
					// Check if it's a timeout or connection error
					errStr := redisErr.Error()
					isTimeout := len(errStr) >= 7 && (errStr[:7] == "timeout" || containsSubstring(errStr, "timeout"))
					return isTimeout || isConnectionError(redisErr)
				}
				// Also check for net.Error (timeout, temporary)
				var netErr net.Error
				if errors.As(err, &netErr) {
					return netErr.Timeout() || netErr.Temporary()
				}
				return false
			},
		},
	}
}

// NewResilientClient creates a new resilient Redis client.
func NewResilientClient(addr string, db int, poolSize int, cfg ResilientConfig) *ResilientClient {
	if cfg.RetryPolicy == nil {
		defaultCfg := DefaultResilientConfig()
		cfg.RetryPolicy = defaultCfg.RetryPolicy
	}
	return &ResilientClient{
		Client:      New(addr, db, poolSize),
		retryPolicy: cfg.RetryPolicy,
	}
}

// WithRetry executes fn with retries.
func (c *ResilientClient) WithRetry(ctx context.Context, fn func(context.Context) error) error {
	return resilience.Retry(ctx, *c.retryPolicy, fn)
}

// GetWithRetry executes GET with retries.
func (c *ResilientClient) GetWithRetry(ctx context.Context, key string) (string, error) {
	var result string
	err := c.WithRetry(ctx, func(ctx context.Context) error {
		var err error
		result, err = c.Client.RDB().Get(ctx, key).Result()
		return err
	})
	return result, err
}

// SetWithRetry executes SET with retries.
func (c *ResilientClient) SetWithRetry(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.WithRetry(ctx, func(ctx context.Context) error {
		return c.Client.RDB().Set(ctx, key, value, expiration).Err()
	})
}

// IncrWithRetry executes INCR with retries.
func (c *ResilientClient) IncrWithRetry(ctx context.Context, key string) (int64, error) {
	var result int64
	err := c.WithRetry(ctx, func(ctx context.Context) error {
		var err error
		result, err = c.Client.RDB().Incr(ctx, key).Result()
		return err
	})
	return result, err
}

// ExistsWithRetry executes EXISTS with retries.
func (c *ResilientClient) ExistsWithRetry(ctx context.Context, keys ...string) (int64, error) {
	var result int64
	err := c.WithRetry(ctx, func(ctx context.Context) error {
		var err error
		result, err = c.Client.RDB().Exists(ctx, keys...).Result()
		return err
	})
	return result, err
}

// DelWithRetry executes DEL with retries.
func (c *ResilientClient) DelWithRetry(ctx context.Context, keys ...string) error {
	return c.WithRetry(ctx, func(ctx context.Context) error {
		return c.Client.RDB().Del(ctx, keys...).Err()
	})
}

// ExpireWithRetry executes EXPIRE with retries.
func (c *ResilientClient) ExpireWithRetry(ctx context.Context, key string, expiration time.Duration) error {
	return c.WithRetry(ctx, func(ctx context.Context) error {
		return c.Client.RDB().Expire(ctx, key, expiration).Err()
	})
}

// PipelineWithRetry executes a pipeline with retries.
func (c *ResilientClient) PipelineWithRetry(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	var result []redis.Cmder
	err := c.WithRetry(ctx, func(ctx context.Context) error {
		var err error
		result, err = c.Client.Pipelined(ctx, fn)
		return err
	})
	return result, err
}

// ScanWithRetry executes SCAN with retries.
func (c *ResilientClient) ScanWithRetry(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	var keys []string
	var nextCursor uint64
	err := c.WithRetry(ctx, func(ctx context.Context) error {
		var err error
		keys, nextCursor, err = c.Client.RDB().Scan(ctx, cursor, match, count).Result()
		return err
	})
	return keys, nextCursor, err
}

// Compile-time check that ResilientClient embeds Client properly.
var _ interface {
	Ping(context.Context) error
	Close() error
	RDB() *redis.Client
	Pipeline() redis.Pipeliner
	Pipelined(context.Context, func(redis.Pipeliner) error) ([]redis.Cmder, error)
} = (*ResilientClient)(nil)

// isConnectionError checks if a redis.Error represents a connection error.
func isConnectionError(err redis.Error) bool {
	errStr := err.Error()
	connErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"dial tcp",
		"timeout",
	}
	for _, e := range connErrors {
		if len(errStr) >= len(e) {
			for i := 0; i <= len(errStr)-len(e); i++ {
				if errStr[i:i+len(e)] == e {
					return true
				}
			}
		}
	}
	return false
}