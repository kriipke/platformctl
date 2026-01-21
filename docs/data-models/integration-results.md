# Integration Service Result Models

**Version:** 1.0  
**Date:** 2026-01-21  
**Phases:** 1C (Integration Services), 1D (Read Model)  

---

## Overview

This document defines the structured result models returned by each integration service in ContextOps. These models ensure consistent data structures for aggregation, caching, and presentation in the read model.

---

## Common Result Patterns

### Base Result Interface
```go
type ServiceResult interface {
    GetStatus() string
    GetLatencyMs() int
    GetError() error
    GetServiceName() string
}

type BaseResult struct {
    Status      string `json:"status"`        // ok, degraded, error
    LatencyMs   int    `json:"latency_ms"`
    ServiceName string `json:"service_name"`
    Error       string `json:"error,omitempty"`
    Timestamp   time.Time `json:"timestamp"`
}
```

### Status Values
- `ok`: Service responding normally, all checks passed
- `degraded`: Service responding but with warnings or partial failures
- `error`: Service unreachable or critical failures

---

## Vault Service Results

### Primary Result Structure
```go
type VaultResult struct {
    BaseResult
    AuthValidation    AuthValidationResult `json:"auth_validation"`
    SecretValidations []SecretValidation   `json:"secret_validations"`
    VaultVersion      string               `json:"vault_version,omitempty"`
    SealStatus        *SealStatus          `json:"seal_status,omitempty"`
}

type AuthValidationResult struct {
    Method      string   `json:"method"`       // token, approle, kubernetes
    Success     bool     `json:"success"`
    TokenTTL    int      `json:"token_ttl_seconds,omitempty"`
    Policies    []string `json:"policies,omitempty"`
    Renewable   bool     `json:"renewable,omitempty"`
    Error       string   `json:"error,omitempty"`
    ValidatedAt time.Time `json:"validated_at"`
}

type SecretValidation struct {
    LogicalName   string              `json:"logical_name"`   // From context spec
    Path          string              `json:"path"`           // Actual Vault path
    Status        string              `json:"status"`         // ok, missing, access_denied
    Keys          []KeyValidation     `json:"keys"`
    Metadata      map[string]string   `json:"metadata,omitempty"`
    Version       int                 `json:"version,omitempty"`
    CreatedTime   *time.Time          `json:"created_time,omitempty"`
    Error         string              `json:"error,omitempty"`
}

type KeyValidation struct {
    Key     string `json:"key"`
    Exists  bool   `json:"exists"`
    Type    string `json:"type,omitempty"`    // string, number, boolean
    Length  int    `json:"length,omitempty"`  // For strings
}

type SealStatus struct {
    Sealed      bool   `json:"sealed"`
    Threshold   int    `json:"threshold"`
    Shares      int    `json:"shares"`
    Progress    int    `json:"progress"`
    Version     string `json:"version"`
    ClusterName string `json:"cluster_name,omitempty"`
}
```

### Health Determination Logic
```go
func (v *VaultResult) DetermineHealth() string {
    if v.Error != "" {
        return "error"
    }
    
    if v.SealStatus != nil && v.SealStatus.Sealed {
        return "error"
    }
    
    if !v.AuthValidation.Success {
        return "error"
    }
    
    errorCount := 0
    missingCount := 0
    
    for _, secret := range v.SecretValidations {
        switch secret.Status {
        case "error", "access_denied":
            errorCount++
        case "missing":
            missingCount++
        }
    }
    
    if errorCount > 0 {
        return "error"
    }
    
    if missingCount > 0 {
        return "degraded"
    }
    
    return "ok"
}
```

---

## Kubernetes Service Results

