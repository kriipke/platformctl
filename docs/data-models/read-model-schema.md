# Read Model Schema

**Version:** 1.0  
**Date:** 2026-01-21  
**Phases:** 1D (Aggregator Service), 2A (Performance Optimization)  

---

## Overview

This document defines the read model schemas used in ContextOps for optimized query performance and data aggregation. The read model implements a CQRS-lite pattern where write operations go through the command/event system, while read operations are served from denormalized, query-optimized structures.

---

## Core Read Model Tables

### Context Status Table
Primary read model for context health and status information.

```sql
CREATE TABLE context_status (
    -- Primary identification
    context_name VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    
    -- Temporal tracking
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    staleness_seconds INTEGER DEFAULT 0,
    last_run_correlation_id VARCHAR(36),
    
    -- Overall health aggregation
    overall_health VARCHAR(20) DEFAULT 'unknown',
    health_summary TEXT,
    
    -- Service status breakdown
    vault_status VARCHAR(20),
    vault_updated_at TIMESTAMP WITH TIME ZONE,
    vault_error TEXT,
    vault_latency_ms INTEGER,
    
    kubernetes_status VARCHAR(20),
    kubernetes_updated_at TIMESTAMP WITH TIME ZONE,
    kubernetes_error TEXT, 
    kubernetes_latency_ms INTEGER,
    
    argocd_status VARCHAR(20),
    argocd_updated_at TIMESTAMP WITH TIME ZONE,
    argocd_error TEXT,
    argocd_latency_ms INTEGER,
    
    newrelic_status VARCHAR(20),
    newrelic_updated_at TIMESTAMP WITH TIME ZONE,
    newrelic_error TEXT,
    newrelic_latency_ms INTEGER,
    
    git_status VARCHAR(20),
    git_updated_at TIMESTAMP WITH TIME ZONE,
    git_error TEXT,
    git_latency_ms INTEGER,
    
    -- Detailed service results (JSONB for flexibility)
    vault_details JSONB,
    kubernetes_details JSONB,
    argocd_details JSONB,
    newrelic_details JSONB,
    git_details JSONB,
    
    -- Quick summary for API responses
    summary_json JSONB,
    
    -- RLS for multi-tenancy
    CONSTRAINT fk_context_status_tenant 
        FOREIGN KEY (tenant_id, context_name) 
        REFERENCES contexts(tenant_id, name)
);

-- Indexes for efficient queries
CREATE INDEX idx_context_status_tenant_health ON context_status(tenant_id, overall_health);
CREATE INDEX idx_context_status_updated_at ON context_status(updated_at);
CREATE INDEX idx_context_status_staleness ON context_status(staleness_seconds) WHERE staleness_seconds > 300;

-- Partial indexes for service-specific queries
CREATE INDEX idx_context_status_vault_error ON context_status(tenant_id) 
    WHERE vault_status = 'error';
CREATE INDEX idx_context_status_k8s_error ON context_status(tenant_id) 
    WHERE kubernetes_status = 'error';

-- GIN index for JSONB queries
CREATE INDEX idx_context_status_details_gin ON context_status 
    USING GIN (summary_json);
```

### Run History Table  
Optimized for historical analysis and trend queries.

```sql
CREATE TABLE run_history (
    correlation_id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    
    -- Run metadata
    action VARCHAR(100) NOT NULL,
    requested_by VARCHAR(255),
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_ms INTEGER NOT NULL,
    
    -- Results summary
    overall_status VARCHAR(20) NOT NULL,
    error_summary TEXT,
    service_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    
    -- Service participation
    services_run TEXT[], -- Array of service names that executed
    
    -- Performance metrics
    total_latency_ms INTEGER,
    avg_service_latency_ms INTEGER,
    max_service_latency_ms INTEGER,
    
    -- Detailed results (for debugging/analysis)
    results_json JSONB
);

-- Indexes for time-based and context queries
CREATE INDEX idx_run_history_context_time ON run_history(context_name, completed_at DESC);
CREATE INDEX idx_run_history_tenant_time ON run_history(tenant_id, completed_at DESC);
CREATE INDEX idx_run_history_status_time ON run_history(overall_status, completed_at DESC);
CREATE INDEX idx_run_history_action ON run_history(action, completed_at DESC);

-- Partial index for errors
CREATE INDEX idx_run_history_errors ON run_history(tenant_id, context_name, completed_at) 
    WHERE overall_status = 'error';

-- GIN index for service array queries
CREATE INDEX idx_run_history_services ON run_history USING GIN(services_run);
```

