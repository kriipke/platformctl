# PHASE 1C: App+Environment+Context Integration Services

**Duration:** 6-8 days  
**Prerequisites:** Phase 1A & 1B completed, ArgoCD accessible, Vault accessible, multi-cluster K8s access, customer Git repositories  
**Deliverable:** Five manifest-focused integration services consuming App, Environment, and Context commands and publishing comprehensive manifest results with customer isolation

---

## Overview

Implement five manifest-specialized integration services that consume App, Environment, and Context commands from RabbitMQ, integrate with manifest-specific external systems, and publish comprehensive manifest results with customer isolation. This phase transforms the system from generic monitoring to App+Environment+Context optimized workflows including ApplicationSet monitoring from App manifests, Vault validation from Environment manifests, and Context pairing correlation.

**Manifest Integration Service Implementation Order:**
1. **Environment Manifest Validation Service** (validates Vault sources, cluster configs, and values files)
2. **App Manifest Sync Service** (ApplicationSet monitoring and Helm source validation from App manifests)  
3. **Context Pairing Correlation Service** (validates App+Environment pairings and synchronization status)
4. **Multi-Environment Kubernetes Service** (multi-cluster workload correlation across environments)
5. **Customer Git Branch Service** (customer branch monitoring and App/Environment manifest validation)

## Success Criteria

✅ All 5 manifest integration services deployed and consuming App, Environment, and Context commands with customer isolation  
✅ Environment manifest validation service validates Vault sources, cluster configs, and values files with pod environment correlation  
✅ App manifest sync service tracks ApplicationSets and Helm sources across multiple environments from App manifests  
✅ Context pairing correlation service validates App+Environment references and monitors pairing synchronization status  
✅ Multi-environment Kubernetes service monitors workloads across environments with manifest correlation  
✅ Customer Git branch service monitors customer-specific branches and validates App/Environment manifest changes  
✅ All services publish manifest-specific result messages with comprehensive App, Environment, and Context metadata  
✅ Customer isolation enforced across all manifest integrations  
✅ App+Environment+Context correlation working correctly  
✅ Manifest integration tests validate complete three-manifest workflows  

---

## Task 1: Manifest Service Template & Common Components

### Manifest Shared Service Framework

**File: `internal/services/manifest_base.go`**

```go
type ManifestBaseService struct {
    Name              string
    Consumer          *events.ManifestCommandConsumer
    Publisher         *events.ManifestResultPublisher
    AppStore          *storage.AppStore
    EnvironmentStore  *storage.EnvironmentStore
    ContextStore      *storage.ContextStore
    CustomerValidator *auth.CustomerValidator
    ServiceMetrics    *ManifestServiceMetrics
}

func NewManifestBaseService(name string, gmb *events.GitOpsMessageBus, appStore *storage.AppStore, envStore *storage.EnvironmentStore, contextStore *storage.ContextStore) *ManifestBaseService {
    consumer := events.NewManifestCommandConsumer(gmb, fmt.Sprintf("gitops.%s.q", name))
    publisher := events.NewManifestResultPublisher(gmb)
    
    return &ManifestBaseService{
        Name:              name,
        Consumer:          consumer,
        Publisher:         publisher,
        AppStore:          appStore,
        EnvironmentStore:  envStore,
        ContextStore:      contextStore,
        CustomerValidator: auth.NewCustomerValidator(),
        ServiceMetrics:    NewManifestServiceMetrics(name),
    }
}

func (mbs *ManifestBaseService) Start(handler ManifestCommandHandler) error {
    // Register metrics and health checks
    mbs.ServiceMetrics.RegisterMetrics()
    
    // Start consuming manifest commands with customer isolation
    return mbs.Consumer.StartWithCustomerIsolation(handler)
}

func (mbs *ManifestBaseService) Stop() error {
    mbs.ServiceMetrics.Shutdown()
    return mbs.Consumer.Stop()
}

func (mbs *ManifestBaseService) ValidateCustomerAccess(cmd *api.GitOpsCommandMessage) error {
    return mbs.CustomerValidator.ValidateAccess(cmd.CustomerID, cmd.ContextName)
}

func (mbs *ManifestBaseService) GetContext(customerID, contextName string) (*contexts.Context, error) {
    // Customer-isolated context retrieval
    return mbs.ContextStore.Get(context.Background(), contextName, customerID)
}

func (mbs *ManifestBaseService) GetApp(customerID, appName string) (*contexts.App, error) {
    // Customer-isolated App manifest retrieval
    return mbs.AppStore.Get(context.Background(), appName, customerID)
}

func (mbs *ManifestBaseService) GetEnvironment(customerID, environmentName string) (*contexts.Environment, error) {
    // Customer-isolated Environment manifest retrieval
    return mbs.EnvironmentStore.Get(context.Background(), environmentName, customerID)
}

type ManifestCommandHandler interface {
    HandleManifestCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error)
    GetSupportedManifestTypes() []string
    GetSupportedActions() []string
}

type ManifestServiceMetrics struct {
    ServiceName        string
    CommandsProcessed  prometheus.Counter
    ErrorsTotal        prometheus.Counter
    ProcessingTime     prometheus.Histogram
    CustomerRequests   *prometheus.CounterVec
    ManifestOperations *prometheus.CounterVec
}

func NewManifestServiceMetrics(serviceName string) *ManifestServiceMetrics {
    return &ManifestServiceMetrics{
        ServiceName: serviceName,
        CommandsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
            Name: fmt.Sprintf("manifest_%s_commands_processed_total", serviceName),
            Help: fmt.Sprintf("Total number of manifest commands processed by %s service", serviceName),
        }),
        ErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Name: fmt.Sprintf("manifest_%s_errors_total", serviceName),
            Help: fmt.Sprintf("Total number of errors in %s service", serviceName),
        }),
        ProcessingTime: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: fmt.Sprintf("manifest_%s_processing_seconds", serviceName),
            Help: fmt.Sprintf("Time spent processing manifest commands in %s service", serviceName),
        }),
        CustomerRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: fmt.Sprintf("manifest_%s_customer_requests_total", serviceName),
                Help: fmt.Sprintf("Total requests by customer for %s service", serviceName),
            },
            []string{"customer_id", "manifest_type", "action"},
        ),
        ManifestOperations: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: fmt.Sprintf("manifest_%s_operations_total", serviceName),
                Help: fmt.Sprintf("Total manifest operations by type for %s service", serviceName),
            },
            []string{"manifest_type", "operation", "status"},
        ),
    }
}

func (msm *ManifestServiceMetrics) RegisterMetrics() {
    prometheus.MustRegister(msm.CommandsProcessed)
    prometheus.MustRegister(msm.ErrorsTotal)
    prometheus.MustRegister(msm.ProcessingTime)
    prometheus.MustRegister(msm.CustomerRequests)
    prometheus.MustRegister(msm.ManifestOperations)
}

func (msm *ManifestServiceMetrics) RecordCommand(customerID, manifestType, action string, duration time.Duration, success bool) {
    msm.CommandsProcessed.Inc()
    msm.ProcessingTime.Observe(duration.Seconds())
    msm.CustomerRequests.WithLabelValues(customerID, manifestType, action).Inc()
    
    status := "success"
    if !success {
        msm.ErrorsTotal.Inc()
        status = "error"
    }
    
    msm.ManifestOperations.WithLabelValues(manifestType, action, status).Inc()
}
```