### Primary Result Structure
```go
type KubernetesResult struct {
    BaseResult
    Namespace     string              `json:"namespace"`
    Cluster       ClusterInfo         `json:"cluster"`
    Workloads     WorkloadSummary     `json:"workloads"`
    Events        []EventInfo         `json:"events"`
    Permissions   []PermissionCheck   `json:"permissions"`
    Resources     ResourceUsage       `json:"resources,omitempty"`
}

type ClusterInfo struct {
    Name         string `json:"name"`
    Version      string `json:"version"`
    Provider     string `json:"provider,omitempty"` // eks, gke, aks, generic
    Region       string `json:"region,omitempty"`
    NodeCount    int    `json:"node_count"`
    Accessible   bool   `json:"accessible"`
}

type WorkloadSummary struct {
    Deployments  []DeploymentStatus  `json:"deployments"`
    StatefulSets []StatefulSetStatus `json:"statefulsets"`
    DaemonSets   []DaemonSetStatus   `json:"daemonsets"`
    Pods         PodSummary          `json:"pod_summary"`
    Services     []ServiceStatus     `json:"services"`
    Ingresses    []IngressStatus     `json:"ingresses,omitempty"`
}

type DeploymentStatus struct {
    Name             string                `json:"name"`
    Ready            string                `json:"ready"`        // "2/3"
    UpToDate         int32                 `json:"up_to_date"`
    Available        int32                 `json:"available"`
    Age              string                `json:"age"`
    Conditions       []DeploymentCondition `json:"conditions"`
    ReplicaSets      []ReplicaSetInfo      `json:"replica_sets,omitempty"`
    Strategy         string                `json:"strategy"`     // RollingUpdate, Recreate
}

type DeploymentCondition struct {
    Type               string    `json:"type"`
    Status             string    `json:"status"`
    Reason             string    `json:"reason,omitempty"`
    Message            string    `json:"message,omitempty"`
    LastTransitionTime time.Time `json:"last_transition_time"`
}

type StatefulSetStatus struct {
    Name         string   `json:"name"`
    Ready        string   `json:"ready"`
    Age          string   `json:"age"`
    Replicas     int32    `json:"replicas"`
    ReadyReplicas int32   `json:"ready_replicas"`
    Conditions   []string `json:"conditions"`
}

type DaemonSetStatus struct {
    Name            string   `json:"name"`
    Desired         int32    `json:"desired"`
    Current         int32    `json:"current"`
    Ready           int32    `json:"ready"`
    UpToDate        int32    `json:"up_to_date"`
    Available       int32    `json:"available"`
    Age             string   `json:"age"`
    Conditions      []string `json:"conditions"`
}

type PodSummary struct {
    Total       int      `json:"total"`
    Running     int      `json:"running"`
    Pending     int      `json:"pending"`
    Failed      int      `json:"failed"`
    Succeeded   int      `json:"succeeded"`
    Unknown     int      `json:"unknown"`
    TopErrors   []string `json:"top_errors"`
    RestartCount int     `json:"restart_count"`
}

type ServiceStatus struct {
    Name         string            `json:"name"`
    Type         string            `json:"type"`        // ClusterIP, NodePort, LoadBalancer
    ClusterIP    string            `json:"cluster_ip"`
    ExternalIPs  []string          `json:"external_ips,omitempty"`
    Ports        []ServicePort     `json:"ports"`
    Selector     map[string]string `json:"selector"`
    Age          string            `json:"age"`
}

type ServicePort struct {
    Name       string `json:"name,omitempty"`
    Protocol   string `json:"protocol"`
    Port       int32  `json:"port"`
    TargetPort string `json:"target_port"`
    NodePort   int32  `json:"node_port,omitempty"`
}

type IngressStatus struct {
    Name      string        `json:"name"`
    Hosts     []string      `json:"hosts"`
    Addresses []string      `json:"addresses"`
    Rules     []IngressRule `json:"rules"`
    TLS       []IngressTLS  `json:"tls,omitempty"`
    Age       string        `json:"age"`
}

type EventInfo struct {
    Type         string    `json:"type"`         // Normal, Warning
    Reason       string    `json:"reason"`
    Message      string    `json:"message"`
    Object       string    `json:"object"`
    Namespace    string    `json:"namespace"`
    Timestamp    time.Time `json:"timestamp"`
    Count        int32     `json:"count"`
    FirstTime    time.Time `json:"first_time"`
    LastTime     time.Time `json:"last_time"`
}

type PermissionCheck struct {
    Resource    string `json:"resource"`
    Verb        string `json:"verb"`
    Allowed     bool   `json:"allowed"`
    Reason      string `json:"reason,omitempty"`
    Group       string `json:"group,omitempty"`
    Version     string `json:"version,omitempty"`
}

type ResourceUsage struct {
    CPURequests    string `json:"cpu_requests"`
    CPULimits      string `json:"cpu_limits"`
    MemoryRequests string `json:"memory_requests"`
    MemoryLimits   string `json:"memory_limits"`
    PVCStorage     string `json:"pvc_storage"`
}
```

