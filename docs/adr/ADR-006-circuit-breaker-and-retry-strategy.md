# ADR-006: Circuit Breaker and Retry Strategy

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 2A - Security Foundation (Resilience Patterns)  

---

## Context

Platformctl integration services make numerous calls to external systems (Vault, ArgoCD, New Relic, Kubernetes API, GitHub) that can experience failures, timeouts, or degraded performance. Without proper resilience patterns, these issues can cascade through the system, causing widespread failures and poor user experience.

### Problem Statement

External system failures create multiple challenges:

1. **Cascade failures:** One slow/failed integration can impact all system operations
2. **Resource exhaustion:** Retrying failed calls can exhaust connection pools and memory
3. **Amplified load:** Naive retry logic can overwhelm already-struggling external systems
4. **Poor user experience:** Long timeouts and failures create frustrating delays
5. **Operational burden:** Manual intervention required to recover from failure modes

### Requirements

- **Fast failure detection:** Quickly identify when external systems are unhealthy
- **Graceful degradation:** Continue operating with partial functionality during outages
- **Automatic recovery:** Detect when external systems recover and resume normal operation
- **Configurable thresholds:** Tune circuit breaker sensitivity per integration
- **Observability:** Provide clear visibility into failure modes and recovery status
- **Minimal overhead:** Resilience patterns should not significantly impact performance

### Considered Alternatives

#### Alternative 1: Simple timeout and retry
**Description:** Basic timeout configuration with fixed retry attempts.

**Pros:**
- Simple to implement and understand
- Minimal code complexity
- Works for transient network issues

**Cons:**
- No protection against sustained failures
- Can amplify load on struggling systems
- No intelligent failure detection
- Fixed retry logic not optimal for all scenarios

#### Alternative 2: Exponential backoff only
**Description:** Implement exponential backoff without circuit breaker logic.

**Pros:**
- Reduces load on failing systems over time
- Simple retry pattern
- Prevents thundering herd effects

**Cons:**
- Still attempts requests during sustained outages
- No fast-fail behavior for known issues
- Resource consumption continues during failures
- No automatic recovery detection

#### Alternative 3: Manual failover switches
**Description:** Operational switches to disable integrations manually during outages.

**Pros:**
- Complete control over system behavior
- Simple implementation
- Clear operational model

**Cons:**
- Requires manual intervention for every issue
- Slow response to failures and recovery
- Risk of leaving systems disabled after recovery
- Poor user experience during failures

---

## Decision

We will implement the **Circuit Breaker Pattern** combined with **intelligent retry strategies** using exponential backoff with jitter.

### Architecture Overview

#### Circuit Breaker States
```
CLOSED → OPEN → HALF_OPEN → CLOSED
```

- **CLOSED:** Normal operation, requests pass through
- **OPEN:** Fast-fail mode, requests immediately return error
- **HALF_OPEN:** Test mode, limited requests to check recovery

#### Implementation Strategy
```go
type CircuitBreaker struct {
    name            string
    maxFailures     int
    timeout         time.Duration
    maxRequests     int           // Max requests in half-open
    interval        time.Duration // Stats collection window
    readyToTrip     func(counts Counts) bool
    onStateChange   func(name string, from, to State)
    
    mutex           sync.RWMutex
    state           State
    generation      uint64
    counts          Counts
    expiry          time.Time
}
```

#### Retry Strategy
- **Exponential backoff:** Base delay × 2^attempt
- **Jitter:** Add randomization to prevent thundering herd
- **Max attempts:** Configurable per integration type
- **Backoff caps:** Maximum delay between attempts

---

## Rationale

### Why Circuit Breaker Pattern?
- **Fast failure:** Immediately fail requests when external system is known to be down
- **Resource protection:** Prevent resource exhaustion from repeated failed requests
- **Automatic recovery:** Detect when external systems recover and resume operation
- **Load shedding:** Reduce load on struggling external systems

### Why Exponential Backoff with Jitter?
- **Reduced load:** Progressively longer delays reduce load on recovering systems
- **Thundering herd prevention:** Jitter prevents all clients retrying simultaneously
- **Adaptive behavior:** Longer delays for persistent failures, shorter for transient issues
- **Proven pattern:** Well-understood approach with predictable behavior

### Why Per-Integration Configuration?
- **Service characteristics:** Different external systems have different failure patterns
- **SLA alignment:** Match circuit breaker behavior to external system SLAs
- **Operational flexibility:** Tune behavior without code changes
- **Risk management:** More conservative settings for critical integrations

---

## Consequences

### Positive

1. **System Resilience**
   - Fast failure detection prevents cascade failures
   - Automatic recovery reduces operational burden
   - Resource protection prevents system-wide impacts

2. **User Experience**
   - Faster error responses during outages (no long timeouts)
   - Partial functionality maintained during external failures
   - Transparent recovery when systems come back online

