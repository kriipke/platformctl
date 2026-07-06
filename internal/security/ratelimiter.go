package security

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	clients    map[string]*ClientBucket
	maxTokens  int
	refillRate time.Duration
	mutex      sync.RWMutex
}

// ClientBucket represents a token bucket for a specific client
type ClientBucket struct {
	tokens     int
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerPeriod int, period time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:    make(map[string]*ClientBucket),
		maxTokens:  requestsPerPeriod,
		refillRate: period,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given client should be allowed
func (rl *RateLimiter) Allow(clientIP string) bool {
	bucket := rl.bucketFor(clientIP)

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	rl.refill(bucket, time.Now())

	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// bucketFor returns the bucket for a client, creating a fresh full bucket the
// first time a client is seen. It uses double-checked locking so that
// concurrent first requests from the same client share one bucket rather than
// racing to overwrite each other (which would leak tokens).
func (rl *RateLimiter) bucketFor(clientIP string) *ClientBucket {
	rl.mutex.RLock()
	bucket, exists := rl.clients[clientIP]
	rl.mutex.RUnlock()
	if exists {
		return bucket
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	if bucket, exists = rl.clients[clientIP]; exists {
		return bucket
	}
	bucket = &ClientBucket{
		tokens:     rl.maxTokens,
		lastRefill: time.Now(),
	}
	rl.clients[clientIP] = bucket
	return bucket
}

// refill tops up a bucket based on the time elapsed since its last refill,
// capped at maxTokens. The caller must hold bucket.mutex.
func (rl *RateLimiter) refill(bucket *ClientBucket, now time.Time) {
	if rl.refillRate <= 0 {
		return
	}
	timeSinceRefill := now.Sub(bucket.lastRefill)
	if timeSinceRefill >= rl.refillRate {
		periodsElapsed := int(timeSinceRefill / rl.refillRate)
		tokensToAdd := periodsElapsed * rl.maxTokens
		bucket.tokens = min(bucket.tokens+tokensToAdd, rl.maxTokens)
		bucket.lastRefill = now
	}
}

// cleanup removes old client entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()

		for clientIP, bucket := range rl.clients {
			bucket.mutex.Lock()
			// Remove clients that haven't been active for 2 hours
			if now.Sub(bucket.lastRefill) > 2*time.Hour {
				delete(rl.clients, clientIP)
			}
			bucket.mutex.Unlock()
		}

		rl.mutex.Unlock()
	}
}

// GetTokenCount returns the current token count for a client (for testing/monitoring)
func (rl *RateLimiter) GetTokenCount(clientIP string) int {
	rl.mutex.RLock()
	bucket, exists := rl.clients[clientIP]
	rl.mutex.RUnlock()

	if !exists {
		return rl.maxTokens
	}

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	// Apply any pending refill so the reported count reflects elapsed time,
	// matching what the next Allow call would see.
	rl.refill(bucket, time.Now())

	return bucket.tokens
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