### Health Determination Logic
```go
func (k *KubernetesResult) DetermineHealth() string {
    if k.Error != "" || !k.Cluster.Accessible {
        return "error"
    }
    
    // Check for failed pods
    if k.Workloads.Pods.Failed > 0 {
        return "degraded"
    }
    
    // Check deployment readiness
    for _, dep := range k.Workloads.Deployments {
        if strings.Contains(dep.Ready, "0/") {
            return "error"
        }
        if !strings.HasSuffix(dep.Ready, dep.Ready[strings.Index(dep.Ready, "/")+1:]) {
            return "degraded"
        }
    }
    
    // Check for warning events in last 5 minutes
    recentWarnings := 0
    fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
    for _, event := range k.Events {
        if event.Type == "Warning" && event.Timestamp.After(fiveMinutesAgo) {
            recentWarnings++
        }
    }
    
    if recentWarnings > 10 {
        return "degraded"
    }
    
    return "ok"
}
```

---

## ArgoCD Service Results

### Primary Result Structure
```go
type ArgoCDResult struct {
    BaseResult
    Server       ServerInfo          `json:"server"`
    Applications []ApplicationStatus `json:"applications"`
    Projects     []ProjectInfo       `json:"projects,omitempty"`
    Repositories []RepositoryInfo    `json:"repositories,omitempty"`
}

type ServerInfo struct {
    Version     string            `json:"version"`
    Address     string            `json:"address"`
    Accessible  bool              `json:"accessible"`
    Settings    map[string]string `json:"settings,omitempty"`
}

type ApplicationStatus struct {
    Name          string            `json:"name"`
    Project       string            `json:"project"`
    Health        string            `json:"health"`        // Healthy, Progressing, Degraded, Suspended, Missing, Unknown
    SyncStatus    string            `json:"sync_status"`   // Synced, OutOfSync, Unknown
    Revision      string            `json:"revision"`
    Sources       []SourceInfo      `json:"sources"`
    Destination   DestinationInfo   `json:"destination"`
    LastSync      *time.Time        `json:"last_sync,omitempty"`
    SyncOperation *SyncOperation    `json:"sync_operation,omitempty"`
    Resources     []ResourceStatus  `json:"resources,omitempty"`
    Conditions    []AppCondition    `json:"conditions,omitempty"`
}

type SourceInfo struct {
    RepoURL        string      `json:"repo_url"`
    Path           string      `json:"path"`
    TargetRevision string      `json:"target_revision"`
    Chart          string      `json:"chart,omitempty"`
    Helm           *HelmSource `json:"helm,omitempty"`
    Kustomize      *Kustomize  `json:"kustomize,omitempty"`
}

type HelmSource struct {
    ValueFiles   []string          `json:"value_files,omitempty"`
    Values       string            `json:"values,omitempty"`
    ReleaseName  string            `json:"release_name,omitempty"`
    Parameters   []HelmParameter   `json:"parameters,omitempty"`
}

type HelmParameter struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type Kustomize struct {
    NamePrefix string            `json:"name_prefix,omitempty"`
    NameSuffix string            `json:"name_suffix,omitempty"`
    Images     []string          `json:"images,omitempty"`
    CommonLabels map[string]string `json:"common_labels,omitempty"`
}

type DestinationInfo struct {
    Server    string `json:"server"`
    Namespace string `json:"namespace"`
    Name      string `json:"name,omitempty"`
}

type SyncOperation struct {
    Status     string               `json:"status"`    // Running, Failed, Succeeded
    StartedAt  time.Time            `json:"started_at"`
    FinishedAt *time.Time           `json:"finished_at,omitempty"`
    Phase      string               `json:"phase"`
    Message    string               `json:"message,omitempty"`
    Resources  []SyncResourceResult `json:"resources,omitempty"`
    Retry      *RetryStrategy       `json:"retry,omitempty"`
}

type SyncResourceResult struct {
    Group     string `json:"group"`
    Kind      string `json:"kind"`
    Namespace string `json:"namespace"`
    Name      string `json:"name"`
    Status    string `json:"status"`    // Synced, OutOfSync, Failed
    Message   string `json:"message,omitempty"`
}

type ResourceStatus struct {
    Group     string `json:"group"`
    Version   string `json:"version"`
    Kind      string `json:"kind"`
    Namespace string `json:"namespace"`
    Name      string `json:"name"`
    Status    string `json:"status"`
    Health    string `json:"health"`
}

type AppCondition struct {
    Type               string    `json:"type"`
    Message            string    `json:"message"`
    LastTransitionTime time.Time `json:"last_transition_time"`
}

type ProjectInfo struct {
    Name         string              `json:"name"`
    Description  string              `json:"description,omitempty"`
    Destinations []DestinationInfo   `json:"destinations"`
    Sources      []string            `json:"sources"`
    Roles        []ProjectRole       `json:"roles,omitempty"`
}

type ProjectRole struct {
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    Policies    []string `json:"policies"`
    Groups      []string `json:"groups,omitempty"`
}

type RepositoryInfo struct {
    Repo            string `json:"repo"`
    Username        string `json:"username,omitempty"`
    ConnectionState string `json:"connection_state"` // Successful, Failed
    Error           string `json:"error,omitempty"`
}
```

