# Platformctl Development Guide for Claude

**Last Updated:** 2026-07-02  
**Status:** Ready for Implementation  

---

## Overview

This guide provides Claude with all essential information needed to build Platformctl, a **GitOps-optimized application monitoring platform** designed specifically for DevOps engineers managing applications deployed via ArgoCD ApplicationSets, Helm umbrella charts, and Vault-secured secrets across multiple environments and customers.

---

## 🎯 What We're Building

**Platformctl** is a GitOps-native application monitoring platform that:
- **Monitors GitOps Workflows**: ApplicationSet → Generated Applications → Helm deployments → Vault secrets
- **Multi-Environment Correlation**: Tracks applications across dev/qa/uat/prod with unified dashboards
- **Real-Time Secret Validation**: Correlates Vault secrets with pod environment variables 
- **Customer Isolation**: Supports customer branches with separate configurations and resource tracking
- **Application-Centric Interface**: Per-app dashboards with environment tabs and cross-environment comparison
- **Helm Values Intelligence**: Correlates values-dev.yaml, values-qa.yaml, values-uat.yaml, values-prod.yaml with deployed state

### GitOps Architecture Pattern
```
Bootstrap App → ApplicationSets → Generated Apps → Helm Charts → VaultStaticSecrets → Pod Env Vars
                        ↓
CLI/Web UI → API Gateway → RabbitMQ → GitOps Integration Services → GitOps Aggregator → Multi-Environment Read Model
```

### Key GitOps Capabilities
- **ApplicationSet Deep Integration**: Bootstrap Application monitoring, ApplicationSet health tracking, generated application correlation
- **Vault-Kubernetes Secret Bridge**: VaultStaticSecret monitoring, real-time secret sync validation, pod environment variable correlation
- **Multi-Cluster Operations**: Cross-cluster application visibility, namespace isolation, unified status aggregation
- **Customer Branch Management**: Git branch isolation, configuration drift detection, customer-specific policy enforcement

---

## 📁 Project Structure

```
platformctl/
├── cmd/                              # Service entry points (one directory per service)
│   ├── gateway/                     # API Gateway (context CRUD + GitOps actions)
│   ├── gitops-aggregator/           # Multi-environment read model aggregator
│   ├── app-sync-svc/                # ApplicationSet monitoring and sync orchestration
│   ├── context-correlation-svc/     # Context pairing and relationship management
│   ├── customer-git-branch-svc/     # Customer branch tracking and values correlation
│   ├── environment-validation-svc/  # Cross-environment validation and compliance
│   ├── multi-environment-kube-svc/  # Multi-cluster Kubernetes monitoring
│   └── test-service/                # Test harness service
├── internal/                        # Private application code
│   ├── config/                     # Environment-variable configuration
│   ├── models/                     # Context/app/environment/customer domain models
│   ├── validation/                 # Domain model validation
│   ├── storage/                    # PostgreSQL stores (contexts, apps, environments)
│   ├── handlers/                   # Gateway HTTP handlers (CRUD, GitOps actions/status)
│   ├── events/                     # RabbitMQ publishers/consumers and queue topology
│   ├── clients/                    # External service clients (vault, kubernetes, git, argocd, helm)
│   ├── aggregator/                 # GitOps aggregation logic
│   ├── readmodel/                  # GitOps read model store
│   ├── observability/              # Logging, metrics, health checks, correlation middleware
│   └── ...                         # audit, auth, security, circuitbreaker, correlation, database, gitops, integration, services, testutil
├── pkg/api/                         # Shared message envelope and integration types
├── migrations/                      # SQL migrations (numbered up/down pairs)
├── docs/                            # All documentation
├── charts/platformctl/              # Helm chart (deployment) + per-env values files
├── scripts/                         # Build scripts
└── test/                            # Test runner + integration tests (test/integration/)
```

---

## 🏗️ Development Phases

**CRITICAL:** Follow the exact phase order for dependency management:

### Phase 1A: Core Foundation
- Context data models and validation
- PostgreSQL database setup with migrations  
- API Gateway with basic routing
- **Key Files:** `internal/models/`, `internal/validation/`, `internal/storage/`, `migrations/`

### Phase 1B: APIs and Messaging Infrastructure  
- RabbitMQ integration with message envelopes
- Command/Result message patterns
- Action endpoints in API Gateway
- **Key Files:** `internal/events/`, `internal/handlers/`, `pkg/api/`

### Phase 1C: Integration Services
- **Priority Order:** Vault → Kubernetes → Git → ArgoCD → New Relic
- Each service implements health checking and result reporting
- **Key Files:** `internal/clients/vault/`, `internal/clients/kubernetes/`, etc., plus the `cmd/*-svc/` entry points

### Phase 1D: Aggregator Service
- Read model database schema
- Result aggregation and health calculation  
- Context status materialization
- **Key Files:** `cmd/gitops-aggregator/`, `internal/aggregator/`, `internal/readmodel/`

### Phase 1E: Basic Observability
- Structured logging with zerolog
- Prometheus metrics
- Health check endpoints
- **Key Files:** `internal/observability/`

