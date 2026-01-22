# PHASE 1D: App+Environment+Context Aggregator Service

**Duration:** 3-4 days  
**Prerequisites:** Phase 1C completed (GitOps integration services working), multi-environment data available  
**Deliverable:** Manifest Aggregator service building consolidated, customer-scoped context status with App manifest synchronization, Environment manifest validation, and Context pairing dashboards

---

## Overview

Implement the Manifest Aggregator service that consumes App, Environment, and Context result messages from all integration services and builds a consolidated, customer-isolated "read model" for each Context pairing. This provides fast, denormalized data optimized for manifest dashboards including ApplicationSet status from App manifests, Vault validation from Environment manifests, and Context pairing correlation views, implementing the CQRS-lite pattern with manifest-specific enhancements.

## Success Criteria

✅ Manifest Aggregator service consuming results from all manifest integration services with customer isolation  
✅ App+Environment+Context read model database schema implemented  
✅ App manifest status aggregation including ApplicationSet monitoring working  
✅ Environment manifest validation correlation including Vault sources and cluster configs functional  
✅ Context pairing status correlation and synchronization tracking implemented  
✅ Customer-scoped status endpoints returning manifest aggregated data  
✅ Manifest operation history tracking with App+Environment correlation functional  
✅ Context-specific staleness tracking and pairing health calculation  
✅ Manifest dashboard data optimized for App+Environment+Context correlation views  
✅ Integration tests validating manifest aggregation and three-manifest correlation logic  

---

## Implementation Tasks

### Task 1: Manifest Read Model Database Schema

**File: `migrations/003_manifest_read_model.up.sql`**

