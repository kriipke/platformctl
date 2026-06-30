# Database Schema

**Version:** 1.0  
**Date:** 2026-01-21  
**Phases:** 1A (Core), 1D (Read Model), 2A (Audit)  

---

## Overview

Platformctl uses PostgreSQL as the primary data store with a schema designed to support both operational data (contexts) and analytics (read model, audit logs). The schema evolves across development phases to support increasing functionality.

---

## Phase 1A: Core Schema

### Contexts Table

```sql
-- Main context storage table
CREATE TABLE contexts (
    name VARCHAR(255) PRIMARY KEY,
    api_version VARCHAR(50) NOT NULL DEFAULT 'platformctl/v1',
    kind VARCHAR(50) NOT NULL DEFAULT 'Context',
    
    -- Metadata
    labels JSONB,
    annotations JSONB,
    
    -- Full spec as JSONB for flexibility
    spec JSONB NOT NULL,
    
    -- Extracted fields for indexing and querying  
    app_name VARCHAR(255) GENERATED ALWAYS AS (spec->'app'->>'name') STORED,
    environment VARCHAR(50) GENERATED ALWAYS AS (spec->'app'->>'environment') STORED,
    tenant_id VARCHAR(255) DEFAULT 'default',  -- For future multi-tenancy
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT valid_name CHECK (name ~ '^[a-z0-9-]+$'),
    CONSTRAINT valid_environment CHECK (environment IN ('dev', 'staging', 'prod')),
    CONSTRAINT valid_api_version CHECK (api_version = 'platformctl/v1'),
    CONSTRAINT valid_kind CHECK (kind = 'Context')
);

-- Indexes for common query patterns
CREATE INDEX idx_contexts_app_env ON contexts (app_name, environment);
CREATE INDEX idx_contexts_tenant ON contexts (tenant_id);
CREATE INDEX idx_contexts_labels ON contexts USING GIN (labels);
CREATE INDEX idx_contexts_annotations ON contexts USING GIN (annotations);
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at DESC);
CREATE INDEX idx_contexts_spec_vault ON contexts USING GIN ((spec->'vault'));

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for automatic updated_at
CREATE TRIGGER update_contexts_updated_at 
    BEFORE UPDATE ON contexts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Schema Validation Functions

```sql
-- Validate context spec structure
CREATE OR REPLACE FUNCTION validate_context_spec(spec JSONB)
RETURNS BOOLEAN AS $$
BEGIN
    -- Check required fields exist
    IF NOT (spec ? 'app' AND spec ? 'policy' AND spec ? 'vault') THEN
        RETURN FALSE;
    END IF;
    
    -- Validate app section
    IF NOT (spec->'app' ? 'name' AND spec->'app' ? 'environment') THEN
        RETURN FALSE;
    END IF;
    
    -- Validate policy section
    IF NOT (spec->'policy' ? 'allowedActions') THEN
        RETURN FALSE;
    END IF;
    
    -- Additional validation logic can be added here
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Add constraint to enforce spec validation
ALTER TABLE contexts ADD CONSTRAINT valid_spec 
    CHECK (validate_context_spec(spec));
```

---

## Phase 1B: Command Tracking

### Command Runs Table

```sql
-- Track command execution for correlation and history
CREATE TABLE command_runs (
    correlation_id UUID PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    tenant_id VARCHAR(255) DEFAULT 'default',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Foreign key to contexts
    CONSTRAINT fk_command_runs_context 
        FOREIGN KEY (context_name) REFERENCES contexts(name) 
        ON DELETE CASCADE,
        
    -- Valid statuses
    CONSTRAINT valid_status 
        CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'cancelled')),
        
    -- Valid actions
    CONSTRAINT valid_action 
        CHECK (action IN ('refresh', 'validate', 'inspect', 'sync'))
);