### Health Determination Logic
```go
func (a *ArgoCDResult) DetermineHealth() string {
    if a.Error != "" || !a.Server.Accessible {
        return "error"
    }
    
    degradedCount := 0
    errorCount := 0
    
    for _, app := range a.Applications {
        switch app.Health {
        case "Missing", "Unknown":
            errorCount++
        case "Degraded", "Progressing":
            degradedCount++
        }
        
        if app.SyncStatus == "Unknown" {
            errorCount++
        }
    }
    
    if errorCount > 0 {
        return "error"
    }
    
    if degradedCount > 0 {
        return "degraded"
    }
    
    return "ok"
}
```

---

## New Relic Service Results

### Primary Result Structure
```go
type NewRelicResult struct {
    BaseResult
    Account   AccountInfo     `json:"account"`
    Entities  []EntityInfo    `json:"entities"`
    Metrics   []MetricResult  `json:"metrics"`
    Incidents []IncidentInfo  `json:"incidents,omitempty"`
    Alerts    []AlertInfo     `json:"alerts,omitempty"`
}

type AccountInfo struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type EntityInfo struct {
    GUID           string            `json:"guid"`
    Name           string            `json:"name"`
    Type           string            `json:"type"`           // APPLICATION, SERVICE, HOST, etc.
    Domain         string            `json:"domain"`         // APM, BROWSER, MOBILE, etc.
    AlertSeverity  string            `json:"alert_severity"` // NOT_ALERTING, WARNING, CRITICAL
    Tags           map[string]string `json:"tags"`
    GoldenMetrics  *GoldenMetrics    `json:"golden_metrics,omitempty"`
    RecentAlerts   []string          `json:"recent_alerts,omitempty"`
    LastReported   *time.Time        `json:"last_reported,omitempty"`
}

type GoldenMetrics struct {
    Throughput      float64 `json:"throughput"`        // requests per minute
    ResponseTime    float64 `json:"response_time_ms"`  // average response time
    ErrorRate       float64 `json:"error_rate_percent"` // error percentage
    Apdex           float64 `json:"apdex,omitempty"`   // application performance index
    CPUUtilization  float64 `json:"cpu_utilization_percent,omitempty"`
    MemoryUsage     float64 `json:"memory_usage_percent,omitempty"`
}

type MetricResult struct {
    Name         string                 `json:"name"`
    DisplayName  string                 `json:"display_name,omitempty"`
    Value        float64                `json:"value"`
    Unit         string                 `json:"unit"`
    Timestamp    time.Time              `json:"timestamp"`
    Attributes   map[string]interface{} `json:"attributes,omitempty"`
    EntityGUID   string                 `json:"entity_guid,omitempty"`
}

type IncidentInfo struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    State       string    `json:"state"`       // CREATED, ACKNOWLEDGED, CLOSED
    Priority    string    `json:"priority"`    // LOW, MEDIUM, HIGH, CRITICAL
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    ClosedAt    *time.Time `json:"closed_at,omitempty"`
    PolicyName  string    `json:"policy_name"`
    ConditionName string  `json:"condition_name,omitempty"`
    Entities    []string  `json:"entities"`
    ViolationURL string   `json:"violation_url,omitempty"`
}

type AlertInfo struct {
    PolicyID      int    `json:"policy_id"`
    PolicyName    string `json:"policy_name"`
    ConditionID   int    `json:"condition_id"`
    ConditionName string `json:"condition_name"`
    ConditionType string `json:"condition_type"`
    Threshold     AlertThreshold `json:"threshold"`
    Enabled       bool   `json:"enabled"`
    Entities      []string `json:"entities"`
}

type AlertThreshold struct {
    Duration      int     `json:"duration"`
    Operator      string  `json:"operator"`      // above, below, equal
    Priority      string  `json:"priority"`      // critical, warning
    Threshold     float64 `json:"threshold"`
    ThresholdOccurrences string `json:"threshold_occurrences"` // all, at_least_once
}
```

