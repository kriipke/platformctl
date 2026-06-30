# ADR-001: Event-Driven Integration Workflows

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 1B - APIs and Messaging Infrastructure  

---

## Context

Platformctl must integrate with multiple external systems (HashiCorp Vault, ArgoCD, New Relic, Kubernetes API, GitHub) to collect operational data and trigger actions. Each integration has different characteristics:

- **Failure-prone:** External services may be temporarily unavailable or rate-limited
- **Variable latency:** Some integrations (Vault auth) are fast, others (ArgoCD sync) can take minutes
- **Independent scaling:** Different integrations have different load patterns
- **Asynchronous nature:** Many operations (especially ArgoCD sync) are inherently async

### Problem Statement

We need an architecture that can:
1. Handle integration failures gracefully without blocking other operations
2. Scale individual integration services independently
3. Provide visibility into long-running operations
4. Support retry logic and dead letter handling
5. Maintain correlation between commands and results across multiple services

### Considered Alternatives

#### Alternative 1: Synchronous HTTP-based integration
**Description:** API Gateway makes direct HTTP calls to integration services, which make synchronous calls to external systems.

**Pros:**
- Simple request/response model
- Easy to implement initially
- Direct error propagation

**Cons:**
- Tight coupling between API Gateway and integration services
- Poor failure isolation (one slow integration blocks others)
- Difficult to implement retries and timeout handling
- No visibility into long-running operations
- Hard to scale individual integrations independently

#### Alternative 2: Database-based job queue
**Description:** Use database tables as job queues with polling workers.

**Pros:**
- Leverages existing database infrastructure
- ACID properties for job handling
- Simple to implement

**Cons:**
- Database becomes a bottleneck for high-volume messaging
- Poor performance characteristics for messaging workloads
- Limited built-in retry and DLQ functionality
- Polling introduces latency

#### Alternative 3: Cloud-native messaging (AWS SQS, Google Pub/Sub)
**Description:** Use managed cloud messaging services.

**Pros:**
- Fully managed, no operational overhead
- Built-in retry and DLQ functionality
- High availability and scalability

**Cons:**
- Cloud vendor lock-in
- Additional cost
- Network latency for cross-region deployments
- Complexity for hybrid/on-premise deployments

---

## Decision

We will use **RabbitMQ** with an event-driven architecture for integration workflows.

### Architecture Components

#### Message Exchanges
- `platformctl.commands` (topic exchange) - for publishing command events
- `platformctl.results` (topic exchange) - for publishing result events

#### Routing Keys
- Command routing: `cmd.context.{action}` (e.g., `cmd.context.refresh`, `cmd.context.sync`)
- Result routing: `evt.context.result.{service}` (e.g., `evt.context.result.vault`)

#### Queue Bindings
```
vault-svc.q     binds to: cmd.context.*
argocd-svc.q    binds to: cmd.context.refresh, cmd.context.sync, cmd.context.inspect  
newrelic-svc.q  binds to: cmd.context.refresh, cmd.context.inspect
kube-svc.q      binds to: cmd.context.refresh, cmd.context.inspect
git-svc.q       binds to: cmd.context.refresh, cmd.context.inspect
aggregator.q    binds to: evt.context.result.*
```

### Message Envelope Format
```json
{
  "schema_version": 1,
  "message_id": "uuid",
  "correlation_id": "uuid", 
  "context_name": "app1-dev",
  "action": "refresh",
  "requested_by": "user:alice",
  "requested_at": "2026-01-21T18:10:00Z",
  "payload": {}
}
```

### Integration Service Pattern
Each integration service:
1. Consumes commands from its dedicated queue
2. Performs external system integration
3. Publishes result events with correlation tracking
4. Implements idempotency based on `message_id`
5. Uses exponential backoff for retries
6. Publishes to DLQ after max retry attempts

---

## Rationale

### Why RabbitMQ?
- **Mature and battle-tested** message broker
- **Rich routing capabilities** with topic exchanges and binding patterns
- **Built-in reliability features** (persistence, clustering, HA)
- **Excellent observability** with management UI and metrics
- **Language-agnostic** client libraries
- **On-premise friendly** for hybrid deployments