### Service Metrics Aggregation
Pre-computed metrics for dashboard and monitoring.

```sql
CREATE TABLE service_metrics (
    tenant_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    service_name VARCHAR(50) NOT NULL,
    date_hour TIMESTAMP WITH TIME ZONE NOT NULL, -- Hour-level granularity
    
    -- Request counts
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    failed_requests INTEGER DEFAULT 0,
    
    -- Response time statistics
    avg_response_time_ms INTEGER,
    p95_response_time_ms INTEGER,
    p99_response_time_ms INTEGER,
    max_response_time_ms INTEGER,
    
    -- Error analysis
    error_rate DECIMAL(5,2), -- Percentage
    top_error_codes TEXT[], -- Most common error types
    
    -- Health status distribution
    status_ok_count INTEGER DEFAULT 0,
    status_degraded_count INTEGER DEFAULT 0,
    status_error_count INTEGER DEFAULT 0,
    
    -- Cache performance
    cache_hit_rate DECIMAL(5,2),
    circuit_breaker_trips INTEGER DEFAULT 0,
    
    -- Metadata
    first_seen TIMESTAMP WITH TIME ZONE,
    last_seen TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    PRIMARY KEY (tenant_id, context_name, service_name, date_hour)
);

-- Indexes for time-series queries
CREATE INDEX idx_service_metrics_time_context ON service_metrics(context_name, date_hour DESC);
CREATE INDEX idx_service_metrics_time_service ON service_metrics(service_name, date_hour DESC);
CREATE INDEX idx_service_metrics_error_rate ON service_metrics(error_rate DESC, date_hour DESC) 
    WHERE error_rate > 1.0;
```

---

## Read Model Views

### Context Health Dashboard View
```sql
CREATE VIEW context_health_dashboard AS
SELECT 
    cs.tenant_id,
    cs.context_name,
    cs.overall_health,
    cs.staleness_seconds,
    cs.updated_at,
    
    -- Service status summary
    JSONB_BUILD_OBJECT(
        'vault', COALESCE(cs.vault_status, 'unknown'),
        'kubernetes', COALESCE(cs.kubernetes_status, 'unknown'),
        'argocd', COALESCE(cs.argocd_status, 'unknown'),
        'newrelic', COALESCE(cs.newrelic_status, 'unknown'),
        'git', COALESCE(cs.git_status, 'unknown')
    ) as service_status,
    
    -- Error summary
    ARRAY_REMOVE(ARRAY[
        CASE WHEN cs.vault_status = 'error' THEN 'vault: ' || COALESCE(cs.vault_error, 'unknown error') END,
        CASE WHEN cs.kubernetes_status = 'error' THEN 'kubernetes: ' || COALESCE(cs.kubernetes_error, 'unknown error') END,
        CASE WHEN cs.argocd_status = 'error' THEN 'argocd: ' || COALESCE(cs.argocd_error, 'unknown error') END,
        CASE WHEN cs.newrelic_status = 'error' THEN 'newrelic: ' || COALESCE(cs.newrelic_error, 'unknown error') END,
        CASE WHEN cs.git_status = 'error' THEN 'git: ' || COALESCE(cs.git_error, 'unknown error') END
    ], NULL) as active_errors,
    
    -- Latest run info
    cs.last_run_correlation_id,
    
    -- Context metadata from contexts table
    c.spec->'environment'->>'name' as environment,
    c.spec->'application'->>'name' as application,
    c.metadata->>'labels' as labels
    
FROM context_status cs
JOIN contexts c ON c.tenant_id = cs.tenant_id AND c.name = cs.context_name
ORDER BY cs.overall_health = 'error' DESC, cs.staleness_seconds DESC, cs.context_name;
```

