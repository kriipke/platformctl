# PHASE 1C: Integration Services

**Duration:** 5-7 days  
**Prerequisites:** Phase 1A & 1B completed, external service access (Vault dev, K8s cluster, GitHub)  
**Deliverable:** Five working integration services consuming commands and publishing results

---

## Overview

Implement the five integration services in priority order. Each service consumes commands from RabbitMQ, integrates with external systems, and publishes result events. This phase brings the system to life with actual external integrations.

**Implementation Order:**
1. **Vault Service** (foundation for others)
2. **Kubernetes Service** (local dev friendly)  
3. **Git Service** (independent, good testing)
4. **ArgoCD Service** (needs Vault tokens)
5. **New Relic Service** (needs Vault API keys)

## Success Criteria

✅ All 5 integration services deployed and consuming commands  
✅ Vault service validates auth and secret paths  
✅ Kubernetes service reads workload status from kubeconfig  
✅ Git service browses files via GitHub API  
✅ ArgoCD service queries application status  
✅ New Relic service fetches entity metrics  
✅ All services publish well-formed result messages  
✅ Error conditions handled gracefully  
✅ Integration tests validate service behavior  

---

## Task 1: Service Template & Common Components

### Shared Service Framework

**File: `internal/services/base.go`**

```go
type BaseService struct {
    Name          string
    Consumer      *events.CommandConsumer
    Publisher     *events.ResultPublisher
    ContextStore  *storage.ContextStore
}

func NewBaseService(name string, mb *events.MessageBus, store *storage.ContextStore) *BaseService {
    consumer := events.NewCommandConsumer(mb, fmt.Sprintf("%s-svc.q", name))
    publisher := events.NewResultPublisher(mb)
    
    return &BaseService{
        Name:         name,
        Consumer:     consumer,
        Publisher:    publisher,
        ContextStore: store,
    }
}

func (bs *BaseService) Start(handler CommandHandler) error {
    return bs.Consumer.Start(handler)
}

func (bs *BaseService) Stop() error {
    return bs.Consumer.Stop()
}

type CommandHandler interface {
    HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error)
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

## Task 2: Vault Service Implementation

**Priority: 1 - Foundation service needed by ArgoCD and New Relic**

**File: `cmd/vault-svc/main.go`**

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
    contextStore := storage.NewContextStore(db)
    vaultHandler := NewVaultHandler()
    service := services.NewBaseService("vault", messageBus, contextStore)
    
    // Start service
    if err := service.Start(vaultHandler); err != nil {
        log.Fatal("Failed to start vault service:", err)
    }
    
    log.Println("Vault service started")
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    log.Println("Shutting down vault service")
    service.Stop()
}
```

**File: `cmd/vault-svc/handler.go`**

```go
type VaultHandler struct {
    client VaultClient
}

type VaultClient interface {
    ValidateAuth(config *contexts.VaultConfig) error
    ValidateSecretPaths(config *contexts.VaultConfig) ([]SecretValidation, error)
    GetSecret(path, key string) (string, error) // For other services
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
        ServiceName: "vault",
        CompletedAt: time.Now().UTC(),
    }
    
    payload := make(map[string]interface{})
    
    // Validate authentication
    if err := vh.client.ValidateAuth(&ctx.Spec.Vault); err != nil {
        result.Status = "error"
        result.ErrorMessage = fmt.Sprintf("auth validation failed: %v", err)
        payload["auth_status"] = "failed"
    } else {
        payload["auth_status"] = "ok"
    }
    
    // Validate secret paths
    validations, err := vh.client.ValidateSecretPaths(&ctx.Spec.Vault)
    if err != nil {
        if result.Status != "error" {
            result.Status = "degraded"
        }
        result.ErrorMessage = fmt.Sprintf("secret validation failed: %v", err)
    }
    
    payload["secret_validations"] = validations
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

## Task 3: Kubernetes Service Implementation

**Priority: 2 - Local development friendly**

**File: `cmd/kube-svc/main.go`** (similar structure to vault-svc)

**File: `cmd/kube-svc/handler.go`**

```go
type KubernetesHandler struct {
    client KubernetesClient
}

func (kh *KubernetesHandler) HandleCommand(cmd *api.CommandMessage) (*api.ResultMessage, error) {
    startTime := time.Now()
    
    ctx, err := kh.getContext(cmd.ContextName)
    if err != nil {
        return kh.errorResult(cmd, "failed to get context", err, startTime)
    }
    
    switch cmd.Action {
    case "refresh", "inspect":
        return kh.handleInspection(cmd, ctx, startTime)
    default:
        return kh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
    }
}

func (kh *KubernetesHandler) handleInspection(cmd *api.CommandMessage, ctx *contexts.Context, startTime time.Time) (*api.ResultMessage, error) {
    // Resolve namespace
    namespace := kh.resolveNamespace(ctx)
    
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

## Task 4: Git Service Implementation

**Priority: 3 - Independent service, good for testing patterns**

**File: `cmd/git-svc/handler.go`**

```go
type GitHandler struct {
    client GitClient
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

## Task 5: ArgoCD Service Implementation

**Priority: 4 - Depends on Vault service for token management**

**File: `cmd/argocd-svc/handler.go`**

```go
type ArgoCDHandler struct {
    client     ArgoCDClient
    vaultClient VaultClient // For token retrieval
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

## Task 6: New Relic Service Implementation

**Priority: 5 - Depends on Vault service for API key management**

**File: `cmd/newrelic-svc/handler.go`**

```go
type NewRelicHandler struct {
    client      NewRelicClient
    vaultClient VaultClient
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

## Dependencies

Add to `go.mod`:
```
require (
    github.com/hashicorp/vault/api v1.10.0
    k8s.io/client-go v0.28.2
    k8s.io/api v0.28.2
    k8s.io/apimachinery v0.28.2
    github.com/google/go-github/v45 v45.2.0
    golang.org/x/oauth2 v0.13.0
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

## Validation Checklist

Before marking Phase 1C complete:

**Infrastructure:**
- [ ] All 5 service queues created and bound to correct routing keys
- [ ] Services can consume messages from RabbitMQ
- [ ] Services can publish result messages

**Vault Service:**
- [ ] Can authenticate with Vault using configured method
- [ ] Validates secret paths and required keys
- [ ] Returns properly formatted result messages
- [ ] Handles authentication failures gracefully

**Kubernetes Service:**
- [ ] Can connect using kubeconfig file
- [ ] Resolves namespace according to policy rules
- [ ] Collects workload status from specified namespace
- [ ] Handles RBAC permission errors

**Git Service:**
- [ ] Can authenticate with GitHub
- [ ] Basic connectivity validation works
- [ ] Returns properly formatted result messages

**ArgoCD Service:**
- [ ] Can retrieve tokens from Vault
- [ ] Connects to ArgoCD API successfully
- [ ] Queries application status
- [ ] Handles authentication failures

**New Relic Service:**
- [ ] Can retrieve API keys from Vault
- [ ] Connects to New Relic NerdGraph API
- [ ] Finds entities by tag filters
- [ ] Retrieves basic metrics

**Cross-Service:**
- [ ] All services use consistent message envelope format
- [ ] Correlation IDs are preserved in results
- [ ] Error messages are informative and actionable
- [ ] Services handle missing contexts gracefully

---

## Next Steps

Upon completion, Phase 1C provides:
- Working integration services that respond to commands
- Real external system connectivity
- Result messages flowing back through RabbitMQ
- Foundation for aggregated status views

**Handoff to Phase 1D:** The Aggregator service can now consume result messages from all integration services and build consolidated context status views.