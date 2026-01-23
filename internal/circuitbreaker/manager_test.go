package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.breakers)
	assert.NotNil(t, manager.configs)
}

func TestManagerRegisterService(t *testing.T) {
	manager := NewManager()
	config := Config{
		Name:        "TestService",
		MaxRequests: 5,
		Timeout:     30 * time.Second,
	}

	err := manager.RegisterService("test-service", config)
	assert.NoError(t, err)

	// Verify the service was registered
	breaker := manager.GetBreaker("test-service")
	assert.NotNil(t, breaker)
	assert.Equal(t, "TestService", breaker.Name())
}

func TestManagerRegisterServiceDuplicate(t *testing.T) {
	manager := NewManager()
	config := Config{Name: "TestService"}

	// Register service first time
	err := manager.RegisterService("test-service", config)
	assert.NoError(t, err)

	// Register same service again - should return error
	err = manager.RegisterService("test-service", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestManagerGetBreakerNonExistent(t *testing.T) {
	manager := NewManager()

	breaker := manager.GetBreaker("non-existent")
	assert.Nil(t, breaker)
}

func TestManagerExecute(t *testing.T) {
	manager := NewManager()
	config := Config{
		Name: "TestService",
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	// Test successful execution
	result, err := manager.Execute("test-service", func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestManagerExecuteNonExistentService(t *testing.T) {
	manager := NewManager()

	result, err := manager.Execute("non-existent", func() (interface{}, error) {
		return "success", nil
	})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerExecuteWithCircuitOpen(t *testing.T) {
	manager := NewManager()
	config := Config{
		Name:    "TestService",
		Timeout: 50 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	}

	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	// Trigger circuit to open
	_, err = manager.Execute("test-service", func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)

	// Circuit should be open, next request should fail immediately
	_, err = manager.Execute("test-service", func() (interface{}, error) {
		return "success", nil
	})
	assert.Equal(t, ErrCircuitOpen, err)
}

func TestManagerGetAllBreakers(t *testing.T) {
	manager := NewManager()

	// Register multiple services
	services := []string{"service1", "service2", "service3"}
	for _, service := range services {
		config := Config{Name: service}
		err := manager.RegisterService(service, config)
		require.NoError(t, err)
	}

	breakers := manager.GetAllBreakers()
	assert.Len(t, breakers, len(services))

	// Verify all services are present
	for _, service := range services {
		_, exists := breakers[service]
		assert.True(t, exists, "Service %s should exist in breakers map", service)
	}
}

func TestManagerUnregisterService(t *testing.T) {
	manager := NewManager()
	config := Config{Name: "TestService"}

	// Register service
	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	// Verify service exists
	breaker := manager.GetBreaker("test-service")
	assert.NotNil(t, breaker)

	// Unregister service
	err = manager.UnregisterService("test-service")
	assert.NoError(t, err)

	// Verify service is removed
	breaker = manager.GetBreaker("test-service")
	assert.Nil(t, breaker)

	// Try to unregister again - should return error
	err = manager.UnregisterService("test-service")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerUpdateConfig(t *testing.T) {
	manager := NewManager()
	originalConfig := Config{
		Name:        "TestService",
		MaxRequests: 3,
		Timeout:     30 * time.Second,
	}

	// Register service
	err := manager.RegisterService("test-service", originalConfig)
	require.NoError(t, err)

	// Update config
	newConfig := Config{
		Name:        "TestService",
		MaxRequests: 10,
		Timeout:     60 * time.Second,
	}
	err = manager.UpdateConfig("test-service", newConfig)
	assert.NoError(t, err)

	// Verify config was updated by checking if new breaker was created
	breaker := manager.GetBreaker("test-service")
	assert.NotNil(t, breaker)
	// Note: We can't directly test internal config values, but we know a new breaker was created
}

func TestManagerUpdateConfigNonExistent(t *testing.T) {
	manager := NewManager()
	config := Config{Name: "TestService"}

	err := manager.UpdateConfig("non-existent", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerGetServiceStats(t *testing.T) {
	manager := NewManager()
	config := Config{Name: "TestService"}

	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	// Execute some requests
	for i := 0; i < 3; i++ {
		_, err := manager.Execute("test-service", func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	// Execute a failure
	_, err = manager.Execute("test-service", func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)

	stats, err := manager.GetServiceStats("test-service")
	assert.NoError(t, err)
	assert.Equal(t, "test-service", stats.ServiceName)
	assert.Equal(t, "closed", stats.State)
	assert.Equal(t, uint32(4), stats.TotalRequests)
	assert.Equal(t, uint32(3), stats.TotalSuccesses)
	assert.Equal(t, uint32(1), stats.TotalFailures)
	assert.NotZero(t, stats.LastUpdated)
}

func TestManagerGetServiceStatsNonExistent(t *testing.T) {
	manager := NewManager()

	stats, err := manager.GetServiceStats("non-existent")
	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerGetAllStats(t *testing.T) {
	manager := NewManager()

	// Register multiple services
	services := []string{"service1", "service2", "service3"}
	for _, service := range services {
		config := Config{Name: service}
		err := manager.RegisterService(service, config)
		require.NoError(t, err)

		// Execute a request for each service
		_, err = manager.Execute(service, func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	stats := manager.GetAllStats()
	assert.Len(t, stats, len(services))

	for _, stat := range stats {
		assert.Contains(t, services, stat.ServiceName)
		assert.Equal(t, "closed", stat.State)
		assert.Equal(t, uint32(1), stat.TotalRequests)
		assert.Equal(t, uint32(1), stat.TotalSuccesses)
		assert.Equal(t, uint32(0), stat.TotalFailures)
	}
}

func TestManagerConcurrentOperations(t *testing.T) {
	manager := NewManager()
	const numServices = 10
	const numGoroutines = 5
	const requestsPerGoroutine = 20

	// Register services
	for i := 0; i < numServices; i++ {
		serviceName := fmt.Sprintf("service-%d", i)
		config := Config{
			Name: serviceName,
			ShouldTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 5
			},
		}
		err := manager.RegisterService(serviceName, config)
		require.NoError(t, err)
	}

	var wg sync.WaitGroup

	// Start goroutines to make concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				serviceName := fmt.Sprintf("service-%d", j%numServices)
				_, err := manager.Execute(serviceName, func() (interface{}, error) {
					// Randomly succeed or fail
					if (goroutineID*requestsPerGoroutine+j)%7 == 0 {
						return nil, errors.New("failure")
					}
					return "success", nil
				})
				// We don't assert here to avoid cluttering the output
				_ = err
			}
		}(i)
	}

	wg.Wait()

	// Verify all services still exist and have recorded requests
	stats := manager.GetAllStats()
	assert.Len(t, stats, numServices)

	totalRequests := uint32(0)
	for _, stat := range stats {
		assert.True(t, stat.TotalRequests > 0, "Service %s should have processed requests", stat.ServiceName)
		totalRequests += stat.TotalRequests
	}

	expectedRequests := uint32(numGoroutines * requestsPerGoroutine)
	assert.Equal(t, expectedRequests, totalRequests, "Total requests should match expected")
}

func TestManagerStateChangeCallbacks(t *testing.T) {
	var stateChanges []string
	var mu sync.Mutex

	manager := NewManager()
	config := Config{
		Name:    "TestService",
		Timeout: 50 * time.Millisecond,
		ShouldTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(name string, from State, to State) {
			mu.Lock()
			stateChanges = append(stateChanges, fmt.Sprintf("%s: %s->%s", name, from.String(), to.String()))
			mu.Unlock()
		},
	}

	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	// Trigger failures to open circuit
	for i := 0; i < 2; i++ {
		_, err := manager.Execute("test-service", func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		assert.Error(t, err)
	}

	// Wait for timeout and recovery
	time.Sleep(60 * time.Millisecond)

	// Execute successful request to close circuit
	_, err = manager.Execute("test-service", func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)

	// Wait for state changes to be processed
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	assert.Contains(t, stateChanges, "TestService: closed->open")
	assert.Contains(t, stateChanges, "TestService: open->half-open")
	assert.Contains(t, stateChanges, "TestService: half-open->closed")
	mu.Unlock()
}

func TestManagerHealthCheck(t *testing.T) {
	manager := NewManager()
	
	// Register services with different states
	configs := []struct {
		name   string
		config Config
	}{
		{
			name: "healthy-service",
			config: Config{
				Name: "HealthyService",
			},
		},
		{
			name: "unhealthy-service",
			config: Config{
				Name: "UnhealthyService",
				ShouldTrip: func(counts Counts) bool {
					return counts.ConsecutiveFailures >= 1
				},
			},
		},
	}

	for _, cfg := range configs {
		err := manager.RegisterService(cfg.name, cfg.config)
		require.NoError(t, err)
	}

	// Make healthy service succeed
	_, err := manager.Execute("healthy-service", func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)

	// Make unhealthy service fail and open circuit
	_, err = manager.Execute("unhealthy-service", func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	assert.Error(t, err)

	// Check health
	health := manager.HealthCheck()
	assert.False(t, health.Healthy, "Manager should be unhealthy with open circuits")
	assert.Len(t, health.Services, 2)

	// Find the unhealthy service
	var unhealthyService *ServiceHealth
	for i := range health.Services {
		if health.Services[i].ServiceName == "unhealthy-service" {
			unhealthyService = &health.Services[i]
			break
		}
	}

	require.NotNil(t, unhealthyService, "Should find unhealthy service")
	assert.False(t, unhealthyService.Healthy)
	assert.Equal(t, "open", unhealthyService.State)
	assert.True(t, unhealthyService.FailureRate > 0)
}

func TestManagerReset(t *testing.T) {
	manager := NewManager()
	config := Config{Name: "TestService"}

	// Register service and execute requests
	err := manager.RegisterService("test-service", config)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := manager.Execute("test-service", func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	// Verify requests were recorded
	stats, err := manager.GetServiceStats("test-service")
	require.NoError(t, err)
	assert.Equal(t, uint32(3), stats.TotalRequests)

	// Reset specific service
	err = manager.Reset("test-service")
	assert.NoError(t, err)

	// Verify stats were reset
	stats, err = manager.GetServiceStats("test-service")
	require.NoError(t, err)
	assert.Equal(t, uint32(0), stats.TotalRequests)
}

func TestManagerResetNonExistent(t *testing.T) {
	manager := NewManager()

	err := manager.Reset("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerResetAll(t *testing.T) {
	manager := NewManager()

	// Register multiple services
	services := []string{"service1", "service2", "service3"}
	for _, service := range services {
		config := Config{Name: service}
		err := manager.RegisterService(service, config)
		require.NoError(t, err)

		// Execute requests
		_, err = manager.Execute(service, func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	// Reset all services
	manager.ResetAll()

	// Verify all services were reset
	for _, service := range services {
		stats, err := manager.GetServiceStats(service)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), stats.TotalRequests, "Service %s should be reset", service)
	}
}

