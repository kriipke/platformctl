# Context Data Model

**Version:** 1.0  
**Date:** 2026-01-21  
**Phase:** 1A - Core Foundation  

---

## Overview

The Context model is the central data structure in ContextOps, representing an application-environment pairing with all necessary configuration for integrating with external systems. This document defines the complete Context schema with validation rules and implementation guidance.

---

## Context Schema

### Root Structure

```go
type Context struct {
    APIVersion string          `json:"apiVersion" yaml:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string          `json:"kind" yaml:"kind" validate:"required,eq=Context"`
    Metadata   ContextMetadata `json:"metadata" yaml:"metadata" validate:"required"`
    Spec       ContextSpec     `json:"spec" yaml:"spec" validate:"required"`
}
```

### Metadata

```go
type ContextMetadata struct {
    Name        string            `json:"name" yaml:"name" validate:"required,contextname"`
    Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
    CreatedAt   *time.Time        `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
    UpdatedAt   *time.Time        `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}
```

**Validation Rules:**
- `name`: Must match regex `^[a-z0-9-]+$` (Kubernetes-friendly naming)
- `name`: Length 1-63 characters
- `labels`: Optional key-value pairs for organization and filtering
- `annotations`: Optional metadata for tooling integration

### Context Specification

```go
type ContextSpec struct {
    App        AppConfig        `json:"app" yaml:"app" validate:"required"`
    Policy     PolicyConfig     `json:"policy" yaml:"policy" validate:"required"`
    Vault      VaultConfig      `json:"vault" yaml:"vault" validate:"required"`
    ArgoCD     ArgoCDConfig     `json:"argocd,omitempty" yaml:"argocd,omitempty"`
    NewRelic   NewRelicConfig   `json:"newrelic,omitempty" yaml:"newrelic,omitempty"`
    Kubernetes KubernetesConfig `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
    Git        GitConfig        `json:"git,omitempty" yaml:"git,omitempty"`
}
```

---

## Application Configuration

```go
type AppConfig struct {
    Name        string `json:"name" yaml:"name" validate:"required"`
    Environment string `json:"environment" yaml:"environment" validate:"required,oneof=dev staging prod"`
    Version     string `json:"version,omitempty" yaml:"version,omitempty"`
    Owner       string `json:"owner,omitempty" yaml:"owner,omitempty"`
    Repository  string `json:"repository,omitempty" yaml:"repository,omitempty"`
}
```

**Validation Rules:**
- `name`: Application identifier, alphanumeric with hyphens
- `environment`: Must be one of: `dev`, `staging`, `prod`
- `version`: Optional semantic version
- `owner`: Optional team or individual responsible
- `repository`: Optional Git repository URL

---

## Policy Configuration

```go
type PolicyConfig struct {
    AllowedActions        []string          `json:"allowedActions" yaml:"allowedActions" validate:"required,min=1"`
    RequireMfaForActions  []string          `json:"requireMfaForActions,omitempty" yaml:"requireMfaForActions,omitempty"`
    Kubernetes           KubernetesPolicyConfig `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
    RateLimit            RateLimitConfig   `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
}

type KubernetesPolicyConfig struct {
    EnforceNamespaceFromKubeconfig bool     `json:"enforceNamespaceFromKubeconfig" yaml:"enforceNamespaceFromKubeconfig"`
    AllowNamespaceOverride         bool     `json:"allowNamespaceOverride" yaml:"allowNamespaceOverride"`
    AllowedNamespaces             []string `json:"allowedNamespaces,omitempty" yaml:"allowedNamespaces,omitempty"`
}

type RateLimitConfig struct {
    RequestsPerMinute int `json:"requestsPerMinute,omitempty" yaml:"requestsPerMinute,omitempty" validate:"min=1,max=1000"`
    BurstSize         int `json:"burstSize,omitempty" yaml:"burstSize,omitempty" validate:"min=1,max=100"`
}
```

**Valid Actions:**
- `refresh`: Update all integration data
- `validate`: Validate configuration and connectivity
- `inspect`: Deep inspection of infrastructure state
- `sync`: Trigger ArgoCD synchronization (restricted)

---

## Vault Configuration

```go
type VaultConfig struct {
    Address   string            `json:"address" yaml:"address" validate:"required,url"`
    Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
    Auth      VaultAuthConfig   `json:"auth" yaml:"auth" validate:"required"`
    Secrets   []VaultSecret     `json:"secrets" yaml:"secrets" validate:"required,min=1,dive"`
}

type VaultAuthConfig struct {
    Method     string                `json:"method" yaml:"method" validate:"required,oneof=kubernetes token aws gcp azure"`
    Kubernetes *VaultKubernetesAuth  `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
    Token      *VaultTokenAuth       `json:"token,omitempty" yaml:"token,omitempty"`
    AWS        *VaultAWSAuth         `json:"aws,omitempty" yaml:"aws,omitempty"`
}

