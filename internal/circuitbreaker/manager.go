package circuitbreaker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Manager manages multiple circuit breakers for different services
type Manager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewManager creates a new circuit breaker manager
func NewManager() *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// RegisterBreaker registers a circuit breaker for a service
func (m *Manager) RegisterBreaker(serviceName string, config Config) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	config.Name = serviceName
	m.breakers[serviceName] = New(config)
}

// GetBreaker returns the circuit breaker for a service
func (m *Manager) GetBreaker(serviceName string) (*CircuitBreaker, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	breaker, exists := m.breakers[serviceName]
	return breaker, exists
}

// Execute executes a function with the circuit breaker for the given service
func (m *Manager) Execute(serviceName string, fn func() (interface{}, error)) (interface{}, error) {
	breaker, exists := m.GetBreaker(serviceName)
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for service: %s", serviceName)
	}

	return breaker.Execute(fn)
}

// ExecuteContext executes a function with context and circuit breaker for the given service
func (m *Manager) ExecuteContext(ctx context.Context, serviceName string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	breaker, exists := m.GetBreaker(serviceName)
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for service: %s", serviceName)
	}

	return breaker.ExecuteContext(ctx, fn)
}

// GetStatus returns the status of all circuit breakers
func (m *Manager) GetStatus() map[string]BreakerStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := make(map[string]BreakerStatus)
	for name, breaker := range m.breakers {
		status[name] = BreakerStatus{
			Name:   breaker.Name(),
			State:  breaker.State(),
			Counts: breaker.Counts(),
		}
	}
	return status
}

// BreakerStatus represents the status of a circuit breaker
type BreakerStatus struct {
	Name   string `json:"name"`
	State  State  `json:"state"`
	Counts Counts `json:"counts"`
}

// DefaultServiceConfigs returns default configurations for common services
func DefaultServiceConfigs() map[string]Config {
	return map[string]Config{
		"vault": {
			MaxRequests:      3,
			Interval:         30 * time.Second,
			Timeout:          60 * time.Second,
			FailureThreshold: 5,
			SuccessThreshold: 2,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 5 || counts.FailureRate() > 0.6
			},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		},
		"argocd": {
			MaxRequests:      3,
			Interval:         30 * time.Second,
			Timeout:          60 * time.Second,
			FailureThreshold: 3,
			SuccessThreshold: 2,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3 || counts.FailureRate() > 0.5
			},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		},
		"kubernetes": {
			MaxRequests:      5,
			Interval:         60 * time.Second,
			Timeout:          120 * time.Second,
			FailureThreshold: 3,
			SuccessThreshold: 3,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3 || counts.FailureRate() > 0.4
			},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		},
		"git": {
			MaxRequests:      2,
			Interval:         45 * time.Second,
			Timeout:          90 * time.Second,
			FailureThreshold: 4,
			SuccessThreshold: 2,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 4 || counts.FailureRate() > 0.6
			},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		},
		"newrelic": {
			MaxRequests:      3,
			Interval:         30 * time.Second,
			Timeout:          60 * time.Second,
			FailureThreshold: 5,
			SuccessThreshold: 2,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 5 || counts.FailureRate() > 0.7
			},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		},
	}
}

// HTTPErrorClassifier classifies HTTP errors for circuit breaker decisions
func HTTPErrorClassifier(err error) bool {
	if err == nil {
		return true
	}

	// Check if it's an HTTP error that should be retried
	if httpErr, ok := err.(*HTTPError); ok {
		// 4xx errors (except 429) should not trip the circuit breaker
		// They indicate client errors, not server problems
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 && httpErr.StatusCode != 429 {
			return true // Don't count as failure for circuit breaker
		}
		
		// 5xx errors and network errors should trip the circuit breaker
		return false
	}

	// Network errors, timeouts, etc. should trip the circuit breaker
	errStr := strings.ToLower(err.Error())
	networkErrors := []string{
		"connection refused",
		"timeout",
		"no such host",
		"network unreachable",
		"connection reset",
		"broken pipe",
		"i/o timeout",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errStr, netErr) {
			return false // Network error, count as failure
		}
	}

	// Unknown error, be conservative and count as failure
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// WrapHTTPResponse wraps an HTTP response to create appropriate errors
func WrapHTTPResponse(resp *http.Response, err error) error {
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return NewHTTPError(resp.StatusCode, resp.Status)
	}

	return nil
}

// RetryConfig defines retry behavior for circuit breaker
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	BaseDelay       time.Duration `json:"base_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	Multiplier      float64       `json:"multiplier"`
	Jitter          bool          `json:"jitter"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
		RetryableErrors: []string{
			"connection refused",
			"timeout",
			"network unreachable",
			"temporary failure",
		},
	}
}