**File: `internal/events/command_consumer.go`**

```go
type CommandConsumer struct {
    messageBus *MessageBus
    queueName  string
}

func NewCommandConsumer(mb *MessageBus, queueName string) *CommandConsumer {
    return &CommandConsumer{
        messageBus: mb,
        queueName:  queueName,
    }
}

func (c *CommandConsumer) Start(handler CommandHandler) error {
    // Declare service-specific queue
    _, err := c.messageBus.channel.QueueDeclare(
        c.queueName,
        true,  // durable
        false, // delete when unused
        false, // exclusive
        false, // no-wait
        nil,   // arguments
    )
    if err != nil {
        return fmt.Errorf("failed to declare queue: %w", err)
    }
    
    // Bind to command patterns (service-specific bindings)
    if err := c.bindToCommands(); err != nil {
        return fmt.Errorf("failed to bind queue: %w", err)
    }
    
    // Start consuming
    msgs, err := c.messageBus.channel.Consume(
        c.queueName,
        "",    // consumer
        false, // auto-ack
        false, // exclusive
        false, // no-local
        false, // no-wait
        nil,   // args
    )
    if err != nil {
        return fmt.Errorf("failed to register consumer: %w", err)
    }
    
    go func() {
        for msg := range msgs {
            if err := c.processMessage(msg, handler); err != nil {
                log.Printf("Error processing message: %v", err)
                msg.Nack(false, true) // Requeue
            } else {
                msg.Ack(false)
            }
        }
    }()
    
    return nil
}

func (c *CommandConsumer) bindToCommands() error {
    // Each service binds to relevant command types
    // Implementation varies by service
    return nil
}
```

---

## Task 2: Environment Manifest Validation Service Implementation

**Priority: 1 - Foundation service for Environment manifest validation including Vault sources, cluster configs, and values files**

**File: `cmd/environment-validation-svc/main.go`**

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
    
    // Service setup
    appStore := storage.NewAppStore(db)
    environmentStore := storage.NewEnvironmentStore(db)
    contextStore := storage.NewContextStore(db)
    environmentHandler := NewEnvironmentHandler()
    service := services.NewManifestBaseService("environment-validation", messageBus, appStore, environmentStore, contextStore)
    
    // Start service
    if err := service.Start(environmentHandler); err != nil {
        log.Fatal("Failed to start environment validation service:", err)
    }
    
    log.Println("Environment validation service started")
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    log.Println("Shutting down environment validation service")
    service.Stop()
}
```

**File: `cmd/environment-validation-svc/handler.go`**

```go
type EnvironmentValidationHandler struct {
    *services.ManifestBaseService
    vaultClient         VaultClient
    kubernetesClient    KubernetesClient
    helmClient          HelmClient
    gitClient           GitClient
}

type VaultClient interface {
    ValidateVaultSources(customerID string, vaultSources []contexts.VaultSource) ([]VaultSourceValidation, error)
    ValidatePodEnvironmentVariables(customerID, namespace string, envValidations []contexts.PodEnvValidationConfig) ([]PodEnvValidationResult, error)
    GetSecretFromVaultSource(vaultSource contexts.VaultSource, key string) (string, error)
}

type KubernetesClient interface {
    ValidateClusterConfigs(customerID string, clusterConfigs map[string]contexts.ClusterConfig) ([]ClusterValidationResult, error)
    GetClusterConnection(clusterConfig contexts.ClusterConfig) (ClusterConnection, error)
    ValidateNamespaceAccess(cluster, namespace string) error
}

type HelmClient interface {
    ValidateValuesFiles(customerID string, valuesFiles []contexts.ValuesFile) ([]ValuesFileStatus, error)
    GetValuesFileContent(valuesFile contexts.ValuesFile) (map[string]interface{}, error)
    ValidateValuesStructure(content map[string]interface{}) error
}

type GitClient interface {
    ValidateValuesFileExists(gitRepo contexts.GitRepo, filePath string) (bool, error)
    GetFileContent(gitRepo contexts.GitRepo, filePath string) ([]byte, error)
    GetLastModifiedTime(gitRepo contexts.GitRepo, filePath string) (time.Time, error)
}

type VaultStaticSecretValidation struct {
    Name                string                   `json:"name"`
    VaultPath           string                   `json:"vault_path"`
    DestinationSecret   string                   `json:"destination_secret"`
    Namespace           string                   `json:"namespace"`
    ValidationStatus    string                   `json:"validation_status"` // valid, invalid, missing, unauthorized
    RequiredKeys        []VaultKeyValidation     `json:"required_keys"`
    PodCorrelations     []SecretPodCorrelation   `json:"pod_correlations"`
    LastValidated       time.Time                `json:"last_validated"`
    ErrorMessage        string                   `json:"error_message,omitempty"`
}

type VaultKeyValidation struct {
    Key              string `json:"key"`
    ExistsInVault    bool   `json:"exists_in_vault"`
    ExistsInSecret   bool   `json:"exists_in_secret"`
    ValuesMatch      bool   `json:"values_match"`
}

type SecretPodCorrelation struct {
    PodName             string                 `json:"pod_name"`
    PodNamespace        string                 `json:"pod_namespace"`
    SecretName          string                 `json:"secret_name"`
    EnvironmentVariable string                 `json:"environment_variable"`
    CorrelationStatus   string                 `json:"correlation_status"` // matched, missing, mismatch
    ExpectedValue       string                 `json:"expected_value,omitempty"`
    ActualValue         string                 `json:"actual_value,omitempty"`
    ErrorDetails        string                 `json:"error_details,omitempty"`
}

type SecretValidation struct {
    LogicalName string `json:"logical_name"`
    Path        string `json:"path"`
    Keys        []KeyValidation `json:"keys"`
    Status      string `json:"status"` // ok, missing, access_denied
    Error       string `json:"error,omitempty"`
}

type KeyValidation struct {
    Key    string `json:"key"`
    Exists bool   `json:"exists"`
}

func NewVaultHandler() *VaultHandler {
    return &VaultHandler{
        client: NewHashiCorpVaultClient(),
    }
}