### Service Performance Summary View
```sql
CREATE VIEW service_performance_summary AS
SELECT 
    sm.tenant_id,
    sm.service_name,
    DATE_TRUNC('day', sm.date_hour) as date,
    
    -- Daily aggregations
    SUM(sm.total_requests) as daily_requests,
    AVG(sm.avg_response_time_ms) as avg_response_time_ms,
    MAX(sm.p99_response_time_ms) as max_p99_response_time_ms,
    
    -- Error rates
    CASE 
        WHEN SUM(sm.total_requests) > 0 
        THEN (SUM(sm.failed_requests)::DECIMAL / SUM(sm.total_requests) * 100)
        ELSE 0 
    END as daily_error_rate,
    
    -- Availability (% of hours with successful requests)
    COUNT(*) FILTER (WHERE sm.successful_requests > 0)::DECIMAL / 24 * 100 as availability_percent,
    
    -- Circuit breaker activity
    SUM(sm.circuit_breaker_trips) as total_cb_trips,
    
    -- Cache performance
    AVG(sm.cache_hit_rate) as avg_cache_hit_rate

FROM service_metrics sm
WHERE sm.date_hour >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY sm.tenant_id, sm.service_name, DATE_TRUNC('day', sm.date_hour)
ORDER BY date DESC, daily_error_rate DESC;
```

### Context Trend Analysis View
```sql
CREATE VIEW context_trend_analysis AS
SELECT 
    rh.tenant_id,
    rh.context_name,
    DATE_TRUNC('hour', rh.completed_at) as hour,
    
    -- Request volume
    COUNT(*) as executions,
    AVG(rh.duration_ms) as avg_duration_ms,
    
    -- Success rates
    COUNT(*) FILTER (WHERE rh.overall_status = 'ok')::DECIMAL / COUNT(*) * 100 as success_rate,
    
    -- Service participation
    ARRAY_AGG(DISTINCT unnest(rh.services_run)) as services_used,
    
    -- Performance percentiles (approximate using array_agg and percentile functions)
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY rh.duration_ms) as p95_duration_ms,
    
    -- Error patterns
    ARRAY_AGG(DISTINCT rh.error_summary) FILTER (WHERE rh.error_summary IS NOT NULL) as error_patterns

FROM run_history rh
WHERE rh.completed_at >= NOW() - INTERVAL '7 days'
GROUP BY rh.tenant_id, rh.context_name, DATE_TRUNC('hour', rh.completed_at)
ORDER BY hour DESC;
```

---

## Read Model Materialization Functions

### Update Context Status Function
```sql
CREATE OR REPLACE FUNCTION update_context_status(
    p_tenant_id VARCHAR(255),
    p_context_name VARCHAR(255),
    p_service_name VARCHAR(50),
    p_status VARCHAR(20),
    p_error TEXT DEFAULT NULL,
    p_latency_ms INTEGER DEFAULT NULL,
    p_details JSONB DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    -- Upsert service-specific status
    INSERT INTO context_status (
        tenant_id, 
        context_name,
        updated_at
    ) VALUES (
        p_tenant_id, 
        p_context_name,
        NOW()
    )
    ON CONFLICT (context_name) DO UPDATE SET
        updated_at = NOW();
    
    -- Update specific service columns dynamically
    CASE p_service_name
        WHEN 'vault' THEN
            UPDATE context_status SET
                vault_status = p_status,
                vault_updated_at = NOW(),
                vault_error = p_error,
                vault_latency_ms = p_latency_ms,
                vault_details = COALESCE(p_details, vault_details)
            WHERE context_name = p_context_name;
            
        WHEN 'kubernetes' THEN
            UPDATE context_status SET
                kubernetes_status = p_status,
                kubernetes_updated_at = NOW(),
                kubernetes_error = p_error,
                kubernetes_latency_ms = p_latency_ms,
                kubernetes_details = COALESCE(p_details, kubernetes_details)
            WHERE context_name = p_context_name;
            
        -- Similar cases for other services...
    END CASE;
    
    -- Recalculate overall health
    PERFORM recalculate_overall_health(p_tenant_id, p_context_name);
END;
$$ LANGUAGE plpgsql;
```

