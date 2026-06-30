package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetryExecutor(t *testing.T) {
	tests := []struct {
		name   string
		config RetryConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: RetryConfig{
				MaxRetries:      3,
				InitialDelay:    100 * time.Millisecond,
				MaxDelay:        5 * time.Second,
				BackoffFactor:   2.0,
				Jitter:          true,
				RetryableErrors: []error{errors.New("retryable")},
			},
			valid: true,
		},
		{
			name:   "default config",
			config: RetryConfig{},
			valid:  true,
		},
		{
			name: "invalid max retries",
			config: RetryConfig{
				MaxRetries: -1,
			},
			valid: false,
		},
		{
			name: "invalid backoff factor",
			config: RetryConfig{
				BackoffFactor: 0.5,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewRetryExecutor(tt.config)

			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, executor)
				
				// Check defaults were applied
				if tt.config.MaxRetries == 0 {
					assert.Equal(t, 3, executor.config.MaxRetries)
				}
				if tt.config.InitialDelay == 0 {
					assert.Equal(t, 100*time.Millisecond, executor.config.InitialDelay)
				}
				if tt.config.MaxDelay == 0 {
					assert.Equal(t, 30*time.Second, executor.config.MaxDelay)
				}
				if tt.config.BackoffFactor == 0 {
					assert.Equal(t, 2.0, executor.config.BackoffFactor)
				}
			} else {
				assert.Error(t, err)
				assert.Nil(t, executor)
			}
		})
	}
}

func TestRetryExecutorExecuteSuccess(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		BackoffFactor: 2.0,
	})
	require.NoError(t, err)

	// Test immediate success
	result, err := executor.Execute(func() (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestRetryExecutorExecuteWithRetries(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false, // Disable jitter for predictable testing
	})
	require.NoError(t, err)

	retryableError := errors.New("retryable error")
	attemptCount := 0

	start := time.Now()
	result, err := executor.Execute(func() (interface{}, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, retryableError
		}
		return "success after retries", nil
	})
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, "success after retries", result)
	assert.Equal(t, 3, attemptCount)
	
	// Should have waited at least for initial delay + backoff delay
	// 10ms (first retry) + 20ms (second retry) = 30ms minimum
	assert.True(t, duration >= 30*time.Millisecond)
}

func TestRetryExecutorExecuteMaxRetriesExceeded(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		BackoffFactor: 2.0,
	})
	require.NoError(t, err)

	retryableError := errors.New("persistent error")
	attemptCount := 0

	result, err := executor.Execute(func() (interface{}, error) {
		attemptCount++
		return nil, retryableError
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 3, attemptCount) // Initial attempt + 2 retries
	assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestRetryExecutorExecuteNonRetryableError(t *testing.T) {
	nonRetryableError := errors.New("non-retryable")
	retryableError := errors.New("retryable")
	
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:      3,
		InitialDelay:    10 * time.Millisecond,
		RetryableErrors: []error{retryableError},
		IsRetryable: func(err error) bool {
			return err == retryableError
		},
	})
	require.NoError(t, err)

	attemptCount := 0

	result, err := executor.Execute(func() (interface{}, error) {
		attemptCount++
		return nil, nonRetryableError
	})

	assert.Error(t, err)
	assert.Equal(t, nonRetryableError, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, attemptCount) // Only initial attempt, no retries
}

func TestRetryExecutorExecuteContext(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Test successful execution with context
	result, err := executor.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		return "success", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestRetryExecutorExecuteContextCancellation(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	attemptCount := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := executor.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		attemptCount++
		return nil, errors.New("retryable error")
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, attemptCount) // Should stop after context cancellation
}

func TestRetryExecutorExecuteContextTimeout(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attemptCount := 0

	result, err := executor.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		attemptCount++
		return nil, errors.New("retryable error")
	})

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, attemptCount) // Should stop after context timeout
}

func TestRetryExecutorBackoffCalculation(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:    4,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	})
	require.NoError(t, err)

	tests := []struct {
		attempt       int
		expectedDelay time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1000 * time.Millisecond}, // Capped at MaxDelay
		{5, 1000 * time.Millisecond}, // Capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := executor.calculateDelay(tt.attempt)
			assert.Equal(t, tt.expectedDelay, delay)
		})
	}
}

func TestRetryExecutorJitter(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		InitialDelay:  100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        true,
	})
	require.NoError(t, err)

	baseDelay := 100 * time.Millisecond
	
	// Test jitter multiple times to ensure variation
	delays := make(map[time.Duration]int)
	for i := 0; i < 100; i++ {
		delay := executor.calculateDelay(0)
		delays[delay]++
		
		// Jittered delay should be between 0.5 * base and 1.5 * base
		assert.True(t, delay >= baseDelay/2, "Delay %v should be >= %v", delay, baseDelay/2)
		assert.True(t, delay <= baseDelay*3/2, "Delay %v should be <= %v", delay, baseDelay*3/2)
	}

	// Should have generated different delay values (not all the same)
	assert.True(t, len(delays) > 1, "Jitter should produce varying delays")
}

