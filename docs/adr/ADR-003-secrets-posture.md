# ADR-003: Secrets Posture

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team, Security Team  
**Phase:** 1A - Core Foundation  

---

## Context

Platformctl must handle sensitive credentials for multiple external systems:
- HashiCorp Vault authentication tokens
- ArgoCD API tokens
- New Relic API keys  
- GitHub personal access tokens or SSH keys
- Kubernetes service account tokens

These secrets are needed by integration services to perform operations, but storing and handling them securely presents significant challenges around compliance, blast radius, and operational security.

### Problem Statement

Common approaches to secret management in systems like Platformctl create security and compliance risks:

1. **Inline secrets in configuration:** Secrets stored directly in context specifications
   - High blast radius (secrets visible in logs, databases, APIs)
   - Difficult to rotate without updating all configurations
   - Poor audit trail for secret access

2. **Centralized secret storage:** System stores all secrets in its own database
   - Single point of compromise
   - Complex key management and encryption requirements
   - Difficult compliance certification (SOX, PCI, SOC2)

3. **Long-lived tokens:** Services cache secrets indefinitely
   - Compromised tokens remain valid until manual rotation
   - Difficult to implement principle of least privilege
   - Limited audit visibility into secret usage

### Requirements

- **Zero secrets in configurations:** Context specs must not contain secret values
- **Minimal secret exposure:** Limit duration and scope of secret access
- **Audit trail:** Full visibility into when and how secrets are accessed
- **Compliance-friendly:** Support SOC2 and PCI compliance requirements
- **Operational simplicity:** Don't create excessive operational burden
- **Rotation-friendly:** Support automated secret rotation workflows

### Considered Alternatives

#### Alternative 1: Inline secrets in context specifications
**Description:** Store secret values directly in context YAML/JSON.

**Pros:**
- Simple to implement and understand
- No additional secret infrastructure required
- Self-contained context definitions

**Cons:**
- Secrets visible in database, logs, API responses
- Massive blast radius - secrets everywhere
- Impossible to rotate without updating all contexts
- Fails compliance requirements
- No audit trail

#### Alternative 2: Encrypted database storage
**Description:** Store secrets encrypted in Platformctl database with system key.

**Pros:**
- Secrets not visible in plaintext
- Centralized secret management
- Can implement access controls

**Cons:**
- Platformctl becomes high-value target
- Key management complexity
- Still need to decrypt for use (exposure risk)
- Difficult to achieve compliance certification
- Single point of failure

#### Alternative 3: External secret management integration
**Description:** Integrate with cloud secret managers (AWS Secrets Manager, Azure Key Vault).

**Pros:**
- Leverage managed secret infrastructure
- Built-in rotation and compliance features
- Professional secret management practices

**Cons:**
- Cloud vendor lock-in
- Additional cost and complexity
- Latency for secret retrieval
- Not suitable for hybrid/on-premise deployments

---

## Decision

We will implement a **secrets-by-reference** approach where Platformctl stores only references to secrets managed by HashiCorp Vault, never the secret values themselves.

### Architecture Principles

#### 1. Reference-Only Storage
Context specifications contain only references to secrets:
```yaml
argocd:
  auth:
    tokenRef:
      vaultSecretLogicalName: "argocd"  # Reference to secret definition
      key: "token"                      # Specific key within secret
```

#### 2. Vault as Single Source of Truth
- All secrets stored and managed in HashiCorp Vault
- Platformctl services authenticate to Vault using short-lived tokens
- Secrets fetched at runtime when needed, not cached

#### 3. Just-In-Time Secret Access
```go
// Integration services fetch secrets only when performing operations
func (s *ArgoCDService) handleCommand(cmd Command) error {
    // Fetch secret at operation time
    token, err := s.vaultClient.GetSecret(cmd.Context.ArgoCD.Auth.TokenRef)
    if err != nil {
        return err
    }
    
    // Use token immediately
    client := argocd.NewClient(token)
    result, err := client.GetApplications()
    
    // Token goes out of scope, not stored
    return s.publishResult(result, err)
}
```

