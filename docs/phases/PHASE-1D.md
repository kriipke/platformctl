# PHASE 1D: Aggregator Service

**Duration:** 2-3 days  
**Prerequisites:** Phase 1C completed (at least 2 integration services working)  
**Deliverable:** Aggregator service building consolidated context status from integration results

---

## Overview

Implement the Aggregator service that consumes result messages from all integration services and builds a consolidated "read model" for each context. This provides fast, denormalized data for the UI and status endpoints, implementing the CQRS-lite pattern described in the README.

## Success Criteria

✅ Aggregator service consuming results from all integration services  
✅ Read model database schema implemented  
✅ Context status aggregation logic working  
✅ Staleness tracking implemented  
✅ Status endpoints returning aggregated data  
✅ Run history tracking functional  
✅ Partial update handling (graceful degradation)  
✅ Integration tests validating aggregation logic  

---

## Implementation Tasks

### Task 1: Read Model Database Schema

**File: `migrations/003_read_model.up.sql`**

```sql
-- Context status read model - one row per context
CREATE TABLE context_status (
    context_name VARCHAR(255) PRIMARY KEY REFERENCES contexts(name) ON DELETE CASCADE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    staleness_seconds INTEGER DEFAULT 0,
    overall_health VARCHAR(20) DEFAULT 'unknown', -- ok, degraded, error, unknown
    
    -- Service-specific status
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
    git_error TEXT
);

-- Index for status queries
CREATE INDEX idx_context_status_health ON context_status (overall_health);
CREATE INDEX idx_context_status_updated ON context_status (updated_at DESC);

-- Result event history - for tracking individual service results
CREATE TABLE result_events (
    id SERIAL PRIMARY KEY,
    correlation_id UUID NOT NULL,
    context_name VARCHAR(255) NOT NULL REFERENCES contexts(name) ON DELETE CASCADE,
    service_name VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL, -- ok, degraded, error
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    latency_ms INTEGER,
    error_message TEXT,
    result_payload JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for result history queries
CREATE INDEX idx_result_events_context ON result_events (context_name, completed_at DESC);
CREATE INDEX idx_result_events_correlation ON result_events (correlation_id);
CREATE INDEX idx_result_events_service ON result_events (service_name, completed_at DESC);

-- Function to calculate overall health based on individual service statuses
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

-- Trigger to update overall health when service statuses change
CREATE OR REPLACE FUNCTION update_overall_health()
RETURNS TRIGGER AS $$
BEGIN
    NEW.overall_health = calculate_overall_health(
        NEW.vault_status,
        NEW.argocd_status, 
        NEW.newrelic_status,
        NEW.kubernetes_status,
        NEW.git_status
    );
    
    NEW.updated_at = NOW();
    
    -- Calculate staleness (seconds since oldest service update)
    NEW.staleness_seconds = EXTRACT(EPOCH FROM (
        NOW() - LEAST(
            COALESCE(NEW.vault_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.argocd_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.newrelic_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.kubernetes_updated_at, '1970-01-01'::timestamp),
            COALESCE(NEW.git_updated_at, '1970-01-01'::timestamp)
        )
    ))::INTEGER;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_context_status_health 
    BEFORE UPDATE ON context_status
    FOR EACH ROW EXECUTE FUNCTION update_overall_health();
```

### Task 2: Aggregator Service Implementation

**File: `cmd/aggregator/main.go`**

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

**File: `cmd/aggregator/aggregator.go`**

```go
type Aggregator struct {
    readStore    *storage.ReadModelStore
    contextStore *storage.ContextStore
}

func NewAggregator(readStore *storage.ReadModelStore, contextStore *storage.ContextStore) *Aggregator {
    return &Aggregator{
        readStore:    readStore,
        contextStore: contextStore,
    }
}

// Implement ResultHandler interface
func (a *Aggregator) HandleResult(result *api.ResultMessage) error {
    log.Printf("Processing result from %s for context %s (correlation: %s)", 
        result.ServiceName, result.ContextName, result.CorrelationID)
    
    // Store the individual result event
    if err := a.readStore.StoreResultEvent(result); err != nil {
        return fmt.Errorf("failed to store result event: %w", err)
    }
    
    // Update the aggregated context status
    if err := a.readStore.UpdateContextStatus(result); err != nil {
        return fmt.Errorf("failed to update context status: %w", err)
    }
    
    log.Printf("Successfully processed result from %s for context %s", 
        result.ServiceName, result.ContextName)
    
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

## Validation Checklist

Before marking Phase 1D complete:

**Database Schema:**
- [ ] Read model tables created successfully
- [ ] Triggers and functions working correctly  
- [ ] Health calculation logic produces expected results
- [ ] Staleness calculation working accurately

**Aggregator Service:**
- [ ] Consumes result messages from all integration services
- [ ] Stores individual result events correctly
- [ ] Updates context status aggregations properly
- [ ] Handles malformed messages gracefully
- [ ] Logs processing activities clearly

**Status Endpoints:**
- [ ] `/contexts/{name}/status` returns aggregated status
- [ ] `/contexts/{name}/runs` returns run history
- [ ] Status response includes all expected fields
- [ ] Run history includes correlation tracking
- [ ] Error responses are appropriate (404, 500)

**Data Consistency:**
- [ ] Overall health reflects individual service statuses
- [ ] Staleness accurately reflects oldest service update
- [ ] Run history groups results by correlation ID
- [ ] Service status updates are atomic

**Integration:**
- [ ] End-to-end flow: action → services → aggregation → status works
- [ ] Multiple concurrent updates handled correctly
- [ ] Service failures don't break aggregation
- [ ] Missing context status records created automatically

---

## Performance Considerations

- **Read Optimization:** Status queries are served from denormalized read model
- **Write Performance:** Asynchronous aggregation doesn't block integration services  
- **Staleness Tracking:** Calculated in database trigger for consistency
- **History Retention:** Consider archiving old result events (future enhancement)

---

## Next Steps

Upon completion, Phase 1D provides:
- Fast, consolidated status views for contexts
- Complete audit trail of service results
- Foundation for Web UI status displays
- Run history for troubleshooting

**Handoff to Phase 1E:** Observability components can now instrument the complete system, including aggregation performance metrics and status query patterns.