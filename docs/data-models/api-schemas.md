# API Message Schemas

**Version:** 1.0  
**Date:** 2026-01-21  
**Phases:** 1B (Commands/Results), 1D (Status APIs)  

---

## Overview

This document defines the message schemas used throughout ContextOps for RabbitMQ messages, REST API requests/responses, and inter-service communication.

---

## RabbitMQ Message Schemas

### Base Message Envelope

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

### Command Messages

```go
type CommandMessage struct {
    MessageEnvelope
    Priority    int               `json:"priority,omitempty"`     // 1-10, higher = more priority
    Timeout     *time.Duration    `json:"timeout,omitempty"`      // Max execution time
    RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"` // Override default retry
}

type RetryPolicy struct {
    MaxAttempts int           `json:"max_attempts"`
    BaseDelay   time.Duration `json:"base_delay"`
    MaxDelay    time.Duration `json:"max_delay"`
    Multiplier  float64       `json:"multiplier"`
}
```

### Result Messages

```go
type ResultMessage struct {
    MessageEnvelope
    ServiceName   string      `json:"service_name"`
    Status        string      `json:"status"`        // ok, degraded, error
    CompletedAt   time.Time   `json:"completed_at"`
    LatencyMs     int         `json:"latency_ms"`
    ErrorMessage  string      `json:"error_message,omitempty"`
    ErrorCode     string      `json:"error_code,omitempty"`
    ResultPayload interface{} `json:"result_payload"`
    Metadata      Metadata    `json:"metadata,omitempty"`
}