3. **External System Protection**
   - Reduced load on struggling external systems
   - Jittered retries prevent thundering herd effects
   - Intelligent backoff allows systems to recover

4. **Operational Benefits**
   - Clear visibility into external system health
   - Reduced need for manual intervention
   - Predictable system behavior during failures

### Negative

1. **Complexity**
   - Additional logic in all external integration points
   - Configuration management for multiple circuit breakers
   - Testing scenarios for all circuit breaker states

2. **False Positives**
   - Circuit breakers may open during brief network blips
   - Conservative settings may unnecessarily fail fast
   - Recovery detection may be too slow or too aggressive

3. **Configuration Overhead**
   - Need to tune parameters for each integration
   - Monitoring and alerting for circuit breaker state changes
   - Operational understanding of circuit breaker behavior

### Technical Debt

1. **Tuning and Optimization**
   - Performance impact measurement and optimization
   - Circuit breaker parameter tuning based on real-world data
   - Integration with service-specific SLA requirements

2. **Observability Integration**
   - Metrics and monitoring for circuit breaker states
   - Alerting on circuit breaker state changes
   - Dashboard visualization of system resilience status

3. **Testing Strategy**
   - Chaos engineering to validate circuit breaker behavior
   - Integration tests for all failure scenarios
   - Load testing with circuit breaker logic

---

## Implementation Guidelines

### Phase 2A Implementation

#### Base Circuit Breaker Implementation
```go
type CircuitBreaker struct {
    name         string
    config       *CircuitBreakerConfig
    state        State
    counts       *Counts
    generation   uint64
    expiry       time.Time
    mutex        sync.RWMutex
}

type CircuitBreakerConfig struct {
    Name            string        `yaml:"name"`
    MaxFailures     int           `yaml:"max_failures"`      // 5
    Timeout         time.Duration `yaml:"timeout"`           // 60s
    MaxRequests     int           `yaml:"max_requests"`      // 1
    Interval        time.Duration `yaml:"interval"`          // 60s
    FailureThreshold float64      `yaml:"failure_threshold"` // 0.6
}

func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
    generation, err := cb.beforeRequest()
    if err != nil {
        return nil, err
    }
    
    defer func() {
        cb.afterRequest(generation, err)
    }()
    
    return fn()
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()
    
    now := time.Now()
    state, generation := cb.currentState(now)
    
    if state == StateOpen {
        return generation, ErrOpenState
    } else if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
        return generation, ErrTooManyRequests
    }
    
    cb.counts.onRequest()
    return generation, nil
}
```

#### Integration Service Wrapper
```go
type ResilientVaultClient struct {
    client         VaultClient
    circuitBreaker *CircuitBreaker
    retryConfig    *RetryConfig
}

func (rv *ResilientVaultClient) GetSecret(path, key string) (string, error) {
    operation := func() (interface{}, error) {
        return rv.client.GetSecret(path, key)
    }
    
    result, err := rv.circuitBreaker.Call(func() (interface{}, error) {
        return rv.executeWithRetry(operation)
    })
    
    if err != nil {
        return "", err
    }
    
    return result.(string), nil
}

func (rv *ResilientVaultClient) executeWithRetry(operation func() (interface{}, error)) (interface{}, error) {
    var lastErr error
    
    for attempt := 0; attempt < rv.retryConfig.MaxAttempts; attempt++ {
        if attempt > 0 {
            delay := rv.calculateBackoff(attempt)
            time.Sleep(delay)
        }
        
        result, err := operation()
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        
        // Check if error is retryable
        if !rv.isRetryable(err) {
            break
        }
    }
    
    return nil, lastErr
}

func (rv *ResilientVaultClient) calculateBackoff(attempt int) time.Duration {
    // Exponential backoff with jitter
    baseDelay := rv.retryConfig.BaseDelay
    maxDelay := rv.retryConfig.MaxDelay
    
    delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
    if delay > maxDelay {
        delay = maxDelay
    }
    
    // Add jitter (±25% randomization)
    jitter := time.Duration(rand.Float64()*0.5 - 0.25) * delay
    return delay + jitter
}
```

#### Configuration per Integration
```yaml
circuit_breakers:
  vault:
    max_failures: 5
    timeout: 60s
    max_requests: 1
    interval: 60s
    failure_threshold: 0.6
    
  argocd:
    max_failures: 3       # ArgoCD more sensitive
    timeout: 120s         # Longer timeout for recovery
    max_requests: 2       # Allow more test requests
    interval: 60s
    failure_threshold: 0.5
    
  newrelic:
    max_failures: 10      # New Relic more tolerant
    timeout: 30s          # Faster recovery detection
    max_requests: 1
    interval: 30s
    failure_threshold: 0.7

retry_policies:
  vault:
    max_attempts: 3
    base_delay: 1s
    max_delay: 30s
    retryable_errors: ["connection_error", "timeout", "server_error"]
    
  argocd:
    max_attempts: 5       # ArgoCD operations can be retried more
    base_delay: 2s
    max_delay: 60s
    retryable_errors: ["connection_error", "timeout", "rate_limit"]
```