func TestRetryExecutorCustomIsRetryable(t *testing.T) {
	temporaryError := &TemporaryError{message: "temporary"}
	permanentError := &PermanentError{message: "permanent"}

	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		IsRetryable: func(err error) bool {
			_, ok := err.(*TemporaryError)
			return ok
		},
	})
	require.NoError(t, err)

	// Test retryable error
	attemptCount := 0
	result, err := executor.Execute(func() (interface{}, error) {
		attemptCount++
		if attemptCount < 2 {
			return nil, temporaryError
		}
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 2, attemptCount)

	// Test non-retryable error
	attemptCount = 0
	result, err = executor.Execute(func() (interface{}, error) {
		attemptCount++
		return nil, permanentError
	})
	assert.Error(t, err)
	assert.Equal(t, permanentError, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, attemptCount)
}

func TestRetryExecutorDefaultRetryableErrors(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "timeout error",
			err:       &TimeoutError{message: "timeout"},
			retryable: true,
		},
		{
			name:      "temporary error",
			err:       &TemporaryError{message: "temporary"},
			retryable: true,
		},
		{
			name:      "connection error",
			err:       &ConnectionError{message: "connection failed"},
			retryable: true,
		},
		{
			name:      "generic error",
			err:       errors.New("generic error"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			result, err := executor.Execute(func() (interface{}, error) {
				attemptCount++
				return nil, tt.err
			})

			assert.Error(t, err)
			assert.Nil(t, result)

			if tt.retryable {
				assert.Equal(t, 3, attemptCount) // Initial + 2 retries
			} else {
				assert.Equal(t, 1, attemptCount) // No retries
			}
		})
	}
}

func TestRetryExecutorWithCircuitBreaker(t *testing.T) {
	// Create circuit breaker that opens after 1 failure
	cb := New(Config{
		Name: "TestService",
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
		Timeout: 50 * time.Millisecond,
	})

	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		CircuitBreaker: cb,
	})
	require.NoError(t, err)

	// First request should fail and open circuit
	result, err := executor.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)
	assert.Nil(t, result)

	// Circuit should be open, subsequent requests should fail immediately
	result, err = executor.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)
	assert.Nil(t, result)

	// Wait for circuit to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Request should now succeed and close circuit
	result, err = executor.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestRetryExecutorStats(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
	})
	require.NoError(t, err)

	// Execute some operations with different outcomes
	
	// Successful operation
	_, err = executor.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)

	// Operation that succeeds after retries
	attemptCount := 0
	_, err = executor.Execute(func() (interface{}, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, errors.New("retryable")
		}
		return "success", nil
	})
	assert.NoError(t, err)

	// Operation that fails after max retries
	_, err = executor.Execute(func() (interface{}, error) {
		return nil, errors.New("retryable")
	})
	assert.Error(t, err)

	stats := executor.GetStats()
	assert.Equal(t, uint64(3), stats.TotalExecutions)
	assert.Equal(t, uint64(2), stats.TotalSuccesses)
	assert.Equal(t, uint64(1), stats.TotalFailures)
	assert.True(t, stats.TotalRetries > 0)
	assert.True(t, stats.TotalRetryTime > 0)
	assert.NotZero(t, stats.LastExecuted)
}

func TestRetryExecutorReset(t *testing.T) {
	executor, err := NewRetryExecutor(RetryConfig{})
	require.NoError(t, err)

	// Execute an operation to generate stats
	_, err = executor.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)

	// Verify stats exist
	stats := executor.GetStats()
	assert.Equal(t, uint64(1), stats.TotalExecutions)

	// Reset stats
	executor.Reset()

	// Verify stats were reset
	stats = executor.GetStats()
	assert.Equal(t, uint64(0), stats.TotalExecutions)
	assert.Equal(t, uint64(0), stats.TotalSuccesses)
	assert.Equal(t, uint64(0), stats.TotalFailures)
	assert.Equal(t, uint64(0), stats.TotalRetries)
	assert.Equal(t, time.Duration(0), stats.TotalRetryTime)
}

// Test error types for retry testing
type TemporaryError struct {
	message string
}

func (e *TemporaryError) Error() string {
	return e.message
}

func (e *TemporaryError) Temporary() bool {
	return true
}

type PermanentError struct {
	message string
}

func (e *PermanentError) Error() string {
	return e.message
}

type TimeoutError struct {
	message string
}

func (e *TimeoutError) Error() string {
	return e.message
}

func (e *TimeoutError) Timeout() bool {
	return true
}

type ConnectionError struct {
	message string
}

func (e *ConnectionError) Error() string {
	return e.message
}

// Benchmark tests
func BenchmarkRetryExecutorExecute(b *testing.B) {
	executor, _ := NewRetryExecutor(RetryConfig{
		MaxRetries:   1,
		InitialDelay: time.Microsecond,
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = executor.Execute(func() (interface{}, error) {
				return "success", nil
			})
		}
	})
}

func BenchmarkRetryExecutorExecuteWithRetries(b *testing.B) {
	executor, _ := NewRetryExecutor(RetryConfig{
		MaxRetries:   2,
		InitialDelay: time.Microsecond,
	})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			_, _ = executor.Execute(func() (interface{}, error) {
				if i%3 != 0 { // Fail 2 out of 3 times, succeed on retry
					return nil, errors.New("retryable")
				}
				return "success", nil
			})
		}
	})
}