### Recalculate Overall Health Function
```sql
CREATE OR REPLACE FUNCTION recalculate_overall_health(
    p_tenant_id VARCHAR(255),
    p_context_name VARCHAR(255)
) RETURNS VOID AS $$
DECLARE
    v_error_count INTEGER := 0;
    v_degraded_count INTEGER := 0;
    v_ok_count INTEGER := 0;
    v_overall_health VARCHAR(20);
    v_staleness INTEGER;
BEGIN
    -- Count status distribution
    SELECT 
        COUNT(*) FILTER (WHERE status = 'error'),
        COUNT(*) FILTER (WHERE status = 'degraded'),
        COUNT(*) FILTER (WHERE status = 'ok')
    INTO v_error_count, v_degraded_count, v_ok_count
    FROM (
        SELECT 
            CASE 
                WHEN vault_status = 'error' OR kubernetes_status = 'error' 
                  OR argocd_status = 'error' OR newrelic_status = 'error' 
                  OR git_status = 'error' THEN 'error'
                WHEN vault_status = 'degraded' OR kubernetes_status = 'degraded' 
                  OR argocd_status = 'degraded' OR newrelic_status = 'degraded' 
                  OR git_status = 'degraded' THEN 'degraded'
                ELSE 'ok'
            END as status
        FROM context_status
        WHERE context_name = p_context_name
    ) service_statuses;
    
    -- Determine overall health
    IF v_error_count > 0 THEN
        v_overall_health := 'error';
    ELSIF v_degraded_count > 0 THEN
        v_overall_health := 'degraded';
    ELSE
        v_overall_health := 'ok';
    END IF;
    
    -- Calculate staleness (seconds since least recent update)
    SELECT EXTRACT(EPOCH FROM (NOW() - LEAST(
        COALESCE(vault_updated_at, '1970-01-01'::timestamp),
        COALESCE(kubernetes_updated_at, '1970-01-01'::timestamp),
        COALESCE(argocd_updated_at, '1970-01-01'::timestamp),
        COALESCE(newrelic_updated_at, '1970-01-01'::timestamp),
        COALESCE(git_updated_at, '1970-01-01'::timestamp)
    )))::INTEGER
    INTO v_staleness
    FROM context_status
    WHERE context_name = p_context_name;
    
    -- Update overall status
    UPDATE context_status SET
        overall_health = v_overall_health,
        staleness_seconds = v_staleness,
        updated_at = NOW(),
        summary_json = JSONB_BUILD_OBJECT(
            'overall_health', v_overall_health,
            'staleness_seconds', v_staleness,
            'error_count', v_error_count,
            'degraded_count', v_degraded_count,
            'ok_count', v_ok_count,
            'last_updated', NOW()
        )
    WHERE context_name = p_context_name;
END;
$$ LANGUAGE plpgsql;
```

