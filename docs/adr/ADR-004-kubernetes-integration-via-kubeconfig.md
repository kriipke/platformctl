# ADR-004: Kubernetes Integration via kubeconfig current-context

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 1C-2 - Kubernetes Integration Service  

---

## Context

Platformctl needs to collect Kubernetes workload status and operational data for each context's environment. This requires authenticating to Kubernetes clusters and scoping access appropriately to match the context's namespace and permissions.

### Problem Statement

Kubernetes access patterns vary significantly across deployment scenarios:

1. **Developer workflows:** Local development using `kubectl` with kubeconfig files
2. **CI/CD pipelines:** Service accounts with specific cluster and namespace access
3. **Hosted services:** Multi-tenant access across different clusters and namespaces
4. **Security requirements:** Principle of least privilege and namespace isolation

### Requirements

- **Developer-friendly:** Match existing `kubectl` workflows and patterns
- **Namespace isolation:** Restrict access to appropriate namespace per context
- **Credential flexibility:** Support both kubeconfig files and service accounts
- **Security-first:** Enforce least privilege and prevent privilege escalation
- **Multi-cluster:** Support contexts spanning different Kubernetes clusters
- **Audit trail:** Log all Kubernetes API access for security monitoring

### Considered Alternatives

#### Alternative 1: Service account per context
**Description:** Create dedicated Kubernetes service accounts for each context with namespace-scoped RBACs.

**Pros:**
- Granular permission control per context
- Native Kubernetes RBAC enforcement
- Clear audit trail per context
- Supports multi-cluster deployments

**Cons:**
- High operational overhead (service account per context)
- Complex credential distribution and rotation
- Doesn't match developer kubectl workflows
- Difficult to manage at scale

#### Alternative 2: Cluster admin credentials
**Description:** Use cluster admin credentials to access all resources across namespaces.

**Pros:**
- Simple credential management
- Full access to all cluster resources
- Easy to implement initially

**Cons:**
- Massive security risk and blast radius
- Violates principle of least privilege
- No namespace isolation
- Fails security compliance requirements
- Single credential compromise affects entire cluster

#### Alternative 3: Cloud-native Kubernetes auth (Workload Identity, IRSA)
**Description:** Use cloud provider Kubernetes authentication mechanisms.

**Pros:**
- Leverages cloud security best practices
- Automatic credential rotation
- Fine-grained IAM integration
- No long-lived credentials

**Cons:**
- Cloud vendor lock-in
- Not available for on-premises deployments
- Additional complexity for hybrid environments
- Requires cloud-specific implementation

---

## Decision

We will use **kubeconfig files with current-context** as the primary Kubernetes authentication mechanism, with namespace enforcement through policy and code-level guards.

### Architecture Overview

#### kubeconfig Context Resolution
Kubernetes documentation defines a kubeconfig context as the combination of:
- **cluster:** Kubernetes API server endpoint and certificate authority
- **user:** Authentication information (certificates, tokens, etc.)
- **namespace:** Default namespace for operations

#### Namespace Resolution Priority
```
1. context.spec.kubernetes.namespaceOverride (if policy allows)
2. namespace from kubeconfig current-context
3. "default" namespace (fallback)
```

#### Access Patterns

**Local Development Mode:**
```yaml
# Context specification
kubernetes:
  kubeconfig:
    path: "~/.kube/config"  # User's local kubeconfig
    contextOverride: ""      # Use current-context
  namespaceOverride: ""      # Use namespace from context
```

**Server/Hosted Mode:**
```yaml
# Context specification  
kubernetes:
  kubeconfig:
    path: "/var/secrets/kubeconfig"  # Mounted kubeconfig
    contextOverride: "production"    # Explicit context
  namespaceOverride: "app1-prod"     # Explicit namespace
```

---

## Rationale

### Why kubeconfig + current-context?
- **Developer alignment:** Matches exactly how developers use `kubectl`
- **Operational familiarity:** Uses standard Kubernetes credential patterns
- **Flexibility:** Supports both interactive and automated use cases
- **Context switching:** Developers can test different clusters/namespaces easily

### Why Namespace Enforcement?
- **Security boundary:** Prevents cross-namespace data access
- **Principle of least privilege:** Limits blast radius of compromised credentials
- **Multi-tenancy support:** Enables safe sharing of cluster access
- **Compliance requirement:** Supports data isolation requirements

### Why Policy-based Override?
- **Security control:** Central policy controls which contexts can override namespaces
- **Flexibility:** Allow specific contexts to access multiple namespaces when appropriate
- **Audit trail:** Policy changes are tracked and reviewed

