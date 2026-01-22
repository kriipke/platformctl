package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreaker(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected *CircuitBreaker
	}{
		{
			name:   "default configuration",
			config: Config{},
			expected: &CircuitBreaker{
				name:        "CircuitBreaker",
				maxRequests: 1,
				interval:    time.Minute,
				timeout:     60 * time.Second,
			},
		},
		{
			name: "custom configuration",
			config: Config{
				Name:             "TestBreaker",
				MaxRequests:      5,
				Interval:         30 * time.Second,
				Timeout:          10 * time.Second,
				FailureThreshold: 3,
				ShouldTrip: func(counts Counts) bool {
					return counts.ConsecutiveFailures >= 3
				},
			},
			expected: &CircuitBreaker{
				name:        "TestBreaker",
				maxRequests: 5,
				interval:    30 * time.Second,
				timeout:     10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.config)

			assert.Equal(t, tt.expected.name, cb.name)
			assert.Equal(t, tt.expected.maxRequests, cb.maxRequests)
			assert.Equal(t, tt.expected.interval, cb.interval)
			assert.Equal(t, tt.expected.timeout, cb.timeout)
			assert.NotNil(t, cb.shouldTrip)
			assert.NotNil(t, cb.isSuccessful)
			assert.Equal(t, StateClosed, cb.State())
		})
	}
}

func TestCircuitBreakerStateClosed(t *testing.T) {
	cb := New(Config{
		Name:        "TestBreaker",
		MaxRequests: 3,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	})

	// Test successful requests
	for i := 0; i < 5; i++ {
		result, err := cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.Equal(t, StateClosed, cb.State())
	}

	counts := cb.Counts()
	assert.Equal(t, uint32(5), counts.Requests)
	assert.Equal(t, uint32(5), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	assert.Equal(t, uint32(5), counts.ConsecutiveSuccesses)
	assert.Equal(t, uint32(0), counts.ConsecutiveFailures)
}

func TestCircuitBreakerStateTransition(t *testing.T) {
	stateChanges := make([]string, 0)
	mu := sync.Mutex{}

	cb := New(Config{
		Name:        "TestBreaker",
		MaxRequests: 2,
		Timeout:     100 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(name string, from State, to State) {
			mu.Lock()
			stateChanges = append(stateChanges, from.String()+"->"+to.String())
			mu.Unlock()
		},
	})

	// Initial state should be closed
	assert.Equal(t, StateClosed, cb.State())

	// Trigger failures to open circuit
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		assert.Error(t, err)
	}

	// Circuit should be open now
	assert.Equal(t, StateOpen, cb.State())

	// Requests should be rejected immediately
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.Equal(t, ErrCircuitOpen, err)

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	// First request in half-open should succeed and close circuit
	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Wait a moment for state change to process
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, StateClosed, cb.State())

	// Verify state transitions
	mu.Lock()
	expectedTransitions := []string{"closed->open", "open->half-open", "half-open->closed"}
	assert.Equal(t, expectedTransitions, stateChanges)
	mu.Unlock()
}

func TestCircuitBreakerHalfOpenMaxRequests(t *testing.T) {
	cb := New(Config{
		Name:        "TestBreaker",
		MaxRequests: 2,
		Timeout:     50 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	})

	// Trigger failure to open circuit
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.State())

	// Wait for transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	// First two requests should be allowed
	result1, err1 := cb.Execute(func() (interface{}, error) {
		return "success1", nil
	})
	assert.NoError(t, err1)
	assert.Equal(t, "success1", result1)

	result2, err2 := cb.Execute(func() (interface{}, error) {
		return "success2", nil
	})
	assert.NoError(t, err2)
	assert.Equal(t, "success2", result2)

	// Circuit should be closed now after maxRequests successes
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreakerHalfOpenTooManyRequests(t *testing.T) {
	cb := New(Config{
		Name:        "TestBreaker",
		MaxRequests: 1,
		Timeout:     50 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	})

	// Trigger failure to open circuit
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.State())

	// Wait for transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	// Start first request (simulate slow request)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := cb.Execute(func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "slow success", nil
		})
		assert.NoError(t, err)
	}()

	// Second request should be rejected with too many requests
	time.Sleep(10 * time.Millisecond) // Ensure first request has started
	_, err = cb.Execute(func() (interface{}, error) {
		return "quick", nil
	})
	assert.Equal(t, ErrTooManyRequests, err)

	wg.Wait()
}

func TestCircuitBreakerExecuteContext(t *testing.T) {
	cb := New(Config{Name: "TestBreaker"})
	ctx := context.Background()

	// Test successful execution with context
	result, err := cb.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = cb.ExecuteContext(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, ctx.Err()
	})
	assert.Equal(t, context.Canceled, err)
}

func TestCircuitBreakerFailureThreshold(t *testing.T) {
	tests := []struct {
		name           string
		failures       int
		shouldTrip     func(counts Counts) bool
		expectedState  State
		expectedClosed bool
	}{
		{
			name:     "below failure threshold",
			failures: 2,
			shouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
			expectedState:  StateClosed,
			expectedClosed: true,
		},
		{
			name:     "at failure threshold",
			failures: 3,
			shouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
			expectedState:  StateOpen,
			expectedClosed: false,
		},
		{
			name:     "failure rate threshold",
			failures: 3,
			shouldTrip: func(counts Counts) bool {
				return counts.Requests >= 5 && counts.FailureRate() >= 0.6
			},
			expectedState:  StateOpen,
			expectedClosed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(Config{
				Name:       "TestBreaker",
				ShouldTrip: tt.shouldTrip,
			})

			// Execute some successful requests first for failure rate test
			if tt.name == "failure rate threshold" {
				for i := 0; i < 2; i++ {
					_, err := cb.Execute(func() (interface{}, error) {
						return "success", nil
					})
					assert.NoError(t, err)
				}
			}

			// Execute failure requests
			for i := 0; i < tt.failures; i++ {
				_, err := cb.Execute(func() (interface{}, error) {
					return nil, errors.New("failure")
				})
				assert.Error(t, err)
			}

			assert.Equal(t, tt.expectedState, cb.State())

			// Test next request behavior
			_, err := cb.Execute(func() (interface{}, error) {
				return "test", nil
			})

			if tt.expectedClosed {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, ErrCircuitOpen, err)
			}
		})
	}
}

