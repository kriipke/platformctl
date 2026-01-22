# PHASE 1B: GitOps Messaging Infrastructure and Event-Driven Workflows

**Duration:** 3-4 days  
**Prerequisites:** Phase 1A completed, RabbitMQ instance available, ArgoCD accessible, Vault accessible  
**Deliverable:** GitOps-aware event-driven command/result system with ApplicationSet monitoring and Vault secret validation

---

## Overview

Extend the GitOps API Gateway with RabbitMQ integration for App+Environment+Context manifest workflows. Add GitOps action endpoints that publish App manifest synchronization commands, Environment manifest validation events, and Context pairing monitoring commands. This phase establishes the async communication backbone for the three-manifest system including ApplicationSet monitoring from App manifests, Vault validation from Environment manifests, and Context pairing correlation.

## Success Criteria

✅ App+Environment+Context aware RabbitMQ topology established with customer isolation  
✅ App manifest command publishing system working (ApplicationSet synchronization)  
✅ Environment manifest validation event publishing system working (Vault secrets, cluster configs)  
✅ Context pairing monitoring command system working (App-Environment correlations)  
✅ GitOps action endpoints implemented (sync-apps, validate-environments, correlate-contexts, inspect-manifests)  
✅ Three-manifest message envelope structure with customer context implemented  
✅ Customer-scoped correlation ID tracking working for manifest operations  
✅ Manifest-specific error handling and DLQ setup  
✅ Integration tests for App+Environment+Context message flow  

---

## Implementation Tasks

### Task 1: GitOps RabbitMQ Integration Setup with Multi-Tenancy

**File: `internal/events/gitops_rabbitmq.go`**

```go
type GitOpsMessageBus struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    config  *config.Config
}

func NewGitOpsMessageBus(url string, cfg *config.Config) (*GitOpsMessageBus, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
    }
    
    channel, err := conn.Channel()
    if err != nil {
        return nil, fmt.Errorf("failed to open channel: %w", err)
    }
    
    gmb := &GitOpsMessageBus{
        conn:    conn,
        channel: channel,
        config:  cfg,
    }
    
    if err := gmb.setupGitOpsTopology(); err != nil {
        return nil, fmt.Errorf("failed to setup GitOps topology: %w", err)
    }
    
    return gmb, nil
}

func (gmb *GitOpsMessageBus) setupGitOpsTopology() error {
    // GitOps Commands Exchange
    if err := gmb.channel.ExchangeDeclare(
        "gitops.commands", // name
        "topic",          // type
        true,             // durable
        false,            // auto-deleted
        false,            // internal
        false,            // no-wait
        amqp.Table{
            "description": "GitOps commands for ApplicationSet monitoring, Vault validation, and environment correlation",
        },
    ); err != nil {
        return err
    }
    
    // GitOps Results Exchange
    if err := gmb.channel.ExchangeDeclare(
        "gitops.results", // name
        "topic",         // type
        true,            // durable
        false,           // auto-deleted
        false,           // internal
        false,           // no-wait
        amqp.Table{
            "description": "GitOps results from ApplicationSet monitoring, Vault validation, and environment correlation",
        },
    ); err != nil {
        return err
    }
    
    // ApplicationSet Monitoring Queue
    _, err := gmb.channel.QueueDeclare(
        "gitops.applicationset-monitor.q", // name
        true,                              // durable
        false,                             // delete when unused
        false,                             // exclusive
        false,                             // no-wait
        amqp.Table{
            "x-message-ttl":            300000, // 5 minutes TTL
            "x-max-priority":           10,
            "description":              "ApplicationSet monitoring commands",
        },
    )
    if err != nil {
        return err
    }
    
    // Vault Secrets Validation Queue
    _, err = gmb.channel.QueueDeclare(
        "gitops.vault-validation.q", // name
        true,                        // durable
        false,                       // delete when unused
        false,                       // exclusive
        false,                       // no-wait
        amqp.Table{
            "x-message-ttl":   600000, // 10 minutes TTL
            "x-max-priority":  10,
            "description":     "Vault secret validation commands",
        },
    )
    if err != nil {
        return err
    }
    
    // Environment Correlation Queue
    _, err = gmb.channel.QueueDeclare(
        "gitops.environment-correlation.q", // name
        true,                               // durable
        false,                              // delete when unused
        false,                              // exclusive
        false,                              // no-wait
        amqp.Table{
            "x-message-ttl":   180000, // 3 minutes TTL
            "x-max-priority":  5,
            "description":     "Multi-environment status correlation commands",
        },
    )
    if err != nil {
        return err
    }
    
    // Aggregator Queue (customer-aware)
    _, err = gmb.channel.QueueDeclare(
        "gitops.aggregator.q", // name
        true,                  // durable
        false,                 // delete when unused
        false,                 // exclusive
        false,                 // no-wait
        amqp.Table{
            "x-message-ttl":   900000, // 15 minutes TTL
            "description":     "GitOps results aggregation queue",
        },
    )
    if err != nil {
        return err
    }
    
    // Bind queues to exchanges with GitOps-specific routing keys
    bindings := []struct {
        queue      string
        exchange   string
        routingKey string
    }{
        // App manifest synchronization bindings
        {"gitops.applicationset-monitor.q", "gitops.commands", "cmd.app.*"},
        {"gitops.applicationset-monitor.q", "gitops.commands", "cmd.applicationset.*"},
        
        // Environment manifest validation bindings
        {"gitops.vault-validation.q", "gitops.commands", "cmd.environment.*"},
        {"gitops.vault-validation.q", "gitops.commands", "cmd.vault.*"},
        
        // Context pairing correlation bindings
        {"gitops.environment-correlation.q", "gitops.commands", "cmd.context.*"},
        {"gitops.environment-correlation.q", "gitops.commands", "cmd.correlate.*"},
        
        // Aggregator bindings for all manifest results
        {"gitops.aggregator.q", "gitops.results", "evt.app.*"},
        {"gitops.aggregator.q", "gitops.results", "evt.environment.*"},
        {"gitops.aggregator.q", "gitops.results", "evt.context.*"},
        {"gitops.aggregator.q", "gitops.results", "evt.correlation.*"},
    }
    
    for _, binding := range bindings {
        if err := gmb.channel.QueueBind(
            binding.queue,      // queue name
            binding.routingKey, // routing key
            binding.exchange,   // exchange
            false,
            nil,
        ); err != nil {
            return fmt.Errorf("failed to bind queue %s to exchange %s with key %s: %w", 
                binding.queue, binding.exchange, binding.routingKey, err)
        }
    }
    
    // Setup Dead Letter Queues for GitOps failures
    if err := gmb.setupGitOpsDLQ(); err != nil {
        return fmt.Errorf("failed to setup GitOps DLQ: %w", err)
    }
    
    return nil
}

func (gmb *GitOpsMessageBus) setupGitOpsDLQ() error {
    // GitOps Dead Letter Exchange
    if err := gmb.channel.ExchangeDeclare(
        "gitops.dlx", // name
        "topic",     // type
        true,        // durable
        false,       // auto-deleted
        false,       // internal
        false,       // no-wait
        amqp.Table{
            "description": "GitOps Dead Letter Exchange for failed commands",
        },
    ); err != nil {
        return err
    }
    
    // GitOps Dead Letter Queue
    _, err := gmb.channel.QueueDeclare(
        "gitops.dlq", // name
        true,         // durable
        false,        // delete when unused
        false,        // exclusive
        false,        // no-wait
        amqp.Table{
            "description": "GitOps Dead Letter Queue",
        },
    )
    if err != nil {
        return err
    }
    
    // Bind DLQ to DLX
    return gmb.channel.QueueBind(
        "gitops.dlq", // queue name
        "#",          // routing key (catch all)
        "gitops.dlx", // exchange
        false,
        nil,
    )
}

func (gmb *GitOpsMessageBus) Close() error {
    if gmb.channel != nil {
        gmb.channel.Close()
    }
    if gmb.conn != nil {
        return gmb.conn.Close()
    }
    return nil
}
```