```sql
-- Context pairing status read model - one row per customer context with App+Environment correlation
CREATE TABLE context_pairing_status (
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    environment_reference VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    overall_health VARCHAR(20) DEFAULT 'unknown', -- healthy, degraded, unhealthy, unknown
    staleness_seconds INTEGER DEFAULT 0,
    
    -- App manifest status aggregation
    app_manifest_health VARCHAR(20) DEFAULT 'unknown',
    app_manifest_updated_at TIMESTAMP WITH TIME ZONE,
    app_manifest_payload JSONB,
    app_manifest_error TEXT,
    applicationset_count INTEGER DEFAULT 0,
    helm_sources_count INTEGER DEFAULT 0,
    git_sources_count INTEGER DEFAULT 0,
    
    -- Environment manifest validation status
    environment_manifest_health VARCHAR(20) DEFAULT 'unknown',
    environment_manifest_updated_at TIMESTAMP WITH TIME ZONE,
    environment_manifest_payload JSONB,
    environment_manifest_error TEXT,
    vault_sources_count INTEGER DEFAULT 0,
    cluster_configs_count INTEGER DEFAULT 0,
    values_files_count INTEGER DEFAULT 0,
    
    -- Context pairing correlation status
    context_pairing_health VARCHAR(20) DEFAULT 'unknown',
    context_pairing_updated_at TIMESTAMP WITH TIME ZONE,
    context_pairing_payload JSONB,
    context_pairing_error TEXT,
    pairing_sync_status VARCHAR(50),
    last_deployment_time TIMESTAMP WITH TIME ZONE,
    
    PRIMARY KEY (context_name, customer_id),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    FOREIGN KEY (app_reference, customer_id) REFERENCES apps(name, customer_id) ON DELETE CASCADE,
    FOREIGN KEY (environment_reference, customer_id) REFERENCES environments(name, customer_id) ON DELETE CASCADE
);

-- App manifest correlation tracking for dashboard views
CREATE TABLE app_manifest_correlation (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    app_name VARCHAR(255) NOT NULL,
    
    -- ApplicationSet correlation from App manifest
    applicationset_name VARCHAR(255),
    applicationset_namespace VARCHAR(255),
    applicationset_health VARCHAR(20) DEFAULT 'unknown',
    applicationset_sync_status VARCHAR(50),
    last_sync_time TIMESTAMP WITH TIME ZONE,
    
    -- Helm sources status from App manifest
    helm_sources_status JSONB,
    git_sources_status JSONB,
    total_applications INTEGER DEFAULT 0,
    helm_values_file VARCHAR(500),
    helm_values_hash VARCHAR(64),
    
    -- Kubernetes workload status  
    deployment_status JSONB,
    pod_count INTEGER DEFAULT 0,
    ready_pod_count INTEGER DEFAULT 0,
    
    -- Vault secrets validation for this environment
    vault_secrets_validated INTEGER DEFAULT 0,
    vault_secrets_total INTEGER DEFAULT 0,
    vault_pod_correlations JSONB,
    
    -- Git correlation
    git_commit VARCHAR(40),
    git_branch VARCHAR(255),
    
    -- Environment health and metadata
    environment_health VARCHAR(20) DEFAULT 'unknown',
    last_deployed TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(context_name, customer_id, app_reference),
    FOREIGN KEY (context_name, customer_id) REFERENCES context_pairing_status(context_name, customer_id) ON DELETE CASCADE,
    FOREIGN KEY (app_reference, customer_id) REFERENCES apps(name, customer_id) ON DELETE CASCADE
);

-- Context pairing operations tracking 
CREATE TABLE context_pairing_operations (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    environment_reference VARCHAR(255) NOT NULL,
    
    -- Context pairing operation tracking
    operation_type VARCHAR(50) NOT NULL, -- sync, validate, correlate
    operation_status VARCHAR(50) DEFAULT 'unknown',
    operation_started_at TIMESTAMP WITH TIME ZONE,
    operation_completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Generated applications across environments
    total_applications INTEGER DEFAULT 0,
    healthy_applications INTEGER DEFAULT 0,
    synced_applications INTEGER DEFAULT 0,
    
    -- Environment distribution
    applications_by_environment JSONB, -- {\"dev\": 2, \"qa\": 2, \"prod\": 1}
    
    -- Bootstrap application correlation
    bootstrap_app_name VARCHAR(255),
    managed_by_bootstrap BOOLEAN DEFAULT false,
    
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(context_name, customer_id, applicationset_name),
    FOREIGN KEY (context_name, customer_id) REFERENCES gitops_context_status(context_name, customer_id) ON DELETE CASCADE
);

-- Vault secrets validation tracking
CREATE TABLE gitops_vault_validation_status (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    environment VARCHAR(20) NOT NULL,
    vault_path VARCHAR(500) NOT NULL,
    secret_name VARCHAR(255) NOT NULL,
    
    validation_status VARCHAR(50) DEFAULT 'pending', -- valid, invalid, missing, error
    required_keys_count INTEGER DEFAULT 0,
    valid_keys_count INTEGER DEFAULT 0,
    
    -- Pod environment correlation
    pod_correlations_count INTEGER DEFAULT 0,
    valid_pod_correlations INTEGER DEFAULT 0,
    pod_correlation_details JSONB,
    
    last_validated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    error_details TEXT,
    
    UNIQUE(context_name, customer_id, environment, vault_path),
    FOREIGN KEY (context_name, customer_id) REFERENCES gitops_context_status(context_name, customer_id) ON DELETE CASCADE
);

-- Indexes for GitOps query patterns
CREATE INDEX idx_gitops_context_status_customer ON gitops_context_status (customer_id);
CREATE INDEX idx_gitops_context_status_health ON gitops_context_status (overall_health);
CREATE INDEX idx_gitops_context_status_updated ON gitops_context_status (updated_at DESC);

CREATE INDEX idx_gitops_environment_status_context_env ON gitops_environment_status (context_name, customer_id, environment);
CREATE INDEX idx_gitops_environment_status_app ON gitops_environment_status (application_name, environment);
CREATE INDEX idx_gitops_environment_status_health ON gitops_environment_status (environment_health);

CREATE INDEX idx_gitops_applicationset_status_customer ON gitops_applicationset_status (customer_id);
CREATE INDEX idx_gitops_applicationset_status_health ON gitops_applicationset_status (health_status, sync_status);

CREATE INDEX idx_gitops_vault_validation_customer_env ON gitops_vault_validation_status (customer_id, environment);
CREATE INDEX idx_gitops_vault_validation_status ON gitops_vault_validation_status (validation_status);

-- GitOps result event history - for tracking individual GitOps service results with customer isolation
CREate TABLE gitops_result_events (
    id SERIAL PRIMARY KEY,
    correlation_id UUID NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    service_name VARCHAR(50) NOT NULL, -- vault-secrets-operator, applicationset-monitor, etc.
    gitops_type VARCHAR(50) NOT NULL, -- applicationset, vault, environment, git, helm
    action VARCHAR(50) NOT NULL,
    environment VARCHAR(20), -- for environment-specific operations
    status VARCHAR(20) NOT NULL, -- healthy, degraded, unhealthy, error
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    latency_ms INTEGER,
    error_message TEXT,
    result_payload JSONB,
    gitops_metadata JSONB, -- ApplicationSet name, Vault path, etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    FOREIGN KEY (context_name, customer_id) REFERENCES gitops_context_status(context_name, customer_id) ON DELETE CASCADE
);

-- Indexes for GitOps result history queries
CREATE INDEX idx_gitops_result_events_context ON gitops_result_events (context_name, customer_id, completed_at DESC);
CREATE INDEX idx_gitops_result_events_correlation ON gitops_result_events (correlation_id);
CREATE INDEX idx_gitops_result_events_service ON gitops_result_events (service_name, completed_at DESC);
CREATE INDEX idx_gitops_result_events_gitops_type ON gitops_result_events (gitops_type, environment);
CREATE INDEX idx_gitops_result_events_customer ON gitops_result_events (customer_id, completed_at DESC);

-- Function to calculate overall manifest health based on individual service statuses
CREATE OR REPLACE FUNCTION calculate_manifest_overall_health(
    app_manifest_health VARCHAR(20),
    environment_manifest_health VARCHAR(20),
    context_pairing_health VARCHAR(20)
) RETURNS VARCHAR(20) AS $$
BEGIN
    -- If any manifest service is unhealthy, overall is unhealthy
    IF applicationset_health = 'unhealthy' OR vault_secrets_health = 'unhealthy' OR 
       kubernetes_multi_env_health = 'unhealthy' OR customer_git_health = 'unhealthy' OR 
       helm_values_health = 'unhealthy' THEN
        RETURN 'unhealthy';
    END IF;
    
    -- If any GitOps service is degraded, overall is degraded  
    IF applicationset_health = 'degraded' OR vault_secrets_health = 'degraded' OR
       kubernetes_multi_env_health = 'degraded' OR customer_git_health = 'degraded' OR
       helm_values_health = 'degraded' THEN
        RETURN 'degraded';
    END IF;
    
    -- If we have at least one healthy status and no unhealthy/degraded, we're healthy
    IF applicationset_health = 'healthy' OR vault_secrets_health = 'healthy' OR
       kubernetes_multi_env_health = 'healthy' OR customer_git_health = 'healthy' OR
       helm_values_health = 'healthy' THEN
        RETURN 'healthy';
    END IF;
    
    -- All services unknown
    RETURN 'unknown';
END;
$$ LANGUAGE plpgsql;

-- Trigger to update overall GitOps health when service statuses change
CREATE OR REPLACE FUNCTION update_gitops_overall_health()
RETURNS TRIGGER AS $$
BEGIN
    NEW.overall_health = calculate_gitops_overall_health(
        NEW.applicationset_health,
        NEW.vault_secrets_health, 
        NEW.kubernetes_multi_env_health,
        NEW.customer_git_health,
        NEW.helm_values_health
    );
    
    NEW.updated_at = NOW();
    
    -- Calculate staleness (seconds since oldest GitOps service update)
    NEW.staleness_seconds = EXTRACT(EPOCH FROM (
        NOW() - LEAST(
            COALESCE(NEW.applicationset_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.vault_secrets_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.kubernetes_multi_env_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.customer_git_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.helm_values_updated_at, '1970-01-01'::timestamp)
        )
    ))::INTEGER;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_gitops_context_status_health 
    BEFORE UPDATE ON gitops_context_status
    FOR EACH ROW EXECUTE FUNCTION update_gitops_overall_health();
```