---

## Consequences

### Positive

1. **Developer Experience**
   - Familiar kubectl-style authentication and context switching
   - Local development works seamlessly with existing tooling
   - Easy testing across different clusters and namespaces

2. **Security**
   - Namespace isolation enforced at multiple levels
   - Leverages existing Kubernetes RBAC systems
   - No need to distribute service account tokens

3. **Operational Simplicity**
   - Uses standard kubeconfig management patterns
   - Compatible with existing credential rotation processes
   - Familiar troubleshooting and debugging approaches

4. **Flexibility**
   - Supports both local development and hosted deployment
   - Works with any Kubernetes authentication method supported by kubeconfig
   - Easy to extend for additional clusters

### Negative

1. **Credential Management**
   - kubeconfig files contain sensitive credentials
   - Need secure storage and transmission for hosted deployments
   - Credential rotation requires kubeconfig updates

2. **Security Risks**
   - kubeconfig compromise gives cluster access
   - Namespace enforcement depends on code-level guards
   - Potential for privilege escalation if guards fail

3. **Deployment Complexity**
   - Different patterns for local vs hosted deployments
   - kubeconfig distribution and mounting complexity
   - Context and namespace resolution logic

### Technical Debt

1. **kubeconfig Security**
   - Secure storage of kubeconfig files in hosted environments
   - Encryption at rest and in transit
   - Access logging and audit trail

2. **Namespace Guard Implementation**
   - Comprehensive testing of namespace isolation
   - Defense-in-depth with RBAC and code-level enforcement
   - Validation of all Kubernetes API calls

3. **Error Handling**
   - Clear error messages for authentication failures
   - Graceful handling of network partitions
   - Fallback behavior when clusters are unavailable

---

## Implementation Guidelines

### Phase 1C-2 Implementation

#### Kubernetes Client Initialization
```go
func NewKubernetesClient(config *contexts.KubernetesConfig) (*KubernetesClient, error) {
    var kubeConfig *rest.Config
    var err error
    
    if config.Kubeconfig.Path != "" {
        // Load from kubeconfig file
        kubeConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig.Path)
    } else {
        // In-cluster configuration
        kubeConfig, err = rest.InClusterConfig()
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
    }
    
    // Override context if specified
    if config.Kubeconfig.ContextOverride != "" {
        overrideConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
            &clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Kubeconfig.Path},
            &clientcmd.ConfigOverrides{CurrentContext: config.Kubeconfig.ContextOverride},
        ).ClientConfig()
        
        if err != nil {
            return nil, fmt.Errorf("failed to override context: %w", err)
        }
        kubeConfig = overrideConfig
    }
    
    clientset, err := kubernetes.NewForConfig(kubeConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
    }
    
    return &KubernetesClient{
        clientset: clientset,
        config:    kubeConfig,
    }, nil
}
```

#### Namespace Resolution
```go
func (kc *KubernetesClient) resolveNamespace(contextConfig *contexts.KubernetesConfig, policy *contexts.PolicyConfig) string {
    // Priority 1: Override if allowed by policy
    if contextConfig.NamespaceOverride != "" {
        if policy.Kubernetes.AllowNamespaceOverride {
            return contextConfig.NamespaceOverride
        }
        log.Warn("namespace override denied by policy")
    }
    
    // Priority 2: Namespace from kubeconfig context
    if ns := kc.getCurrentContextNamespace(); ns != "" {
        return ns
    }
    
    // Priority 3: Default fallback
    return "default"
}

func (kc *KubernetesClient) getCurrentContextNamespace() string {
    config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        clientcmd.NewDefaultClientConfigLoadingRules(),
        &clientcmd.ConfigOverrides{},
    )
    
    namespace, _, err := config.Namespace()
    if err != nil {
        log.WithError(err).Warn("failed to get namespace from kubeconfig")
        return ""
    }
    
    return namespace
}
```

#### Namespace Guard Implementation
```go
func (kc *KubernetesClient) enforceNamespaceGuard(namespace string) error {
    if kc.allowedNamespace != "" && kc.allowedNamespace != namespace {
        return fmt.Errorf("access denied: namespace %s not allowed, restricted to %s", 
                          namespace, kc.allowedNamespace)
    }
    return nil
}

func (kc *KubernetesClient) GetPods(namespace string) (*v1.PodList, error) {
    // Enforce namespace guard
    if err := kc.enforceNamespaceGuard(namespace); err != nil {
        return nil, err
    }
    
    // Log access for audit
    log.WithFields(logrus.Fields{
        "namespace": namespace,
        "resource":  "pods",
        "action":    "list",
    }).Info("kubernetes api access")
    
    return kc.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
}
```