### Phase 1F-1H: Deployment, Testing, CLI
- Continue with remaining phases as documented

---

## 🔑 Core Technologies & Dependencies

### Required Go Modules
```go
// Database
github.com/lib/pq                 // PostgreSQL driver
github.com/golang-migrate/migrate // Database migrations
github.com/jmoiron/sqlx           // SQL extensions

// Messaging  
github.com/streadway/amqp         // RabbitMQ client
github.com/google/uuid            // UUID generation

// Web Framework
github.com/gin-gonic/gin          // HTTP router
github.com/gin-contrib/cors       // CORS middleware

// Validation
github.com/go-playground/validator/v10  // Struct validation

// Configuration
github.com/caarlos0/env/v6        // Environment variable parsing

// External Service Clients
k8s.io/client-go                  // Kubernetes client
github.com/hashicorp/vault/api    // Vault client  
github.com/google/go-github/v45   // GitHub API client
```

### External Dependencies
- **PostgreSQL 14+** with JSONB support
- **RabbitMQ 3.9+** with management plugin
- **Redis 6+** for caching (Phase 2)

---

## 📋 Implementation Checklists

### Before Starting Any Phase
- [ ] Read the specific phase document in `docs/phases/`
- [ ] Review related ADRs in `docs/adr/`  
- [ ] Check data model documentation in `docs/data-models/`
- [ ] Verify all dependencies are available

### Essential Commands to Implement
```bash
# CLI Commands (Phase 1H)
platformctl context create <file>     # Create context from YAML
platformctl context list             # List all contexts  
platformctl context status <name>    # Get context status
platformctl context run <name> <action>  # Execute action

# Development Commands  
make build                           # Build all binaries
make test                           # Run test suite
make db-migrate                     # Run database migrations
make docker-build                   # Build container images
```

### Required Environment Variables
```bash
# Database
DATABASE_URL=postgres://user:pass@localhost/platformctl

# RabbitMQ  
RABBITMQ_URL=amqp://user:pass@localhost:5672/

# Service Configuration
PORT=8080
LOG_LEVEL=info
ENABLE_METRICS=true

# External Services (optional, configured per context)
VAULT_ADDR=https://vault.example.com
ARGOCD_ADDR=https://argocd.example.com  
NEWRELIC_API_KEY=<key>
```

---

## 🗄️ Database Schema Overview

### Core Tables (Phase 1A)
```sql
contexts              -- Main context definitions  
context_revisions     -- Change history
audit_logs           -- Audit trail
```

### Operational Tables (Phase 1B-1D)
```sql  
command_runs         -- Command execution tracking
result_events        -- Service results from integrations
context_status       -- Read model for current status
run_history         -- Historical run data
service_metrics     -- Aggregated performance metrics
```

**Key Insight:** Use `tenant_id` in ALL tables for multi-tenancy support.

---

## 🔄 Message Flow Architecture

### Command Flow
1. **CLI/Web UI** → API Gateway (`POST /contexts/{name}/actions/{action}`)
2. **API Gateway** → RabbitMQ (`command.{context-name}` queue)
3. **Integration Services** → Process and return results  
4. **Results** → RabbitMQ (`results.{service-name}` queue)
5. **Aggregator Service** → Updates read model

### Message Envelope Structure
```go
type MessageEnvelope struct {
    SchemaVersion int                    `json:"schema_version"`
    MessageID     string                 `json:"message_id"`  
    CorrelationID string                 `json:"correlation_id"`
    ContextName   string                 `json:"context_name"`
    Action        string                 `json:"action"`
    RequestedBy   string                 `json:"requested_by"`
    RequestedAt   time.Time              `json:"requested_at"`
    Payload       map[string]interface{} `json:"payload"`
}
```

---

## 🛡️ Security & Validation Requirements

### Context Validation Rules
- **Name:** Must match `^[a-z0-9-]+$`, 1-63 characters
- **API Version:** Must be `platformctl/v1`
- **Kind:** Must be `Context`  
- **Spec:** All service configurations must validate against schemas

### Authentication Strategy
- **Phase 1:** No authentication (internal tool)
- **Phase 2:** JWT-based authentication
- **Phase 3:** RBAC with tenant isolation

### Secrets Management
- **NEVER** store actual secrets in contexts
- Use Vault paths and references only
- Implement secrets-by-reference pattern

---

## 🔍 Service Integration Patterns

### Health Check Implementation
Each service must implement:
```go
type HealthChecker interface {
    CheckHealth(ctx context.Context, contextName string) (*ServiceResult, error)
}

type ServiceResult struct {
    Status      string    `json:"status"`        // ok, degraded, error
    LatencyMs   int       `json:"latency_ms"`
    ServiceName string    `json:"service_name"`  
    Error       string    `json:"error,omitempty"`
    Details     interface{} `json:"details"`
    Timestamp   time.Time `json:"timestamp"`
}
```

### Error Handling Standards
- Use structured errors with codes
- Implement circuit breaker pattern (Phase 2)
- Include retry logic with exponential backoff
- Log errors with correlation IDs