### Task 2: GitOps Message Envelope Structure with Customer Context

**File: `pkg/api/gitops_messages.go`**

```go
type GitOpsMessageEnvelope struct {
    SchemaVersion    int                    `json:"schema_version"`
    MessageID        string                 `json:"message_id"`
    CorrelationID    string                 `json:"correlation_id"`
    CustomerID       string                 `json:"customer_id"`
    ContextName      string                 `json:"context_name"`
    Action           string                 `json:"action"`
    RequestedBy      string                 `json:"requested_by"`
    RequestedAt      time.Time              `json:"requested_at"`
    ManifestType     string                 `json:"manifest_type"` // app, environment, context
    AppName          string                 `json:"app_name,omitempty"`
    EnvironmentName  string                 `json:"environment_name,omitempty"`
    Priority         int                    `json:"priority"` // 1-10, higher is more urgent
    Payload          map[string]interface{} `json:"payload"`
    ManifestMetadata ManifestMetadata       `json:"manifest_metadata"`
}

type ManifestMetadata struct {
    // App manifest metadata
    ApplicationSetName string            `json:"applicationset_name,omitempty"`
    HelmSources        []HelmSourceInfo  `json:"helm_sources,omitempty"`
    GitSources         []GitSourceInfo   `json:"git_sources,omitempty"`
    
    // Environment manifest metadata
    VaultSources       []VaultSourceInfo `json:"vault_sources,omitempty"`
    ClusterConfigs     []ClusterInfo     `json:"cluster_configs,omitempty"`
    ValuesFiles        []string          `json:"values_files,omitempty"`
    
    // Context pairing metadata
    AppReference       string            `json:"app_reference,omitempty"`
    EnvironmentReference string          `json:"environment_reference,omitempty"`
    CustomerBranch     string            `json:"customer_branch,omitempty"`
}

type HelmSourceInfo struct {
    Name     string `json:"name"`
    Type     string `json:"type"` // registry, git, oci
    URL      string `json:"url"`
    Version  string `json:"version,omitempty"`
}

type GitSourceInfo struct {
    URL       string `json:"url"`
    Path      string `json:"path"`
    Revision  string `json:"revision"`
}

type VaultSourceInfo struct {
    Path       string `json:"path"`
    SecretName string `json:"secret_name"`
}

type ClusterInfo struct {
    Name      string `json:"name"`
    Server    string `json:"server"`
    Namespace string `json:"namespace"`
}

type GitOpsCommandMessage struct {
    GitOpsMessageEnvelope
    CommandType     string            `json:"command_type"` // sync, validate, inspect, correlate
    TargetService   string            `json:"target_service"` // app-sync-service, environment-validator, context-correlator
    Timeout         time.Duration     `json:"timeout"`
    RetryPolicy     GitOpsRetryPolicy `json:"retry_policy"`
}

type GitOpsResultMessage struct {
    GitOpsMessageEnvelope
    ServiceName            string                     `json:"service_name"`
    Status                 string                     `json:"status"` // healthy, degraded, unhealthy, error
    CompletedAt            time.Time                  `json:"completed_at"`
    ErrorMessage           string                     `json:"error_message,omitempty"`
    ResultPayload          interface{}                `json:"result_payload"`
    AppManifestData        *AppManifestResult         `json:"app_manifest_data,omitempty"`
    EnvironmentManifestData *EnvironmentManifestResult `json:"environment_manifest_data,omitempty"`
    ContextPairingData     *ContextPairingResult      `json:"context_pairing_data,omitempty"`
    PerformanceMetrics     GitOpsPerformanceMetrics   `json:"performance_metrics"`
}

type GitOpsRetryPolicy struct {
    MaxRetries      int           `json:"max_retries"`
    RetryDelay      time.Duration `json:"retry_delay"`
    BackoffStrategy string        `json:"backoff_strategy"` // linear, exponential
}

type AppManifestResult struct {
    AppName             string                    `json:"app_name"`
    ApplicationSetName  string                    `json:"applicationset_name"`
    Namespace           string                    `json:"namespace"`
    SyncStatus          string                    `json:"sync_status"`
    HealthStatus        string                    `json:"health_status"`
    HelmSources         []HelmSourceStatus        `json:"helm_sources"`
    GitSources          []GitSourceStatus         `json:"git_sources"`
    Applications        []ApplicationStatus       `json:"applications"`
    LastSyncTime        *time.Time                `json:"last_sync_time,omitempty"`
    Generator           ApplicationSetGenerator   `json:"generator"`
}

type HelmSourceStatus struct {
    Name         string     `json:"name"`
    Type         string     `json:"type"`
    URL          string     `json:"url"`
    Version      string     `json:"version"`
    Status       string     `json:"status"` // available, unavailable, error
    LastChecked  *time.Time `json:"last_checked,omitempty"`
}

type GitSourceStatus struct {
    URL         string     `json:"url"`
    Path        string     `json:"path"`
    Revision    string     `json:"revision"`
    Status      string     `json:"status"` // available, unavailable, error
    LastCommit  string     `json:"last_commit,omitempty"`
    LastChecked *time.Time `json:"last_checked,omitempty"`
}

type ApplicationStatus struct {
    Name         string     `json:"name"`
    Environment  string     `json:"environment"`
    Cluster      string     `json:"cluster"`
    Namespace    string     `json:"namespace"`
    SyncStatus   string     `json:"sync_status"`
    HealthStatus string     `json:"health_status"`
    LastDeployed *time.Time `json:"last_deployed,omitempty"`
    HelmRevision string     `json:"helm_revision,omitempty"`
}

type ApplicationSetGenerator struct {
    Type       string                 `json:"type"` // git, clusters, list
    Parameters map[string]interface{} `json:"parameters"`
}

type EnvironmentManifestResult struct {
    EnvironmentName     string                   `json:"environment_name"`
    VaultValidations    []VaultValidationResult  `json:"vault_validations"`
    ClusterValidations  []ClusterValidationResult `json:"cluster_validations"`
    ValuesFileStatus    []ValuesFileStatus       `json:"values_file_status"`
    LastValidated       time.Time                `json:"last_validated"`
}

type VaultValidationResult struct {
    VaultPath           string                   `json:"vault_path"`
    SecretName          string                   `json:"secret_name"`
    ValidationStatus    string                   `json:"validation_status"` // valid, invalid, missing, error
    MissingKeys         []string                 `json:"missing_keys,omitempty"`
    ExtraKeys           []string                 `json:"extra_keys,omitempty"`
    PodEnvValidations   []PodEnvValidationResult `json:"pod_env_validations"`
    LastValidated       time.Time                `json:"last_validated"`
}

type ClusterValidationResult struct {
    ClusterName      string `json:"cluster_name"`
    Server           string `json:"server"`
    Namespace        string `json:"namespace"`
    ConnectionStatus string `json:"connection_status"` // connected, disconnected, error
    LastChecked      time.Time `json:"last_checked"`
}

type ValuesFileStatus struct {
    FilePath     string     `json:"file_path"`
    Status       string     `json:"status"` // available, missing, error
    LastModified *time.Time `json:"last_modified,omitempty"`
    Size         int64      `json:"size,omitempty"`
}

type PodEnvValidationResult struct {
    PodName          string `json:"pod_name"`
    Namespace        string `json:"namespace"`
    EnvVarName       string `json:"env_var_name"`
    ExpectedValue    string `json:"expected_value,omitempty"`
    ActualValue      string `json:"actual_value,omitempty"`
    ValidationStatus string `json:"validation_status"` // match, mismatch, missing
    ErrorMessage     string `json:"error_message,omitempty"`
}

type ContextPairingResult struct {
    ContextName         string                 `json:"context_name"`
    AppReference        string                 `json:"app_reference"`
    EnvironmentReference string                `json:"environment_reference"`
    PairingStatus       string                 `json:"pairing_status"` // valid, invalid, missing_app, missing_environment
    SyncStatus          string                 `json:"sync_status"`
    HealthStatus        string                 `json:"health_status"`
    CorrelationData     map[string]interface{} `json:"correlation_data"`
    ResourceCount       int                    `json:"resource_count"`
    LastDeploymentTime  *time.Time             `json:"last_deployment_time,omitempty"`
    ValidationErrors    []string               `json:"validation_errors,omitempty"`
}

type GitOpsPerformanceMetrics struct {
    ProcessingTimeMs int64   `json:"processing_time_ms"`
    ApiCallsCount    int     `json:"api_calls_count"`
    CacheHitRate     float64 `json:"cache_hit_rate,omitempty"`
}

// GitOps Message creation helpers
func NewGitOpsCommandMessage(customerID, contextName, action, manifestType, user string) *GitOpsCommandMessage {
    return &GitOpsCommandMessage{
        GitOpsMessageEnvelope: GitOpsMessageEnvelope{
            SchemaVersion: 1,
            MessageID:     generateUUID(),
            CorrelationID: generateUUID(),
            CustomerID:    customerID,
            ContextName:   contextName,
            Action:        action,
            ManifestType:  manifestType,
            RequestedBy:   user,
            RequestedAt:   time.Now().UTC(),
            Priority:      5, // Default priority
            Payload:       make(map[string]interface{}),
            ManifestMetadata: ManifestMetadata{},
        },
        CommandType:   "sync", // Default command type
        Timeout:       5 * time.Minute, // Default timeout
        RetryPolicy: GitOpsRetryPolicy{
            MaxRetries:      3,
            RetryDelay:      30 * time.Second,
            BackoffStrategy: "exponential",
        },
    }
}

func NewAppManifestCommandMessage(customerID, contextName, appName, user string) *GitOpsCommandMessage {
    cmd := NewGitOpsCommandMessage(customerID, contextName, "sync-app", "app", user)
    cmd.TargetService = "app-sync-service"
    cmd.AppName = appName
    cmd.Priority = 8 // High priority for App manifest synchronization
    return cmd
}

func NewEnvironmentManifestCommandMessage(customerID, contextName, environmentName, user string) *GitOpsCommandMessage {
    cmd := NewGitOpsCommandMessage(customerID, contextName, "validate-environment", "environment", user)
    cmd.TargetService = "environment-validator"
    cmd.EnvironmentName = environmentName
    cmd.Priority = 7 // High priority for Environment manifest validation
    return cmd
}

func NewContextPairingCommandMessage(customerID, contextName, user string) *GitOpsCommandMessage {
    cmd := NewGitOpsCommandMessage(customerID, contextName, "correlate-context", "context", user)
    cmd.TargetService = "context-correlator"
    cmd.Priority = 6 // Medium-high priority for Context pairing correlation
    return cmd
}
```