#### 4. Vault Authentication Strategy
- **Kubernetes Auth Method** for services running in Kubernetes
- **Service Account Tokens** rotated automatically by Kubernetes
- **Short TTL** on Vault tokens (1 hour max)

---

## Rationale

### Why Vault-Centric?
- **Industry standard:** Vault is purpose-built for secret management
- **Rich auth methods:** Supports Kubernetes, cloud, and traditional auth
- **Audit logging:** Built-in audit trail for all secret access
- **Rotation support:** Native support for secret rotation workflows
- **Compliance ready:** SOC2 and other certifications available

### Why References Not Values?
- **Blast radius reduction:** Compromise of Platformctl doesn't expose secrets
- **Audit visibility:** All secret access logged in Vault
- **Rotation friendly:** Rotate secrets in Vault, no Platformctl changes needed
- **Compliance alignment:** Meets requirements for secret handling

### Why Just-In-Time Access?
- **Minimizes exposure window:** Secrets only in memory during operations
- **Principle of least privilege:** Services only access secrets when needed
- **Reduces cache poisoning risk:** No long-lived secret caches to compromise
- **Simplifies rotation:** No need to invalidate caches on rotation

---

## Consequences

### Positive

1. **Security**
   - Minimal blast radius - Platformctl compromise doesn't expose secrets
   - Comprehensive audit trail in Vault for all secret access
   - Short-lived tokens limit exposure window
   - Reference-only configuration prevents accidental secret leakage

2. **Compliance**
   - Clear separation of secret storage (Vault) and business logic (Platformctl)
   - Professional secret management practices
   - Audit logs suitable for compliance requirements
   - Supports secret rotation requirements

3. **Operational Benefits**
   - Secret rotation handled entirely in Vault
   - No secret synchronization between systems
   - Clear operational model - Vault owns secrets, Platformctl owns references

### Negative

1. **Runtime Dependencies**
   - Integration services depend on Vault availability for operations
   - Network latency added to integration operations
   - Complexity in handling Vault authentication failures

2. **Development Complexity**
   - Every secret access requires Vault integration code
   - Error handling for secret retrieval failures
   - Testing requires Vault infrastructure or extensive mocking

3. **Operational Overhead**
   - Vault infrastructure must be highly available
   - Vault auth policies must be maintained
   - Secret reference validation needs to be implemented

### Technical Debt

1. **Error Handling**
   - Graceful degradation when Vault is unavailable
   - Retry policies for transient Vault failures
   - Clear error messages for secret access failures

2. **Performance Impact**
   - Latency from Vault secret retrieval
   - Potential need for caching with careful TTL management
   - Connection pooling for Vault client connections

3. **Secret Reference Validation**
   - Validate secret references exist during context creation
   - Handle secret reference changes (path restructuring)
   - Migrate contexts when Vault organization changes

---

## Implementation Guidelines

### Phase 1A Implementation

#### Context Validation
```go
func validateSecretReferences(ctx *Context) error {
    for _, secret := range ctx.Spec.Vault.Secrets {
        // Validate reference format
        if secret.LogicalName == "" || secret.Path == "" {
            return fmt.Errorf("invalid secret reference")
        }
        
        // Validate no inline secret material
        if containsSecretPattern(secret) {
            return fmt.Errorf("inline secrets forbidden")
        }
    }
    return nil
}

var forbiddenSecretPatterns = []string{
    "password", "token", "key", "secret", "auth",
}
```