### RBAC Configuration
```yaml
# Namespace-scoped role for Platformctl service
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: app1-prod
  name: platformctl-reader
rules:
- apiGroups: [""]
  resources: ["pods", "services", "endpoints", "events", "configmaps"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: platformctl-reader
  namespace: app1-prod
subjects:
- kind: ServiceAccount
  name: platformctl-kube
  namespace: platformctl
roleRef:
  kind: Role
  name: platformctl-reader
  apiGroup: rbac.authorization.k8s.io
```

### Policy Configuration
```yaml
# Context policy for namespace access control
policy:
  kubernetes:
    enforceNamespaceFromKubeconfig: true  # Honor kubeconfig namespace
    allowNamespaceOverride: false         # Deny context-level override
    allowedNamespaces: ["app1-prod"]      # Explicit allowlist (optional)
```

---

## Security Considerations

### kubeconfig Security Best Practices
1. **File permissions:** Restrict kubeconfig files to 600 (owner read/write only)
2. **Encryption at rest:** Encrypt kubeconfig files in hosted environments
3. **Secure transmission:** Use TLS for kubeconfig distribution
4. **Access logging:** Log all kubeconfig access and usage
5. **Rotation schedule:** Regular rotation of cluster credentials

### Namespace Isolation Enforcement
```go
// Defense-in-depth namespace validation
func validateNamespaceAccess(requestedNamespace, allowedNamespace string) error {
    if allowedNamespace != "" && requestedNamespace != allowedNamespace {
        // Log security violation
        log.WithFields(logrus.Fields{
            "requested_namespace": requestedNamespace,
            "allowed_namespace":   allowedNamespace,
            "security_event":     "namespace_access_violation",
        }).Error("unauthorized namespace access attempt")
        
        return fmt.Errorf("access denied to namespace %s", requestedNamespace)
    }
    return nil
}
```

### RBAC Integration
```yaml
# Principle of least privilege RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: app1-prod
  name: platformctl-minimal
rules:
# Read-only access to essential resources
- apiGroups: [""]
  resources: ["pods", "services", "events"]
  verbs: ["get", "list"]
# No secrets access (use Vault instead)  
# No write operations
# No cluster-scoped resources
```

---

## Monitoring and Alerting

### Key Metrics
- **API call success rate** by namespace and resource type
- **Authentication failure rate** for kubeconfig access
- **Namespace guard violations** and attempted unauthorized access
- **API call latency** to Kubernetes clusters
- **RBAC denial rate** by operation type

### Alert Conditions
- Namespace guard violations (immediate alert)
- Authentication failures > 5% over 5 minutes
- API call failure rate > 10% over 5 minutes
- Unauthorized resource access attempts
- Cluster connectivity issues

### Security Monitoring
```go
// Security event logging
func logSecurityEvent(event string, details map[string]interface{}) {
    log.WithFields(logrus.Fields{
        "event_type":     "security",
        "security_event": event,
        "timestamp":     time.Now().UTC(),
        "details":       details,
    }).Warn("kubernetes security event")
    
    // Send to SIEM/security monitoring
    securityEventBus.Publish(SecurityEvent{
        Type:      event,
        Service:   "kubernetes-service",
        Details:   details,
        Timestamp: time.Now().UTC(),
    })
}
```

---

## Evolution Path

### Phase 2 Enhancements
- **Multi-cluster support** with cluster routing based on context
- **Workload Identity integration** for cloud-native authentication
- **Fine-grained RBAC** with resource-level permissions

### Phase 3 Advanced Features
- **Dynamic service accounts** created per context
- **Certificate-based authentication** with automatic rotation
- **Cross-cluster resource aggregation** for distributed applications

---

## References

- [Kubernetes Documentation: Organizing Cluster Access Using kubeconfig Files](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
- [Kubernetes API Reference: kubeconfig (v1)](https://kubernetes.io/docs/reference/config-api/kubeconfig.v1/)
- [Kubernetes RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [client-go Authentication](https://kubernetes.io/docs/reference/using-api/client-libraries/#authentication)

---

## Related ADRs

- ADR-003: Secrets posture - Defines how kubeconfig credentials are managed securely
- ADR-001: Event-driven integration workflows - Kubernetes service participates in event-driven architecture
- ADR-008: Configuration management strategy - Defines how kubeconfig paths are configured