-- Indexes for command tracking
CREATE INDEX idx_command_runs_context_action ON command_runs (context_name, action);
CREATE INDEX idx_command_runs_status ON command_runs (status);
CREATE INDEX idx_command_runs_requested_at ON command_runs (requested_at DESC);
CREATE INDEX idx_command_runs_tenant ON command_runs (tenant_id);
CREATE INDEX idx_command_runs_correlation ON command_runs (correlation_id);
```

---

## Phase 1D: Read Model Schema

### Context Status Table

```sql
-- Aggregated context status for fast reads
CREATE TABLE context_status (
    context_name VARCHAR(255) PRIMARY KEY,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    staleness_seconds INTEGER DEFAULT 0,
    overall_health VARCHAR(20) DEFAULT 'unknown',
    
    -- Service-specific status and payloads
    vault_status VARCHAR(20) DEFAULT 'unknown',
    vault_updated_at TIMESTAMP WITH TIME ZONE,
    vault_payload JSONB,
    vault_error TEXT,
    
    argocd_status VARCHAR(20) DEFAULT 'unknown',
    argocd_updated_at TIMESTAMP WITH TIME ZONE,
    argocd_payload JSONB,
    argocd_error TEXT,
    
    newrelic_status VARCHAR(20) DEFAULT 'unknown', 
    newrelic_updated_at TIMESTAMP WITH TIME ZONE,
    newrelic_payload JSONB,
    newrelic_error TEXT,
    
    kubernetes_status VARCHAR(20) DEFAULT 'unknown',
    kubernetes_updated_at TIMESTAMP WITH TIME ZONE,
    kubernetes_payload JSONB,
    kubernetes_error TEXT,
    
    git_status VARCHAR(20) DEFAULT 'unknown',
    git_updated_at TIMESTAMP WITH TIME ZONE,
    git_payload JSONB,
    git_error TEXT,
    
    tenant_id VARCHAR(255) DEFAULT 'default',
    
    -- Foreign key to contexts
    CONSTRAINT fk_context_status_context 
        FOREIGN KEY (context_name) REFERENCES contexts(name) 
        ON DELETE CASCADE,
        
    -- Valid health statuses
    CONSTRAINT valid_overall_health 
        CHECK (overall_health IN ('ok', 'degraded', 'error', 'unknown')),
    CONSTRAINT valid_service_status
        CHECK (
            vault_status IN ('ok', 'degraded', 'error', 'unknown') AND
            argocd_status IN ('ok', 'degraded', 'error', 'unknown') AND
            newrelic_status IN ('ok', 'degraded', 'error', 'unknown') AND
            kubernetes_status IN ('ok', 'degraded', 'error', 'unknown') AND
            git_status IN ('ok', 'degraded', 'error', 'unknown')
        )
);

-- Indexes for status queries
CREATE INDEX idx_context_status_health ON context_status (overall_health);
CREATE INDEX idx_context_status_updated ON context_status (updated_at DESC);
CREATE INDEX idx_context_status_tenant ON context_status (tenant_id);
CREATE INDEX idx_context_status_staleness ON context_status (staleness_seconds DESC);

-- Service-specific indexes for filtering
CREATE INDEX idx_context_status_vault ON context_status (vault_status, vault_updated_at);
CREATE INDEX idx_context_status_argocd ON context_status (argocd_status, argocd_updated_at);
```

### Result Events Table

```sql
-- Individual service result events for history and analysis
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
    result_payload JSONB,
    tenant_id VARCHAR(255) DEFAULT 'default',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Foreign keys
    CONSTRAINT fk_result_events_context 
        FOREIGN KEY (context_name) REFERENCES contexts(name) 
        ON DELETE CASCADE,
    CONSTRAINT fk_result_events_correlation
        FOREIGN KEY (correlation_id) REFERENCES command_runs(correlation_id)
        ON DELETE CASCADE,
        
    -- Valid values
    CONSTRAINT valid_service_name 
        CHECK (service_name IN ('vault', 'argocd', 'newrelic', 'kubernetes', 'git')),
    CONSTRAINT valid_status 
        CHECK (status IN ('ok', 'degraded', 'error')),
    CONSTRAINT valid_action 
        CHECK (action IN ('refresh', 'validate', 'inspect', 'sync')),
    CONSTRAINT valid_latency
        CHECK (latency_ms >= 0)
);

-- Indexes for result event queries
CREATE INDEX idx_result_events_context ON result_events (context_name, completed_at DESC);
CREATE INDEX idx_result_events_correlation ON result_events (correlation_id);
CREATE INDEX idx_result_events_service ON result_events (service_name, completed_at DESC);
CREATE INDEX idx_result_events_status ON result_events (status, completed_at DESC);
CREATE INDEX idx_result_events_tenant ON result_events (tenant_id);

-- Composite index for run history queries
CREATE INDEX idx_result_events_run_history ON result_events 
    (context_name, correlation_id, completed_at DESC);