### Task 3: GitOps Command Publishing Service

**File: `internal/events/gitops_publisher.go`**

```go
type GitOpsCommandPublisher struct {
    messageBus *GitOpsMessageBus
    logger     *log.Logger
}

func NewGitOpsCommandPublisher(gmb *GitOpsMessageBus) *GitOpsCommandPublisher {
    return &GitOpsCommandPublisher{
        messageBus: gmb,
        logger:     log.New(os.Stdout, "[GitOpsPublisher] ", log.LstdFlags),
    }
}

func (p *GitOpsCommandPublisher) PublishGitOpsCommand(cmd *api.GitOpsCommandMessage) error {
    body, err := json.Marshal(cmd)
    if err != nil {
        return fmt.Errorf("failed to marshal GitOps command: %w", err)
    }
    
    routingKey := p.generateRoutingKey(cmd)
    
    // Set message priority and properties based on GitOps type
    priority := uint8(cmd.Priority)
    if priority > 10 {
        priority = 10
    }
    
    err = p.messageBus.channel.Publish(
        "gitops.commands", // exchange
        routingKey,        // routing key
        false,             // mandatory
        false,             // immediate
        amqp.Publishing{
            ContentType:   "application/json",
            DeliveryMode:  amqp.Persistent,
            MessageId:     cmd.MessageID,
            CorrelationId: cmd.CorrelationID,
            Timestamp:     cmd.RequestedAt,
            Priority:      priority,
            Body:          body,
            Headers: amqp.Table{
                "customer_id":           cmd.CustomerID,
                "context_name":          cmd.ContextName,
                "action":               cmd.Action,
                "manifest_type":        cmd.ManifestType,
                "app_name":             cmd.AppName,
                "environment_name":     cmd.EnvironmentName,
                "requested_by":         cmd.RequestedBy,
                "target_service":       cmd.TargetService,
                "app_reference":        cmd.ManifestMetadata.AppReference,
                "environment_reference": cmd.ManifestMetadata.EnvironmentReference,
                "customer_branch":      cmd.ManifestMetadata.CustomerBranch,
            },
        },
    )
    
    if err != nil {
        return fmt.Errorf("failed to publish GitOps command: %w", err)
    }
    
    p.logger.Printf("Published GitOps command: %s (manifest: %s, customer: %s, correlation: %s)", 
        cmd.Action, cmd.ManifestType, cmd.CustomerID, cmd.CorrelationID)
    
    return nil
}

func (p *GitOpsCommandPublisher) generateRoutingKey(cmd *api.GitOpsCommandMessage) string {
    switch cmd.ManifestType {
    case "app":
        return fmt.Sprintf("cmd.app.%s", cmd.Action)
    case "environment":
        return fmt.Sprintf("cmd.environment.%s", cmd.Action)
    case "context":
        return fmt.Sprintf("cmd.context.%s", cmd.Action)
    default:
        return fmt.Sprintf("cmd.manifest.%s", cmd.Action)
    }
}

// Manifest-specific command publishing methods
func (p *GitOpsCommandPublisher) PublishAppSync(customerID, contextName, appName, user string) (*api.GitOpsCommandMessage, error) {
    cmd := api.NewAppManifestCommandMessage(customerID, contextName, appName, user)
    cmd.Action = "sync-app"
    cmd.CommandType = "sync"
    
    // Add App manifest specific payload
    cmd.Payload["sync_applicationset"] = true
    cmd.Payload["validate_helm_sources"] = true
    cmd.Payload["check_git_sources"] = true
    
    return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishEnvironmentValidation(customerID, contextName, environmentName, user string) (*api.GitOpsCommandMessage, error) {
    cmd := api.NewEnvironmentManifestCommandMessage(customerID, contextName, environmentName, user)
    cmd.Action = "validate-environment"
    cmd.CommandType = "validate"
    
    // Add Environment manifest specific payload
    cmd.Payload["validate_vault_sources"] = true
    cmd.Payload["validate_cluster_configs"] = true
    cmd.Payload["validate_values_files"] = true
    cmd.Payload["check_pod_env"] = true
    
    return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishContextCorrelation(customerID, contextName, user string, appReference, environmentReference string) (*api.GitOpsCommandMessage, error) {
    cmd := api.NewContextPairingCommandMessage(customerID, contextName, user)
    cmd.Action = "correlate-context"
    cmd.CommandType = "correlate"
    cmd.ManifestMetadata.AppReference = appReference
    cmd.ManifestMetadata.EnvironmentReference = environmentReference
    
    // Add Context pairing correlation payload
    cmd.Payload["app_reference"] = appReference
    cmd.Payload["environment_reference"] = environmentReference
    cmd.Payload["validate_pairing"] = true
    cmd.Payload["sync_after_correlation"] = true
    
    return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishManifestInspection(customerID, contextName, user string, manifestType string) (*api.GitOpsCommandMessage, error) {
    cmd := api.NewGitOpsCommandMessage(customerID, contextName, "inspect-manifests", manifestType, user)
    cmd.TargetService = fmt.Sprintf("%s-manifest-inspector", manifestType)
    cmd.CommandType = "inspect"
    cmd.Priority = 6 // Medium-high priority for manifest inspection
    
    // Add manifest inspection specific payload
    cmd.Payload["manifest_type"] = manifestType
    cmd.Payload["deep_inspection"] = true
    cmd.Payload["validate_references"] = true
    cmd.Payload["include_performance_metrics"] = true
    
    return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishCustomerBranchSync(customerID, contextName, customerBranch, user string) (*api.GitOpsCommandMessage, error) {
    cmd := api.NewGitOpsCommandMessage(customerID, contextName, "sync-customer-branch", "git", user)
    cmd.TargetService = "git-sync-service"
    cmd.GitOpsMetadata.CustomerBranch = customerBranch
    cmd.Priority = 7 // High priority for customer branch changes
    
    // Add customer branch sync payload
    cmd.Payload["customer_branch"] = customerBranch
    cmd.Payload["sync_all_environments"] = true
    cmd.Payload["validate_after_sync"] = true
    
    return cmd, p.PublishGitOpsCommand(cmd)
}

// Batch operations for multiple Context pairing commands
func (p *GitOpsCommandPublisher) PublishMultiContextCorrelation(customerID, contextName, user string, contextPairings []struct{ AppRef, EnvRef string }) ([]*api.GitOpsCommandMessage, error) {
    var commands []*api.GitOpsCommandMessage
    var errors []error
    
    for _, pairing := range contextPairings {
        cmd, err := p.PublishContextCorrelation(customerID, contextName, user, pairing.AppRef, pairing.EnvRef)
        if err != nil {
            errors = append(errors, fmt.Errorf("failed to publish command for context pairing %s+%s: %w", pairing.AppRef, pairing.EnvRef, err))
            continue
        }
        commands = append(commands, cmd)
    }
    
    if len(errors) > 0 {
        return commands, fmt.Errorf("some context pairing commands failed: %v", errors)
    }
    
    return commands, nil
}
```