### Service Priority Order
1. **Vault** - Critical for secrets access
2. **Kubernetes** - Core infrastructure 
3. **Git** - Source of truth for configurations
4. **ArgoCD** - Deployment status
5. **New Relic** - Observability data

---

## 📊 Monitoring & Observability

### Required Metrics (Phase 1E)
```go
// Request metrics
platformctl_requests_total{method, endpoint, status}
platformctl_request_duration_seconds{method, endpoint}

// Service metrics  
platformctl_service_health{service, context, status}
platformctl_service_latency{service, context}

// Business metrics
platformctl_contexts_total{tenant, health_status}
platformctl_actions_total{action, status}
```

### Structured Logging Format
```go
logger.Info().
    Str("correlation_id", correlationID).
    Str("context_name", contextName).
    Str("service", serviceName).
    Int("latency_ms", latency).
    Str("status", status).
    Msg("Service health check completed")
```

---

## 🧪 Testing Strategy

### Test Categories
1. **Unit Tests** - All business logic functions
2. **Integration Tests** - Database operations and external APIs
3. **Contract Tests** - Service interfaces and message schemas
4. **End-to-End Tests** - Complete workflows via CLI/API

### Test Data Strategy
- Use `testdata/` directory for sample contexts
- Implement test database with migrations
- Mock external services for unit tests
- Use real services for integration tests (optional)

### Required Test Coverage
- **Minimum:** 70% overall coverage
- **Critical Paths:** 90+ coverage (validation, health checks)

---

## 🚀 Deployment Architecture

### Container Strategy
```dockerfile
# Multi-stage builds for each service
FROM golang:1.21 AS builder
# Build steps...

FROM alpine:latest
# Runtime setup...
```

### Kubernetes Resources (Phase 1F)
```yaml
# Required resources per service:
- Deployment
- Service  
- ConfigMap
- Secret (for external service credentials)
- ServiceAccount (with RBAC for Kubernetes integration)
```

### Health Checks
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30

readinessProbe:  
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
```

---

## 🎯 Success Criteria Per Phase

### Phase 1A Complete When:
- [ ] Context CRUD operations working
- [ ] Database migrations run successfully  
- [ ] API Gateway serves basic endpoints
- [ ] Context validation passes all test cases

### Phase 1B Complete When:
- [ ] RabbitMQ connection established
- [ ] Messages publish/consume correctly
- [ ] Action endpoints trigger command messages
- [ ] Correlation IDs track requests end-to-end

### Phase 1C Complete When:  
- [ ] All 5 services can execute health checks
- [ ] Service results follow standard schema
- [ ] Circuit breakers protect against failures
- [ ] Integration tests pass for each service

### Phase 1D Complete When:
- [ ] Read model aggregates service results
- [ ] Context status API returns current health
- [ ] Historical data queryable via API
- [ ] Dashboard views perform efficiently

---

## ⚠️ Common Pitfalls to Avoid

1. **Database Migrations:** Always use transactions and rollback plans
2. **Message Ordering:** Don't assume message delivery order
3. **External Service Timeouts:** Implement proper timeout handling
4. **Memory Leaks:** Close connections and clean up goroutines  
5. **Configuration:** Validate all config at startup, not runtime
6. **Multi-tenancy:** Include `tenant_id` in ALL database queries
7. **Secrets:** Never log or store actual secret values

---

## 📚 Documentation References

### Essential Reading Order
1. [README.md](./README.md) - Architecture overview
2. [ROADMAP.md](./ROADMAP.md) - Full feature roadmap  
3. [docs/phases/PHASE-1A.md](./docs/phases/PHASE-1A.md) - Start here for implementation
4. [docs/data-models/](./docs/data-models/) - Data structure references
5. [docs/adr/](./docs/adr/) - Architectural decisions and rationale

### Quick Reference Files
- `docs/data-models/context-model.md` - Context YAML structure
- `docs/data-models/api-schemas.md` - Message and API formats
- `docs/data-models/database-schema.md` - Complete schema reference
- `docs/adr/ADR-008-configuration-management-strategy.md` - Environment variables

---

## 🛠️ Development Workflow

### Getting Started
1. **Read Phase 1A document completely**
2. **Set up development environment** (PostgreSQL, RabbitMQ)
3. **Create basic project structure** (`cmd/`, `internal/`, `pkg/`)
4. **Implement Context data model** with validation
5. **Test database operations** before moving to Phase 1B

### Between Phases
1. **Run full test suite** before starting next phase
2. **Update documentation** if implementation differs from design
3. **Commit working code** at each phase completion
4. **Review next phase requirements** and dependencies

### When Stuck
1. **Check ADR documents** for architectural decisions
2. **Review data model schemas** for structure requirements
3. **Look at phase dependencies** - may need to implement prerequisites
4. **Validate environment setup** - external services configured correctly

---

## 🎉 Ready to Start!

You now have all the information needed to build Platformctl. Begin with **Phase 1A** and follow the implementation guide in `docs/phases/PHASE-1A.md`.

Remember: **Quality over speed** - implement each phase completely before moving to the next. The architecture is designed for this sequential approach.

Good luck building! 🚀