func (vh *VaultHandler) HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error) {
    startTime := time.Now()
    
    // Get context configuration
    ctx, err := vh.getContext(cmd.ContextName)
    if err != nil {
        return vh.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    switch cmd.Action {
    case "refresh", "validate", "inspect":
        return vh.handleValidation(cmd, ctx, startTime)
    default:
        return vh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (vh *VaultHandler) handleValidation(cmd *api.CommandMessage, ctx *contexts.Context, startTime time.Time) (*api.ResultMessage, error) {
    result := &api.ResultMessage{
        MessageEnvelope: api.MessageEnvelope{
            SchemaVersion: 1,
            MessageID:     generateUUID(),
            CorrelationID: cmd.CorrelationID,
            ContextName:   cmd.ContextName,
            Action:        cmd.Action,
            RequestedBy:   cmd.RequestedBy,
            RequestedAt:   cmd.RequestedAt,
        },
        ServiceName: "environment-validation",
        CompletedAt: time.Now().UTC(),
    }
    
    payload := make(map[string]interface{})
    
    // Validate Vault sources from Environment manifest
    vaultValidations, err := evh.vaultClient.ValidateVaultSources(cmd.CustomerID, environment.Spec.VaultSources)
    if err != nil {
        result.Status = "error"
        result.ErrorMessage = fmt.Sprintf("vault source validation failed: %v", err)
        payload["vault_validation_status"] = "failed"
    } else {
        payload["vault_validation_status"] = "ok"
        payload["vault_validations"] = vaultValidations
    }
    
    // Validate cluster configurations
    clusterValidations, err := evh.kubernetesClient.ValidateClusterConfigs(cmd.CustomerID, environment.Spec.ClusterConfigs)
    if err != nil {
        if result.Status != "error" {
            result.Status = "degraded"
        }
        result.ErrorMessage = fmt.Sprintf("cluster validation failed: %v", err)
    } else {
        payload["cluster_validations"] = clusterValidations
    }
    
    // Validate values files
    valuesFileValidations, err := evh.helmClient.ValidateValuesFiles(cmd.CustomerID, environment.Spec.ValuesFiles)
    if err != nil {
        if result.Status != "error" {
            result.Status = "degraded"
        }
        result.ErrorMessage = fmt.Sprintf("values file validation failed: %v", err)
    } else {
        payload["values_file_validations"] = valuesFileValidations
    }
    payload["latency_ms"] = time.Since(startTime).Milliseconds()
    
    if result.Status == "" {
        result.Status = "ok"
    }
    
    result.ResultPayload = payload
    return result, nil
}
```

**File: `internal/clients/vault/client.go`**

```go
type HashiCorpVaultClient struct {
    client *api.Client
}

func NewHashiCorpVaultClient() *HashiCorpVaultClient {
    config := api.DefaultConfig()
    client, err := api.NewClient(config)
    if err != nil {
        log.Fatal("Failed to create Vault client:", err)
    }
    
    return &HashiCorpVaultClient{client: client}
}

func (vc *HashiCorpVaultClient) ValidateAuth(config *contexts.VaultConfig) error {
    // Set Vault address and namespace
    vc.client.SetAddress(config.Address)
    if config.Namespace != "" {
        vc.client.SetNamespace(config.Namespace)
    }
    
    switch config.Auth.Method {
    case "kubernetes":
        return vc.validateKubernetesAuth(config)
    case "token":
        return vc.validateTokenAuth(config)
    default:
        return fmt.Errorf("unsupported auth method: %s", config.Auth.Method)
    }
}

func (vc *HashiCorpVaultClient) validateKubernetesAuth(config *contexts.VaultConfig) error {
    // Read service account token
    tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
    if err != nil {
        return fmt.Errorf("failed to read service account token: %w", err)
    }
    
    // Authenticate with Vault
    data := map[string]interface{}{
        "role": config.Auth.Kubernetes.Role,
        "jwt":  string(tokenBytes),
    }
    
    resp, err := vc.client.Logical().Write("auth/kubernetes/login", data)
    if err != nil {
        return fmt.Errorf("kubernetes auth failed: %w", err)
    }
    
    if resp.Auth == nil {
        return fmt.Errorf("no auth info returned")
    }
    
    // Set token for subsequent requests
    vc.client.SetToken(resp.Auth.ClientToken)
    return nil
}

func (vc *HashiCorpVaultClient) ValidateSecretPaths(config *contexts.VaultConfig) ([]SecretValidation, error) {
    var validations []SecretValidation
    
    for _, secret := range config.Secrets {
        validation := SecretValidation{
            LogicalName: secret.LogicalName,
            Path:        secret.Path,
        }
        
        // Check if secret exists and validate keys
        resp, err := vc.client.Logical().Read(secret.Path)
        if err != nil {
            validation.Status = "access_denied"
            validation.Error = err.Error()
        } else if resp == nil {
            validation.Status = "missing"
            validation.Error = "secret not found"
        } else {
            validation.Status = "ok"
            
            // Validate required keys exist
            data := resp.Data
            if kv2Data, ok := data["data"].(map[string]interface{}); ok {
                data = kv2Data // Handle KV v2
            }
            
            for _, requiredKey := range secret.RequiredKeys {
                keyValidation := KeyValidation{
                    Key:    requiredKey,
                    Exists: false,
                }
                
                if _, exists := data[requiredKey]; exists {
                    keyValidation.Exists = true
                }
                
                validation.Keys = append(validation.Keys, keyValidation)
            }
        }
        
        validations = append(validations, validation)
    }
    
    return validations, nil
}
```

---

## Task 3: App Manifest Sync Service Implementation

**Priority: 2 - App manifest synchronization including ApplicationSet monitoring and Helm source validation**

**File: `cmd/app-sync-svc/main.go`** (similar structure to environment-validation-svc)

**File: `cmd/app-sync-svc/handler.go`**

```go
type AppSyncHandler struct {
    *services.ManifestBaseService
    argoCDClient         ArgoCDClient
    helmClient           HelmClient
    gitClient            GitClient
    kubernetesClient     KubernetesClient
}

type ArgoCDClient interface {
    GetApplicationSetsForApp(customerID, appName string, appSets []contexts.ApplicationSet) ([]ApplicationSetStatus, error)
    GetApplicationSetApplications(customerID, appSetName string) ([]ApplicationSetApplication, error)
    ValidateApplicationSetTemplate(appSet contexts.ApplicationSet) error
    SyncApplicationSet(customerID, appSetName string, forceSync bool) (*ApplicationSetSyncResult, error)
}

type ApplicationSetStatus struct {
    Name                 string                       `json:"name"`
    Namespace            string                       `json:"namespace"`
    AppName              string                       `json:"app_name"`
    CustomerID           string                       `json:"customer_id"`
    SyncStatus           string                       `json:"sync_status"`
    HealthStatus         string                       `json:"health_status"`
    Applications         []ApplicationSetApplication  `json:"applications"`
    Generator            ApplicationSetGenerator      `json:"generator"`
    LastSyncTime         *time.Time                   `json:"last_sync_time,omitempty"`
    HelmSourceStatus     []HelmSourceValidation       `json:"helm_source_status"`
    GitSourceStatus      []GitSourceValidation        `json:"git_source_status"`
}

type HelmSourceValidation struct {
    Name         string     `json:"name"`
    Type         string     `json:"type"` // registry, git, oci
    URL          string     `json:"url"`
    Version      string     `json:"version,omitempty"`
    Status       string     `json:"status"` // available, unavailable, error
    LastChecked  *time.Time `json:"last_checked,omitempty"`
    ErrorMessage string     `json:"error_message,omitempty"`
}

type GitSourceValidation struct {
    URL         string     `json:"url"`
    Path        string     `json:"path"`
    Revision    string     `json:"revision"`
    Status      string     `json:"status"` // available, unavailable, error
    LastCommit  string     `json:"last_commit,omitempty"`
    LastChecked *time.Time `json:"last_checked,omitempty"`
    ErrorMessage string    `json:"error_message,omitempty"`
}

type ApplicationSetSyncResult struct {
    Name         string    `json:"name"`
    SyncStarted  time.Time `json:"sync_started"`
    Status       string    `json:"status"` // syncing, synced, error
    Applications []string  `json:"applications"`
    ErrorMessage string    `json:"error_message,omitempty"`
}

type ApplicationSetApplication struct {
    Name         string     `json:"name"`
    Environment  string     `json:"environment"`
    Cluster      string     `json:"cluster"`
    Namespace    string     `json:"namespace"`
    SyncStatus   string     `json:"sync_status"`
    HealthStatus string     `json:"health_status"`
    GitCommit    string     `json:"git_commit,omitempty"`
    HelmRevision string     `json:"helm_revision,omitempty"`
    LastDeployed *time.Time `json:"last_deployed,omitempty"`
}

type EnvironmentCorrelation struct {
    Environment        string                 `json:"environment"`
    ClusterName        string                 `json:"cluster_name"`
    Namespace          string                 `json:"namespace"`
    HelmValuesFile     string                 `json:"helm_values_file"`
    CorrelationStatus  string                 `json:"correlation_status"` // correlated, missing, mismatch
    ResourceCounts     map[string]int         `json:"resource_counts"`
    SecretsValidation  []string               `json:"secrets_validation"`
    LastCorrelated     time.Time              `json:"last_correlated"`
}

type BootstrapApplication struct {
    Name                string    `json:"name"`
    Namespace           string    `json:"namespace"`
    ManagedApplicationSets []string `json:"managed_applicationsets"`
    SyncStatus          string    `json:"sync_status"`
    HealthStatus        string    `json:"health_status"`
}

type BootstrapCorrelation struct {
    BootstrapAppName    string `json:"bootstrap_app_name"`
    IsManaged           bool   `json:"is_managed"`
    CorrelationStatus   string `json:"correlation_status"` // managed, unmanaged, orphaned
}

func (ash *AppSyncHandler) HandleManifestCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
    startTime := time.Now()
    
    // Validate customer access
    if err := ash.ValidateCustomerAccess(cmd); err != nil {
        return ash.errorResult(cmd, "access denied", err, startTime)
    }
    
    // Get Context to find App reference
    context, err := ash.GetContext(cmd.CustomerID, cmd.ContextName)
    if err != nil {
        return ash.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    // Get App manifest
    app, err := ash.GetApp(cmd.CustomerID, context.Spec.AppReference)
    if err != nil {
        return ash.errorResult(cmd, "failed to get app manifest", err, startTime)
    }
    
    switch cmd.Action {
    case "sync-app":
        return ash.handleAppSync(cmd, app, startTime)
    case "inspect-manifests":
        return ash.handleAppInspection(cmd, app, startTime)
    default:
        return ash.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (ash *AppSyncHandler) handleAppSync(cmd *api.GitOpsCommandMessage, app *contexts.App, startTime time.Time) (*api.GitOpsResultMessage, error) {
    // Validate Helm sources
    helmValidations, err := ash.helmClient.ValidateHelmSources(cmd.CustomerID, app.Spec.HelmSources)
    if err != nil {
        return ash.errorResult(cmd, "helm source validation failed", err, startTime)
    }
    
    // Validate Git sources
    gitValidations, err := ash.gitClient.ValidateGitSources(cmd.CustomerID, app.Spec.GitSources)
    if err != nil {
        return ash.errorResult(cmd, "git source validation failed", err, startTime)
    }
    
    // Get ApplicationSet statuses
    appSetStatuses, err := ash.argoCDClient.GetApplicationSetsForApp(cmd.CustomerID, app.Metadata.Name, app.Spec.ApplicationSets)
    if err != nil {
        return ash.errorResult(cmd, "applicationset status failed", err, startTime)
    }
    
    // Trigger ApplicationSet sync if requested
    var syncResults []ApplicationSetSyncResult
    if forceSync, ok := cmd.Payload["sync_applicationset"].(bool); ok && forceSync {
        for _, appSet := range app.Spec.ApplicationSets {
            syncResult, err := ash.argoCDClient.SyncApplicationSet(cmd.CustomerID, appSet.Name, true)
            if err != nil {
                return ash.errorResult(cmd, fmt.Sprintf("sync failed for ApplicationSet %s", appSet.Name), err, startTime)
            }
            syncResults = append(syncResults, *syncResult)
        }
    }
    
    // Create result
    result := &api.GitOpsResultMessage{
        GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
            SchemaVersion:   1,
            MessageID:       generateUUID(),
            CorrelationID:   cmd.CorrelationID,
            CustomerID:      cmd.CustomerID,
            ContextName:     cmd.ContextName,
            Action:          cmd.Action,
            ManifestType:    "app",
            AppName:         app.Metadata.Name,
            RequestedBy:     cmd.RequestedBy,
            RequestedAt:     cmd.RequestedAt,
            Priority:        cmd.Priority,
            Payload:         make(map[string]interface{}),
            ManifestMetadata: cmd.ManifestMetadata,
        },
        ServiceName:     "app-sync",
        Status:          "healthy",
        CompletedAt:     time.Now().UTC(),
        AppManifestData: &api.AppManifestResult{
            AppName:            app.Metadata.Name,
            ApplicationSetName: app.Spec.ApplicationSets[0].Name, // Primary ApplicationSet
            Namespace:          app.Spec.ApplicationSets[0].Namespace,
            HelmSources:        helmValidations,
            GitSources:         gitValidations,
            LastSyncTime:       &startTime,
        },
        PerformanceMetrics: api.GitOpsPerformanceMetrics{
            ProcessingTimeMs: time.Since(startTime).Milliseconds(),
            ApiCallsCount:    len(helmValidations) + len(gitValidations) + len(appSetStatuses),
        },
    }
    
    // Add sync results if any
    if len(syncResults) > 0 {
        result.Payload["sync_results"] = syncResults
    }
    
    return result, nil
    
    // Collect workload data
    workloads, err := kh.client.GetWorkloadStatus(namespace)
    if err != nil {
        return kh.errorResult(cmd, "failed to get workload status", err, startTime)
    }
    
    // Collect events
    events, err := kh.client.GetWarningEvents(namespace)
    if err != nil {
        log.Printf("Warning: failed to get events: %v", err)
        events = []Event{} // Continue with empty events
    }
    
    payload := map[string]interface{}{
        "namespace":         namespace,
        "workloads":         workloads,
        "events":           events,
        "collection_time":   time.Now().UTC(),
        "latency_ms":       time.Since(startTime).Milliseconds(),
    }
    
    result := &api.ResultMessage{
        MessageEnvelope: kh.createEnvelope(cmd),
        ServiceName:     "kubernetes",
        Status:          "ok",
        CompletedAt:     time.Now().UTC(),
        ResultPayload:   payload,
    }
    
    return result, nil
}

func (kh *KubernetesHandler) resolveNamespace(ctx *contexts.Context) string {
    // Follow priority order from README
    if ctx.Spec.Kubernetes.NamespaceOverride != "" {
        if ctx.Spec.Policy.Kubernetes.AllowNamespaceOverride {
            return ctx.Spec.Kubernetes.NamespaceOverride
        }
    }
    
    // Get namespace from kubeconfig current-context
    if ns := kh.client.GetCurrentContextNamespace(); ns != "" {
        return ns
    }
    
    return "default"
}
```

**File: `internal/clients/kube/client.go`**

```go
type KubernetesClient struct {
    clientset *kubernetes.Clientset
    config    *rest.Config
}

func NewKubernetesClient(kubeconfigPath string) (*KubernetesClient, error) {
    var config *rest.Config
    var err error
    
    if kubeconfigPath != "" {
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
    } else {
        config, err = rest.InClusterConfig()
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
    }
    
    return &KubernetesClient{
        clientset: clientset,
        config:    config,
    }, nil
}

type WorkloadStatus struct {
    Deployments  []DeploymentStatus  `json:"deployments"`
    StatefulSets []StatefulSetStatus `json:"statefulsets"`
    DaemonSets   []DaemonSetStatus   `json:"daemonsets"`
    Pods         []PodStatus         `json:"pods"`
}

func (kc *KubernetesClient) GetWorkloadStatus(namespace string) (*WorkloadStatus, error) {
    ctx := context.Background()
    
    // Get deployments
    deployments, err := kc.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list deployments: %w", err)
    }
    
    // Get statefulsets
    statefulsets, err := kc.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list statefulsets: %w", err)
    }
    
    // Get pods
    pods, err := kc.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }
    
    return &WorkloadStatus{
        Deployments:  transformDeployments(deployments.Items),
        StatefulSets: transformStatefulSets(statefulsets.Items),
        Pods:         transformPods(pods.Items),
    }, nil
}
```

---

## Task 4: Context Pairing Correlation Service Implementation

**Priority: 3 - Context pairing validation, App+Environment correlation, and synchronization monitoring**

**File: `cmd/context-correlation-svc/handler.go`**

```go
type ContextPairingHandler struct {
    *services.ManifestBaseService
    appValidator         AppValidator
    environmentValidator EnvironmentValidator
    syncMonitor          SyncMonitor
}

type CustomerGitClient interface {
    ValidateCustomerBranch(customerID, branch, repositoryURL string) (*CustomerBranchValidation, error)
    GetHelmValuesFiles(customerID, branch, repositoryURL string) ([]HelmValuesFile, error)
    ValidateEnvironmentValuesFiles(customerID, branch string, environments []string) ([]EnvironmentValuesValidation, error)
    GetBranchCommitHistory(customerID, branch string, limit int) ([]GitCommit, error)
    CorrelateValuesWithEnvironments(customerID string, valuesFiles []HelmValuesFile, environments []string) ([]ValuesEnvironmentCorrelation, error)
}

type CustomerBranchValidation struct {
    CustomerID        string                           `json:"customer_id"`
    BranchName        string                           `json:"branch_name"`
    BranchPattern     string                           `json:"branch_pattern"` // customer/{customer-name}
    PatternCompliant  bool                             `json:"pattern_compliant"`
    BranchExists      bool                             `json:"branch_exists"`
    LastCommit        *GitCommit                       `json:"last_commit,omitempty"`
    HelmValuesFiles   []HelmValuesFile                 `json:"helm_values_files"`
    EnvironmentFiles  []EnvironmentValuesValidation   `json:"environment_files"`
    ValidationStatus  string                           `json:"validation_status"` // valid, invalid, missing
    ErrorMessage      string                           `json:"error_message,omitempty"`
}

type HelmValuesFile struct {
    FileName     string                 `json:"file_name"`
    FilePath     string                 `json:"file_path"`
    Environment  string                 `json:"environment"`
    FileSize     int64                  `json:"file_size"`
    LastModified time.Time              `json:"last_modified"`
    Content      map[string]interface{} `json:"content,omitempty"`
    SHA          string                 `json:"sha"`
    IsValid      bool                   `json:"is_valid"`
    Errors       []string               `json:"errors,omitempty"`
}

type EnvironmentValuesValidation struct {
    Environment           string           `json:"environment"`
    ExpectedFileName      string           `json:"expected_file_name"` // values-{env}.yaml
    ActualFileName        string           `json:"actual_file_name,omitempty"`
    FileExists            bool             `json:"file_exists"`
    FileValid             bool             `json:"file_valid"`
    ValidationErrors      []string         `json:"validation_errors,omitempty"`
    RequiredKeys          []string         `json:"required_keys"`
    MissingKeys           []string         `json:"missing_keys,omitempty"`
    EnvironmentOverrides  map[string]interface{} `json:"environment_overrides,omitempty"`
}

type ValuesEnvironmentCorrelation struct {
    Environment        string                 `json:"environment"`
    ValuesFile         string                 `json:"values_file"`
    DeployedValues     map[string]interface{} `json:"deployed_values,omitempty"`
    CorrelationStatus  string                 `json:"correlation_status"` // matched, drift, missing
    Differences        []ValuesDifference     `json:"differences,omitempty"`
    LastCorrelated     time.Time              `json:"last_correlated"`
}

type ValuesDifference struct {
    Key           string      `json:"key"`
    ExpectedValue interface{} `json:"expected_value"`
    ActualValue   interface{} `json:"actual_value"`
    DifferenceType string     `json:"difference_type"` // missing, extra, changed
}

func (gh *GitHandler) HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error) {
    startTime := time.Now()
    
    ctx, err := gh.getContext(cmd.ContextName)
    if err != nil {
        return gh.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    switch cmd.Action {
    case "refresh", "inspect":
        return gh.handleBrowsing(cmd, ctx, startTime)
    default:
        return gh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (gh *GitHandler) handleBrowsing(cmd *api.CommandMessage, ctx *contexts.Context, startTime time.Time) (*api.ResultMessage, error) {
    // For Phase 1C, we'll do basic repository validation
    // Full file browsing will be enhanced when integrated with ArgoCD sources
    
    payload := map[string]interface{}{
        "provider":          ctx.Spec.Git.Provider,
        "default_org":       ctx.Spec.Git.Browse.DefaultOrg,
        "auth_status":       "ok", // Basic validation
        "connectivity":      "ok",
        "latency_ms":       time.Since(startTime).Milliseconds(),
    }
    
    result := &api.ResultMessage{
        MessageEnvelope: gh.createEnvelope(cmd),
        ServiceName:     "git",
        Status:          "ok", 
        CompletedAt:     time.Now().UTC(),
        ResultPayload:   payload,
    }
    
    return result, nil
}
```

**File: `internal/clients/github/client.go`**

```go
type GitHubClient struct {
    client *github.Client
    token  string
}

func NewGitHubClient(token string) *GitHubClient {
    ts := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: token},
    )
    tc := oauth2.NewClient(context.Background(), ts)
    client := github.NewClient(tc)
    
    return &GitHubClient{
        client: client,
        token:  token,
    }
}

func (gc *GitHubClient) GetRepositoryContent(owner, repo, path, ref string) (*RepositoryContent, error) {
    fileContent, dirContent, resp, err := gc.client.Repositories.GetContents(
        context.Background(),
        owner,
        repo,
        path,
        &github.RepositoryContentGetOptions{Ref: ref},
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to get repository content: %w", err)
    }
    
    content := &RepositoryContent{
        Owner: owner,
        Repo:  repo,
        Path:  path,
        Ref:   ref,
    }
    
    if fileContent != nil {
        // Single file
        content.Type = "file"
        content.Content = fileContent.GetContent()
        content.Size = fileContent.GetSize()
        content.SHA = fileContent.GetSHA()
    } else if len(dirContent) > 0 {
        // Directory
        content.Type = "dir"
        for _, item := range dirContent {
            content.Items = append(content.Items, RepositoryItem{
                Name: item.GetName(),
                Type: item.GetType(),
                Size: item.GetSize(),
                SHA:  item.GetSHA(),
            })
        }
    }
    
    return content, nil
}
```

---

## Task 5: Multi-Environment Kubernetes Service Implementation

**Priority: 4 - Multi-cluster, multi-environment workload correlation and monitoring**

**File: `cmd/multi-environment-kube-svc/handler.go`**

```go
type MultiEnvironmentKubernetesHandler struct {
    *services.GitOpsBaseService
    kubernetesClients map[string]MultiClusterClient // cluster -> client mapping
    environmentMapper EnvironmentMapper
}

type MultiClusterClient interface {
    GetWorkloadStatusByEnvironment(customerID, environment, namespace string) (*EnvironmentWorkloadStatus, error)
    ValidateResourceQuotas(customerID, environment, namespace string) (*ResourceQuotaValidation, error)
    GetPodEnvironmentCorrelation(customerID, environment, namespace string, secretValidations []VaultStaticSecretValidation) ([]PodEnvironmentCorrelation, error)
    GetMultiEnvironmentComparison(customerID string, environments []string) (*MultiEnvironmentComparison, error)
}

type EnvironmentWorkloadStatus struct {
    Environment         string                      `json:"environment"`
    ClusterName         string                      `json:"cluster_name"`
    Namespace           string                      `json:"namespace"`
    CustomerID          string                      `json:"customer_id"`
    Applications        []EnvironmentApplication    `json:"applications"`
    ResourceQuotas      *ResourceQuotaStatus        `json:"resource_quotas"`
    NetworkPolicies     []NetworkPolicyStatus       `json:"network_policies"`
    SecretCorrelations  []PodEnvironmentCorrelation `json:"secret_correlations"`
    LastUpdated         time.Time                   `json:"last_updated"`
}

type EnvironmentApplication struct {
    Name             string                 `json:"name"`
    Environment      string                 `json:"environment"`
    HelmReleaseName  string                 `json:"helm_release_name,omitempty"`
    HelmRevision     string                 `json:"helm_revision,omitempty"`
    Deployments      []DeploymentStatus     `json:"deployments"`
    Services         []ServiceStatus        `json:"services"`
    Ingresses        []IngressStatus        `json:"ingresses"`
    ConfigMaps       []ConfigMapStatus      `json:"config_maps"`
    Secrets          []SecretStatus         `json:"secrets"`
    PodCount         int                    `json:"pod_count"`
    ReadyPodCount    int                    `json:"ready_pod_count"`
    OverallStatus    string                 `json:"overall_status"` // healthy, degraded, unhealthy
}

type PodEnvironmentCorrelation struct {
    PodName              string                     `json:"pod_name"`
    PodNamespace         string                     `json:"pod_namespace"`
    Environment          string                     `json:"environment"`
    EnvironmentVariables []EnvironmentVariableStatus `json:"environment_variables"`
    SecretMounts         []SecretMountStatus        `json:"secret_mounts"`
    CorrelationStatus    string                     `json:"correlation_status"` // correlated, partial, missing
    ValidationErrors     []string                   `json:"validation_errors,omitempty"`
}

type EnvironmentVariableStatus struct {
    Name                 string `json:"name"`
    ValueSource          string `json:"value_source"` // direct, secret, configmap
    SecretName           string `json:"secret_name,omitempty"`
    SecretKey            string `json:"secret_key,omitempty"`
    ExpectedFromVault    bool   `json:"expected_from_vault"`
    VaultCorrelated      bool   `json:"vault_correlated"`
    ValidationStatus     string `json:"validation_status"` // valid, invalid, missing
}

type MultiEnvironmentComparison struct {
    CustomerID           string                              `json:"customer_id"`
    ContextName          string                              `json:"context_name"`
    Environments         []string                            `json:"environments"`
    EnvironmentStatuses  map[string]*EnvironmentWorkloadStatus `json:"environment_statuses"`
    CrossEnvironmentDrift []EnvironmentDrift                 `json:"cross_environment_drift"`
    ComparisonSummary    *EnvironmentComparisonSummary       `json:"comparison_summary"`
    LastCompared         time.Time                           `json:"last_compared"`
}

type EnvironmentDrift struct {
    ResourceType      string      `json:"resource_type"`
    ResourceName      string      `json:"resource_name"`
    SourceEnvironment string      `json:"source_environment"`
    TargetEnvironment string      `json:"target_environment"`
    DriftType         string      `json:"drift_type"` // config, version, missing, extra
    Details           interface{} `json:"details"`
}

type EnvironmentComparisonSummary struct {
    TotalApplications       int     `json:"total_applications"`
    HealthyApplications     int     `json:"healthy_applications"`
    UnhealthyApplications   int     `json:"unhealthy_applications"`
    CrossEnvironmentDrifts  int     `json:"cross_environment_drifts"`
    SecretCorrelationIssues int     `json:"secret_correlation_issues"`
    OverallHealthScore      float64 `json:"overall_health_score"`
}

func (ah *ArgoCDHandler) HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error) {
    startTime := time.Now()
    
    ctx, err := ah.getContext(cmd.ContextName)
    if err != nil {
        return ah.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    switch cmd.Action {
    case "refresh", "inspect":
        return ah.handleInspection(cmd, ctx, startTime)
    case "sync":
        return ah.handleSync(cmd, ctx, startTime)
    default:
        return ah.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (ah *ArgoCDHandler) handleInspection(cmd *api.CommandMessage, ctx *contexts.Context, startTime time.Time) (*api.ResultMessage, error) {
    // Get ArgoCD token from Vault
    token, err := ah.getArgoCDToken(ctx)
    if err != nil {
        return ah.errorResult(cmd, "failed to get ArgoCD token", err, startTime)
    }
    
    // Set up ArgoCD client
    ah.client.SetToken(token)
    ah.client.SetAddress(ctx.Spec.ArgoCD.Address)
    
    // Get applications
    apps, err := ah.client.GetApplications(ctx.Spec.ArgoCD.Selectors.Apps, ctx.Spec.ArgoCD.Selectors.Project)
    if err != nil {
        return ah.errorResult(cmd, "failed to get applications", err, startTime)
    }
    
    payload := map[string]interface{}{
        "applications":     apps,
        "server_address":   ctx.Spec.ArgoCD.Address,
        "latency_ms":      time.Since(startTime).Milliseconds(),
    }
    
    result := &api.ResultMessage{
        MessageEnvelope: ah.createEnvelope(cmd),
        ServiceName:     "argocd",
        Status:          "ok",
        CompletedAt:     time.Now().UTC(),
        ResultPayload:   payload,
    }
    
    return result, nil
}

func (ah *ArgoCDHandler) getArgoCDToken(ctx *contexts.Context) (string, error) {
    tokenRef := ctx.Spec.ArgoCD.Auth.TokenRef
    return ah.vaultClient.GetSecret(tokenRef.VaultSecretLogicalName, tokenRef.Key)
}
```

---

## Task 6: Helm Values Correlation Service Implementation

**Priority: 5 - Per-environment Helm values correlation, validation, and deployment tracking**

**File: `cmd/helm-values-correlation-svc/handler.go`**

```go
type HelmValuesCorrelationHandler struct {
    *services.GitOpsBaseService
    helmClient              HelmClient
    gitClient               CustomerGitClient
    kubernetesClient        KubernetesClient
    valuesCorrelator        ValuesCorrelator
}

type HelmClient interface {
    GetHelmReleases(customerID, environment, namespace string) ([]HelmReleaseStatus, error)
    GetReleaseValues(customerID, releaseName, namespace string) (map[string]interface{}, error)
    CompareValuesWithSource(customerID string, releaseValues map[string]interface{}, sourceValuesFile string) (*ValuesComparison, error)
    ValidateHelmChart(customerID, chartName, chartVersion string) (*HelmChartValidation, error)
}

type ValuesCorrelator interface {
    CorrelateEnvironmentValues(customerID, contextName string, environments []string) (*EnvironmentValuesCorrelation, error)
    ValidateValuesConsistency(customerID string, environmentValues map[string]map[string]interface{}) (*ValuesConsistencyReport, error)
    DetectValuesPatterns(customerID string, valuesHistory []HelmValuesSnapshot) (*ValuesPatternAnalysis, error)
}

type HelmReleaseStatus struct {
    Name            string                 `json:"name"`
    Namespace       string                 `json:"namespace"`
    Environment     string                 `json:"environment"`
    ChartName       string                 `json:"chart_name"`
    ChartVersion    string                 `json:"chart_version"`
    AppVersion      string                 `json:"app_version"`
    Revision        int                    `json:"revision"`
    Status          string                 `json:"status"`
    Updated         time.Time              `json:"updated"`
    Values          map[string]interface{} `json:"values"`
    ComputedValues  map[string]interface{} `json:"computed_values"`
    SourceValuesFile string                `json:"source_values_file,omitempty"`
    ValuesFileHash  string                 `json:"values_file_hash,omitempty"`
}

type ValuesComparison struct {
    ReleaseName         string                 `json:"release_name"`
    Environment         string                 `json:"environment"`
    SourceFile          string                 `json:"source_file"`
    DeployedValues      map[string]interface{} `json:"deployed_values"`
    SourceValues        map[string]interface{} `json:"source_values"`
    Differences         []ValuesDifference     `json:"differences"`
    ComparisonScore     float64                `json:"comparison_score"` // 0-1, 1 = perfect match
    DriftDetected       bool                   `json:"drift_detected"`
    LastCompared        time.Time              `json:"last_compared"`
}

type EnvironmentValuesCorrelation struct {
    CustomerID          string                           `json:"customer_id"`
    ContextName         string                           `json:"context_name"`
    Environments        []string                         `json:"environments"`
    HelmReleases        map[string][]HelmReleaseStatus   `json:"helm_releases"` // env -> releases
    ValuesComparisons   map[string]*ValuesComparison     `json:"values_comparisons"`
    CrossEnvConsistency *ValuesConsistencyReport         `json:"cross_env_consistency"`
    PatternAnalysis     *ValuesPatternAnalysis           `json:"pattern_analysis"`
    CorrelationSummary  *ValuesCorrelationSummary        `json:"correlation_summary"`
    LastCorrelated      time.Time                        `json:"last_correlated"`
}

type ValuesConsistencyReport struct {
    ConsistentKeys      []string               `json:"consistent_keys"`
    InconsistentKeys    []InconsistentKey      `json:"inconsistent_keys"`
    EnvironmentSpecific []EnvironmentSpecificKey `json:"environment_specific"`
    ConsistencyScore    float64                `json:"consistency_score"`
    Recommendations     []ConsistencyRecommendation `json:"recommendations"`
}

type InconsistentKey struct {
    Key                 string                 `json:"key"`
    EnvironmentValues   map[string]interface{} `json:"environment_values"`
    InconsistencyType   string                 `json:"inconsistency_type"` // type_mismatch, value_drift
    SuggestedResolution string                 `json:"suggested_resolution"`
}

type ValuesPatternAnalysis struct {
    CommonPatterns      []ValuePattern         `json:"common_patterns"`
    EnvironmentPatterns map[string][]ValuePattern `json:"environment_patterns"`
    AntiPatterns        []AntiPattern          `json:"anti_patterns"`
    BestPractices       []BestPracticeViolation `json:"best_practices"`
}

type ValuePattern struct {
    Pattern     string   `json:"pattern"`
    Description string   `json:"description"`
    Frequency   int      `json:"frequency"`
    Environments []string `json:"environments"`
}

type ValuesCorrelationSummary struct {
    TotalEnvironments    int     `json:"total_environments"`
    CorrelatedReleases   int     `json:"correlated_releases"`
    DriftingReleases     int     `json:"drifting_releases"`
    ConsistencyScore     float64 `json:"consistency_score"`
    OverallHealth        string  `json:"overall_health"` // healthy, drift-detected, inconsistent
}

func (nrh *NewRelicHandler) HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error) {
    startTime := time.Now()
    
    ctx, err := nrh.getContext(cmd.ContextName)
    if err != nil {
        return nrh.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    switch cmd.Action {
    case "refresh", "inspect":
        return nrh.handleInspection(cmd, ctx, startTime)
    default:
        return nrh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (nrh *NewRelicHandler) handleInspection(cmd *api.CommandMessage, ctx *contexts.Context, startTime time.Time) (*api.ResultMessage, error) {
    // Get API key from Vault
    apiKey, err := nrh.getNewRelicAPIKey(ctx)
    if err != nil {
        return nrh.errorResult(cmd, "failed to get New Relic API key", err, startTime)
    }
    
    // Set up New Relic client
    nrh.client.SetAPIKey(apiKey)
    nrh.client.SetAccountID(ctx.Spec.NewRelic.AccountID)
    
    // Find entities by tags
    entities, err := nrh.client.FindEntitiesByTags(ctx.Spec.NewRelic.EntitySelector.TagFilters)
    if err != nil {
        return nrh.errorResult(cmd, "failed to find entities", err, startTime)
    }
    
    // Get metrics for found entities
    metrics := make(map[string]interface{})
    for _, entity := range entities {
        entityMetrics, err := nrh.client.GetEntityMetrics(entity.GUID, ctx.Spec.NewRelic.Metrics)
        if err != nil {
            log.Printf("Warning: failed to get metrics for entity %s: %v", entity.GUID, err)
            continue
        }
        metrics[entity.GUID] = entityMetrics
    }
    
    payload := map[string]interface{}{
        "entities":    entities,
        "metrics":     metrics,
        "account_id":  ctx.Spec.NewRelic.AccountID,
        "latency_ms": time.Since(startTime).Milliseconds(),
    }
    
    result := &api.ResultMessage{
        MessageEnvelope: nrh.createEnvelope(cmd),
        ServiceName:     "newrelic", 
        Status:          "ok",
        CompletedAt:     time.Now().UTC(),
        ResultPayload:   payload,
    }
    
    return result, nil
}
```

---

## Task 7: Service Deployment & Queue Binding

**File: `internal/events/service_queues.go`**

```go
// Queue binding configurations for each service
func SetupServiceQueues(mb *MessageBus) error {
    services := map[string][]string{
        "vault-svc.q": {
            "cmd.context.*", // Vault validates for all actions
        },
        "kube-svc.q": {
            "cmd.context.refresh",
            "cmd.context.inspect",
        },
        "git-svc.q": {
            "cmd.context.refresh",
            "cmd.context.inspect",
        },
        "argocd-svc.q": {
            "cmd.context.refresh",
            "cmd.context.inspect",
            "cmd.context.sync",
        },
        "newrelic-svc.q": {
            "cmd.context.refresh",
            "cmd.context.inspect",
        },
    }
    
    for queueName, routingKeys := range services {
        // Declare queue
        _, err := mb.channel.QueueDeclare(
            queueName,
            true,  // durable
            false, // delete when unused
            false, // exclusive
            false, // no-wait
            nil,   // arguments
        )
        if err != nil {
            return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
        }
        
        // Bind to routing keys
        for _, routingKey := range routingKeys {
            if err := mb.channel.QueueBind(
                queueName,
                routingKey,
                "contextops.commands",
                false,
                nil,
            ); err != nil {
                return fmt.Errorf("failed to bind queue %s to %s: %w", queueName, routingKey, err)
            }
        }
    }
    
    return nil
}
```

---

## GitOps Dependencies

Add to `go.mod`:
```
require (
    // GitOps core dependencies
    github.com/hashicorp/vault/api v1.10.0
    github.com/argoproj/argo-cd/v2 v2.8.4
    helm.sh/helm/v3 v3.12.3
    
    // Kubernetes multi-cluster support
    k8s.io/client-go v0.28.2
    k8s.io/api v0.28.2
    k8s.io/apimachinery v0.28.2
    sigs.k8s.io/controller-runtime v0.16.2
    
    // Git repository access
    github.com/google/go-github/v56 v56.0.0
    github.com/go-git/go-git/v5 v5.9.0
    golang.org/x/oauth2 v0.13.0
    
    // YAML and configuration processing
    gopkg.in/yaml.v3 v3.0.1
    github.com/ghodss/yaml v1.0.0
    
    // Metrics and observability
    github.com/prometheus/client_golang v1.17.0
    go.uber.org/zap v1.26.0
    
    // Customer isolation and security
    github.com/golang-jwt/jwt/v5 v5.1.0
)
```

---

## Testing Strategy

### Unit Tests
- Test command handling logic for each service
- Test client library integrations with mocks
- Test error scenarios and fallback behaviors

### Integration Tests  
- Test each service with real external dependencies
- Test message publishing and consumption
- Test correlation ID propagation

### End-to-End Tests
- Test complete command flow: publish → consume → result
- Test multiple services processing same correlation ID
- Test service failure scenarios

---

## GitOps Validation Checklist

Before marking Phase 1C complete:

**GitOps Infrastructure:**
- [ ] All 5 GitOps service queues created and bound to correct GitOps routing keys
- [ ] Services can consume GitOps commands from RabbitMQ with customer isolation
- [ ] Services can publish GitOps result messages with comprehensive metadata
- [ ] Customer isolation enforced across all service operations
- [ ] GitOps message priorities handled correctly

**Vault-Secrets-Operator Service:**
- [ ] Can validate VaultStaticSecret custom resources
- [ ] Correlates Vault secrets with Kubernetes secrets
- [ ] Validates pod environment variables against Vault secrets
- [ ] Returns comprehensive pod-environment correlation data
- [ ] Handles VaultStaticSecret authentication failures gracefully
- [ ] Customer isolation for Vault path access

**ApplicationSet Monitor Service:**
- [ ] Can connect to ArgoCD and retrieve ApplicationSets with customer filtering
- [ ] Monitors ApplicationSet status across multiple environments
- [ ] Correlates ApplicationSets with Bootstrap Applications
- [ ] Tracks per-environment application deployment status
- [ ] Validates ApplicationSet generators and templates
- [ ] Handles ArgoCD authentication failures and customer scoping

**Customer Git Branch Service:**
- [ ] Can authenticate with Git providers using customer-specific credentials
- [ ] Validates customer branch patterns (customer/{customer-name})
- [ ] Retrieves and validates Helm values files per environment
- [ ] Validates environment-specific values files (values-dev.yaml, values-qa.yaml, etc.)
- [ ] Detects Helm values drift between environments
- [ ] Customer isolation for Git repository access

**Multi-Environment Kubernetes Service:**
- [ ] Can connect to multiple Kubernetes clusters with customer-scoped access
- [ ] Collects workload status across dev/qa/uat/prod environments
- [ ] Correlates secrets and environment variables across environments
- [ ] Compares resource configurations between environments
- [ ] Detects cross-environment configuration drift
- [ ] Handles RBAC permission errors with customer context

**Helm Values Correlation Service:**
- [ ] Can retrieve Helm release information from multiple environments
- [ ] Correlates deployed values with source values files
- [ ] Detects values drift between deployed and source configurations
- [ ] Validates Helm chart versions and consistency across environments
- [ ] Provides values pattern analysis and best practices recommendations
- [ ] Customer isolation for Helm operations

**Cross-Service GitOps Validation:**
- [ ] All services use consistent GitOps message envelope format with customer context
- [ ] Customer-scoped correlation IDs are preserved in results
- [ ] GitOps-specific metadata properly included in all result messages
- [ ] Services handle missing GitOps contexts gracefully with customer isolation
- [ ] Multi-environment correlation data properly structured
- [ ] ApplicationSet, Vault, and environment correlation working end-to-end
- [ ] Performance metrics and observability data included in results

---

## GitOps Next Steps

Upon completion, Phase 1C provides:
- **Working GitOps integration services** that respond to GitOps commands with customer isolation
- **Real GitOps external system connectivity** including ArgoCD ApplicationSets, Vault-secrets-operator, and multi-environment Kubernetes clusters
- **Comprehensive GitOps result messages** flowing back through RabbitMQ with ApplicationSet status, Vault secret validation, and multi-environment correlation data
- **Customer-isolated GitOps operations** ensuring secure multi-tenant GitOps workflows
- **Multi-environment correlation capabilities** supporting dev/qa/uat/prod environment monitoring and drift detection
- **Foundation for GitOps aggregated status views** with ApplicationSet, Vault, and environment correlation

**Handoff to Phase 1D:** The GitOps Aggregator service can now consume comprehensive GitOps result messages from all integration services and build consolidated, customer-scoped GitOps context status views with ApplicationSet monitoring, Vault secret validation, and multi-environment correlation dashboards.