type VaultKubernetesAuth struct {
    Role           string `json:"role" yaml:"role" validate:"required"`
    ServiceAccount string `json:"serviceAccount,omitempty" yaml:"serviceAccount,omitempty"`
    TokenPath      string `json:"tokenPath,omitempty" yaml:"tokenPath,omitempty"`
}

type VaultTokenAuth struct {
    TokenRef SecretReference `json:"tokenRef" yaml:"tokenRef" validate:"required"`
}

type VaultSecret struct {
    LogicalName  string   `json:"logicalName" yaml:"logicalName" validate:"required"`
    Path         string   `json:"path" yaml:"path" validate:"required"`
    RequiredKeys []string `json:"requiredKeys" yaml:"requiredKeys" validate:"required,min=1"`
    Optional     bool     `json:"optional,omitempty" yaml:"optional,omitempty"`
}

type SecretReference struct {
    VaultSecretLogicalName string `json:"vaultSecretLogicalName" yaml:"vaultSecretLogicalName" validate:"required"`
    Key                    string `json:"key" yaml:"key" validate:"required"`
}
```

**Security Constraints:**
- No inline secret values allowed anywhere in the Context
- All secrets referenced via `SecretReference` pointing to Vault paths
- Validation must reject any detected secret patterns

---

## ArgoCD Configuration

```go
type ArgoCDConfig struct {
    Address    string               `json:"address" yaml:"address" validate:"required,url"`
    Auth       ArgoCDAuthConfig     `json:"auth" yaml:"auth" validate:"required"`
    Selectors  ArgoCDSelectors      `json:"selectors" yaml:"selectors" validate:"required"`
    Operations ArgoCDOperations     `json:"operations,omitempty" yaml:"operations,omitempty"`
}

type ArgoCDAuthConfig struct {
    TokenRef SecretReference `json:"tokenRef" yaml:"tokenRef" validate:"required"`
}