### Task 2: GitOps Aggregator Service Implementation

**File: `cmd/gitops-aggregator/main.go`**

```go
func main() {
    cfg := loadConfig()
    
    // Database connection
    db := setupDatabase(cfg.DatabaseURL)
    defer db.Close()
    
    // RabbitMQ connection
    messageBus, err := events.NewMessageBus(cfg.RabbitMQURL)
    if err != nil {
        log.Fatal("Failed to connect to RabbitMQ:", err)
    }
    defer messageBus.Close()
    
    // Aggregator setup
    contextStore := storage.NewContextStore(db)
    readStore := storage.NewReadModelStore(db)
    aggregator := NewAggregator(readStore, contextStore)
    
    // Result consumer
    consumer := events.NewResultConsumer(messageBus)
    consumer.RegisterHandler("vault", aggregator)
    consumer.RegisterHandler("argocd", aggregator)
    consumer.RegisterHandler("newrelic", aggregator)
    consumer.RegisterHandler("kubernetes", aggregator)
    consumer.RegisterHandler("git", aggregator)
    
    if err := consumer.Start(); err != nil {
        log.Fatal("Failed to start consumer:", err)
    }
    defer consumer.Stop()
    
    log.Println("Aggregator service started")
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    log.Println("Shutting down aggregator service")
}
```

