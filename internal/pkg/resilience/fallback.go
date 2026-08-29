package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// FallbackProvider provides fallback behavior when primary operation fails.
type FallbackProvider interface {
	// Primary is the main operation to execute.
	Primary(ctx context.Context) (any, error)

	// Fallback is called when primary fails (circuit open, retries exhausted, etc.).
	Fallback(ctx context.Context, primaryErr error) (any, error)
}

// FallbackConfig holds configuration for fallback behavior.
type FallbackConfig struct {
	// Circuit breaker to protect the primary operation
	CircuitBreaker *CircuitBreaker

	// Retry policy for the primary operation
	RetryPolicy *RetryPolicy

	// Timeout for the entire operation (primary + retries + fallback)
	Timeout time.Duration

	// OnFallback is called when fallback is triggered
	OnFallback func(ctx context.Context, primaryErr error)
}

// ExecuteWithFallback executes primary with circuit breaker, retries, and fallback.
func ExecuteWithFallback(ctx context.Context, config FallbackConfig, provider FallbackProvider) (any, error) {
	// Apply timeout
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// Wrap primary with retries
	primaryWithRetry := provider.Primary
	if config.RetryPolicy != nil {
		primaryWithRetry = func(ctx context.Context) (any, error) {
			var result any
			err := Retry(ctx, *config.RetryPolicy, func(ctx context.Context) error {
				var err error
				result, err = provider.Primary(ctx)
				return err
			})
			return result, err
		}
	}

	// Execute with circuit breaker
	var result any
	var primaryErr error

	if config.CircuitBreaker != nil {
		primaryErr = config.CircuitBreaker.Execute(ctx, func(ctx context.Context) error {
			var err error
			result, err = primaryWithRetry(ctx)
			return err
		})
	} else {
		result, primaryErr = primaryWithRetry(ctx)
	}

	// If primary succeeded, return result
	if primaryErr == nil {
		return result, nil
	}

	// Primary failed - check if we should try fallback
	var fallbackErr error
	if config.CircuitBreaker != nil && errors.Is(primaryErr, ErrCircuitOpen) {
		// Circuit is open, use fallback
		if config.OnFallback != nil {
			config.OnFallback(ctx, primaryErr)
		}
		result, fallbackErr = provider.Fallback(ctx, primaryErr)
	} else if config.CircuitBreaker == nil {
		// No circuit breaker, but primary failed - try fallback
		if config.OnFallback != nil {
			config.OnFallback(ctx, primaryErr)
		}
		result, fallbackErr = provider.Fallback(ctx, primaryErr)
	} else {
		// Other error (non-retryable, context cancelled, etc.)
		return nil, primaryErr
	}

	if fallbackErr != nil {
		return nil, fmt.Errorf("primary failed: %w; fallback failed: %w", primaryErr, fallbackErr)
	}

	return result, nil
}

// SimpleFallback executes primary, and on any error, executes fallback.
func SimpleFallback(ctx context.Context, primary func(context.Context) (any, error), fallback func(context.Context, error) (any, error)) (any, error) {
	result, err := primary(ctx)
	if err == nil {
		return result, nil
	}
	return fallback(ctx, err)
}

// ChainFallbacks tries multiple fallbacks in sequence until one succeeds.
func ChainFallbacks(ctx context.Context, primary func(context.Context) (any, error), fallbacks ...func(context.Context, error) (any, error)) (any, error) {
	result, err := primary(ctx)
	if err == nil {
		return result, nil
	}

	lastErr := err
	for _, fb := range fallbacks {
		result, lastErr = fb(ctx, lastErr)
		if lastErr == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("all fallbacks failed, last error: %w", lastErr)
}