type ArgoCDSelectors struct {
    Apps    []string `json:"apps,omitempty" yaml:"apps,omitempty"`
    Project string   `json:"project,omitempty" yaml:"project,omitempty"`
    Labels  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type ArgoCDOperations struct {
    Sync ArgoCDSyncConfig `json:"sync,omitempty" yaml:"sync,omitempty"`
}

type ArgoCDSyncConfig struct {
    Allowed      bool `json:"allowed" yaml:"allowed"`
    Prune        bool `json:"prune,omitempty" yaml:"prune,omitempty"`
    DryRunDefault bool `json:"dryRunDefault,omitempty" yaml:"dryRunDefault,omitempty"`
}
```

---

## New Relic Configuration

```go
type NewRelicConfig struct {
    AccountID      int64                    `json:"accountId" yaml:"accountId" validate:"required,min=1"`
    Region         string                   `json:"region" yaml:"region" validate:"required,oneof=US EU"`
    Auth           NewRelicAuthConfig       `json:"auth" yaml:"auth" validate:"required"`
    EntitySelector NewRelicEntitySelector   `json:"entitySelector" yaml:"entitySelector" validate:"required"`
    Metrics        []NewRelicMetricConfig   `json:"metrics,omitempty" yaml:"metrics,omitempty"`
    Alerts         NewRelicAlertsConfig     `json:"alerts,omitempty" yaml:"alerts,omitempty"`
}

type NewRelicAuthConfig struct {
    APIKeyRef SecretReference `json:"apiKeyRef" yaml:"apiKeyRef" validate:"required"`
}

type NewRelicEntitySelector struct {
    TagFilters []NewRelicTagFilter `json:"tagFilters" yaml:"tagFilters" validate:"required,min=1,dive"`
    Types      []string             `json:"types,omitempty" yaml:"types,omitempty"`
}

type NewRelicTagFilter struct {
    Key   string `json:"key" yaml:"key" validate:"required"`
    Value string `json:"value" yaml:"value" validate:"required"`
}

type NewRelicMetricConfig struct {
    Name   string `json:"name" yaml:"name" validate:"required"`
    Window string `json:"window" yaml:"window" validate:"required,duration"`
}

type NewRelicAlertsConfig struct {
    IncludeIncidents bool `json:"includeIncidents,omitempty" yaml:"includeIncidents,omitempty"`
}
```

---

## Kubernetes Configuration

```go
type KubernetesConfig struct {
    Kubeconfig        KubeconfigConfig `json:"kubeconfig,omitempty" yaml:"kubeconfig,omitempty"`
    NamespaceOverride string           `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
    Resources         []string         `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type KubeconfigConfig struct {
    Path            string `json:"path,omitempty" yaml:"path,omitempty"`
    ContextOverride string `json:"contextOverride,omitempty" yaml:"contextOverride,omitempty"`
}
```

**Default Resources Collected:**
- `pods`, `services`, `deployments`, `statefulsets`, `daemonsets`
- `ingresses`, `configmaps`, `secrets` (metadata only)
- `events`, `horizontalpodautoscalers`

---

## Git Configuration

```go
type GitConfig struct {
    Provider string          `json:"provider" yaml:"provider" validate:"required,oneof=github gitlab bitbucket"`
    Auth     GitAuthConfig   `json:"auth" yaml:"auth" validate:"required"`
    Browse   GitBrowseConfig `json:"browse,omitempty" yaml:"browse,omitempty"`
}

type GitAuthConfig struct {
    Method    string          `json:"method" yaml:"method" validate:"required,oneof=pat ssh github_app"`
    SecretRef SecretReference `json:"secretRef" yaml:"secretRef" validate:"required"`
}

type GitBrowseConfig struct {
    DefaultOrg      string `json:"defaultOrg,omitempty" yaml:"defaultOrg,omitempty"`
    CacheTTLSeconds int    `json:"cacheTtlSeconds,omitempty" yaml:"cacheTtlSeconds,omitempty" validate:"min=60,max=3600"`
}
```

---

## Example Context

```yaml
apiVersion: contextops/v1
kind: Context
metadata:
  name: webapp-prod
  labels:
    app: webapp
    env: prod
    team: platform
  annotations:
    description: "Production web application context"
    owner: "platform-team@example.com"
    
spec:
  app:
    name: webapp
    environment: prod
    version: "v2.1.0"
    owner: platform-team
    repository: "https://github.com/example/webapp"
    
  policy:
    allowedActions: ["refresh", "validate", "inspect"]
    requireMfaForActions: ["sync"]
    kubernetes:
      enforceNamespaceFromKubeconfig: true
      allowNamespaceOverride: false
      allowedNamespaces: ["webapp-prod"]
    rateLimit:
      requestsPerMinute: 60
      burstSize: 10
      
  vault:
    address: "https://vault.example.com"
    namespace: "platform/prod"
    auth:
      method: kubernetes
      kubernetes:
        role: "contextops-webapp-prod"
        serviceAccount: "contextops-vault-reader"
    secrets:
      - logicalName: argocd
        path: "kv/platform/prod/argocd"
        requiredKeys: ["token"]
      - logicalName: newrelic
        path: "kv/platform/prod/newrelic"
        requiredKeys: ["api_key"]
      - logicalName: github
        path: "kv/platform/prod/github"
        requiredKeys: ["token"]
        
  argocd:
    address: "https://argocd.example.com"
    auth:
      tokenRef:
        vaultSecretLogicalName: argocd
        key: token
    selectors:
      apps: ["webapp-prod"]
      project: "webapp"
      labels:
        env: "prod"
    operations:
      sync:
        allowed: false  # Production sync requires manual approval
        prune: false
        dryRunDefault: true
        
  newrelic:
    accountId: 1234567
    region: "US"
    auth:
      apiKeyRef:
        vaultSecretLogicalName: newrelic
        key: api_key
    entitySelector:
      tagFilters:
        - key: "app"
          value: "webapp"
        - key: "env"
          value: "prod"
      types: ["APPLICATION"]
    metrics:
      - name: "apm.service.transaction.duration"
        window: "5m"
      - name: "apm.service.error.rate"
        window: "5m"
      - name: "apm.service.throughput"
        window: "5m"
    alerts:
      includeIncidents: true
      
  kubernetes:
    kubeconfig:
      path: "/var/secrets/kubeconfig"
      contextOverride: "prod-cluster"
    namespaceOverride: ""  # Use namespace from kubeconfig
    resources: ["pods", "services", "deployments", "ingresses", "events"]
    
  git:
    provider: github
    auth:
      method: github_app
      secretRef:
        vaultSecretLogicalName: github
        key: token
    browse:
      defaultOrg: "example"
      cacheTtlSeconds: 300
```

---

## Validation Rules

### Custom Validators

```go
// Context name validation
func validateContextName(fl validator.FieldLevel) bool {
    name := fl.Field().String()
    matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, name)
    return matched && len(name) >= 1 && len(name) <= 63
}

// Duration validation for strings like "5m", "1h"
func validateDuration(fl validator.FieldLevel) bool {
    _, err := time.ParseDuration(fl.Field().String())
    return err == nil
}

// URL validation with additional checks
func validateContextURL(fl validator.FieldLevel) bool {
    u, err := url.Parse(fl.Field().String())
    if err != nil {
        return false
    }
    
    // Must be HTTPS in production contexts
    if u.Scheme != "https" {
        return false
    }
    
    return true
}
```

### Secret Detection

```go
var forbiddenSecretPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)(password|token|secret|key|auth)["']?\s*[:=]\s*["']?([^"\s,}]+)`),
    regexp.MustCompile(`Bearer\s+[A-Za-z0-9\-._~+/]+=*`),
    regexp.MustCompile(`(?i)authorization:\s*[^\n\r]+`),
    regexp.MustCompile(`-----BEGIN [A-Z ]+-----`), // PEM keys
}