### Health Determination Logic
```go
func (n *NewRelicResult) DetermineHealth() string {
    if n.Error != "" {
        return "error"
    }
    
    // Check for critical incidents
    for _, incident := range n.Incidents {
        if incident.State != "CLOSED" && incident.Priority == "CRITICAL" {
            return "error"
        }
    }
    
    // Check entity alert severity
    criticalCount := 0
    warningCount := 0
    
    for _, entity := range n.Entities {
        switch entity.AlertSeverity {
        case "CRITICAL":
            criticalCount++
        case "WARNING":
            warningCount++
        }
        
        // Check if entity has stopped reporting
        if entity.LastReported != nil {
            if time.Since(*entity.LastReported) > 15*time.Minute {
                warningCount++
            }
        }
        
        // Check golden metrics if available
        if gm := entity.GoldenMetrics; gm != nil {
            if gm.ErrorRate > 5.0 { // 5% error rate threshold
                warningCount++
            }
            if gm.Apdex > 0 && gm.Apdex < 0.7 { // Poor Apdex score
                warningCount++
            }
        }
    }
    
    if criticalCount > 0 {
        return "error"
    }
    
    if warningCount > 0 {
        return "degraded"
    }
    
    return "ok"
}
```

---

## Git Service Results

### Primary Result Structure
```go
type GitResult struct {
    BaseResult
    Provider     string         `json:"provider"`     // github, gitlab, bitbucket
    Sources      []GitSource    `json:"sources"`
    Files        []FileInfo     `json:"files,omitempty"`
    Commits      []CommitInfo   `json:"commits,omitempty"`
    Branches     []BranchInfo   `json:"branches,omitempty"`
    Tags         []TagInfo      `json:"tags,omitempty"`
    RateLimit    *RateLimitInfo `json:"rate_limit,omitempty"`
}

type GitSource struct {
    Repository     string    `json:"repository"`      // owner/repo format
    Revision       string    `json:"revision"`        // commit SHA, branch, or tag
    Path           string    `json:"path"`            // subdirectory path
    Type           string    `json:"type"`            // helm, kustomize, plain
    LastFetch      time.Time `json:"last_fetch"`
    CommitSHA      string    `json:"commit_sha"`      // resolved SHA from revision
    RefType        string    `json:"ref_type"`        // branch, tag, commit
    Accessible     bool      `json:"accessible"`
    DefaultBranch  string    `json:"default_branch,omitempty"`
}

type FileInfo struct {
    Path         string    `json:"path"`
    Name         string    `json:"name"`
    Size         int       `json:"size"`
    Type         string    `json:"type"`         // file, dir, symlink
    SHA          string    `json:"sha"`
    LastModified time.Time `json:"last_modified"`
    ContentType  string    `json:"content_type,omitempty"`
    Encoding     string    `json:"encoding,omitempty"`
    Content      string    `json:"content,omitempty"` // For small files only
}

type CommitInfo struct {
    SHA         string      `json:"sha"`
    Message     string      `json:"message"`
    Author      AuthorInfo  `json:"author"`
    Committer   AuthorInfo  `json:"committer"`
    Timestamp   time.Time   `json:"timestamp"`
    ParentSHAs  []string    `json:"parent_shas"`
    TreeSHA     string      `json:"tree_sha"`
    URL         string      `json:"url,omitempty"`
    Verified    bool        `json:"verified"`
    ChangedFiles int        `json:"changed_files,omitempty"`
    Additions   int         `json:"additions,omitempty"`
    Deletions   int         `json:"deletions,omitempty"`
}

type AuthorInfo struct {
    Name  string    `json:"name"`
    Email string    `json:"email"`
    Date  time.Time `json:"date"`
}

type BranchInfo struct {
    Name      string     `json:"name"`
    SHA       string     `json:"sha"`
    Protected bool       `json:"protected"`
    Default   bool       `json:"default"`
    Commit    CommitInfo `json:"commit"`
}

type TagInfo struct {
    Name   string     `json:"name"`
    SHA    string     `json:"sha"`
    Commit CommitInfo `json:"commit"`
    Tagger *AuthorInfo `json:"tagger,omitempty"`
}

type RateLimitInfo struct {
    Limit     int       `json:"limit"`
    Remaining int       `json:"remaining"`
    Reset     time.Time `json:"reset"`
    Resource  string    `json:"resource"` // core, search, graphql
}
```

