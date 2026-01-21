# ADR-007: Caching Layers and TTL Policies

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 3C - Performance Optimization  

---

## Context

ContextOps makes frequent calls to external systems and performs expensive computations that could benefit from caching. As the system scales, reducing redundant external API calls and computation overhead becomes critical for performance, cost, and reliability.

### Problem Statement

Without strategic caching, ContextOps experiences:

1. **High external API usage:** Repeated calls to Vault, ArgoCD, New Relic, GitHub APIs
2. **Poor response times:** Status queries require real-time external calls
3. **Rate limit exhaustion:** GitHub and New Relic API limits reached quickly
4. **Resource waste:** Repeated computation of the same aggregation results
5. **Cascading load:** Multiple users requesting same data simultaneously

### Requirements

- **Multi-layer caching:** Different cache strategies for different data types
- **Intelligent TTL:** Cache duration based on data characteristics and usage patterns
- **Cache invalidation:** Mechanism to refresh stale data when needed
- **High availability:** Cache failures shouldn't break core functionality
- **Memory efficiency:** Bounded cache sizes with intelligent eviction
- **Observability:** Clear metrics on cache hit rates and effectiveness

---

## Decision

We will implement a **multi-layer caching strategy** with Redis for distributed caching and in-memory caches for hot data, combined with intelligent TTL policies based on data characteristics.

### Architecture Overview

#### Layer 1: In-Memory Cache (Hot Data)
- **L1 Cache:** In-process cache for frequently accessed data
- **Size:** 100MB per service instance
- **Eviction:** LRU with TTL expiration
- **Use cases:** Active context status, recent query results

#### Layer 2: Distributed Cache (Redis)  
- **L2 Cache:** Shared cache across all service instances
- **Persistence:** Optional persistence for cache warmup
- **Clustering:** Redis cluster for high availability
- **Use cases:** Integration results, Git file content, aggregated views

#### Layer 3: Fallback to Source
- **Source systems:** Direct calls to external APIs when cache misses
- **Circuit breaker integration:** Respect circuit breaker states
- **Async refresh:** Background cache warming for critical data

---

## Consequences

### Positive

1. **Performance Improvement**
   - Sub-100ms response times for cached status queries
   - Reduced load on external systems
   - Better user experience with consistent response times

2. **Cost Optimization**
   - Reduced API usage costs (New Relic, GitHub)
   - Lower resource consumption on external systems
   - Improved rate limit utilization

3. **Reliability Enhancement**
   - Graceful degradation when external systems are down
   - Reduced impact of external system latency spikes
   - Better handling of rate limiting scenarios

### Negative

1. **Complexity**
   - Cache invalidation logic
   - Multi-layer cache coordination
   - TTL policy management

2. **Consistency Challenges**
   - Eventual consistency between cache layers
   - Stale data scenarios
   - Cache warming strategies

3. **Operational Overhead**
   - Redis infrastructure management
   - Cache monitoring and alerting
   - Memory usage optimization

---

## Implementation Guidelines

### TTL Policy Matrix
```yaml
cache_policies:
  # Fast-changing data
  vault_secrets:
    l1_ttl: 30s      # Short in-memory cache
    l2_ttl: 2m       # Slightly longer distributed cache
    refresh_ahead: true
    
  # Stable configuration data  
  context_definitions:
    l1_ttl: 5m
    l2_ttl: 15m
    refresh_ahead: false
    
  # External API results
  github_files:
    l1_ttl: 2m
    l2_ttl: 5m
    refresh_ahead: true
    
  argocd_status:
    l1_ttl: 1m
    l2_ttl: 3m
    refresh_ahead: true
    
  newrelic_metrics:
    l1_ttl: 30s
    l2_ttl: 2m
    refresh_ahead: true
    
  # Aggregated views
  context_status:
    l1_ttl: 30s
    l2_ttl: 2m
    refresh_ahead: true
    
  run_history:
    l1_ttl: 5m
    l2_ttl: 15m
    refresh_ahead: false
```

### Cache Key Strategy
```go
type CacheKey struct {
    Namespace string // "contextops"
    Service   string // "vault", "argocd", etc.
    Operation string // "status", "file", "secret"
    Context   string // context name
    Resource  string // specific resource identifier
    Version   string // schema/API version
}

func (ck CacheKey) String() string {
    return fmt.Sprintf("%s:%s:%s:%s:%s:%s", 
        ck.Namespace, ck.Service, ck.Operation, 
        ck.Context, ck.Resource, ck.Version)
}
```

### Multi-Layer Cache Implementation
```go
type CacheManager struct {
    l1Cache cache.Cache      // In-memory cache
    l2Cache *redis.Client    // Distributed cache
    policies map[string]*CachePolicy
}

func (cm *CacheManager) Get(key CacheKey) (interface{}, bool) {
    policy := cm.policies[key.Service]
    
    // Try L1 cache first
    if val, found := cm.l1Cache.Get(key.String()); found {
        return val, true
    }
    
    // Try L2 cache
    if val, err := cm.l2Cache.Get(key.String()).Result(); err == nil {
        // Populate L1 cache
        cm.l1Cache.Set(key.String(), val, policy.L1TTL)
        return val, true
    }
    
    return nil, false
}

func (cm *CacheManager) Set(key CacheKey, value interface{}) error {
    policy := cm.policies[key.Service]
    
    // Set in both caches
    cm.l1Cache.Set(key.String(), value, policy.L1TTL)
    return cm.l2Cache.Set(key.String(), value, policy.L2TTL).Err()
}
```

---

## Related ADRs

- ADR-005: Git browsing via GitHub Contents API - Benefits from file content caching
- ADR-006: Circuit breaker and retry strategy - Cache provides fallback during outages
- ADR-002: Read model (CQRS-lite) - Read model acts as a form of materialized cache