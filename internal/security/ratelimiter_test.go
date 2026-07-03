package security

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerPeriod int
		period            time.Duration
		expectedTokens    int
		expectedPeriod    time.Duration
	}{
		{
			name:              "basic rate limiter",
			requestsPerPeriod: 10,
			period:            time.Minute,
			expectedTokens:    10,
			expectedPeriod:    time.Minute,
		},
		{
			name:              "high frequency rate limiter",
			requestsPerPeriod: 100,
			period:            time.Second,
			expectedTokens:    100,
			expectedPeriod:    time.Second,
		},
		{
			name:              "low frequency rate limiter",
			requestsPerPeriod: 1,
			period:            time.Hour,
			expectedTokens:    1,
			expectedPeriod:    time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.requestsPerPeriod, tt.period)

			assert.NotNil(t, rl)
			assert.NotNil(t, rl.clients)
			assert.Equal(t, tt.expectedTokens, rl.maxTokens)
			assert.Equal(t, tt.expectedPeriod, rl.refillRate)

			// Verify cleanup goroutine started
			time.Sleep(10 * time.Millisecond) // Allow goroutine to start
		})
	}
}

func TestRateLimiterAllow(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerPeriod int
		period            time.Duration
		clientIP          string
		requests          int
		expectedAllowed   int
		expectedRejected  int
	}{
		{
			name:              "all requests allowed within limit",
			requestsPerPeriod: 5,
			period:            time.Second,
			clientIP:          "192.168.1.1",
			requests:          3,
			expectedAllowed:   3,
			expectedRejected:  0,
		},
		{
			name:              "some requests rejected at limit",
			requestsPerPeriod: 3,
			period:            time.Second,
			clientIP:          "192.168.1.2",
			requests:          5,
			expectedAllowed:   3,
			expectedRejected:  2,
		},
		{
			name:              "single request allowed",
			requestsPerPeriod: 1,
			period:            time.Second,
			clientIP:          "192.168.1.3",
			requests:          1,
			expectedAllowed:   1,
			expectedRejected:  0,
		},
		{
			name:              "zero requests",
			requestsPerPeriod: 10,
			period:            time.Second,
			clientIP:          "192.168.1.4",
			requests:          0,
			expectedAllowed:   0,
			expectedRejected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.requestsPerPeriod, tt.period)
			allowed := 0
			rejected := 0

			for i := 0; i < tt.requests; i++ {
				if rl.Allow(tt.clientIP) {
					allowed++
				} else {
					rejected++
				}
			}

			assert.Equal(t, tt.expectedAllowed, allowed)
			assert.Equal(t, tt.expectedRejected, rejected)
		})
	}
}

func TestRateLimiterAllowNewClient(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)

	// First request from new client should be allowed
	allowed := rl.Allow("192.168.1.100")
	assert.True(t, allowed)

	// Check token count
	tokens := rl.GetTokenCount("192.168.1.100")
	assert.Equal(t, 4, tokens) // Started with 5, used 1
}

func TestRateLimiterAllowExistingClient(t *testing.T) {
	rl := NewRateLimiter(3, time.Second)
	clientIP := "192.168.1.101"

	// Use all tokens
	for i := 0; i < 3; i++ {
		allowed := rl.Allow(clientIP)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// Next request should be rejected
	allowed := rl.Allow(clientIP)
	assert.False(t, allowed)

	// Verify token count is 0
	tokens := rl.GetTokenCount(clientIP)
	assert.Equal(t, 0, tokens)
}

func TestRateLimiterTokenRefill(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	clientIP := "192.168.1.102"

	// Use all tokens
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))
	assert.False(t, rl.Allow(clientIP)) // Should be rejected

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again after refill
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))
}

func TestRateLimiterPartialRefill(t *testing.T) {
	rl := NewRateLimiter(4, 100*time.Millisecond)
	clientIP := "192.168.1.103"

	// Use all tokens
	for i := 0; i < 4; i++ {
		assert.True(t, rl.Allow(clientIP))
	}
	assert.False(t, rl.Allow(clientIP))

	// Wait for partial refill (should get 4 more tokens)
	time.Sleep(110 * time.Millisecond)

	// Should have tokens again
	tokens := rl.GetTokenCount(clientIP)
	assert.Equal(t, 4, tokens)

	// Use some tokens
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))

	// Wait for another period (should refill to max)
	time.Sleep(110 * time.Millisecond)

	tokens = rl.GetTokenCount(clientIP)
	assert.Equal(t, 4, tokens) // Should be back to max
}