### Health Determination Logic
```go
func (g *GitResult) DetermineHealth() string {
    if g.Error != "" {
        return "error"
    }
    
    // Check if any sources are inaccessible
    inaccessibleCount := 0
    for _, source := range g.Sources {
        if !source.Accessible {
            inaccessibleCount++
        }
    }
    
    if inaccessibleCount == len(g.Sources) {
        return "error"
    }
    
    if inaccessibleCount > 0 {
        return "degraded"
    }
    
    // Check rate limit status
    if rl := g.RateLimit; rl != nil {
        remaining := float64(rl.Remaining) / float64(rl.Limit)
        if remaining < 0.1 { // Less than 10% remaining
            return "degraded"
        }
    }
    
    return "ok"
}
```

---

## Result Aggregation Patterns

### Cross-Service Health Calculation
```go
type AggregatedHealth struct {
    OverallStatus string                    `json:"overall_status"`
    ServiceStatus map[string]string         `json:"service_status"`
    ErrorSummary  []string                  `json:"error_summary,omitempty"`
    LastUpdated   time.Time                 `json:"last_updated"`
    Staleness     map[string]time.Duration  `json:"staleness"`
}

func AggregateServiceResults(results map[string]ServiceResult) AggregatedHealth {
    agg := AggregatedHealth{
        ServiceStatus: make(map[string]string),
        LastUpdated:   time.Now(),
        Staleness:     make(map[string]time.Duration),
    }
    
    errorCount := 0
    degradedCount := 0
    
    for serviceName, result := range results {
        status := result.GetStatus()
        agg.ServiceStatus[serviceName] = status
        
        // Calculate staleness
        if timestamp := result.GetTimestamp(); !timestamp.IsZero() {
            agg.Staleness[serviceName] = time.Since(timestamp)
        }
        
        // Aggregate errors
        if err := result.GetError(); err != nil {
            agg.ErrorSummary = append(agg.ErrorSummary, 
                fmt.Sprintf("%s: %s", serviceName, err.Error()))
        }
        
        // Count status types
        switch status {
        case "error":
            errorCount++
        case "degraded":
            degradedCount++
        }
    }
    
    // Determine overall status
    if errorCount > 0 {
        agg.OverallStatus = "error"
    } else if degradedCount > 0 {
        agg.OverallStatus = "degraded"
    } else {
        agg.OverallStatus = "ok"
    }
    
    return agg
}
```

---

## Caching and TTL Strategies

### Service-Specific TTL Values
```go
var ServiceTTLConfig = map[string]time.Duration{
    "vault":      2 * time.Minute,  // Secrets change infrequently
    "kubernetes": 30 * time.Second, // Workloads change frequently
    "argocd":     1 * time.Minute,  // Application sync status
    "newrelic":   1 * time.Minute,  // Metrics and alerts
    "git":        5 * time.Minute,  // Repository content
}

type CachedResult struct {
    Result    ServiceResult `json:"result"`
    CachedAt  time.Time     `json:"cached_at"`
    TTL       time.Duration `json:"ttl"`
    Fresh     bool          `json:"fresh"`
}

func (c *CachedResult) IsStale() bool {
    return time.Since(c.CachedAt) > c.TTL
}

func (c *CachedResult) StalenessSeconds() int {
    elapsed := time.Since(c.CachedAt)
    if elapsed <= c.TTL {
        return 0
    }
    return int((elapsed - c.TTL).Seconds())
}
```

---

## Related Documentation

- [Context Data Model](./context-model.md) - Context structure definition
- [Database Schema](./database-schema.md) - Storage layer for results  
- [API Message Schemas](./api-schemas.md) - Message envelope structures
- [ADR-001: Event-driven integration workflows](../adr/ADR-001-event-driven-integration-workflows.md)
- [ADR-002: Read model (CQRS-lite)](../adr/ADR-002-read-model-cqrs-lite.md)
- [ADR-007: Caching layers and TTL policies](../adr/ADR-007-caching-layers-and-ttl-policies.md)