### Why Event-Driven Architecture?
- **Failure isolation:** One integration failure doesn't affect others
- **Independent scaling:** Scale services based on individual queue depth
- **Asynchronous by design:** Natural fit for long-running operations
- **Observability:** Message tracing provides visibility into workflows
- **Resilience:** Built-in retry, DLQ, and error handling patterns

### Why Topic Exchanges?
- **Flexible routing:** Services can subscribe to specific action types
- **Easy to extend:** New integration services can bind to relevant patterns
- **Service evolution:** Change service responsibilities without affecting message publishing

---

## Consequences

### Positive

1. **Fault Tolerance**
   - Integration failures are isolated and don't cascade
   - Built-in retry mechanisms with exponential backoff
   - Dead letter queues capture failed messages for investigation

2. **Scalability**
   - Integration services scale independently based on queue depth
   - Horizontal scaling of consumer instances
   - Load distribution across multiple workers

3. **Observability** 
   - Message correlation across entire workflow
   - Queue depth metrics indicate system health
   - End-to-end tracing of operations

4. **Operational Simplicity**
   - Standard message broker operational patterns
   - Rich monitoring and alerting capabilities
   - Well-understood debugging approaches

### Negative

1. **Complexity**
   - Additional infrastructure component (RabbitMQ)
   - Eventual consistency model requires careful handling
   - Message ordering considerations for related operations

2. **Operational Overhead**
   - RabbitMQ requires monitoring, backup, and HA configuration
   - Message retention and queue management policies needed
   - Network partitioning scenarios must be handled

3. **Development Overhead**
   - Idempotency must be implemented in all consumers
   - Message schema evolution requires careful planning
   - Testing requires message infrastructure setup

### Technical Debt

1. **Message Schema Evolution**
   - Need versioning strategy for message formats
   - Backward compatibility requirements
   - Migration path for schema changes

2. **Monitoring and Alerting**
   - Queue depth alerting thresholds
   - DLQ monitoring and response procedures  
   - End-to-end workflow SLA tracking

3. **Performance Optimization**
   - Message throughput tuning
   - Queue configuration optimization
   - Consumer scaling strategies

---

## Implementation Guidelines

### Phase 1B Implementation
- Set up basic RabbitMQ topology with exchanges and queues
- Implement command publishing from API Gateway
- Create base integration service framework with result publishing
- Add correlation ID tracking and basic retry logic

### Message Design Principles
1. **Idempotent consumers:** Use `message_id` for deduplication
2. **Correlation tracking:** Propagate `correlation_id` through all events
3. **Schema versioning:** Include `schema_version` in all messages
4. **Timeout handling:** Set appropriate message TTL values
5. **Error context:** Include sufficient error information in failed results

### Monitoring Requirements
- Queue depth metrics and alerting
- Message processing latency tracking
- DLQ depth monitoring with alerts
- End-to-end workflow completion tracking
- Consumer lag monitoring

---

## Alternatives Revisited

This decision can be revisited if:

1. **Scale requirements exceed RabbitMQ capabilities**
   - Consider cloud-native messaging services
   - Evaluate Apache Kafka for high-throughput scenarios

2. **Operational complexity becomes prohibitive**
   - Consider managed message brokers (e.g., Amazon MQ)
   - Evaluate serverless messaging patterns

3. **Strong consistency requirements emerge**
   - Consider saga pattern with compensating transactions
   - Evaluate event sourcing approaches

---

## References

- [RabbitMQ Documentation](https://www.rabbitmq.com/documentation.html)
- [Enterprise Integration Patterns](https://www.enterpriseintegrationpatterns.com/)
- [Building Event-Driven Microservices](https://www.oreilly.com/library/view/building-event-driven-microservices/9781492057888/)
- [Pattern: Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html)

---

## Related ADRs

- ADR-002: Read model (CQRS-lite) - Defines how result events are aggregated
- ADR-006: Circuit breaker and retry strategy - Defines resilience patterns for external calls
- ADR-008: Configuration management strategy - Defines how integration services are configured