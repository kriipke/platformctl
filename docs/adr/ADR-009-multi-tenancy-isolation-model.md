# ADR-009: Multi-Tenancy Isolation Model

**Status:** Proposed  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 3D - Multi-tenancy and Resource Quotas  

---

## Context

As Platformctl grows beyond single-organization use, we need to support multiple organizations (tenants) using the same Platformctl deployment while ensuring complete data isolation, security boundaries, and resource fairness.

### Problem Statement

Multi-tenancy introduces several challenges:

1. **Data isolation:** Prevent tenant A from accessing tenant B's contexts and data
2. **Resource fairness:** Ensure one tenant cannot consume all system resources
3. **Security boundaries:** Isolate credentials and external system access per tenant
4. **Operational complexity:** Manage multiple tenants without operational overhead explosion
5. **Cost allocation:** Track resource usage and costs per tenant

### Requirements

- **Complete data isolation:** Tenants cannot access each other's data under any circumstances
- **Resource quotas:** Configurable limits per tenant for contexts, API calls, storage
- **Security isolation:** Separate credential stores and external system access
- **Operational efficiency:** Single deployment supporting multiple tenants
- **Audit compliance:** Complete audit trails separated by tenant
- **Cost transparency:** Clear resource usage tracking per tenant

---

## Decision

We will implement **namespace-based isolation** with tenant-scoped resources, shared infrastructure, and strict access controls.

### Tenant Isolation Strategy

#### Database Isolation
- **Tenant-scoped tables:** All tables include `tenant_id` column with strict filtering
- **Row-level security:** PostgreSQL RLS policies enforce tenant boundaries
- **Connection pooling:** Separate connection pools per tenant for resource control

#### Message Queue Isolation  
- **Tenant-prefixed queues:** All RabbitMQ queues include tenant identifier
- **Virtual hosts:** Separate RabbitMQ vhosts for complete message isolation
- **Resource limits:** Per-tenant queue limits and connection quotas

#### Cache Isolation
- **Namespace separation:** All cache keys prefixed with tenant identifier
- **Memory quotas:** Per-tenant memory limits in Redis
- **Eviction policies:** Tenant-aware eviction to prevent starvation

---

## Consequences

### Positive

1. **Security Isolation**
   - Complete data separation prevents cross-tenant data access
   - Separate credential management per tenant
   - Independent audit trails for compliance

2. **Resource Control**
   - Fair resource allocation across tenants
   - Protection against noisy neighbor problems
   - Clear cost allocation and billing capabilities

3. **Operational Efficiency**
   - Single deployment and operations team
   - Shared infrastructure reduces costs
   - Centralized monitoring and management

### Negative

1. **Complexity**
   - Tenant-aware code throughout the application
   - Complex resource quota enforcement
   - Multi-dimensional monitoring and alerting

2. **Resource Overhead**
   - Metadata overhead for tenant scoping
   - Additional security checks on all operations
   - Resource tracking and quota enforcement overhead

---

## Implementation Guidelines

### Tenant Context Propagation
```go
type TenantContext struct {
    TenantID string
    UserID   string
    Roles    []string
}

func TenantFromContext(ctx context.Context) (*TenantContext, error) {
    tenant, ok := ctx.Value(tenantContextKey).(*TenantContext)
    if !ok {
        return nil, errors.New("no tenant context found")
    }
    return tenant, nil
}
```

### Database Row-Level Security
```sql
-- Enable RLS on all tenant-scoped tables
ALTER TABLE contexts ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_status ENABLE ROW LEVEL SECURITY;
ALTER TABLE result_events ENABLE ROW LEVEL SECURITY;

-- Create policies to enforce tenant isolation
CREATE POLICY tenant_isolation_contexts ON contexts
    USING (tenant_id = current_setting('app.current_tenant_id'));
    
CREATE POLICY tenant_isolation_context_status ON context_status  
    USING (tenant_id = current_setting('app.current_tenant_id'));
```

### Resource Quota Enforcement
```go
type ResourceQuotas struct {
    MaxContexts     int `json:"max_contexts"`
    MaxAPICallsDay  int `json:"max_api_calls_day"`
    MaxStorageMB    int `json:"max_storage_mb"`
    MaxConcurrency  int `json:"max_concurrency"`
}

func (q *QuotaEnforcer) CheckQuota(tenantID string, resource ResourceType) error {
    usage := q.getCurrentUsage(tenantID, resource)
    quota := q.getQuota(tenantID, resource)
    
    if usage >= quota {
        return fmt.Errorf("quota exceeded for tenant %s: %s usage %d >= limit %d", 
            tenantID, resource, usage, quota)
    }
    
    return nil
}
```

---

## Related ADRs

- ADR-003: Secrets posture - Tenant isolation requires separate secret management
- ADR-002: Read model (CQRS-lite) - Read models must be tenant-aware
- ADR-008: Configuration management strategy - Tenant-specific configuration requirements