### Task 4: GitOps Action Endpoints Implementation

**File: `internal/handlers/gitops_actions.go`**

```go
type GitOpsActionHandler struct {
    appStore         *storage.AppStore
    environmentStore *storage.EnvironmentStore
    contextStore     *storage.ContextStore
    publisher        *events.GitOpsCommandPublisher
}

func NewGitOpsActionHandler(appStore *storage.AppStore, envStore *storage.EnvironmentStore, contextStore *storage.ContextStore, pub *events.GitOpsCommandPublisher) *GitOpsActionHandler {
    return &GitOpsActionHandler{
        appStore:         appStore,
        environmentStore: envStore,
        contextStore:     contextStore,
        publisher:        pub,
    }
}

type GitOpsActionResponse struct {
    Success              bool     `json:"success"`
    CorrelationID        string   `json:"correlation_id"`
    Message              string   `json:"message"`
    Action               string   `json:"action"`
    ManifestType         string   `json:"manifest_type"`
    CustomerID           string   `json:"customer_id"`
    AppNames             []string `json:"app_names,omitempty"`
    EnvironmentNames     []string `json:"environment_names,omitempty"`
    ContextPairings      []string `json:"context_pairings,omitempty"`
    ApplicationSets      []string `json:"applicationsets,omitempty"`
    VaultSources         []string `json:"vault_sources,omitempty"`
    ClusterConfigs       []string `json:"cluster_configs,omitempty"`
}

// App manifest synchronization endpoint
func (h *GitOpsActionHandler) HandleSyncApps(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    customer, _ := auth.GetCustomerFromContext(r.Context())
    
    // Verify Context exists with customer isolation
    context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
    if err != nil {
        if err == storage.ErrContextNotFound {
            http.Error(w, "Context not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
        return
    }
    
    // Get App manifest referenced by Context
    app, err := h.appStore.Get(r.Context(), context.Spec.AppReference, customer.CustomerID)
    if err != nil {
        http.Error(w, "Failed to get App manifest", http.StatusInternalServerError)
        return
    }
    
    var correlationIDs []string
    var applicationSetNames []string
    
    // Publish App sync command
    cmd, err := h.publisher.PublishAppSync(customer.CustomerID, contextName, app.Metadata.Name, customer.UserID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to publish App sync command for %s", app.Metadata.Name), http.StatusInternalServerError)
        return
    }
    correlationIDs = append(correlationIDs, cmd.CorrelationID)
    
    // Collect ApplicationSet names from App manifest
    for _, appSet := range app.Spec.ApplicationSets {
        applicationSetNames = append(applicationSetNames, appSet.Name)
    }
    
    response := GitOpsActionResponse{
        Success:         true,
        CorrelationID:   strings.Join(correlationIDs, ","),
        Message:         fmt.Sprintf("App sync command published successfully for %s with %d ApplicationSets", app.Metadata.Name, len(applicationSetNames)),
        Action:          "sync-apps",
        ManifestType:    "app",
        CustomerID:      customer.CustomerID,
        AppNames:        []string{app.Metadata.Name},
        ApplicationSets: applicationSetNames,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Environment manifest validation endpoint
func (h *GitOpsActionHandler) HandleValidateEnvironments(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    customer, _ := auth.GetCustomerFromContext(r.Context())
    
    // Verify Context exists with customer isolation
    context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
    if err != nil {
        if err == storage.ErrContextNotFound {
            http.Error(w, "Context not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
        return
    }
    
    // Get Environment manifest referenced by Context
    environment, err := h.environmentStore.Get(r.Context(), context.Spec.EnvironmentReference, customer.CustomerID)
    if err != nil {
        http.Error(w, "Failed to get Environment manifest", http.StatusInternalServerError)
        return
    }
    
    var correlationIDs []string
    var vaultSources []string
    var clusterConfigs []string
    
    // Publish Environment validation command
    cmd, err := h.publisher.PublishEnvironmentValidation(customer.CustomerID, contextName, environment.Metadata.Name, customer.UserID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to publish Environment validation command for %s", environment.Metadata.Name), http.StatusInternalServerError)
        return
    }
    correlationIDs = append(correlationIDs, cmd.CorrelationID)
    
    // Collect Vault sources from Environment manifest
    for _, vaultSource := range environment.Spec.VaultSources {
        vaultSources = append(vaultSources, vaultSource.Path)
    }
    
    // Collect cluster configs from Environment manifest
    for clusterName := range environment.Spec.ClusterConfigs {
        clusterConfigs = append(clusterConfigs, clusterName)
    }
    
    response := GitOpsActionResponse{
        Success:          true,
        CorrelationID:    strings.Join(correlationIDs, ","),
        Message:          fmt.Sprintf("Environment validation command published successfully for %s with %d vault sources and %d clusters", environment.Metadata.Name, len(vaultSources), len(clusterConfigs)),
        Action:           "validate-environments",
        ManifestType:     "environment",
        CustomerID:       customer.CustomerID,
        EnvironmentNames: []string{environment.Metadata.Name},
        VaultSources:     vaultSources,
        ClusterConfigs:   clusterConfigs,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Context pairing correlation endpoint
func (h *GitOpsActionHandler) HandleCorrelateContexts(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    customer, _ := auth.GetCustomerFromContext(r.Context())
    
    // Verify Context exists with customer isolation
    context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
    if err != nil {
        if err == storage.ErrContextNotFound {
            http.Error(w, "Context not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
        return
    }
    
    // Publish Context pairing correlation command
    cmd, err := h.publisher.PublishContextCorrelation(customer.CustomerID, contextName, customer.UserID, context.Spec.AppReference, context.Spec.EnvironmentReference)
    if err != nil {
        http.Error(w, "Failed to publish Context correlation command", http.StatusInternalServerError)
        return
    }
    
    contextPairingDescription := fmt.Sprintf("%s+%s", context.Spec.AppReference, context.Spec.EnvironmentReference)
    
    response := GitOpsActionResponse{
        Success:         true,
        CorrelationID:   cmd.CorrelationID,
        Message:         fmt.Sprintf("Context correlation command published successfully for pairing: %s", contextPairingDescription),
        Action:          "correlate-contexts",
        ManifestType:    "context",
        CustomerID:      customer.CustomerID,
        ContextPairings: []string{contextPairingDescription},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Manifest inspection endpoint for detailed analysis
func (h *GitOpsActionHandler) HandleInspectManifests(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    customer, _ := auth.GetCustomerFromContext(r.Context())
    manifestType := r.URL.Query().Get("type") // app, environment, context, all
    
    if manifestType == "" {
        manifestType = "all"
    }
    
    // Verify Context exists with customer isolation
    _, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
    if err != nil {
        if err == storage.ErrContextNotFound {
            http.Error(w, "Context not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
        return
    }
    
    // Publish manifest inspection command
    cmd, err := h.publisher.PublishManifestInspection(customer.CustomerID, contextName, customer.UserID, manifestType)
    if err != nil {
        http.Error(w, "Failed to publish manifest inspection command", http.StatusInternalServerError)
        return
    }
    
    response := GitOpsActionResponse{
        Success:      true,
        CorrelationID: cmd.CorrelationID,
        Message:      fmt.Sprintf("Manifest inspection command published successfully (type: %s)", manifestType),
        Action:       "inspect-manifests",
        ManifestType: manifestType,
        CustomerID:   customer.CustomerID,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### Task 5: Result Consumer Framework

**File: `internal/events/consumer.go`**

```go
type ResultConsumer struct {
    messageBus   *MessageBus
    handlers     map[string]ResultHandler
    stopChan     chan struct{}
}