**File: `cmd/gitops-aggregator/aggregator.go`**

```go
type GitOpsAggregator struct {
    readStore           *storage.GitOpsReadModelStore
    contextStore        *storage.GitOpsContextStore
    customerValidator   *auth.CustomerValidator
    environmentCorrelator *correlation.EnvironmentCorrelator
}

func NewGitOpsAggregator(readStore *storage.GitOpsReadModelStore, contextStore *storage.GitOpsContextStore) *GitOpsAggregator {
    return &GitOpsAggregator{
        readStore:           readStore,
        contextStore:        contextStore,
        customerValidator:   auth.NewCustomerValidator(),
        environmentCorrelator: correlation.NewEnvironmentCorrelator(),
    }
}

// Implement GitOpsResultHandler interface
func (ga *GitOpsAggregator) HandleGitOpsResult(result *api.GitOpsResultMessage) error {
    log.Printf("Processing GitOps result from %s for context %s (customer: %s, type: %s, correlation: %s)", 
        result.ServiceName, result.ContextName, result.CustomerID, result.GitOpsType, result.CorrelationID)
    
    // Validate customer access
    if err := ga.customerValidator.ValidateAccess(result.CustomerID, result.ContextName); err != nil {
        return fmt.Errorf("customer access validation failed: %w", err)
    }
    
    // Store the individual GitOps result event
    if err := ga.readStore.StoreGitOpsResultEvent(result); err != nil {
        return fmt.Errorf("failed to store GitOps result event: %w", err)
    }
    
    // Update the aggregated GitOps context status
    if err := ga.readStore.UpdateGitOpsContextStatus(result); err != nil {
        return fmt.Errorf("failed to update GitOps context status: %w", err)
    }
    
    // Handle environment-specific aggregation
    if result.Environment != "" {
        if err := ga.readStore.UpdateEnvironmentStatus(result); err != nil {
            return fmt.Errorf("failed to update environment status: %w", err)
        }
    }
    
    // Handle ApplicationSet-specific aggregation
    if result.GitOpsType == "applicationset" && result.ApplicationSetData != nil {
        if err := ga.readStore.UpdateApplicationSetStatus(result); err != nil {
            return fmt.Errorf("failed to update ApplicationSet status: %w", err)
        }
    }
    
    // Handle Vault validation aggregation
    if result.GitOpsType == "vault" && result.VaultValidationData != nil {
        if err := ga.readStore.UpdateVaultValidationStatus(result); err != nil {
            return fmt.Errorf("failed to update Vault validation status: %w", err)
        }
    }
    
    // Trigger cross-environment correlation if needed
    if err := ga.environmentCorrelator.CorrelateEnvironments(result.CustomerID, result.ContextName); err != nil {
        log.Printf("Warning: environment correlation failed for %s/%s: %v", result.CustomerID, result.ContextName, err)
        // Don't fail the entire aggregation for correlation issues
    }
    
    log.Printf("Successfully processed GitOps result from %s for context %s (customer: %s)", 
        result.ServiceName, result.ContextName, result.CustomerID)
    
    return nil
}
```

