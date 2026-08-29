package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines how retries should be performed.
type RetryPolicy struct {
	MaxRetries      int           // Maximum number of retry attempts (default: 3)
	BaseDelay       time.Duration // Initial delay (default: 100ms)
	MaxDelay        time.Duration // Maximum delay cap (default: 10s)
	Multiplier      float64       // Exponential multiplier (default: 2.0)
	Jitter          float64       // Random jitter factor 0-1 (default: 0.1)
	RetryableErrors []error       // Errors that should trigger a retry
	IsRetryable     func(error) bool // Custom function to determine if error is retryable
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
	}
}

// Retry executes fn with retries according to the policy.
func Retry(ctx context.Context, policy RetryPolicy, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err, policy) {
			return err // Non-retryable error
		}

		// Don't sleep after the last attempt
		if attempt == policy.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := calculateDelay(attempt, policy)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// RetryWithCallback executes fn with retries and calls onRetry before each retry.
func RetryWithCallback(ctx context.Context, policy RetryPolicy, fn func(context.Context) error, onRetry func(attempt int, err error, delay time.Duration)) error {
	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if !isRetryable(err, policy) {
			return err
		}

		if attempt == policy.MaxRetries {
			break
		}

		delay := calculateDelay(attempt, policy)

		if onRetry != nil {
			onRetry(attempt+1, err, delay)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

func isRetryable(err error, policy RetryPolicy) bool {
	if policy.IsRetryable != nil {
		return policy.IsRetryable(err)
	}

	// Check against known retryable errors
	for _, retryable := range policy.RetryableErrors {
		if errors.Is(err, retryable) {
			return true
		}
	}

	// Default: retry on context deadline exceeded and temporary network errors
	var netErr interface{ Temporary() bool }
	if errors.As(err, &netErr) && netErr.Temporary() {
		return true
	}

	return false
}

func calculateDelay(attempt int, policy RetryPolicy) time.Duration {
	// Exponential backoff: base * multiplier^attempt
	delay := float64(policy.BaseDelay) * math.Pow(policy.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(policy.MaxDelay) {
		delay = float64(policy.MaxDelay)
	}

	// Add jitter
	if policy.Jitter > 0 {
		jitter := delay * policy.Jitter * (rand.Float64()*2 - 1) // -jitter to +jitter
		delay += jitter
	}

	// Ensure minimum delay
	if delay < float64(policy.BaseDelay) {
		delay = float64(policy.BaseDelay)
	}

	return time.Duration(delay)
}

// WithRetry returns a new function that wraps fn with retry logic.
func WithRetry(policy RetryPolicy, fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return Retry(ctx, policy, fn)
	}
}