```

### Health Calculation Functions

```sql
-- Calculate overall health based on individual service statuses
CREATE OR REPLACE FUNCTION calculate_overall_health(
    vault_status VARCHAR(20),
    argocd_status VARCHAR(20),
    newrelic_status VARCHAR(20),
    kubernetes_status VARCHAR(20),
    git_status VARCHAR(20)
) RETURNS VARCHAR(20) AS $$
BEGIN
    -- If any service is in error state, overall is error
    IF vault_status = 'error' OR argocd_status = 'error' OR 
       newrelic_status = 'error' OR kubernetes_status = 'error' OR 
       git_status = 'error' THEN
        RETURN 'error';
    END IF;
    
    -- If any service is degraded, overall is degraded  
    IF vault_status = 'degraded' OR argocd_status = 'degraded' OR
       newrelic_status = 'degraded' OR kubernetes_status = 'degraded' OR
       git_status = 'degraded' THEN
        RETURN 'degraded';
    END IF;
    
    -- If we have at least one OK status and no errors/degraded, we're OK
    IF vault_status = 'ok' OR argocd_status = 'ok' OR
       newrelic_status = 'ok' OR kubernetes_status = 'ok' OR
       git_status = 'ok' THEN
        RETURN 'ok';
    END IF;
    
    -- All services unknown
    RETURN 'unknown';
END;
$$ LANGUAGE plpgsql;

-- Calculate staleness (seconds since oldest service update)
CREATE OR REPLACE FUNCTION calculate_staleness(
    vault_updated_at TIMESTAMP WITH TIME ZONE,
    argocd_updated_at TIMESTAMP WITH TIME ZONE,
    newrelic_updated_at TIMESTAMP WITH TIME ZONE,
    kubernetes_updated_at TIMESTAMP WITH TIME ZONE,
    git_updated_at TIMESTAMP WITH TIME ZONE
) RETURNS INTEGER AS $$
BEGIN
    RETURN EXTRACT(EPOCH FROM (
        NOW() - LEAST(
            COALESCE(vault_updated_at, '1970-01-01'::timestamp with time zone),
            COALESCE(argocd_updated_at, '1970-01-01'::timestamp with time zone),
            COALESCE(newrelic_updated_at, '1970-01-01'::timestamp with time zone),
            COALESCE(kubernetes_updated_at, '1970-01-01'::timestamp with time zone),
            COALESCE(git_updated_at, '1970-01-01'::timestamp with time zone)
        )
    ))::INTEGER;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update overall health and staleness when service statuses change
CREATE OR REPLACE FUNCTION update_context_status_calculated_fields()
RETURNS TRIGGER AS $$
BEGIN
    NEW.overall_health = calculate_overall_health(
        NEW.vault_status,
        NEW.argocd_status, 
        NEW.newrelic_status,
        NEW.kubernetes_status,
        NEW.git_status
    );
    
    NEW.staleness_seconds = calculate_staleness(
        NEW.vault_updated_at,
        NEW.argocd_updated_at,
        NEW.newrelic_updated_at,
        NEW.kubernetes_updated_at,
        NEW.git_updated_at
    );
    
    NEW.updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_context_status_health 
    BEFORE INSERT OR UPDATE ON context_status
    FOR EACH ROW EXECUTE FUNCTION update_context_status_calculated_fields();
```

---

## Phase 2A: Audit and Security Schema

### Audit Log Table

```sql
-- Comprehensive audit log for security and compliance
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    event_id UUID DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Event identification
    event_type VARCHAR(50) NOT NULL,  -- 'context.created', 'action.triggered', etc.
    resource_type VARCHAR(50),        -- 'context', 'secret', etc.
    resource_id VARCHAR(255),         -- context name, secret path, etc.
    
    -- Actor information
    user_id VARCHAR(255) NOT NULL,
    user_type VARCHAR(50) DEFAULT 'human',  -- 'human', 'service', 'system'
    source_ip INET,
    user_agent TEXT,
    
    -- Request details
    request_id UUID,
    correlation_id UUID,
    http_method VARCHAR(10),
    endpoint VARCHAR(255),
    
    -- Event payload
    old_values JSONB,  -- Previous state (for updates)
    new_values JSONB,  -- New state (for creates/updates)
    metadata JSONB,    -- Additional event-specific data
    
    -- Outcome
    success BOOLEAN NOT NULL,
    error_message TEXT,
    
    -- Tenant isolation
    tenant_id VARCHAR(255) DEFAULT 'default',
    
    -- Constraints
    CONSTRAINT valid_event_type CHECK (
        event_type ~ '^[a-z]+\.[a-z_]+$'  -- Format: resource.action
    ),
    CONSTRAINT valid_user_type CHECK (
        user_type IN ('human', 'service', 'system')
    )
);

-- Indexes for audit queries
CREATE INDEX idx_audit_logs_timestamp ON audit_logs (timestamp DESC);
CREATE INDEX idx_audit_logs_event_type ON audit_logs (event_type);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_user ON audit_logs (user_id, timestamp DESC);
CREATE INDEX idx_audit_logs_tenant ON audit_logs (tenant_id);
CREATE INDEX idx_audit_logs_correlation ON audit_logs (correlation_id);
CREATE INDEX idx_audit_logs_request ON audit_logs (request_id);