type ResultHandler interface {
    HandleResult(result *api.ResultMessage) error
}

func NewResultConsumer(mb *MessageBus) *ResultConsumer {
    return &ResultConsumer{
        messageBus: mb,
        handlers:   make(map[string]ResultHandler),
        stopChan:   make(chan struct{}),
    }
}

func (c *ResultConsumer) RegisterHandler(serviceName string, handler ResultHandler) {
    c.handlers[serviceName] = handler
}

func (c *ResultConsumer) Start() error {
    msgs, err := c.messageBus.channel.Consume(
        "aggregator.q", // queue
        "",            // consumer
        false,         // auto-ack (we'll manually ack)
        false,         // exclusive
        false,         // no-local
        false,         // no-wait
        nil,           // args
    )
    if err != nil {
        return fmt.Errorf("failed to register consumer: %w", err)
    }
    
    go func() {
        for {
            select {
            case msg := <-msgs:
                if err := c.processMessage(msg); err != nil {
                    log.Printf("Error processing message: %v", err)
                    msg.Nack(false, true) // Requeue on error
                } else {
                    msg.Ack(false)
                }
            case <-c.stopChan:
                return
            }
        }
    }()
    
    return nil
}

func (c *ResultConsumer) processMessage(msg amqp.Delivery) error {
    var result api.ResultMessage
    if err := json.Unmarshal(msg.Body, &result); err != nil {
        return fmt.Errorf("failed to unmarshal result: %w", err)
    }
    
    handler, exists := c.handlers[result.ServiceName]
    if !exists {
        log.Printf("No handler registered for service: %s", result.ServiceName)
        return nil // Not an error, just no handler
    }
    
    return handler.HandleResult(&result)
}