### Aggregate Service Metrics Function
```sql
CREATE OR REPLACE FUNCTION aggregate_service_metrics(
    p_start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '1 hour',
    p_end_time TIMESTAMP WITH TIME ZONE DEFAULT NOW()
) RETURNS VOID AS $$
BEGIN
    -- Aggregate metrics from result_events into service_metrics table
    INSERT INTO service_metrics (
        tenant_id,
        context_name,
        service_name,
        date_hour,
        total_requests,
        successful_requests,
        failed_requests,
        avg_response_time_ms,
        p95_response_time_ms,
        p99_response_time_ms,
        max_response_time_ms,
        error_rate,
        status_ok_count,
        status_degraded_count,
        status_error_count,
        first_seen,
        last_seen
    )
    SELECT 
        tenant_id,
        context_name,
        service_name,
        DATE_TRUNC('hour', created_at) as date_hour,
        
        COUNT(*) as total_requests,
        COUNT(*) FILTER (WHERE status = 'ok') as successful_requests,
        COUNT(*) FILTER (WHERE status = 'error') as failed_requests,
        
        AVG(latency_ms)::INTEGER as avg_response_time_ms,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms)::INTEGER as p95_response_time_ms,
        PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms)::INTEGER as p99_response_time_ms,
        MAX(latency_ms) as max_response_time_ms,
        
        (COUNT(*) FILTER (WHERE status = 'error')::DECIMAL / COUNT(*) * 100) as error_rate,
        
        COUNT(*) FILTER (WHERE status = 'ok') as status_ok_count,
        COUNT(*) FILTER (WHERE status = 'degraded') as status_degraded_count, 
        COUNT(*) FILTER (WHERE status = 'error') as status_error_count,
        
        MIN(created_at) as first_seen,
        MAX(created_at) as last_seen
        
    FROM result_events
    WHERE created_at BETWEEN p_start_time AND p_end_time
    GROUP BY tenant_id, context_name, service_name, DATE_TRUNC('hour', created_at)
    
    ON CONFLICT (tenant_id, context_name, service_name, date_hour) 
    DO UPDATE SET
        total_requests = EXCLUDED.total_requests,
        successful_requests = EXCLUDED.successful_requests,
        failed_requests = EXCLUDED.failed_requests,
        avg_response_time_ms = EXCLUDED.avg_response_time_ms,
        p95_response_time_ms = EXCLUDED.p95_response_time_ms,
        p99_response_time_ms = EXCLUDED.p99_response_time_ms,
        max_response_time_ms = EXCLUDED.max_response_time_ms,
        error_rate = EXCLUDED.error_rate,
        status_ok_count = EXCLUDED.status_ok_count,
        status_degraded_count = EXCLUDED.status_degraded_count,
        status_error_count = EXCLUDED.status_error_count,
        last_seen = EXCLUDED.last_seen,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
```

---

## Read Model Go Structures

### Context Status Response Models
```go
type ContextStatusResponse struct {
    ContextName      string                 `json:"context_name"`
    TenantID         string                 `json:"tenant_id"`
    UpdatedAt        time.Time              `json:"updated_at"`
    StalenessSeconds int                    `json:"staleness_seconds"`
    OverallHealth    string                 `json:"overall_health"`
    Summary          map[string]string      `json:"summary"`
    Details          map[string]interface{} `json:"details"`
    LastRun          *RunSummary            `json:"last_run,omitempty"`
    ServiceStatus    ServiceStatusBreakdown `json:"service_status"`
}

type ServiceStatusBreakdown struct {
    Vault      ServiceStatus `json:"vault"`
    Kubernetes ServiceStatus `json:"kubernetes"`
    ArgoCD     ServiceStatus `json:"argocd"`
    NewRelic   ServiceStatus `json:"newrelic"`
    Git        ServiceStatus `json:"git"`
}

type ServiceStatus struct {
    Status      string    `json:"status"`
    UpdatedAt   time.Time `json:"updated_at"`
    Error       string    `json:"error,omitempty"`
    LatencyMs   int       `json:"latency_ms,omitempty"`
    Fresh       bool      `json:"fresh"`
    StalenessSeconds int  `json:"staleness_seconds"`
}

type RunSummary struct {
    CorrelationID string    `json:"correlation_id"`
    Action        string    `json:"action"`
    Status        string    `json:"status"`
    RequestedAt   time.Time `json:"requested_at"`
    CompletedAt   time.Time `json:"completed_at"`
    DurationMs    int       `json:"duration_ms"`
    ErrorSummary  string    `json:"error_summary,omitempty"`
}
```