-- Composite indexes for common queries
CREATE INDEX idx_audit_logs_security_events ON audit_logs 
    (event_type, success, timestamp DESC) 
    WHERE success = FALSE;

-- Partial index for failed events
CREATE INDEX idx_audit_logs_failures ON audit_logs (timestamp DESC, event_type) 
    WHERE success = FALSE;
```

### Security Events Table

```sql
-- Dedicated table for security-related events
CREATE TABLE security_events (
    id SERIAL PRIMARY KEY,
    event_id UUID DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Security event classification
    event_category VARCHAR(50) NOT NULL,  -- 'authentication', 'authorization', 'data_access'
    event_severity VARCHAR(20) NOT NULL DEFAULT 'medium',  -- 'low', 'medium', 'high', 'critical'
    event_description TEXT NOT NULL,
    
    -- Actor and source
    user_id VARCHAR(255),
    source_ip INET NOT NULL,
    user_agent TEXT,
    
    -- Context
    resource_accessed VARCHAR(255),
    action_attempted VARCHAR(100),
    
    -- Investigation data
    raw_request_data JSONB,
    additional_context JSONB,
    
    -- Response
    blocked BOOLEAN DEFAULT FALSE,
    alert_sent BOOLEAN DEFAULT FALSE,
    investigated BOOLEAN DEFAULT FALSE,
    investigation_notes TEXT,
    
    tenant_id VARCHAR(255) DEFAULT 'default',
    
    -- Constraints
    CONSTRAINT valid_event_category CHECK (
        event_category IN ('authentication', 'authorization', 'data_access', 'configuration', 'system')
    ),
    CONSTRAINT valid_event_severity CHECK (
        event_severity IN ('low', 'medium', 'high', 'critical')
    )
);

-- Indexes for security monitoring
CREATE INDEX idx_security_events_timestamp ON security_events (timestamp DESC);
CREATE INDEX idx_security_events_severity ON security_events (event_severity, timestamp DESC);
CREATE INDEX idx_security_events_category ON security_events (event_category, timestamp DESC);
CREATE INDEX idx_security_events_user ON security_events (user_id, timestamp DESC);
CREATE INDEX idx_security_events_source_ip ON security_events (source_ip, timestamp DESC);
CREATE INDEX idx_security_events_investigation ON security_events (investigated) 
    WHERE investigated = FALSE;
```

---

## Phase 3: Performance and Analytics Schema

### Metrics Table

```sql
-- System metrics for monitoring and alerting
CREATE TABLE metrics (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Metric identification
    metric_name VARCHAR(100) NOT NULL,
    metric_type VARCHAR(20) NOT NULL DEFAULT 'gauge',  -- 'gauge', 'counter', 'histogram'
    
    -- Dimensions/labels
    labels JSONB,
    
    -- Values
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(20),
    
    -- Context
    service_name VARCHAR(50),
    tenant_id VARCHAR(255) DEFAULT 'default',
    
    -- Constraints
    CONSTRAINT valid_metric_type CHECK (
        metric_type IN ('gauge', 'counter', 'histogram', 'summary')
    )
);

-- Time-series optimized indexes
CREATE INDEX idx_metrics_name_timestamp ON metrics (metric_name, timestamp DESC);
CREATE INDEX idx_metrics_service_timestamp ON metrics (service_name, timestamp DESC);
CREATE INDEX idx_metrics_timestamp ON metrics (timestamp DESC);
CREATE INDEX idx_metrics_labels ON metrics USING GIN (labels);

-- Partitioning by time for performance (optional)
-- CREATE TABLE metrics_2026_01 PARTITION OF metrics 
--     FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
```

---

## Multi-Tenancy Support (Phase 3D)

### Row Level Security Policies

```sql
-- Enable RLS on all tenant-scoped tables
ALTER TABLE contexts ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_status ENABLE ROW LEVEL SECURITY;
ALTER TABLE result_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE command_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_events ENABLE ROW LEVEL SECURITY;

-- Create policies to enforce tenant isolation
CREATE POLICY tenant_isolation_contexts ON contexts
    USING (tenant_id = current_setting('app.current_tenant_id', true));

CREATE POLICY tenant_isolation_context_status ON context_status  
    USING (tenant_id = current_setting('app.current_tenant_id', true));

CREATE POLICY tenant_isolation_result_events ON result_events
    USING (tenant_id = current_setting('app.current_tenant_id', true));