func (c *ResultConsumer) Stop() {
    close(c.stopChan)
}
```

### Task 6: Enhanced API Gateway Main

**File: `cmd/gateway/main.go`** (update existing)

```go
func main() {
    // Configuration loading
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
    
    // Dependencies
    contextStore := storage.NewContextStore(db)
    publisher := events.NewCommandPublisher(messageBus)
    
    // Handlers
    contextHandler := handlers.NewContextHandler(contextStore)
    actionHandler := handlers.NewActionHandler(contextStore, publisher)
    
    // Router setup with new action endpoints
    router := setupRouter(contextHandler, actionHandler)
    
    // Start result consumer (basic setup for Phase 1D)
    consumer := events.NewResultConsumer(messageBus)
    if err := consumer.Start(); err != nil {
        log.Fatal("Failed to start consumer:", err)
    }
    defer consumer.Stop()
    
    // Server startup
    server := &http.Server{
        Addr:    cfg.Port,
        Handler: router,
    }
    
    log.Printf("Server starting on %s", cfg.Port)
    log.Fatal(server.ListenAndServe())
}

func setupRouter(contextHandler *handlers.ContextHandler, actionHandler *handlers.ActionHandler) *mux.Router {
    r := mux.NewRouter()
    
    // Middleware
    r.Use(auth.BasicAuthMiddleware())
    r.Use(loggingMiddleware())
    
    // Context CRUD routes (from Phase 1A)
    r.HandleFunc("/contexts", contextHandler.CreateContext).Methods("POST")
    r.HandleFunc("/contexts", contextHandler.ListContexts).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.GetContext).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.UpdateContext).Methods("PUT")
    r.HandleFunc("/contexts/{name}", contextHandler.DeleteContext).Methods("DELETE")
    
    // New action routes (Phase 1B)
    r.HandleFunc("/contexts/{name}/actions/refresh", actionHandler.HandleRefresh).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/validate", actionHandler.HandleValidate).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/inspect", actionHandler.HandleInspect).Methods("POST")
    
    // Health check
    r.HandleFunc("/health", healthCheck).Methods("GET")
    
    return r
}
```

### Task 7: Configuration Updates

**File: `internal/config/config.go`** (update existing)

```go
type Config struct {
    Port         string `env:"PORT" envDefault:":8080"`
    DatabaseURL  string `env:"DATABASE_URL" envDefault:"postgres://localhost/contextops?sslmode=disable"`
    RabbitMQURL  string `env:"RABBITMQ_URL" envDefault:"amqp://localhost:5672/"`
    LogLevel     string `env:"LOG_LEVEL" envDefault:"info"`
}
```

### Task 8: Run Tracking Database Schema

**File: `migrations/002_run_tracking.up.sql`**

```sql
-- Table to track command runs and their correlation IDs
CREATE TABLE command_runs (
    correlation_id UUID PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL REFERENCES contexts(name) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- pending, in_progress, completed, failed
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_command_runs_context_action ON command_runs (context_name, action);
CREATE INDEX idx_command_runs_status ON command_runs (status);
CREATE INDEX idx_command_runs_requested_at ON command_runs (requested_at DESC);
```

---

## Testing Requirements

### Unit Tests

**File: `internal/events/publisher_test.go`**
- Test message envelope creation
- Test command publishing with different actions
- Test error handling for RabbitMQ connection issues

**File: `internal/events/consumer_test.go`**  
- Test message consumption and handler registration
- Test error handling and message requeuing
- Test graceful shutdown

### Integration Tests

**File: `cmd/gateway/actions_integration_test.go`**
- Test complete command publishing flow
- Test action endpoints return proper responses
- Test correlation ID tracking
- Test error scenarios (context not found, RabbitMQ down)

**File: `internal/events/rabbitmq_integration_test.go`**
- Test RabbitMQ topology creation
- Test message round-trip (publish → consume)
- Test exchange and queue bindings

---

## GitOps Dependencies

Add to `go.mod`:
```
require (
    // Message queue for GitOps events
    github.com/rabbitmq/amqp091-go v1.9.0
    github.com/google/uuid v1.4.0
    
    // GitOps integrations (from Phase 1A)
    github.com/argoproj/argo-cd/v2 v2.8.4
    github.com/hashicorp/vault/api v1.10.0
    helm.sh/helm/v3 v3.12.3
    
    // Additional dependencies for GitOps messaging
    github.com/prometheus/client_golang v1.17.0 // metrics
    go.uber.org/zap v1.26.0 // structured logging
)
```

---

## GitOps Validation Checklist

Before marking Phase 1B complete:

**App+Environment+Context RabbitMQ Infrastructure:**
- [ ] Manifest-aware RabbitMQ topology (exchanges, queues) created correctly with customer isolation
- [ ] App manifest synchronization queue configured with proper TTL and priority settings
- [ ] Environment manifest validation queue configured with appropriate retry policies
- [ ] Context pairing correlation queue configured for manifest correlation operations
- [ ] GitOps Dead Letter Queue (DLQ) setup for failed manifest command handling
- [ ] Queue bindings work correctly for manifest routing keys (cmd.app.*, cmd.environment.*, cmd.context.*)

**Manifest Action Endpoints:**
- [ ] App manifest sync endpoints publish commands to correct routing keys
- [ ] Environment manifest validation endpoints publish validation commands
- [ ] Context pairing correlation endpoints publish correlation commands
- [ ] Manifest inspection endpoints publish deep analysis commands
- [ ] Customer isolation works correctly across all manifest endpoints
- [ ] All manifest action endpoints return proper JSON responses with manifest-specific fields

**Manifest Message Handling:**
- [ ] Manifest message envelopes include all required fields (customer_id, manifest_type, app_name, environment_name, etc.)
- [ ] App manifest metadata properly included in messages (ApplicationSets, Helm sources, Git sources)
- [ ] Environment manifest metadata properly included (Vault sources, cluster configs, values files)
- [ ] Context pairing correlation metadata properly included (app_reference, environment_reference)
- [ ] Customer-scoped correlation IDs are properly generated and tracked
- [ ] Manifest message priorities work correctly (App=8, Environment=7, Context=6)

**Database and Persistence:**
- [ ] Database migrations for manifest run tracking applied successfully
- [ ] Customer-scoped command tracking works correctly for all manifest types
- [ ] App manifest status tracking tables operational
- [ ] Environment manifest validation status tracking tables operational
- [ ] Context pairing correlation tracking tables operational

**Integration and Error Handling:**
- [ ] Consumer can receive and process manifest result messages with proper deserialization
- [ ] Integration tests pass for manifest message flow (App, Environment, Context)
- [ ] Error handling works for RabbitMQ connection failures with manifest context
- [ ] Manifest DLQ properly handles failed commands with retry logic
- [ ] Customer isolation prevents cross-tenant message visibility

**Performance and Monitoring:**
- [ ] Health check endpoint indicates RabbitMQ connectivity and manifest queue health
- [ ] Manifest message throughput meets performance requirements
- [ ] Memory and CPU usage acceptable under manifest workload
- [ ] Proper logging for manifest command publishing and error scenarios

---

## GitOps Message Flow Verification

Test the complete GitOps flow:

**App Manifest Synchronization Flow:**
1. **POST** `/api/v1/contexts/{name}/actions/sync-apps`
2. Verify manifest command published to `gitops.commands` exchange  
3. Verify routing key is `cmd.app.sync-app`
4. Verify App manifest specific metadata in message envelope (ApplicationSets, Helm sources, Git sources)
5. Verify customer isolation and correlation ID returned in response

**Environment Manifest Validation Flow:**
1. **POST** `/api/v1/contexts/{name}/actions/validate-environments`
2. Verify command published to `gitops.commands` exchange
3. Verify routing key is `cmd.environment.validate-environment`
4. Verify Environment manifest metadata (Vault sources, cluster configs, values files)
5. Verify customer-scoped correlation tracking

**Context Pairing Correlation Flow:**
1. **POST** `/api/v1/contexts/{name}/actions/correlate-contexts`
2. Verify Context pairing correlation command published
3. Verify routing key is `cmd.context.correlate-context`
4. Verify app_reference and environment_reference metadata properly included
5. Verify pairing validation and correlation ID handling

**Manifest Inspection Flow:**
1. **POST** `/api/v1/contexts/{name}/actions/inspect-manifests?type=app`
2. Verify inspection command published to appropriate queue
3. Verify deep inspection payload and manifest reference validation
4. Verify customer isolation throughout inspection process

---

## GitOps Next Steps

Upon completion, Phase 1B provides:
- **Manifest-aware event-driven command publishing system** with App, Environment, and Context support
- **Customer-isolated RabbitMQ topology** ready for manifest integration services  
- **Manifest action endpoints** for triggering App synchronization, Environment validation, and Context correlation workflows
- **Comprehensive message envelope structure** with manifest metadata and customer context
- **Foundation for manifest result aggregation** with App+Environment+Context correlation support

**Handoff to Phase 1C:** Manifest integration services can now consume App synchronization commands, Environment validation commands, and Context correlation commands from RabbitMQ and publish comprehensive manifest results. The manifest message envelope structure is established with customer isolation, and correlation tracking supports the three-manifest architecture.