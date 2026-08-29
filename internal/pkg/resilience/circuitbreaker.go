package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota // Normal operation, requests pass through
	StateOpen                              // Circuit is open, requests fail fast
	StateHalfOpen                          // Testing if service recovered
)

var (
	ErrCircuitOpen   = errors.New("circuit breaker is open")
	ErrTooManyErrors = errors.New("too many errors")
)

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
type CircuitBreaker struct {
	mu                sync.RWMutex
	state             CircuitBreakerState
	failureCount      int64
	successCount      int64
	lastFailureTime   time.Time
	lastStateChange   time.Time

	// Configuration
	failureThreshold  int           // Number of failures before opening circuit
	successThreshold  int           // Number of successes in half-open before closing
	timeout           time.Duration // Time to wait before transitioning to half-open
	excludedErrors    []error       // Errors that don't count as failures
}

// CircuitBreakerConfig holds configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	FailureThreshold  int           // Default: 5
	SuccessThreshold  int           // Default: 3
	Timeout           time.Duration // Default: 30s
	ExcludedErrors    []error       // Errors that don't trip the breaker
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          cfg.Timeout,
		excludedErrors:   cfg.ExcludedErrors,
		lastStateChange:  time.Now(),
	}
}

// Execute runs the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn(ctx)
	cb.recordResult(err)
	return err
}

// ExecuteWithFallback runs fn with circuit breaker, falling back to fallback on open circuit or error.
func (cb *CircuitBreaker) ExecuteWithFallback(ctx context.Context, fn func(context.Context) error, fallback func(context.Context) error) error {
	if !cb.allowRequest() {
		if fallback != nil {
			return fallback(ctx)
		}
		return ErrCircuitOpen
	}

	err := fn(ctx)
	if err != nil {
		cb.recordResult(err)
		if fallback != nil {
			return fallback(ctx)
		}
		return err
	}

	cb.recordResult(nil)
	return nil
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		cb.mu.Lock()
		defer cb.mu.Unlock()
		if time.Since(cb.lastStateChange) >= cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.lastStateChange = time.Now()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) recordResult(err error) {
	if err != nil && !cb.isExcludedError(err) {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

func (cb *CircuitBreaker) isExcludedError(err error) bool {
	for _, excluded := range cb.excludedErrors {
		if errors.Is(err, excluded) {
			return true
		}
	}
	return false
}

func (cb *CircuitBreaker) onFailure() {
	failures := atomic.AddInt64(&cb.failureCount, 1)
	atomic.StoreInt64(&cb.successCount, 0)
	cb.lastFailureTime = time.Now()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		// Any failure in half-open goes back to open
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	} else if cb.state == StateClosed && int(failures) >= cb.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) onSuccess() {
	successes := atomic.AddInt64(&cb.successCount, 1)
	atomic.StoreInt64(&cb.failureCount, 0)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen && int(successes) >= cb.successThreshold {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the current failure count.
func (cb *CircuitBreaker) FailureCount() int64 {
	return atomic.LoadInt64(&cb.failureCount)
}

// SuccessCount returns the current success count.
func (cb *CircuitBreaker) SuccessCount() int64 {
	return atomic.LoadInt64(&cb.successCount)
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()
}