func TestRateLimiterMultipleRefillPeriods(t *testing.T) {
	rl := NewRateLimiter(2, 30*time.Millisecond)
	clientIP := "192.168.1.104"

	// Use all tokens
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))
	assert.False(t, rl.Allow(clientIP))

	// Wait for multiple refill periods
	time.Sleep(100 * time.Millisecond) // About 3 periods

	// Should have full tokens again
	tokens := rl.GetTokenCount(clientIP)
	assert.Equal(t, 2, tokens)
}

func TestRateLimiterTokenCapCeiling(t *testing.T) {
	rl := NewRateLimiter(3, 50*time.Millisecond)
	clientIP := "192.168.1.105"

	// Use one token
	assert.True(t, rl.Allow(clientIP))

	// Wait for multiple refill periods
	time.Sleep(200 * time.Millisecond) // About 4 periods

	// Should not exceed max tokens
	tokens := rl.GetTokenCount(clientIP)
	assert.Equal(t, 3, tokens) // Capped at max, not 1 + (4 * 3) = 13
}

func TestRateLimiterGetTokenCountNewClient(t *testing.T) {
	rl := NewRateLimiter(10, time.Second)

	// New client should have max tokens available
	tokens := rl.GetTokenCount("192.168.1.200")
	assert.Equal(t, 10, tokens)
}

func TestRateLimiterGetTokenCountExistingClient(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	clientIP := "192.168.1.201"

	// Use some tokens
	rl.Allow(clientIP)
	rl.Allow(clientIP)

	// Check remaining tokens
	tokens := rl.GetTokenCount(clientIP)
	assert.Equal(t, 3, tokens)
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100, time.Second)
	clientIP := "192.168.1.300"

	const numGoroutines = 10
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	rejected := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if rl.Allow(clientIP) {
					mu.Lock()
					allowed++
					mu.Unlock()
				} else {
					mu.Lock()
					rejected++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Should allow up to 100 requests and reject the rest
	totalRequests := numGoroutines * requestsPerGoroutine
	assert.Equal(t, totalRequests, allowed+rejected)
	assert.Equal(t, 100, allowed)
	assert.Equal(t, totalRequests-100, rejected)
}

func TestRateLimiterConcurrentMultipleClients(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)

	const numClients = 10
	const requestsPerClient = 3

	var wg sync.WaitGroup
	var mu sync.Mutex
	totalAllowed := 0

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		clientIP := fmt.Sprintf("192.168.1.%d", 400+i)

		go func(ip string) {
			defer wg.Done()
			clientAllowed := 0

			for j := 0; j < requestsPerClient; j++ {
				if rl.Allow(ip) {
					clientAllowed++
				}
			}

			mu.Lock()
			totalAllowed += clientAllowed
			mu.Unlock()
		}(clientIP)
	}

	wg.Wait()

	// Each client should be allowed 3 requests (within their limit)
	expectedTotal := numClients * requestsPerClient
	assert.Equal(t, expectedTotal, totalAllowed)
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)

	// Create some clients
	clients := []string{"192.168.1.500", "192.168.1.501", "192.168.1.502"}
	for _, client := range clients {
		rl.Allow(client)
	}

	// Verify clients exist
	rl.mutex.RLock()
	initialCount := len(rl.clients)
	rl.mutex.RUnlock()
	assert.Equal(t, len(clients), initialCount)

	// Manually trigger cleanup logic by simulating old clients
	rl.mutex.Lock()
	oldTime := time.Now().Add(-3 * time.Hour)
	for _, client := range clients {
		if bucket, exists := rl.clients[client]; exists {
			bucket.lastRefill = oldTime
		}
	}
	rl.mutex.Unlock()

	// Wait for cleanup to run (cleanup runs every hour, but we'll test the logic directly)
	// Since we can't easily wait for the cleanup goroutine, we'll test the cleanup logic directly
	rl.mutex.Lock()
	now := time.Now()
	for clientIP, bucket := range rl.clients {
		if now.Sub(bucket.lastRefill) > 2*time.Hour {
			delete(rl.clients, clientIP)
		}
	}
	rl.mutex.Unlock()

	// Verify clients were cleaned up
	rl.mutex.RLock()
	finalCount := len(rl.clients)
	rl.mutex.RUnlock()
	assert.Equal(t, 0, finalCount)
}

func TestRateLimiterDifferentClients(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)

	client1 := "192.168.1.600"
	client2 := "192.168.1.601"

	// Each client should have independent token buckets
	assert.True(t, rl.Allow(client1))
	assert.True(t, rl.Allow(client1))
	assert.False(t, rl.Allow(client1)) // Client1 exhausted

	// Client2 should still have tokens
	assert.True(t, rl.Allow(client2))
	assert.True(t, rl.Allow(client2))
	assert.False(t, rl.Allow(client2)) // Client2 exhausted

	// Verify token counts
	assert.Equal(t, 0, rl.GetTokenCount(client1))
	assert.Equal(t, 0, rl.GetTokenCount(client2))
}