#### Vault Client Pattern
```go
type VaultClient interface {
    GetSecret(logicalName, key string) (string, error)
    ValidateSecretExists(logicalName string, keys []string) error
}

func (vc *HashiCorpVaultClient) GetSecret(logicalName, key string) (string, error) {
    // Authenticate with short-lived token
    if err := vc.ensureAuthenticated(); err != nil {
        return "", err
    }
    
    // Resolve logical name to Vault path
    path := vc.resolveSecretPath(logicalName)
    
    // Fetch secret
    secret, err := vc.client.Logical().Read(path)
    if err != nil {
        return "", fmt.Errorf("failed to read secret: %w", err)
    }
    
    // Extract specific key
    value, ok := secret.Data[key].(string)
    if !ok {
        return "", fmt.Errorf("key %s not found", key)
    }
    
    return value, nil
}
```

### Vault Authentication Setup
```yaml
# Vault Kubernetes auth configuration
path "auth/kubernetes/config" {
  capabilities = ["create", "read", "update"]
}

# Role for Platformctl services
path "auth/kubernetes/role/platformctl-*" {
  capabilities = ["create", "read", "update"]
}

# Secret access policies
path "kv/data/platform/+/+/*" {
  capabilities = ["read"]
}
```

### Logging and Audit Controls
```go
func (vc *VaultClient) GetSecret(logicalName, key string) (string, error) {
    // Log secret access (without values)
    log.WithFields(logrus.Fields{
        "logical_name": logicalName,
        "key":         key,
        "service":     vc.serviceName,
    }).Info("accessing secret from vault")
    
    secret, err := vc.fetchSecret(logicalName, key)
    
    if err != nil {
        log.WithFields(logrus.Fields{
            "logical_name": logicalName,
            "error":       err.Error(),
        }).Error("failed to access secret")
    }
    
    return secret, err
}
```

---

## Security Considerations

### Secret Handling Guidelines
1. **Never log secret values** - only references and access patterns
2. **Clear secrets from memory** explicitly when possible
3. **Use secure string types** that zero memory on deallocation
4. **Validate secret format** before use to prevent injection attacks
5. **Implement secret rotation testing** to ensure system resilience

### Vault Configuration Security
```hcl
# Minimum Vault configuration for Platformctl
storage "raft" {
  path = "/vault/data"
}

listener "tcp" {
  address = "0.0.0.0:8200"
  tls_cert_file = "/vault/tls/vault.crt"
  tls_key_file = "/vault/tls/vault.key"
}

# Enable audit logging
audit "file" {
  file_path = "/vault/logs/audit.log"
}

# Enable Kubernetes auth
auth "kubernetes" {
  path = "kubernetes"
}
```

### Network Security
- **TLS required** for all Vault communications
- **Network policies** to restrict Vault access to authorized services
- **Firewall rules** limiting Vault port access
- **VPN/private networks** for Vault infrastructure

---

## Monitoring and Alerting

### Key Metrics
- **Secret access rate** per service and logical name
- **Secret access latency** from Vault
- **Vault authentication failure rate**
- **Secret reference validation failures**
- **Token renewal success rate**

### Alert Conditions
- Secret access failure rate > 5%
- Vault authentication failures for any service
- Secret access latency > 2 seconds (p95)
- Token renewal failures
- Invalid secret references in context creation

---

## Evolution Path

### Phase 2 Enhancements
- **Secret caching** with careful TTL management for performance
- **Multiple Vault instances** for high availability
- **Secret rotation automation** integrated with context updates

### Phase 3 Advanced Features
- **Dynamic secrets** for short-lived credentials
- **Secret versioning** support for rollback scenarios
- **Cross-region secret replication** for disaster recovery

---

## References

- [HashiCorp Vault Documentation](https://www.vaultproject.io/docs)
- [Vault Kubernetes Auth Method](https://www.vaultproject.io/docs/auth/kubernetes)
- [OWASP Secrets Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)
- [NIST Guidelines for Managing Secrets](https://csrc.nist.gov/publications/detail/sp/800-57-part-1/rev-5/final)

---

## Related ADRs

- ADR-001: Event-driven integration workflows - Services need secrets to perform integrations
- ADR-004: Kubernetes integration via kubeconfig - Discusses credential management for K8s access
- ADR-008: Configuration management strategy - Defines how secret references are configured