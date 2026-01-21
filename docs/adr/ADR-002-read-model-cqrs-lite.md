# ADR-002: Read Model (CQRS-lite)

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 1D - Aggregator Service  

---

## Context

ContextOps needs to provide fast, consolidated status views for contexts by aggregating data from multiple integration services (Vault, ArgoCD, New Relic, Kubernetes, Git). The challenge is balancing read performance with data consistency and system complexity.

### Problem Statement

Direct approaches create performance and coupling issues:

1. **Fan-out on read:** UI queries would need to call 5+ services synchronously
   - High latency (sum of all service latencies)
   - Cascading failures when any service is down
   - Complex error handling and partial result scenarios

2. **Integration service storage:** Each service stores its own state
   - No consolidated view across services
   - Complex cross-service queries
   - Inconsistent data models and APIs

3. **Real-time aggregation:** Calculate status on every request
   - CPU-intensive operations on critical path
   - Inconsistent results during concurrent updates
   - Poor caching characteristics

### Requirements

- **Fast reads:** Status queries must complete in <250ms (p95)
- **Eventual consistency:** Accept short delays for data freshness
- **Partial results:** Graceful degradation when services are unavailable
- **Staleness tracking:** Users need visibility into data freshness
- **Run history:** Track operation history across all services
- **Schema evolution:** Support adding new services without breaking changes

### Considered Alternatives

#### Alternative 1: Synchronous aggregation
**Description:** API Gateway fan-out to all integration services on each status request.

**Pros:**
- Always fresh data
- Simple architecture
- No additional storage requirements

**Cons:**
- High latency (sum of all service latencies)
- Poor availability (dependent on all services)
- Resource intensive (multiplied load on integration services)
- No historical data

#### Alternative 2: Shared database approach
**Description:** All integration services write directly to shared database tables.

**Pros:**
- Single source of truth
- ACID consistency
- Simple querying

**Cons:**
- Tight coupling between services
- Schema evolution challenges  
- Single database becomes bottleneck
- Difficult to optimize for different service needs

#### Alternative 3: Full CQRS with Event Sourcing
**Description:** Complete event-sourced system with separate command and query models.

**Pros:**
- Full audit trail
- Time travel capabilities
- Perfect data consistency

**Cons:**
- High complexity overhead
- Significant storage requirements
- Complex to implement and debug
- Overkill for current requirements

---

## Decision

We will implement a **CQRS-lite pattern** with a dedicated Aggregator service that maintains a read-optimized data model.

### Architecture Overview

#### Components
1. **Aggregator Service** - Consumes result events and maintains read model
2. **Read Model Database** - Denormalized tables optimized for status queries
3. **Result Events** - Published by integration services after completing work

#### Data Flow
```
Integration Services → Result Events → Aggregator → Read Model → API Gateway → UI
```

#### Read Model Schema
```sql
-- Consolidated status per context (optimized for status queries)
CREATE TABLE context_status (
    context_name VARCHAR(255) PRIMARY KEY,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    staleness_seconds INTEGER DEFAULT 0,
    overall_health VARCHAR(20) DEFAULT 'unknown',
    
    -- Per-service status and payloads
    vault_status VARCHAR(20) DEFAULT 'unknown',
    vault_updated_at TIMESTAMP WITH TIME ZONE,
    vault_payload JSONB,
    vault_error TEXT,
    
    argocd_status VARCHAR(20) DEFAULT 'unknown',
    argocd_updated_at TIMESTAMP WITH TIME ZONE,
    argocd_payload JSONB,
    argocd_error TEXT,
    
    -- ... similar for other services
);

-- Event history (optimized for run history queries)
CREATE TABLE result_events (
    id SERIAL PRIMARY KEY,
    correlation_id UUID NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    service_name VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    latency_ms INTEGER,
    error_message TEXT,
    result_payload JSONB
);
```

---

## Rationale

### Why CQRS-lite?
- **Read optimization:** Denormalized structure optimized for common query patterns
- **Incremental complexity:** Start simple, evolve toward full CQRS if needed
- **Proven pattern:** Well-understood approach with clear boundaries
- **Performance predictable:** Read performance independent of integration service load

### Why Aggregator Service?
- **Single responsibility:** Focused on read model maintenance
- **Event-driven updates:** Natural fit with message-based architecture (ADR-001)
- **Scalability:** Can scale aggregator independently of integration services
- **Isolation:** Read model updates don't affect integration service performance

### Why Database-based Read Model?
- **ACID guarantees:** Ensure read model consistency
- **Rich querying:** SQL provides flexible query capabilities
- **Indexing:** Optimize for common access patterns
- **Familiar operations:** Standard database operational practices

---

## Consequences

### Positive

1. **Performance**
   - Status queries are fast and predictable (single database query)
   - Read performance independent of integration service availability
   - Efficient indexing for common query patterns

2. **Availability**
   - Read operations continue even when integration services are down
   - Graceful degradation with staleness indicators
   - Historical data preserved for troubleshooting

3. **Scalability**
   - Read replicas can scale read capacity
   - Aggregator service scales independently
   - Integration services not impacted by read load

4. **Observability**
   - Complete audit trail of all service results
   - Run history provides troubleshooting context
   - Staleness metrics indicate data freshness