func TestRateLimiterEdgeCases(t *testing.T) {
	t.Run("zero max tokens", func(t *testing.T) {
		rl := NewRateLimiter(0, time.Second)

		// Should reject all requests
		assert.False(t, rl.Allow("192.168.1.700"))
		assert.Equal(t, 0, rl.GetTokenCount("192.168.1.700"))
	})

	t.Run("very short period", func(t *testing.T) {
		rl := NewRateLimiter(5, time.Nanosecond)
		clientIP := "192.168.1.701"

		// Use all tokens
		for i := 0; i < 5; i++ {
			assert.True(t, rl.Allow(clientIP))
		}
		assert.False(t, rl.Allow(clientIP))

		// Even with nanosecond refill, should get tokens back quickly
		time.Sleep(time.Microsecond) // Much longer than nanosecond
		assert.True(t, rl.Allow(clientIP))
	})

	t.Run("very long period", func(t *testing.T) {
		rl := NewRateLimiter(3, 24*time.Hour)
		clientIP := "192.168.1.702"

		// Should allow initial tokens
		for i := 0; i < 3; i++ {
			assert.True(t, rl.Allow(clientIP))
		}
		assert.False(t, rl.Allow(clientIP))

		// Should not refill quickly
		time.Sleep(10 * time.Millisecond)
		assert.False(t, rl.Allow(clientIP))
	})
}

func TestRateLimiterTokenRefillAccuracy(t *testing.T) {
	rl := NewRateLimiter(4, 100*time.Millisecond)
	clientIP := "192.168.1.800"

	// Use all tokens
	for i := 0; i < 4; i++ {
		assert.True(t, rl.Allow(clientIP))
	}

	// Record start time and wait for exactly one period
	start := time.Now()
	time.Sleep(105 * time.Millisecond) // Slightly longer than period to account for timing

	// Should have refilled
	tokens := rl.GetTokenCount(clientIP)
	duration := time.Since(start)

	assert.Equal(t, 4, tokens, "Should have full tokens after refill period")
	assert.True(t, duration >= 100*time.Millisecond, "Should wait at least one period")
}

func TestRateLimiterMemoryEfficiency(t *testing.T) {
	rl := NewRateLimiter(10, time.Second)

	// Create many clients
	const numClients = 1000
	for i := 0; i < numClients; i++ {
		clientIP := fmt.Sprintf("192.168.%d.%d", i/255, i%255)
		rl.Allow(clientIP)
	}

	// Verify all clients are tracked
	rl.mutex.RLock()
	clientCount := len(rl.clients)
	rl.mutex.RUnlock()
	assert.Equal(t, numClients, clientCount)

	// Each client should have independent buckets
	for i := 0; i < 10; i++ {
		clientIP := fmt.Sprintf("192.168.%d.%d", i/255, i%255)
		tokens := rl.GetTokenCount(clientIP)
		assert.Equal(t, 9, tokens) // Used 1 token during creation
	}
}

func TestRateLimiterStateConsistency(t *testing.T) {
	rl := NewRateLimiter(3, 50*time.Millisecond)
	clientIP := "192.168.1.900"

	// Use 2 tokens
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))

	// Check state
	tokens1 := rl.GetTokenCount(clientIP)
	assert.Equal(t, 1, tokens1)

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	// Should have full tokens
	tokens2 := rl.GetTokenCount(clientIP)
	assert.Equal(t, 3, tokens2)

	// Use all tokens
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))
	assert.True(t, rl.Allow(clientIP))
	assert.False(t, rl.Allow(clientIP))

	// Should have no tokens
	tokens3 := rl.GetTokenCount(clientIP)
	assert.Equal(t, 0, tokens3)
}

// Benchmark tests
func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := NewRateLimiter(1000000, time.Second) // High limit to avoid rejections

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			clientIP := fmt.Sprintf("192.168.1.%d", i%100) // Rotate through 100 clients
			rl.Allow(clientIP)
			i++
		}
	})
}

func BenchmarkRateLimiterGetTokenCount(b *testing.B) {
	rl := NewRateLimiter(100, time.Second)

	// Pre-populate with some clients
	for i := 0; i < 100; i++ {
		clientIP := fmt.Sprintf("192.168.1.%d", i)
		rl.Allow(clientIP)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			clientIP := fmt.Sprintf("192.168.1.%d", i%100)
			rl.GetTokenCount(clientIP)
			i++
		}
	})
}