### Service Metrics Response Models
```go
type ServiceMetricsResponse struct {
    TenantID    string           `json:"tenant_id"`
    ServiceName string           `json:"service_name"`
    Period      string           `json:"period"`      // hour, day, week
    Metrics     []MetricDataPoint `json:"metrics"`
    Summary     MetricsSummary   `json:"summary"`
}

type MetricDataPoint struct {
    Timestamp           time.Time `json:"timestamp"`
    TotalRequests       int       `json:"total_requests"`
    SuccessfulRequests  int       `json:"successful_requests"`
    FailedRequests      int       `json:"failed_requests"`
    AvgResponseTimeMs   int       `json:"avg_response_time_ms"`
    P95ResponseTimeMs   int       `json:"p95_response_time_ms"`
    P99ResponseTimeMs   int       `json:"p99_response_time_ms"`
    ErrorRate           float64   `json:"error_rate"`
    CacheHitRate        float64   `json:"cache_hit_rate,omitempty"`
    CircuitBreakerTrips int       `json:"circuit_breaker_trips"`
}

type MetricsSummary struct {
    TotalRequests        int     `json:"total_requests"`
    OverallErrorRate     float64 `json:"overall_error_rate"`
    AvgResponseTimeMs    int     `json:"avg_response_time_ms"`
    P99ResponseTimeMs    int     `json:"p99_response_time_ms"`
    AvailabilityPercent  float64 `json:"availability_percent"`
    MostCommonErrors     []string `json:"most_common_errors"`
    TrendDirection       string   `json:"trend_direction"` // improving, stable, degrading
}
```

---

## Read Model Maintenance

### Automated Cleanup Jobs
```sql
-- Delete old run history (keep 90 days)
CREATE OR REPLACE FUNCTION cleanup_old_run_history() RETURNS VOID AS $$
BEGIN
    DELETE FROM run_history 
    WHERE completed_at < NOW() - INTERVAL '90 days';
    
    -- Update statistics
    ANALYZE run_history;
END;
$$ LANGUAGE plpgsql;

-- Delete old service metrics (keep 1 year)  
CREATE OR REPLACE FUNCTION cleanup_old_service_metrics() RETURNS VOID AS $$
BEGIN
    DELETE FROM service_metrics 
    WHERE date_hour < NOW() - INTERVAL '1 year';
    
    -- Update statistics
    ANALYZE service_metrics;
END;
$$ LANGUAGE plpgsql;
```

### Row Level Security for Read Models
```sql
-- Enable RLS on read model tables
ALTER TABLE context_status ENABLE ROW LEVEL SECURITY;
ALTER TABLE run_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_metrics ENABLE ROW LEVEL SECURITY;

-- Create tenant isolation policies
CREATE POLICY tenant_isolation_context_status ON context_status
    USING (tenant_id = current_setting('app.current_tenant_id'));

CREATE POLICY tenant_isolation_run_history ON run_history
    USING (tenant_id = current_setting('app.current_tenant_id'));

CREATE POLICY tenant_isolation_service_metrics ON service_metrics
    USING (tenant_id = current_setting('app.current_tenant_id'));
```

---

## Related Documentation

- [Context Data Model](./context-model.md) - Source data structures
- [Database Schema](./database-schema.md) - Write model tables
- [Integration Results](./integration-results.md) - Service result structures
- [ADR-002: Read model (CQRS-lite)](../adr/ADR-002-read-model-cqrs-lite.md)
- [ADR-007: Caching layers and TTL policies](../adr/ADR-007-caching-layers-and-ttl-policies.md)