### Negative

1. **Eventual Consistency**
   - Read model may lag behind actual system state
   - Complex to handle concurrent updates to same context
   - Potential for temporary inconsistencies during updates

2. **Storage Overhead**
   - Duplicate data storage (events + aggregated state)
   - JSONB payloads can consume significant space
   - Need retention policies for historical data

3. **Complexity**
   - Additional service to operate and monitor
   - Schema evolution requires careful planning
   - Error handling for aggregation failures

### Technical Debt

1. **Schema Evolution**
   - Adding new integration services requires schema changes
   - Payload structure changes need migration strategies
   - Version compatibility between services and aggregator

2. **Data Consistency**
   - Handle out-of-order message processing
   - Implement conflict resolution for concurrent updates
   - Manage partial results during service failures

3. **Performance Tuning**
   - Query optimization for complex status displays
   - Indexing strategy for large-scale deployments
   - Archival strategy for historical events

---

## Implementation Guidelines

### Phase 1D Implementation

#### Aggregator Service
```go
// Event handler interface
type ResultHandler interface {
    HandleResult(result *api.ResultMessage) error
}

// Aggregator processes results and updates read model
func (a *Aggregator) HandleResult(result *api.ResultMessage) error {
    // 1. Store raw event
    if err := a.storeResultEvent(result); err != nil {
        return err
    }
    
    // 2. Update aggregated status
    if err := a.updateContextStatus(result); err != nil {
        return err
    }
    
    return nil
}
```

#### Database Design Principles
1. **Denormalized for reads:** Optimize table structure for common queries
2. **JSONB for flexibility:** Store service payloads as JSONB for schema evolution
3. **Calculated fields:** Use triggers to calculate overall health and staleness
4. **Proper indexing:** Index on context_name, updated_at, correlation_id

#### Staleness Calculation
```sql
-- Trigger to calculate staleness on status updates
CREATE OR REPLACE FUNCTION update_staleness()
RETURNS TRIGGER AS $$
BEGIN
    NEW.staleness_seconds = EXTRACT(EPOCH FROM (
        NOW() - LEAST(
            COALESCE(NEW.vault_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.argocd_updated_at, '1970-01-01'::timestamp),
            -- ... other services
        )
    ))::INTEGER;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

### Query Patterns
```sql
-- Status query (primary use case)
SELECT context_name, overall_health, staleness_seconds, 
       vault_status, argocd_status, kubernetes_status
FROM context_status 
WHERE context_name = $1;

-- Run history (troubleshooting)
SELECT correlation_id, action, overall_status, latency_ms, requested_at
FROM (
    SELECT correlation_id, action, requested_by,
           MIN(requested_at) as requested_at,
           CASE 
               WHEN COUNT(CASE WHEN status = 'error' THEN 1 END) > 0 THEN 'error'
               WHEN COUNT(CASE WHEN status = 'degraded' THEN 1 END) > 0 THEN 'degraded'
               ELSE 'ok'
           END as overall_status,
           EXTRACT(EPOCH FROM (MAX(completed_at) - MIN(requested_at)))::INTEGER * 1000 as latency_ms
    FROM result_events 
    WHERE context_name = $1
    GROUP BY correlation_id, action, requested_by
    ORDER BY MIN(requested_at) DESC
    LIMIT $2
) grouped_runs;
```

### Error Handling Strategy
1. **Idempotent processing:** Handle duplicate result events gracefully
2. **Partial results:** Continue processing even if some fields are invalid
3. **Conflict resolution:** Last-writer-wins for concurrent updates to same service
4. **Fallback values:** Use sensible defaults for missing data

---

## Evolution Path

### Phase 2 Enhancements
- **Read replicas** for geographic distribution
- **Caching layer** (Redis) for frequently accessed contexts
- **Materialized views** for complex aggregate queries

### Phase 3 Scalability  
- **Partitioning** context_status table by context name ranges
- **Event archival** strategy for result_events table
- **Cross-region replication** for disaster recovery

### Future Considerations
- **Full Event Sourcing** if audit requirements become more stringent
- **Separate read databases** per service type for specialized queries
- **Stream processing** (Apache Kafka) for real-time aggregations

---

## Monitoring and Alerting

### Key Metrics
- **Aggregation latency:** Time from result event to read model update
- **Staleness distribution:** Histogram of staleness_seconds across contexts
- **Query performance:** p95 latency for status and history queries
- **Event processing rate:** Result events processed per second
- **Aggregator lag:** Delay between event publish and consumption

### Alert Conditions
- Aggregation latency > 10 seconds (p95)
- Any context with staleness > 1 hour
- Status query latency > 500ms (p95) 
- Aggregator service error rate > 1%
- Read model database connection failures

---

## References

- [CQRS Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/cqrs)
- [Materialized View Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/materialized-view)
- [Event-Driven Architecture](https://martinfowler.com/articles/201701-event-driven.html)
- [PostgreSQL JSONB Performance](https://www.postgresql.org/docs/current/datatype-json.html)

---

## Related ADRs

- ADR-001: Event-driven integration workflows - Provides result events for aggregation
- ADR-007: Caching layers and TTL policies - Defines caching strategy for read model
- ADR-014: Database scaling and partitioning strategy - Future scaling approaches