### Task 3: Read Model Store Implementation

**File: `internal/storage/readmodel.go`**

```go
type ReadModelStore struct {
    db *sql.DB
}

func NewReadModelStore(db *sql.DB) *ReadModelStore {
    return &ReadModelStore{db: db}
}

func (r *ReadModelStore) StoreResultEvent(result *api.ResultMessage) error {
    query := `
        INSERT INTO result_events (
            correlation_id, context_name, service_name, action, status,
            requested_by, requested_at, completed_at, latency_ms, 
            error_message, result_payload
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `
    
    var latencyMs *int
    if payload, ok := result.ResultPayload.(map[string]interface{}); ok {
        if latency, exists := payload["latency_ms"]; exists {
            if l, ok := latency.(float64); ok {
                latencyInt := int(l)
                latencyMs = &latencyInt
            }
        }
    }
    
    var errorMessage *string
    if result.ErrorMessage != "" {
        errorMessage = &result.ErrorMessage
    }
    
    var payloadJSON []byte
    if result.ResultPayload != nil {
        var err error
        payloadJSON, err = json.Marshal(result.ResultPayload)
        if err != nil {
            return fmt.Errorf("failed to marshal result payload: %w", err)
        }
    }
    
    _, err := r.db.Exec(query,
        result.CorrelationID,
        result.ContextName,
        result.ServiceName,
        result.Action,
        result.Status,
        result.RequestedBy,
        result.RequestedAt,
        result.CompletedAt,
        latencyMs,
        errorMessage,
        payloadJSON,
    )
    
    if err != nil {
        return fmt.Errorf("failed to insert result event: %w", err)
    }
    
    return nil
}

func (r *ReadModelStore) UpdateContextStatus(result *api.ResultMessage) error {
    // First ensure the context status record exists
    if err := r.ensureContextStatusExists(result.ContextName); err != nil {
        return fmt.Errorf("failed to ensure context status exists: %w", err)
    }
    
    // Build the update query based on service name
    var query string
    var args []interface{}
    
    switch result.ServiceName {
    case "vault":
        query = `
            UPDATE context_status 
            SET vault_status = $2, 
                vault_updated_at = $3, 
                vault_payload = $4, 
                vault_error = $5
            WHERE context_name = $1
        `
    case "argocd":
        query = `
            UPDATE context_status 
            SET argocd_status = $2, 
                argocd_updated_at = $3, 
                argocd_payload = $4, 
                argocd_error = $5
            WHERE context_name = $1
        `
    case "newrelic":
        query = `
            UPDATE context_status 
            SET newrelic_status = $2, 
                newrelic_updated_at = $3, 
                newrelic_payload = $4, 
                newrelic_error = $5
            WHERE context_name = $1
        `
    case "kubernetes":
        query = `
            UPDATE context_status 
            SET kubernetes_status = $2, 
                kubernetes_updated_at = $3, 
                kubernetes_payload = $4, 
                kubernetes_error = $5
            WHERE context_name = $1
        `
    case "git":
        query = `
            UPDATE context_status 
            SET git_status = $2, 
                git_updated_at = $3, 
                git_payload = $4, 
                git_error = $5
            WHERE context_name = $1
        `
    default:
        return fmt.Errorf("unknown service name: %s", result.ServiceName)
    }
    
    // Prepare arguments
    args = []interface{}{
        result.ContextName,
        result.Status,
        result.CompletedAt,
    }
    
    // Marshal payload
    var payloadJSON []byte
    if result.ResultPayload != nil {
        var err error
        payloadJSON, err = json.Marshal(result.ResultPayload)
        if err != nil {
            return fmt.Errorf("failed to marshal payload: %w", err)
        }
    }
    args = append(args, payloadJSON)
    
    // Add error message
    var errorMessage *string
    if result.ErrorMessage != "" {
        errorMessage = &result.ErrorMessage
    }
    args = append(args, errorMessage)
    
    // Execute update
    _, err := r.db.Exec(query, args...)
    if err != nil {
        return fmt.Errorf("failed to update context status: %w", err)
    }
    
    return nil
}

func (r *ReadModelStore) ensureContextStatusExists(contextName string) error {
    query := `
        INSERT INTO context_status (context_name) 
        VALUES ($1) 
        ON CONFLICT (context_name) DO NOTHING
    `
    
    _, err := r.db.Exec(query, contextName)
    return err
}

type ContextStatus struct {
    ContextName     string                 `json:"context_name"`
    UpdatedAt       time.Time              `json:"updated_at"`
    StalenessSeconds int                   `json:"staleness_seconds"`
    OverallHealth   string                 `json:"overall_health"`
    Summary         map[string]string      `json:"summary"`
    Details         map[string]interface{} `json:"details"`
}

func (r *ReadModelStore) GetContextStatus(contextName string) (*ContextStatus, error) {
    query := `
        SELECT 
            context_name, updated_at, staleness_seconds, overall_health,
            vault_status, vault_updated_at, vault_payload, vault_error,
            argocd_status, argocd_updated_at, argocd_payload, argocd_error,
            newrelic_status, newrelic_updated_at, newrelic_payload, newrelic_error,
            kubernetes_status, kubernetes_updated_at, kubernetes_payload, kubernetes_error,
            git_status, git_updated_at, git_payload, git_error
        FROM context_status 
        WHERE context_name = $1
    `
    
    row := r.db.QueryRow(query, contextName)
    
    var status ContextStatus
    var vaultPayload, argoCDPayload, newrelicPayload, kubernetesPayload, gitPayload *string
    var vaultError, argoCDError, newrelicError, kubernetesError, gitError *string
    var vaultUpdated, argoCDUpdated, newrelicUpdated, kubernetesUpdated, gitUpdated *time.Time
    var vaultStatus, argoCDStatus, newrelicStatus, kubernetesStatus, gitStatus string
    
    err := row.Scan(
        &status.ContextName, &status.UpdatedAt, &status.StalenessSeconds, &status.OverallHealth,
        &vaultStatus, &vaultUpdated, &vaultPayload, &vaultError,
        &argoCDStatus, &argoCDUpdated, &argoCDPayload, &argoCDError,
        &newrelicStatus, &newrelicUpdated, &newrelicPayload, &newrelicError,
        &kubernetesStatus, &kubernetesUpdated, &kubernetesPayload, &kubernetesError,
        &gitStatus, &gitUpdated, &gitPayload, &gitError,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, storage.ErrContextStatusNotFound
        }
        return nil, fmt.Errorf("failed to get context status: %w", err)
    }
    
    // Build summary
    status.Summary = map[string]string{
        "vault":      vaultStatus,
        "argocd":     argoCDStatus,
        "newrelic":   newrelicStatus,
        "kubernetes": kubernetesStatus,
        "git":        gitStatus,
    }
    
    // Build details
    status.Details = make(map[string]interface{})
    
    if vaultPayload != nil {
        var payload interface{}
        if err := json.Unmarshal([]byte(*vaultPayload), &payload); err == nil {
            status.Details["vault"] = payload
        }
    }
    
    // Similar unmarshaling for other services...
    // (repeat pattern for argocd, newrelic, kubernetes, git)
    
    return &status, nil
}

type RunHistoryItem struct {
    CorrelationID string    `json:"correlation_id"`
    Action        string    `json:"action"`
    RequestedBy   string    `json:"requested_by"`
    RequestedAt   time.Time `json:"requested_at"`
    CompletedAt   time.Time `json:"completed_at"`
    Services      []string  `json:"services"`
    OverallStatus string    `json:"overall_status"`
    LatencyMs     int       `json:"latency_ms"`
}

func (r *ReadModelStore) GetRunHistory(contextName string, limit int) ([]*RunHistoryItem, error) {
    query := `
        SELECT 
            correlation_id,
            action,
            requested_by,
            MIN(requested_at) as requested_at,
            MAX(completed_at) as completed_at,
            ARRAY_AGG(DISTINCT service_name) as services,
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
    `
    
    rows, err := r.db.Query(query, contextName, limit)
    if err != nil {
        return nil, fmt.Errorf("failed to query run history: %w", err)
    }
    defer rows.Close()
    
    var history []*RunHistoryItem
    
    for rows.Next() {
        var item RunHistoryItem
        var servicesArray pq.StringArray
        
        err := rows.Scan(
            &item.CorrelationID,
            &item.Action,
            &item.RequestedBy,
            &item.RequestedAt,
            &item.CompletedAt,
            &servicesArray,
            &item.OverallStatus,
            &item.LatencyMs,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan run history row: %w", err)
        }
        
        item.Services = []string(servicesArray)
        history = append(history, &item)
    }
    
    return history, nil
}
```

### Task 4: Status API Endpoints

**File: `internal/handlers/status.go`**

```go
type StatusHandler struct {
    readStore *storage.ReadModelStore
}

func NewStatusHandler(readStore *storage.ReadModelStore) *StatusHandler {
    return &StatusHandler{readStore: readStore}
}

func (sh *StatusHandler) GetContextStatus(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    
    status, err := sh.readStore.GetContextStatus(contextName)
    if err != nil {
        if err == storage.ErrContextStatusNotFound {
            http.Error(w, "Context status not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to get context status", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

func (sh *StatusHandler) GetRunHistory(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    
    // Parse limit parameter
    limitStr := r.URL.Query().Get("limit")
    limit := 50 // default
    if limitStr != "" {
        if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
            limit = parsedLimit
        }
    }
    
    history, err := sh.readStore.GetRunHistory(contextName, limit)
    if err != nil {
        http.Error(w, "Failed to get run history", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "runs": history,
        "limit": limit,
        "count": len(history),
    })
}
```

### Task 5: Update API Gateway with Status Endpoints

**File: `cmd/gateway/main.go`** (update existing)

```go
func setupRouter(contextHandler *handlers.ContextHandler, 
                actionHandler *handlers.ActionHandler,
                statusHandler *handlers.StatusHandler) *mux.Router {
    r := mux.NewRouter()
    
    // Middleware
    r.Use(auth.BasicAuthMiddleware())
    r.Use(loggingMiddleware())
    
    // Context CRUD routes
    r.HandleFunc("/contexts", contextHandler.CreateContext).Methods("POST")
    r.HandleFunc("/contexts", contextHandler.ListContexts).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.GetContext).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.UpdateContext).Methods("PUT")
    r.HandleFunc("/contexts/{name}", contextHandler.DeleteContext).Methods("DELETE")
    
    // Action routes
    r.HandleFunc("/contexts/{name}/actions/refresh", actionHandler.HandleRefresh).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/validate", actionHandler.HandleValidate).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/inspect", actionHandler.HandleInspect).Methods("POST")
    
    // NEW: Status routes (from read model)
    r.HandleFunc("/contexts/{name}/status", statusHandler.GetContextStatus).Methods("GET")
    r.HandleFunc("/contexts/{name}/runs", statusHandler.GetRunHistory).Methods("GET")
    
    // Health check
    r.HandleFunc("/health", healthCheck).Methods("GET")
    
    return r
}
```

---

## Testing Requirements

### Unit Tests

**File: `internal/storage/readmodel_test.go`**
- Test result event storage
- Test context status updates  
- Test status aggregation logic
- Test run history queries
- Test staleness calculation

**File: `cmd/aggregator/aggregator_test.go`**
- Test result message processing
- Test error handling for malformed messages
- Test concurrent result processing

### Integration Tests

**File: `cmd/aggregator/integration_test.go`**
- Test complete flow: result message → aggregation → status query
- Test multiple services updating same context
- Test staleness tracking accuracy
- Test overall health calculation

**Database Tests**
- Test database triggers and functions
- Test health calculation logic with various combinations
- Test staleness calculation accuracy

---

## GitOps Validation Checklist

Before marking Phase 1D complete:

**GitOps Database Schema:**
- [ ] GitOps read model tables created successfully with customer isolation
- [ ] Multi-environment status tracking tables operational
- [ ] ApplicationSet status tracking working across environments
- [ ] Vault validation status tracking with pod correlations functional
- [ ] GitOps triggers and functions working correctly  
- [ ] GitOps health calculation logic produces expected results with environment awareness
- [ ] Multi-environment staleness calculation working accurately

**GitOps Aggregator Service:**
- [ ] Consumes GitOps result messages from all integration services with customer isolation
- [ ] Stores individual GitOps result events correctly with metadata
- [ ] Updates GitOps context status aggregations properly
- [ ] Handles ApplicationSet result aggregation across environments
- [ ] Processes Vault secret validation results with pod correlation
- [ ] Updates environment-specific status correctly
- [ ] Handles malformed GitOps messages gracefully
- [ ] Customer isolation enforced throughout aggregation process
- [ ] Logs GitOps processing activities clearly

**GitOps Status Endpoints:**
- [ ] `/api/v1/contexts/{name}/status` returns aggregated GitOps status with customer filtering
- [ ] `/api/v1/contexts/{name}/environments` returns per-environment status for app tabs
- [ ] `/api/v1/contexts/{name}/applicationsets` returns ApplicationSet monitoring data
- [ ] `/api/v1/contexts/{name}/vault-validations` returns secret validation status
- [ ] `/api/v1/contexts/{name}/runs` returns GitOps run history with environment correlation
- [ ] Status responses include all expected GitOps fields and metadata
- [ ] Environment-specific data properly structured for UI tabs (dev/qa/uat/prod)
- [ ] ApplicationSet correlation data includes Bootstrap Application relationships
- [ ] Customer isolation enforced in all status endpoints
- [ ] Error responses are appropriate (404, 500) with customer context

**GitOps Data Consistency:**
- [ ] Overall GitOps health reflects individual service statuses across environments
- [ ] Environment-specific health calculations work correctly
- [ ] ApplicationSet health aggregation spans multiple environments properly
- [ ] Vault secret validation correlates with pod environment variables
- [ ] Multi-environment staleness accurately reflects oldest environment update
- [ ] GitOps run history groups results by correlation ID with environment context
- [ ] GitOps service status updates are atomic with customer isolation
- [ ] Cross-environment correlation maintains data consistency

**GitOps Integration:**
- [ ] End-to-end GitOps flow: action → services → aggregation → status works with customer isolation
- [ ] Multiple concurrent GitOps updates handled correctly per customer
- [ ] ApplicationSet monitoring updates don't interfere with Vault validation
- [ ] Environment-specific updates maintain consistency across dev/qa/uat/prod
- [ ] GitOps service failures don't break aggregation for other services
- [ ] Missing GitOps context status records created automatically with customer context
- [ ] Multi-environment correlation triggers properly on relevant updates

---

## Performance Considerations

- **Read Optimization:** Status queries are served from denormalized read model
- **Write Performance:** Asynchronous aggregation doesn't block integration services  
- **Staleness Tracking:** Calculated in database trigger for consistency
- **History Retention:** Consider archiving old result events (future enhancement)

---

## GitOps Next Steps

Upon completion, Phase 1D provides:
- **Fast, consolidated GitOps status views** for contexts with customer isolation and multi-environment support
- **Per-application environment tabs** optimized for GitOps dashboards (dev/qa/uat/prod)
- **ApplicationSet monitoring aggregation** with Bootstrap Application correlation and environment tracking
- **Vault secret validation correlation** with pod environment variable tracking across environments
- **Multi-environment drift detection** and consistency tracking
- **Complete GitOps audit trail** of service results with customer isolation and environment context
- **Customer-scoped GitOps dashboards** ready for Web UI implementation
- **Environment-specific run history** for GitOps troubleshooting with correlation tracking
- **Foundation for GitOps observability** with performance metrics and multi-environment query patterns

**Handoff to Phase 1E:** GitOps Observability components can now instrument the complete GitOps system, including ApplicationSet monitoring performance, Vault validation metrics, multi-environment correlation efficiency, and customer-scoped GitOps status query patterns optimized for DevOps workflows.