package circuitbreaker

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryableFunc represents a function that can be retried
type RetryableFunc func() (interface{}, error)

// RetryableFuncContext represents a function with context that can be retried
type RetryableFuncContext func(context.Context) (interface{}, error)

// RetryExecutor handles retry logic with exponential backoff
type RetryExecutor struct {
	config RetryConfig
}

// NewRetryExecutor creates a new retry executor with the given configuration
func NewRetryExecutor(config RetryConfig) *RetryExecutor {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.BaseDelay <= 0 {
		config.BaseDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 1.0 {
		config.Multiplier = 2.0
	}

	return &RetryExecutor{config: config}
}

// Execute executes a function with retry logic
func (r *RetryExecutor) Execute(fn RetryableFunc) (interface{}, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			time.Sleep(delay)
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			return nil, err
		}

		// If this is the last attempt, don't retry
		if attempt == r.config.MaxRetries {
			break
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ExecuteContext executes a function with context and retry logic
func (r *RetryExecutor) ExecuteContext(ctx context.Context, fn RetryableFuncContext) (interface{}, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			
			// Use context-aware sleep
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			return nil, err
		}

		// If this is the last attempt, don't retry
		if attempt == r.config.MaxRetries {
			break
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateDelay calculates the delay for the given attempt using exponential backoff
func (r *RetryExecutor) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	
	// Cap at max delay
	if time.Duration(delay) > r.config.MaxDelay {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter if enabled
	if r.config.Jitter {
		jitter := delay * 0.1 * (rand.Float64()*2 - 1) // ±10% jitter
		delay += jitter
	}

	return time.Duration(delay)
}

// isRetryable checks if an error is retryable based on configuration
func (r *RetryExecutor) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP errors
	if httpErr, ok := err.(*HTTPError); ok {
		// Retry on 5xx errors and 429 (rate limiting)
		if httpErr.StatusCode >= 500 || httpErr.StatusCode == 429 {
			return true
		}
		// Don't retry on 4xx errors (client errors)
		return false
	}

	// Check against configured retryable errors
	errStr := strings.ToLower(err.Error())
	for _, retryableErr := range r.config.RetryableErrors {
		if strings.Contains(errStr, strings.ToLower(retryableErr)) {
			return true
		}
	}

	// Default behavior for common network errors
	networkErrors := []string{
		"connection refused",
		"timeout",
		"no such host",
		"network unreachable",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"temporary failure",
		"service unavailable",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errStr, netErr) {
			return true
		}
	}

	return false
}

// CircuitBreakerWithRetry combines circuit breaker and retry patterns
type CircuitBreakerWithRetry struct {
	circuitBreaker *CircuitBreaker
	retryExecutor  *RetryExecutor
}

// NewCircuitBreakerWithRetry creates a new circuit breaker with retry
func NewCircuitBreakerWithRetry(cbConfig Config, retryConfig RetryConfig) *CircuitBreakerWithRetry {
	return &CircuitBreakerWithRetry{
		circuitBreaker: New(cbConfig),
		retryExecutor:  NewRetryExecutor(retryConfig),
	}
}

// Execute executes a function with both circuit breaker and retry logic
func (cbr *CircuitBreakerWithRetry) Execute(fn RetryableFunc) (interface{}, error) {
	return cbr.circuitBreaker.Execute(func() (interface{}, error) {
		return cbr.retryExecutor.Execute(fn)
	})
}

// ExecuteContext executes a function with context, circuit breaker and retry logic
func (cbr *CircuitBreakerWithRetry) ExecuteContext(ctx context.Context, fn RetryableFuncContext) (interface{}, error) {
	return cbr.circuitBreaker.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		return cbr.retryExecutor.ExecuteContext(ctx, fn)
	})
}

// State returns the current state of the circuit breaker
func (cbr *CircuitBreakerWithRetry) State() State {
	return cbr.circuitBreaker.State()
}

// Counts returns the current counts from the circuit breaker
func (cbr *CircuitBreakerWithRetry) Counts() Counts {
	return cbr.circuitBreaker.Counts()
}

// Name returns the name of the circuit breaker
func (cbr *CircuitBreakerWithRetry) Name() string {
	return cbr.circuitBreaker.Name()
}

// ServiceRetryConfigs returns service-specific retry configurations
func ServiceRetryConfigs() map[string]RetryConfig {
	return map[string]RetryConfig{
		"vault": {
			MaxRetries: 3,
			BaseDelay:  200 * time.Millisecond,
			MaxDelay:   2 * time.Second,
			Multiplier: 2.0,
			Jitter:     true,
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"vault sealed",
				"temporary failure",
			},
		},
		"argocd": {
			MaxRetries: 2,
			BaseDelay:  300 * time.Millisecond,
			MaxDelay:   3 * time.Second,
			Multiplier: 2.0,
			Jitter:     true,
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"service unavailable",
				"temporary failure",
			},
		},
		"kubernetes": {
			MaxRetries: 4,
			BaseDelay:  500 * time.Millisecond,
			MaxDelay:   5 * time.Second,
			Multiplier: 1.5,
			Jitter:     true,
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"api server unavailable",
				"etcd unavailable",
				"temporary failure",
			},
		},
		"git": {
			MaxRetries: 3,
			BaseDelay:  1 * time.Second,
			MaxDelay:   10 * time.Second,
			Multiplier: 2.0,
			Jitter:     true,
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"remote end hung up",
				"repository temporarily unavailable",
				"temporary failure",
			},
		},
		"newrelic": {
			MaxRetries: 2,
			BaseDelay:  500 * time.Millisecond,
			MaxDelay:   3 * time.Second,
			Multiplier: 2.0,
			Jitter:     true,
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"rate limited",
				"service unavailable",
				"temporary failure",
			},
		},
	}
}