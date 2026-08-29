package resilience

import (
	"context"
	"time"
)

// ResilientClient wraps an operation with circuit breaker, retry, and fallback.
type ResilientClient struct {
	circuitBreaker *CircuitBreaker
	retryPolicy    *RetryPolicy
	fallback       func(context.Context, error) (any, error)
	timeout        time.Duration
	onFallback     func(context.Context, error)
}

// ResilientClientConfig holds configuration for ResilientClient.
type ResilientClientConfig struct {
	CircuitBreaker *CircuitBreakerConfig
	RetryPolicy    *RetryPolicy
	Timeout        time.Duration
	Fallback       func(context.Context, error) (any, error)
	OnFallback     func(context.Context, error)
}

// NewResilientClient creates a new resilient client.
func NewResilientClient(cfg ResilientClientConfig) *ResilientClient {
	var cb *CircuitBreaker
	if cfg.CircuitBreaker != nil {
		cb = NewCircuitBreaker(*cfg.CircuitBreaker)
	}

	var rp *RetryPolicy
	if cfg.RetryPolicy != nil {
		rp = cfg.RetryPolicy
	}

	return &ResilientClient{
		circuitBreaker: cb,
		retryPolicy:    rp,
		fallback:       cfg.Fallback,
		timeout:        cfg.Timeout,
		onFallback:     cfg.OnFallback,
	}
}

// Execute runs the operation with all resilience patterns applied.
func (rc *ResilientClient) Execute(ctx context.Context, primary func(context.Context) (any, error)) (any, error) {
	// Apply timeout
	if rc.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rc.timeout)
		defer cancel()
	}

	// Wrap primary with retries
	primaryWithRetry := primary
	if rc.retryPolicy != nil {
		primaryWithRetry = func(ctx context.Context) (any, error) {
			var result any
			err := Retry(ctx, *rc.retryPolicy, func(ctx context.Context) error {
				var err error
				result, err = primary(ctx)
				return err
			})
			return result, err
		}
	}

	// Execute with circuit breaker
	var result any
	var primaryErr error

	if rc.circuitBreaker != nil {
		primaryErr = rc.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
			var err error
			result, err = primaryWithRetry(ctx)
			return err
		})
	} else {
		result, primaryErr = primaryWithRetry(ctx)
	}

	if primaryErr == nil {
		return result, nil
	}

	// Try fallback
	if rc.fallback != nil {
		if rc.onFallback != nil {
			rc.onFallback(ctx, primaryErr)
		}
		return rc.fallback(ctx, primaryErr)
	}

	return nil, primaryErr
}

// ExecuteWithFallback is a convenience method that takes primary and fallback separately.
func (rc *ResilientClient) ExecuteWithFallback(ctx context.Context, primary func(context.Context) (any, error), fallback func(context.Context, error) (any, error)) (any, error) {
	oldFallback := rc.fallback
	rc.fallback = fallback
	defer func() { rc.fallback = oldFallback }()
	return rc.Execute(ctx, primary)
}