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
	configs  map[string]Config
	stats    map[string]*serviceStats
	mutex    sync.RWMutex
}

// serviceStats tracks cumulative request statistics for a service. The
// underlying circuit breaker resets its own counts whenever it changes
// generation (e.g. when it trips open), so the manager keeps independent
// running totals that survive state transitions.
type serviceStats struct {
	mu          sync.Mutex
	requests    uint32
	successes   uint32
	failures    uint32
	lastUpdated time.Time
}

func (s *serviceStats) record(success bool, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	if success {
		s.successes++
	} else {
		s.failures++
	}
	s.lastUpdated = now
}

func (s *serviceStats) snapshot() (requests, successes, failures uint32, lastUpdated time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests, s.successes, s.failures, s.lastUpdated
}

// NewManager creates a new circuit breaker manager
func NewManager() *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		configs:  make(map[string]Config),
		stats:    make(map[string]*serviceStats),
	}
}

// RegisterBreaker registers a circuit breaker for a service. It overwrites any
// existing breaker registered under the same name.
func (m *Manager) RegisterBreaker(serviceName string, config Config) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	config.Name = serviceName
	m.breakers[serviceName] = New(config)
	m.configs[serviceName] = config
	m.stats[serviceName] = &serviceStats{lastUpdated: time.Now()}
}

// RegisterService registers a circuit breaker for a service, returning an error
// if a breaker is already registered under the same name.
func (m *Manager) RegisterService(serviceName string, config Config) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.breakers[serviceName]; exists {
		return fmt.Errorf("service %q is already registered", serviceName)
	}

	m.breakers[serviceName] = New(config)
	m.configs[serviceName] = config
	m.stats[serviceName] = &serviceStats{lastUpdated: time.Now()}
	return nil
}

// UnregisterService removes a registered service, returning an error if it does
// not exist.
func (m *Manager) UnregisterService(serviceName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.breakers[serviceName]; !exists {
		return fmt.Errorf("service %q not found", serviceName)
	}

	delete(m.breakers, serviceName)
	delete(m.configs, serviceName)
	delete(m.stats, serviceName)
	return nil
}

// UpdateConfig replaces the circuit breaker for an existing service with one
// built from the supplied config, returning an error if the service does not
// exist.
func (m *Manager) UpdateConfig(serviceName string, config Config) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.breakers[serviceName]; !exists {
		return fmt.Errorf("service %q not found", serviceName)
	}

	m.breakers[serviceName] = New(config)
	m.configs[serviceName] = config
	return nil
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
	m.mutex.RLock()
	breaker, exists := m.breakers[serviceName]
	stats := m.stats[serviceName]
	m.mutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for service: %s", serviceName)
	}

	result, err := breaker.Execute(fn)
	if stats != nil {
		stats.record(err == nil, time.Now())
	}
	return result, err
}

// ExecuteContext executes a function with context and circuit breaker for the given service
func (m *Manager) ExecuteContext(ctx context.Context, serviceName string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	m.mutex.RLock()
	breaker, exists := m.breakers[serviceName]
	stats := m.stats[serviceName]
	m.mutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("circuit breaker not found for service: %s", serviceName)
	}

	result, err := breaker.ExecuteContext(ctx, fn)
	if stats != nil {
		stats.record(err == nil, time.Now())
	}
	return result, err
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

// GetAllBreakers returns a snapshot map of all registered circuit breakers
// keyed by service name.
func (m *Manager) GetAllBreakers() map[string]*CircuitBreaker {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	breakers := make(map[string]*CircuitBreaker, len(m.breakers))
	for name, breaker := range m.breakers {
		breakers[name] = breaker
	}
	return breakers
}

// ServiceStats holds cumulative request statistics for a registered service.
type ServiceStats struct {
	ServiceName    string    `json:"service_name"`
	State          string    `json:"state"`
	TotalRequests  uint32    `json:"total_requests"`
	TotalSuccesses uint32    `json:"total_successes"`
	TotalFailures  uint32    `json:"total_failures"`
	LastUpdated    time.Time `json:"last_updated"`
}

// GetServiceStats returns cumulative statistics for a single service, returning
// an error if the service is not registered.
func (m *Manager) GetServiceStats(serviceName string) (*ServiceStats, error) {
	m.mutex.RLock()
	breaker, exists := m.breakers[serviceName]
	st := m.stats[serviceName]
	m.mutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("service %q not found", serviceName)
	}

	requests, successes, failures, lastUpdated := st.snapshot()
	return &ServiceStats{
		ServiceName:    serviceName,
		State:          breaker.State().String(),
		TotalRequests:  requests,
		TotalSuccesses: successes,
		TotalFailures:  failures,
		LastUpdated:    lastUpdated,
	}, nil
}

// GetAllStats returns cumulative statistics for every registered service.
func (m *Manager) GetAllStats() []ServiceStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make([]ServiceStats, 0, len(m.breakers))
	for name, breaker := range m.breakers {
		requests, successes, failures, lastUpdated := m.stats[name].snapshot()
		stats = append(stats, ServiceStats{
			ServiceName:    name,
			State:          breaker.State().String(),
			TotalRequests:  requests,
			TotalSuccesses: successes,
			TotalFailures:  failures,
			LastUpdated:    lastUpdated,
		})
	}
	return stats
}

// ServiceHealth describes the health of a single service's circuit breaker.
type ServiceHealth struct {
	ServiceName string  `json:"service_name"`
	Healthy     bool    `json:"healthy"`
	State       string  `json:"state"`
	FailureRate float64 `json:"failure_rate"`
}

// HealthStatus aggregates the health of every registered service.
type HealthStatus struct {
	Healthy  bool            `json:"healthy"`
	Services []ServiceHealth `json:"services"`
}

// HealthCheck reports the health of all registered services. The manager is
// considered healthy only when every breaker is closed.
func (m *Manager) HealthCheck() HealthStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := HealthStatus{
		Healthy:  true,
		Services: make([]ServiceHealth, 0, len(m.breakers)),
	}

	for name, breaker := range m.breakers {
		state := breaker.State()
		requests, _, failures, _ := m.stats[name].snapshot()

		failureRate := 0.0
		if requests > 0 {
			failureRate = float64(failures) / float64(requests)
		}

		healthy := state == StateClosed
		if !healthy {
			status.Healthy = false
		}

		status.Services = append(status.Services, ServiceHealth{
			ServiceName: name,
			Healthy:     healthy,
			State:       state.String(),
			FailureRate: failureRate,
		})
	}

	return status
}

// Reset recreates the circuit breaker for a service from its stored config and
// clears its cumulative statistics, returning an error if the service does not
// exist.
func (m *Manager) Reset(serviceName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	config, exists := m.configs[serviceName]
	if !exists {
		return fmt.Errorf("service %q not found", serviceName)
	}

	m.breakers[serviceName] = New(config)
	m.stats[serviceName] = &serviceStats{lastUpdated: time.Now()}
	return nil
}

// ResetAll recreates every registered circuit breaker and clears all
// cumulative statistics.
func (m *Manager) ResetAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for name, config := range m.configs {
		m.breakers[name] = New(config)
		m.stats[name] = &serviceStats{lastUpdated: time.Now()}
	}
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