func TestCircuitBreakerIsSuccessful(t *testing.T) {
	customError := errors.New("custom error")
	ignoredError := errors.New("ignored error")

	cb := New(Config{
		Name: "TestBreaker",
		IsSuccessful: func(err error) bool {
			return err == nil || err == ignoredError
		},
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	})

	// Success case
	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Ignored error case (should be treated as success)
	result, err = cb.Execute(func() (interface{}, error) {
		return "ignored", ignoredError
	})
	assert.Equal(t, ignoredError, err)
	assert.Equal(t, "ignored", result)

	counts := cb.Counts()
	assert.Equal(t, uint32(2), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	assert.Equal(t, StateClosed, cb.State())

	// Actual failure case
	for i := 0; i < 2; i++ {
		_, err = cb.Execute(func() (interface{}, error) {
			return nil, customError
		})
		assert.Equal(t, customError, err)
	}

	// Circuit should be open due to consecutive failures
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreakerPanicRecovery(t *testing.T) {
	cb := New(Config{Name: "TestBreaker"})

	// Test panic recovery
	assert.Panics(t, func() {
		_, _ = cb.Execute(func() (interface{}, error) {
			panic("test panic")
		})
	})

	// Circuit breaker should still function after panic
	counts := cb.Counts()
	assert.Equal(t, uint32(1), counts.TotalFailures)

	// Should still be able to execute successful requests
	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := New(Config{
		Name:        "ConcurrencyTest",
		MaxRequests: 5,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 10
		},
	})

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines*requestsPerGoroutine)

	// Start multiple goroutines making requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				_, err := cb.Execute(func() (interface{}, error) {
					// Randomly succeed or fail
					if (id*requestsPerGoroutine+j)%3 == 0 {
						return nil, errors.New("failure")
					}
					return "success", nil
				})
				results <- err == nil
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Count results
	var successes, failures int
	for success := range results {
		if success {
			successes++
		} else {
			failures++
		}
	}

	// Verify the circuit breaker recorded some requests
	counts := cb.Counts()
	assert.True(t, counts.Requests > 0, "Circuit breaker should have recorded requests")
	assert.True(t, counts.TotalSuccesses > 0, "Should have some successes")

	t.Logf("Processed %d total requests (%d successes, %d failures)", successes+failures, successes, failures)
	t.Logf("Circuit breaker counts: %+v", counts)
	t.Logf("Final state: %s", cb.State())
}

func TestCircuitBreakerGeneration(t *testing.T) {
	cb := New(Config{
		Name:     "GenerationTest",
		Interval: 50 * time.Millisecond,
	})

	// Make some requests
	for i := 0; i < 3; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	initialCounts := cb.Counts()
	assert.Equal(t, uint32(3), initialCounts.Requests)

	// Wait for interval to pass (generation should reset)
	time.Sleep(60 * time.Millisecond)

	// Make another request (this should be in new generation)
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)

	newCounts := cb.Counts()
	// In new generation, request count should reset
	assert.Equal(t, uint32(1), newCounts.Requests)
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateHalfOpen, "half-open"},
		{StateOpen, "open"},
		{State(999), "unknown state: 999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestCountsFailureRate(t *testing.T) {
	tests := []struct {
		name     string
		counts   Counts
		expected float64
	}{
		{
			name:     "no requests",
			counts:   Counts{},
			expected: 0.0,
		},
		{
			name: "all successes",
			counts: Counts{
				Requests:       10,
				TotalSuccesses: 10,
				TotalFailures:  0,
			},
			expected: 0.0,
		},
		{
			name: "all failures",
			counts: Counts{
				Requests:       10,
				TotalSuccesses: 0,
				TotalFailures:  10,
			},
			expected: 1.0,
		},
		{
			name: "mixed results",
			counts: Counts{
				Requests:       10,
				TotalSuccesses: 7,
				TotalFailures:  3,
			},
			expected: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.counts.FailureRate())
		})
	}
}

func TestCircuitBreakerTimeout(t *testing.T) {
	cb := New(Config{
		Name:    "TimeoutTest",
		Timeout: 50 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	})

	// Trigger circuit to open
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.State())

	// Requests should be rejected immediately
	_, err = cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.Equal(t, ErrCircuitOpen, err)

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow request
	assert.Equal(t, StateHalfOpen, cb.State())
	result, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Should transition back to closed
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, StateClosed, cb.State())
}

// Benchmark tests
func BenchmarkCircuitBreakerExecute(b *testing.B) {
	cb := New(Config{Name: "BenchmarkTest"})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cb.Execute(func() (interface{}, error) {
				return "success", nil
			})
		}
	})
}

func BenchmarkCircuitBreakerExecuteWithFailures(b *testing.B) {
	cb := New(Config{
		Name: "BenchmarkFailureTest",
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1000 // High threshold to avoid opening during benchmark
		},
	})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			_, _ = cb.Execute(func() (interface{}, error) {
				if i%10 == 0 {
					return nil, errors.New("failure")
				}
				return "success", nil
			})
		}
	})
}