#### Error Classification
```go
func (rv *ResilientVaultClient) isRetryable(err error) bool {
    if err == nil {
        return false
    }
    
    // Network errors are generally retryable
    if netErr, ok := err.(net.Error); ok {
        return netErr.Timeout() || netErr.Temporary()
    }
    
    // HTTP status code classification
    if httpErr, ok := err.(*HTTPError); ok {
        switch httpErr.StatusCode {
        case 429: // Rate limited
            return true
        case 500, 502, 503, 504: // Server errors
            return true
        case 400, 401, 403, 404: // Client errors
            return false
        default:
            return false
        }
    }
    
    // Vault-specific errors
    if vaultErr, ok := err.(*VaultError); ok {
        return vaultErr.Retryable
    }
    
    return false
}
```

---

## Configuration Guidelines

### Circuit Breaker Tuning
- **MaxFailures:** Start with 5, adjust based on external system behavior
- **Timeout:** 2-3x normal response time for the external system
- **MaxRequests:** Start with 1, increase for high-traffic integrations
- **Interval:** Match to external system monitoring window (30-60s)
- **FailureThreshold:** 60% for most systems, adjust based on SLA

### Retry Configuration
- **MaxAttempts:** 3-5 for most operations, higher for critical paths
- **BaseDelay:** 1-2 seconds, longer for heavy operations
- **MaxDelay:** 30-60 seconds, balance recovery time with user experience
- **Jitter:** 25% randomization to prevent thundering herd

### Per-Integration Recommendations
```yaml
# Critical path integrations (Vault)
vault:
  circuit_breaker:
    max_failures: 3      # Conservative
    timeout: 60s
    failure_threshold: 0.5
  retry:
    max_attempts: 3
    base_delay: 1s
    max_delay: 30s

# User-facing operations (ArgoCD sync)
argocd:
  circuit_breaker:
    max_failures: 5
    timeout: 120s        # Sync operations take time
    failure_threshold: 0.6
  retry:
    max_attempts: 5
    base_delay: 2s
    max_delay: 60s

# Monitoring/observability (New Relic)
newrelic:
  circuit_breaker:
    max_failures: 10     # More tolerant
    timeout: 30s
    failure_threshold: 0.7
  retry:
    max_attempts: 3
    base_delay: 1s
    max_delay: 15s
```

---

## Monitoring and Alerting

### Key Metrics
- **Circuit breaker state** (closed/open/half-open) per integration
- **Request success rate** before and after circuit breaker implementation
- **Response time distribution** for external calls
- **Retry attempt distribution** by integration and error type
- **Recovery time** from open to closed state

### Alert Conditions
```yaml
alerts:
  - name: CircuitBreakerOpen
    condition: circuit_breaker_state == "open"
    duration: 1m
    severity: warning
    
  - name: CircuitBreakerOpenCritical
    condition: circuit_breaker_state == "open" AND integration IN ["vault"]
    duration: 30s
    severity: critical
    
  - name: HighRetryRate
    condition: retry_rate > 0.3
    duration: 5m
    severity: warning
    
  - name: SlowRecovery
    condition: circuit_breaker_state == "half_open" FOR > 10m
    duration: 0s
    severity: warning
```

### Dashboard Visualizations
- Circuit breaker state timeline by integration
- Request success rate with circuit breaker events overlay
- Retry attempt heatmap by integration and error type
- External system response time percentiles
- Recovery time distribution

---

## Testing Strategy

### Unit Tests
- Circuit breaker state transitions
- Retry logic with different error types
- Backoff calculation with jitter
- Configuration validation

### Integration Tests
- Circuit breaker behavior with real external systems
- Network partition scenarios
- Rate limiting and recovery patterns
- Concurrent request handling

### Chaos Engineering
- Inject failures into external systems
- Validate circuit breaker opening and recovery
- Test thundering herd prevention
- Measure system behavior under various failure modes

---

## References

- [Circuit Breaker Pattern - Martin Fowler](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Release It! - Michael Nygard](https://pragprog.com/titles/mnee2/release-it-second-edition/)
- [Hystrix - Netflix](https://github.com/Netflix/Hystrix/wiki)
- [Go Circuit Breaker Libraries](https://github.com/sony/gobreaker)

---

## Related ADRs

- ADR-001: Event-driven integration workflows - Circuit breakers protect integration services
- ADR-003: Secrets posture - Circuit breakers protect Vault secret retrieval
- ADR-007: Caching layers and TTL policies - Cache can provide fallback during circuit breaker open state