CREATE POLICY tenant_isolation_command_runs ON command_runs
    USING (tenant_id = current_setting('app.current_tenant_id', true));

-- Audit logs have special read-only access for security team
CREATE POLICY tenant_isolation_audit_logs ON audit_logs
    USING (
        tenant_id = current_setting('app.current_tenant_id', true) OR 
        current_setting('app.user_role', true) = 'security_admin'
    );
```

### Tenant Management Tables

```sql
-- Tenant configuration and quotas
CREATE TABLE tenants (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    
    -- Contact information
    admin_email VARCHAR(255) NOT NULL,
    billing_contact VARCHAR(255),
    
    -- Status
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Resource quotas
    max_contexts INTEGER DEFAULT 100,
    max_api_calls_per_day INTEGER DEFAULT 10000,
    max_storage_mb INTEGER DEFAULT 1000,
    max_concurrent_requests INTEGER DEFAULT 50,
    
    -- Configuration
    settings JSONB DEFAULT '{}',
    
    CONSTRAINT valid_status CHECK (status IN ('active', 'suspended', 'deleted'))
);

-- Tenant resource usage tracking
CREATE TABLE tenant_usage (
    tenant_id VARCHAR(255) NOT NULL REFERENCES tenants(id),
    date DATE NOT NULL,
    
    -- Usage counters
    contexts_count INTEGER DEFAULT 0,
    api_calls_count INTEGER DEFAULT 0,
    storage_used_mb INTEGER DEFAULT 0,
    max_concurrent_requests INTEGER DEFAULT 0,
    
    -- Calculated fields
    quota_utilization JSONB,  -- Percentage utilization by resource type
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    PRIMARY KEY (tenant_id, date)
);

-- Index for usage queries
CREATE INDEX idx_tenant_usage_tenant_date ON tenant_usage (tenant_id, date DESC);
```

---

## Database Maintenance

### Partitioning Strategy

```sql
-- Partition large tables by time for performance
-- Result events partitioned monthly
CREATE TABLE result_events_2026_01 PARTITION OF result_events 
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

-- Audit logs partitioned monthly  
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
```

### Data Retention Policies

```sql
-- Function to clean up old data
CREATE OR REPLACE FUNCTION cleanup_old_data() 
RETURNS void AS $$
BEGIN
    -- Clean up result events older than 1 year
    DELETE FROM result_events 
    WHERE created_at < NOW() - INTERVAL '1 year';
    
    -- Clean up audit logs older than 7 years (compliance requirement)
    DELETE FROM audit_logs 
    WHERE timestamp < NOW() - INTERVAL '7 years';
    
    -- Clean up metrics older than 90 days
    DELETE FROM metrics 
    WHERE timestamp < NOW() - INTERVAL '90 days';
    
    -- Vacuum to reclaim space
    VACUUM ANALYZE result_events;
    VACUUM ANALYZE audit_logs;
    VACUUM ANALYZE metrics;
END;
$$ LANGUAGE plpgsql;

-- Schedule cleanup job (requires pg_cron extension)
-- SELECT cron.schedule('cleanup-old-data', '0 2 * * *', 'SELECT cleanup_old_data();');
```

---

## Connection Configuration

### Connection Pool Settings

```sql
-- Recommended PostgreSQL configuration for Platformctl
-- postgresql.conf settings:

max_connections = 200
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB
maintenance_work_mem = 64MB

# JSON/JSONB performance
gin_pending_list_limit = 4MB

# Logging for monitoring
log_statement = 'mod'
log_min_duration_statement = 1000  # Log slow queries

# WAL settings for high availability
wal_level = replica
max_wal_senders = 3
```

### Application Connection Pools

```go
// Database configuration per service
type DatabaseConfig struct {
    URL             string
    MaxOpenConns    int    // 25 per service
    MaxIdleConns    int    // 5 per service  
    ConnMaxLifetime time.Duration // 1 hour
    ConnMaxIdleTime time.Duration // 15 minutes
}
```

---

## Migration Strategy

### Schema Versioning

```sql
-- Schema version tracking
CREATE TABLE schema_migrations (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    description TEXT
);

-- Record initial schema
INSERT INTO schema_migrations (version, description) 
VALUES ('001', 'Initial Platformctl schema');
```

---

## Related Documentation

- [Context Data Model](./context-model.md) - Context structure definition
- [API Schemas](./api-schemas.md) - REST API data formats  
- [ADR-002: Read Model (CQRS-lite)](../adr/ADR-002-read-model-cqrs-lite.md)
- [PHASE-1A Implementation Guide](../phases/PHASE-1A.md)