type Metadata struct {
    RetryAttempt    int                    `json:"retry_attempt,omitempty"`
    CircuitBreaker  string                 `json:"circuit_breaker,omitempty"`
    CacheHit        bool                   `json:"cache_hit,omitempty"`
    ExternalAPITime int                    `json:"external_api_time_ms,omitempty"`
    Custom          map[string]interface{} `json:"custom,omitempty"`
}
```

---

## REST API Schemas

### Context API

#### Create/Update Context Request
```go
type ContextRequest struct {
    Context Context `json:"context" validate:"required"`
}
```

#### Context Response
```go
type ContextResponse struct {
    Context   Context   `json:"context"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

#### List Contexts Response
```go
type ContextListResponse struct {
    Contexts []ContextResponse `json:"contexts"`
    Total    int               `json:"total"`
    Page     int               `json:"page,omitempty"`
    PageSize int               `json:"page_size,omitempty"`
}
```

### Action API

#### Action Request
```go
type ActionRequest struct {
    DryRun   bool                   `json:"dry_run,omitempty"`
    Options  map[string]interface{} `json:"options,omitempty"`
    Priority int                    `json:"priority,omitempty"`
}
```

#### Action Response
```go
type ActionResponse struct {
    Success       bool      `json:"success"`
    CorrelationID string    `json:"correlation_id"`
    Message       string    `json:"message"`
    Action        string    `json:"action"`
    Context       string    `json:"context"`
    RequestedAt   time.Time `json:"requested_at"`
    EstimatedTime string    `json:"estimated_completion,omitempty"`
}
```

### Status API

#### Context Status Response
```go
type ContextStatusResponse struct {
    ContextName     string                 `json:"context_name"`
    UpdatedAt       time.Time              `json:"updated_at"`
    StalenessSeconds int                   `json:"staleness_seconds"`
    OverallHealth   string                 `json:"overall_health"`
    Summary         map[string]string      `json:"summary"`
    Details         map[string]interface{} `json:"details"`
    LastRun         *RunSummary            `json:"last_run,omitempty"`
}

type RunSummary struct {
    CorrelationID string    `json:"correlation_id"`
    Action        string    `json:"action"`
    Status        string    `json:"status"`
    RequestedAt   time.Time `json:"requested_at"`
    CompletedAt   time.Time `json:"completed_at"`
    LatencyMs     int       `json:"latency_ms"`
}
```

#### Run History Response
```go
type RunHistoryResponse struct {
    Runs  []RunHistoryItem `json:"runs"`
    Total int              `json:"total"`
    Limit int              `json:"limit"`
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
    ErrorSummary  string    `json:"error_summary,omitempty"`
}
```

---

## Service-Specific Result Schemas

### Vault Service Results

```go
type VaultResult struct {
    Status           string                `json:"status"`
    AuthValidation   AuthValidationResult  `json:"auth_validation"`
    SecretValidations []SecretValidation    `json:"secret_validations"`
    LatencyMs        int                   `json:"latency_ms"`
    VaultVersion     string                `json:"vault_version,omitempty"`
}

type AuthValidationResult struct {
    Method    string    `json:"method"`
    Success   bool      `json:"success"`
    TokenTTL  int       `json:"token_ttl_seconds,omitempty"`
    Policies  []string  `json:"policies,omitempty"`
    Error     string    `json:"error,omitempty"`
}

type SecretValidation struct {
    LogicalName string            `json:"logical_name"`
    Path        string            `json:"path"`
    Status      string            `json:"status"` // ok, missing, access_denied
    Keys        []KeyValidation   `json:"keys"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    Error       string            `json:"error,omitempty"`
}

type KeyValidation struct {
    Key    string `json:"key"`
    Exists bool   `json:"exists"`
    Type   string `json:"type,omitempty"` // string, number, boolean
}
```

### ArgoCD Service Results

```go
type ArgoCDResult struct {
    Status      string              `json:"status"`
    Applications []ApplicationStatus `json:"applications"`
    Server       ServerInfo          `json:"server"`
    LatencyMs   int                 `json:"latency_ms"`
}

type ApplicationStatus struct {
    Name         string            `json:"name"`
    Project      string            `json:"project"`
    Health       string            `json:"health"`
    SyncStatus   string            `json:"sync_status"`
    Revision     string            `json:"revision"`
    Sources      []SourceInfo      `json:"sources"`
    LastSync     *time.Time        `json:"last_sync,omitempty"`
    SyncOperation *SyncOperation    `json:"sync_operation,omitempty"`
}

type SourceInfo struct {
    RepoURL        string `json:"repo_url"`
    Path           string `json:"path"`
    TargetRevision string `json:"target_revision"`
    Helm           *HelmSource `json:"helm,omitempty"`
}

type HelmSource struct {
    ValueFiles   []string `json:"value_files,omitempty"`
    Values       string   `json:"values,omitempty"`
    ReleaseName  string   `json:"release_name,omitempty"`
}

type SyncOperation struct {
    Status    string    `json:"status"`
    StartedAt time.Time `json:"started_at"`
    Phase     string    `json:"phase"`
    Message   string    `json:"message,omitempty"`
}

type ServerInfo struct {
    Version string `json:"version"`
    Address string `json:"address"`
}
```

### New Relic Service Results

```go
type NewRelicResult struct {
    Status    string          `json:"status"`
    Entities  []EntityInfo    `json:"entities"`
    Metrics   []MetricResult  `json:"metrics"`
    Incidents []IncidentInfo  `json:"incidents,omitempty"`
    LatencyMs int             `json:"latency_ms"`
}

type EntityInfo struct {
    GUID         string            `json:"guid"`
    Name         string            `json:"name"`
    Type         string            `json:"type"`
    AlertSeverity string           `json:"alert_severity"`
    Tags         map[string]string `json:"tags"`
    GoldenMetrics *GoldenMetrics   `json:"golden_metrics,omitempty"`
}

type GoldenMetrics struct {
    Throughput    float64 `json:"throughput"`
    ResponseTime  float64 `json:"response_time_ms"`
    ErrorRate     float64 `json:"error_rate_percent"`
    Apdex         float64 `json:"apdex,omitempty"`
}

type MetricResult struct {
    Name      string    `json:"name"`
    Value     float64   `json:"value"`
    Unit      string    `json:"unit"`
    Timestamp time.Time `json:"timestamp"`
}

type IncidentInfo struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    State       string    `json:"state"`
    Priority    string    `json:"priority"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

### Kubernetes Service Results

```go
type KubernetesResult struct {
    Status      string            `json:"status"`
    Namespace   string            `json:"namespace"`
    Workloads   WorkloadSummary   `json:"workloads"`
    Events      []EventInfo       `json:"events"`
    Permissions []PermissionCheck `json:"permissions"`
    LatencyMs   int               `json:"latency_ms"`
}

type WorkloadSummary struct {
    Deployments  []DeploymentStatus  `json:"deployments"`
    StatefulSets []StatefulSetStatus `json:"statefulsets"`
    DaemonSets   []DaemonSetStatus   `json:"daemonsets"`
    Pods         PodSummary          `json:"pod_summary"`
}

type DeploymentStatus struct {
    Name             string    `json:"name"`
    Ready            string    `json:"ready"`        // "2/3"
    UpToDate         int32     `json:"up_to_date"`
    Available        int32     `json:"available"`
    Age              string    `json:"age"`
    Conditions       []string  `json:"conditions"`
}

type PodSummary struct {
    Total     int      `json:"total"`
    Running   int      `json:"running"`
    Pending   int      `json:"pending"`
    Failed    int      `json:"failed"`
    TopErrors []string `json:"top_errors"`
}

type EventInfo struct {
    Type      string    `json:"type"`
    Reason    string    `json:"reason"`
    Message   string    `json:"message"`
    Object    string    `json:"object"`
    Timestamp time.Time `json:"timestamp"`
    Count     int32     `json:"count"`
}

type PermissionCheck struct {
    Resource string `json:"resource"`
    Verb     string `json:"verb"`
    Allowed  bool   `json:"allowed"`
    Reason   string `json:"reason,omitempty"`
}
```

### Git Service Results

```go
type GitResult struct {
    Status      string         `json:"status"`
    Provider    string         `json:"provider"`
    Sources     []GitSource    `json:"sources"`
    Files       []FileInfo     `json:"files,omitempty"`
    LatencyMs   int            `json:"latency_ms"`
}

type GitSource struct {
    Repository string `json:"repository"`
    Revision   string `json:"revision"`
    Path       string `json:"path"`
    Type       string `json:"type"` // helm, kustomize, plain
}

type FileInfo struct {
    Path         string    `json:"path"`
    Size         int       `json:"size"`
    SHA          string    `json:"sha"`
    LastModified time.Time `json:"last_modified"`
    Type         string    `json:"type"` // file, directory
    ContentType  string    `json:"content_type,omitempty"`
}
```

---

## Error Response Schemas

### Standard Error Response

```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code        string            `json:"code"`
    Message     string            `json:"message"`
    Details     string            `json:"details,omitempty"`
    Timestamp   time.Time         `json:"timestamp"`
    RequestID   string            `json:"request_id"`
    Context     map[string]string `json:"context,omitempty"`
    Suggestions []string          `json:"suggestions,omitempty"`
}
```

### Validation Error Response

```go
type ValidationErrorResponse struct {
    Error ValidationErrorDetail `json:"error"`
}

type ValidationErrorDetail struct {
    Code      string             `json:"code"`
    Message   string             `json:"message"`
    Timestamp time.Time          `json:"timestamp"`
    RequestID string             `json:"request_id"`
    Violations []FieldViolation  `json:"violations"`
}

type FieldViolation struct {
    Field       string `json:"field"`
    Value       string `json:"value,omitempty"`
    Constraint  string `json:"constraint"`
    Message     string `json:"message"`
}
```

---

## OpenAPI Schema Definitions

### Context Schema (Subset)

```yaml
Context:
  type: object
  required: [apiVersion, kind, metadata, spec]
  properties:
    apiVersion:
      type: string
      enum: [contextops/v1]
    kind:
      type: string
      enum: [Context]
    metadata:
      $ref: '#/components/schemas/ContextMetadata'
    spec:
      $ref: '#/components/schemas/ContextSpec'

ContextMetadata:
  type: object
  required: [name]
  properties:
    name:
      type: string
      pattern: '^[a-z0-9-]+$'
      minLength: 1
      maxLength: 63
    labels:
      type: object
      additionalProperties:
        type: string
    annotations:
      type: object
      additionalProperties:
        type: string
```

---

## Message Validation

### JSON Schema Validation

```go
// Validate message envelope structure
func ValidateMessageEnvelope(data []byte) error {
    var envelope MessageEnvelope
    if err := json.Unmarshal(data, &envelope); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    
    // Schema version check
    if envelope.SchemaVersion != 1 {
        return fmt.Errorf("unsupported schema version: %d", envelope.SchemaVersion)
    }
    
    // Required fields
    if envelope.MessageID == "" || envelope.CorrelationID == "" {
        return fmt.Errorf("missing required fields")
    }
    
    // UUID validation
    if _, err := uuid.Parse(envelope.MessageID); err != nil {
        return fmt.Errorf("invalid message_id format: %w", err)
    }
    
    if _, err := uuid.Parse(envelope.CorrelationID); err != nil {
        return fmt.Errorf("invalid correlation_id format: %w", err)
    }
    
    return nil
}
```

---

## Content Type Handling

### Request/Response Content Types

```go
const (
    ContentTypeJSON = "application/json"
    ContentTypeYAML = "application/yaml"
    ContentTypeText = "text/plain"
)

// Content negotiation based on Accept header
func DetermineResponseFormat(acceptHeader string) string {
    if strings.Contains(acceptHeader, "application/yaml") {
        return ContentTypeYAML
    }
    return ContentTypeJSON // Default
}
```

---

## Related Documentation

- [Context Data Model](./context-model.md) - Context structure definition
- [Database Schema](./database-schema.md) - Database representation
- [Integration Service Results](./integration-results.md) - Service-specific schemas
- [ADR-001: Event-driven integration workflows](../adr/ADR-001-event-driven-integration-workflows.md)