func validateNoInlineSecrets(ctx *Context) error {
    contextJSON, _ := json.Marshal(ctx)
    contextStr := string(contextJSON)
    
    for _, pattern := range forbiddenSecretPatterns {
        if pattern.MatchString(contextStr) {
            return fmt.Errorf("inline secret detected: contexts must use secret references only")
        }
    }
    
    return nil
}
```

---

## Database Representation

The Context model is stored in the database as:

```sql
CREATE TABLE contexts (
    name VARCHAR(255) PRIMARY KEY,
    api_version VARCHAR(50) NOT NULL DEFAULT 'contextops/v1',
    kind VARCHAR(50) NOT NULL DEFAULT 'Context',
    
    -- Metadata
    labels JSONB,
    annotations JSONB,
    
    -- Full spec as JSONB for flexibility
    spec JSONB NOT NULL,
    
    -- Extracted fields for indexing and querying
    app_name VARCHAR(255) GENERATED ALWAYS AS (spec->>'app'->>'name') STORED,
    environment VARCHAR(50) GENERATED ALWAYS AS (spec->>'app'->>'environment') STORED,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT valid_name CHECK (name ~ '^[a-z0-9-]+$'),
    CONSTRAINT valid_environment CHECK (environment IN ('dev', 'staging', 'prod'))
);

-- Indexes for common query patterns
CREATE INDEX idx_contexts_app_env ON contexts (app_name, environment);
CREATE INDEX idx_contexts_labels ON contexts USING GIN (labels);
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at DESC);
```

---

## JSON Schema

For API documentation and client generation:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://contextops.io/schemas/context/v1",
  "title": "ContextOps Context",
  "description": "Application-environment context configuration",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "contextops/v1"
    },
    "kind": {
      "type": "string", 
      "const": "Context"
    },
    "metadata": {
      "$ref": "#/$defs/metadata"
    },
    "spec": {
      "$ref": "#/$defs/contextSpec"
    }
  },
  "$defs": {
    "metadata": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string",
          "pattern": "^[a-z0-9-]+$",
          "minLength": 1,
          "maxLength": 63
        }
      }
    }
  }
}
```

---

## Related Documentation

- [ADR-003: Secrets Posture](../adr/ADR-003-secrets-posture.md) - Secret reference strategy
- [Database Schema](./database-schema.md) - Complete database design
- [API Schemas](./api-schemas.md) - REST API request/response formats
- [PHASE-1A Implementation Guide](../phases/PHASE-1A.md) - Implementation details