package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// RetryableFunc represents a function that can be retried
type RetryableFunc func() (interface{}, error)

// RetryableFuncContext represents a function with context that can be retried
type RetryableFuncContext func(context.Context) (interface{}, error)

// RetryConfig defines retry behavior with exponential backoff.
type RetryConfig struct {
	// MaxRetries is the number of retries attempted after the initial call.
	MaxRetries int
	// InitialDelay is the base delay used for the first retry.
	InitialDelay time.Duration
	// MaxDelay caps the computed backoff delay.
	MaxDelay time.Duration
	// BackoffFactor is the exponential growth factor applied per attempt.
	BackoffFactor float64
	// Jitter randomizes the delay by ±50% to avoid thundering herds.
	Jitter bool
	// RetryableErrors is an optional set of sentinel errors that are
	// considered retryable (matched with errors.Is).
	RetryableErrors []error
	// IsRetryable, when set, fully determines whether an error is retryable
	// and takes precedence over all other classification.
	IsRetryable func(error) bool
	// CircuitBreaker, when set, wraps every attempt so that retries respect
	// the breaker's open/closed state.
	CircuitBreaker *CircuitBreaker
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// RetryStats holds cumulative statistics for a retry executor.
type RetryStats struct {
	TotalExecutions uint64        `json:"total_executions"`
	TotalSuccesses  uint64        `json:"total_successes"`
	TotalFailures   uint64        `json:"total_failures"`
	TotalRetries    uint64        `json:"total_retries"`
	TotalRetryTime  time.Duration `json:"total_retry_time"`
	LastExecuted    time.Time     `json:"last_executed"`
}

// RetryExecutor handles retry logic with exponential backoff
type RetryExecutor struct {
	config RetryConfig

	mu    sync.Mutex
	stats RetryStats
}

// NewRetryExecutor creates a new retry executor, applying defaults for unset
// fields and validating the supplied configuration.
func NewRetryExecutor(config RetryConfig) (*RetryExecutor, error) {
	if config.MaxRetries < 0 {
		return nil, fmt.Errorf("max retries cannot be negative")
	}
	if config.BackoffFactor != 0 && config.BackoffFactor < 1 {
		return nil, fmt.Errorf("backoff factor must be >= 1")
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.BackoffFactor == 0 {
		config.BackoffFactor = 2.0
	}

	return &RetryExecutor{config: config}, nil
}

// Execute executes a function with retry logic.
func (r *RetryExecutor) Execute(fn RetryableFunc) (interface{}, error) {
	return r.ExecuteContext(context.Background(), func(context.Context) (interface{}, error) {
		return fn()
	})
}

// ExecuteContext executes a function with context and retry logic.
func (r *RetryExecutor) ExecuteContext(ctx context.Context, fn RetryableFuncContext) (interface{}, error) {
	var lastErr error
	var retries uint64
	var retryTime time.Duration

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Honor context cancellation before each attempt.
		select {
		case <-ctx.Done():
			r.recordStats(false, retries, retryTime)
			return nil, ctx.Err()
		default:
		}

		if attempt > 0 {
			delay := r.calculateDelay(attempt - 1)
			select {
			case <-ctx.Done():
				r.recordStats(false, retries, retryTime)
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			retries++
			retryTime += delay
		}

		result, err := r.callOnce(ctx, fn)
		if err == nil {
			r.recordStats(true, retries, retryTime)
			return result, nil
		}

		lastErr = err

		if !r.isRetryable(err) {
			r.recordStats(false, retries, retryTime)
			return nil, err
		}

		if attempt == r.config.MaxRetries {
			break
		}
	}

	r.recordStats(false, retries, retryTime)
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// callOnce performs a single attempt, routing it through the configured circuit
// breaker when one is present.
func (r *RetryExecutor) callOnce(ctx context.Context, fn RetryableFuncContext) (interface{}, error) {
	if r.config.CircuitBreaker != nil {
		return r.config.CircuitBreaker.ExecuteContext(ctx, fn)
	}
	return fn(ctx)
}

func (r *RetryExecutor) recordStats(success bool, retries uint64, retryTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalExecutions++
	if success {
		r.stats.TotalSuccesses++
	} else {
		r.stats.TotalFailures++
	}
	r.stats.TotalRetries += retries
	r.stats.TotalRetryTime += retryTime
	r.stats.LastExecuted = time.Now()
}

// GetStats returns a snapshot of the executor's cumulative statistics.
func (r *RetryExecutor) GetStats() RetryStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

// Reset clears the executor's cumulative statistics.
func (r *RetryExecutor) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats = RetryStats{}
}

// calculateDelay calculates the delay for the given (zero-based) retry index
// using exponential backoff capped at MaxDelay, with optional jitter.
func (r *RetryExecutor) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt))

	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	if r.config.Jitter {
		// ±50% jitter keeps the delay within [0.5x, 1.5x) of the base value.
		jitter := delay * 0.5 * (rand.Float64()*2 - 1)
		delay += jitter
	}

	return time.Duration(delay)
}

// defaultTransientPatterns are substrings that mark an error message as a
// transient failure worth retrying. Used only when neither IsRetryable nor
// RetryableErrors is configured.
var defaultTransientPatterns = []string{
	"connection",
	"timeout",
	"temporary",
	"unavailable",
	"network",
	"reset",
	"refused",
	"broken pipe",
	"no such host",
	"i/o timeout",
	"try again",
	"rate limit",
	"retryable",
}

// isRetryable reports whether an error should trigger a retry.
func (r *RetryExecutor) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// An explicit predicate fully determines retryability.
	if r.config.IsRetryable != nil {
		return r.config.IsRetryable(err)
	}

	// Never retry through an open circuit breaker.
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrTooManyRequests) {
		return false
	}

	// HTTP errors: retry server errors and rate limiting, not client errors.
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
	}

	// Configured sentinel errors.
	for _, target := range r.config.RetryableErrors {
		if errors.Is(err, target) {
			return true
		}
	}

	// Errors that describe themselves as transient via the standard
	// net.Error-style interfaces.
	var temp interface{ Temporary() bool }
	if errors.As(err, &temp) && temp.Temporary() {
		return true
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}

	// Fall back to substring heuristics for common transient failures.
	msg := strings.ToLower(err.Error())
	for _, pattern := range defaultTransientPatterns {
		if strings.Contains(msg, pattern) {
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
func NewCircuitBreakerWithRetry(cbConfig Config, retryConfig RetryConfig) (*CircuitBreakerWithRetry, error) {
	executor, err := NewRetryExecutor(retryConfig)
	if err != nil {
		return nil, err
	}
	return &CircuitBreakerWithRetry{
		circuitBreaker: New(cbConfig),
		retryExecutor:  executor,
	}, nil
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
			MaxRetries:    3,
			InitialDelay:  200 * time.Millisecond,
			MaxDelay:      2 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		"argocd": {
			MaxRetries:    2,
			InitialDelay:  300 * time.Millisecond,
			MaxDelay:      3 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		"kubernetes": {
			MaxRetries:    4,
			InitialDelay:  500 * time.Millisecond,
			MaxDelay:      5 * time.Second,
			BackoffFactor: 1.5,
			Jitter:        true,
		},
		"git": {
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		"newrelic": {
			MaxRetries:    2,
			InitialDelay:  500 * time.Millisecond,
			MaxDelay:      3 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
	}
}
