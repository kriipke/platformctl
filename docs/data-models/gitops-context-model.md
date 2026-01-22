# GitOps Context Data Model

**Version:** 2.0  
**Date:** 2026-01-21  
**Phase:** 1A - GitOps Foundation  

---

## Overview

The GitOps Context model is the central data structure in ContextOps, representing an application deployed across multiple environments using GitOps workflows. It integrates deeply with ArgoCD ApplicationSets, Helm umbrella charts, Vault-secrets-operator, and customer branch management. This document defines the complete GitOps Context schema with validation rules and implementation guidance.

---

## GitOps Context Schema

### Root Structure

```go
type Context struct {
    APIVersion string          `json:"apiVersion" yaml:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string          `json:"kind" yaml:"kind" validate:"required,eq=Context"`
    Metadata   ContextMetadata `json:"metadata" yaml:"metadata" validate:"required"`
    Spec       GitOpsContextSpec `json:"spec" yaml:"spec" validate:"required"`
}
```

### GitOps Metadata

```go
type ContextMetadata struct {
    Name        string            `json:"name" yaml:"name" validate:"required,contextname"`
    Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty" validate:"required"`
    Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
    CreatedAt   *time.Time        `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
    UpdatedAt   *time.Time        `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}
```

**Required Labels:**
- `customer`: Customer identifier (e.g., "acme-corp")
- `tier`: Service tier ("production", "staging", "development")

**Validation Rules:**
- `name`: Must match regex `^[a-z0-9-]+$` (Kubernetes-friendly naming)
- `name`: Length 1-63 characters
- `labels.customer`: Required for customer isolation
- `labels`: Used for RBAC and filtering across environments

### GitOps Context Specification

```go
type GitOpsContextSpec struct {
    Application     ApplicationConfig    `json:"application" yaml:"application" validate:"required"`
    GitOps          GitOpsConfig        `json:"gitops" yaml:"gitops" validate:"required"`
    Helm            HelmConfig          `json:"helm" yaml:"helm" validate:"required"`
    Vault           VaultConfig         `json:"vault" yaml:"vault" validate:"required"`
    ArgoCD          ArgoCDConfig        `json:"argocd" yaml:"argocd" validate:"required"`
    Kubernetes      KubernetesConfig    `json:"kubernetes" yaml:"kubernetes" validate:"required"`
    Git             GitConfig           `json:"git" yaml:"git" validate:"required"`
    NewRelic        NewRelicConfig      `json:"newrelic" yaml:"newrelic,omitempty"`
    Policy          PolicyConfig        `json:"policy" yaml:"policy" validate:"required"`
}
```

---

## Application Configuration

```go
type ApplicationConfig struct {
    Name         string               `json:"name" yaml:"name" validate:"required,dns1123label"`
    Environments []EnvironmentConfig  `json:"environments" yaml:"environments" validate:"required,min=1"`
}

type EnvironmentConfig struct {
    Name   string `json:"name" yaml:"name" validate:"required,oneof=dev qa uat prod"`
    Active bool   `json:"active" yaml:"active"`
}
```

**Validation Rules:**
- At least one environment must be active
- Environment names must be from allowed list: dev, qa, uat, prod
- Application name must be valid DNS label

---

## GitOps Configuration

```go
type GitOpsConfig struct {
    BootstrapApplication BootstrapAppConfig    `json:"bootstrapApplication" yaml:"bootstrapApplication" validate:"required"`
    ApplicationSets      []ApplicationSetConfig `json:"applicationSets" yaml:"applicationSets" validate:"required,min=1"`
    CustomerBranch       CustomerBranchConfig   `json:"customerBranch" yaml:"customerBranch"`
}

type BootstrapAppConfig struct {
    Name      string `json:"name" yaml:"name" validate:"required"`
    Namespace string `json:"namespace" yaml:"namespace" validate:"required"`
    RepoURL   string `json:"repoUrl" yaml:"repoUrl" validate:"required,url"`
    Path      string `json:"path" yaml:"path" validate:"required"`
    Branch    string `json:"branch" yaml:"branch" validate:"required"`
}

type ApplicationSetConfig struct {
    Name      string                    `json:"name" yaml:"name" validate:"required,dns1123label"`
    Namespace string                    `json:"namespace" yaml:"namespace" validate:"required"`
    Generator ApplicationSetGenerator   `json:"generator" yaml:"generator" validate:"required"`
    Template  ApplicationSetTemplate    `json:"template" yaml:"template" validate:"required"`
}

type ApplicationSetGenerator struct {
    Type        string   `json:"type" yaml:"type" validate:"required,oneof=git matrix clusters"`
    Directories []string `json:"directories" yaml:"directories,omitempty"`
    Files       []string `json:"files" yaml:"files,omitempty"`
}

type ApplicationSetTemplate struct {
    Metadata ApplicationSetMetadata `json:"metadata" yaml:"metadata" validate:"required"`
    Spec     ApplicationSetSpec     `json:"spec" yaml:"spec" validate:"required"`
}

type ApplicationSetMetadata struct {
    Name string `json:"name" yaml:"name" validate:"required"`
}

type ApplicationSetSpec struct {
    Project string `json:"project" yaml:"project" validate:"required"`
}

type CustomerBranchConfig struct {
    Enabled    bool   `json:"enabled" yaml:"enabled"`
    Branch     string `json:"branch" yaml:"branch" validate:"customer_branch_pattern"`
    Repository string `json:"repository" yaml:"repository" validate:"required_if=Enabled true,url"`
}
```

**Validation Rules:**
- Bootstrap application must point to valid Git repository
- ApplicationSet names must be valid Kubernetes resource names
- Customer branch must follow pattern: `customer/{customer-name}` if enabled
- Generator type must be supported by ArgoCD ApplicationSet controller

---

## Helm Configuration

```go
type HelmConfig struct {
    Chart        ChartConfig              `json:"chart" yaml:"chart" validate:"required"`
    ValuesFiles  map[string]string        `json:"valuesFiles" yaml:"valuesFiles" validate:"required"`
    Dependencies []HelmDependency         `json:"dependencies" yaml:"dependencies,omitempty"`
}

type ChartConfig struct {
    Name       string `json:"name" yaml:"name" validate:"required"`
    Repository string `json:"repository" yaml:"repository" validate:"required,url"`
    Version    string `json:"version" yaml:"version" validate:"required,semver"`
}

type HelmDependency struct {
    Name       string `json:"name" yaml:"name" validate:"required"`
    Version    string `json:"version" yaml:"version" validate:"required,semver"`
    Repository string `json:"repository" yaml:"repository" validate:"required,url"`
    Condition  string `json:"condition,omitempty" yaml:"condition,omitempty"`
}
```

**Validation Rules:**
- Chart version must be valid semantic version
- Repository URLs must be accessible Helm repositories
- Values files must follow pattern: `values-{environment}.yaml`
- Each active environment must have corresponding values file

---

## Vault Configuration (Vault-secrets-operator)

```go
type VaultConfig struct {
    Address           string                  `json:"address" yaml:"address" validate:"required,url"`
    Namespace         string                  `json:"namespace" yaml:"namespace,omitempty"`
    Auth              VaultAuthConfig         `json:"auth" yaml:"auth" validate:"required"`
    StaticSecrets     []VaultStaticSecret     `json:"staticSecrets" yaml:"staticSecrets" validate:"required,min=1"`
    PodEnvValidation  PodEnvValidationConfig  `json:"podEnvValidation" yaml:"podEnvValidation"`
}

type VaultAuthConfig struct {
    Method     string                `json:"method" yaml:"method" validate:"required,oneof=kubernetes approle"`
    Kubernetes *VaultKubernetesAuth  `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty" validate:"required_if=Method kubernetes"`
    AppRole    *VaultAppRoleAuth     `json:"approle,omitempty" yaml:"approle,omitempty" validate:"required_if=Method approle"`
}

type VaultKubernetesAuth struct {
    Role           string `json:"role" yaml:"role" validate:"required"`
    ServiceAccount string `json:"serviceAccount" yaml:"serviceAccount" validate:"required"`
}

type VaultAppRoleAuth struct {
    RoleID   string `json:"roleId" yaml:"roleId" validate:"required"`
    SecretID string `json:"secretId" yaml:"secretId" validate:"required"`
}

type VaultStaticSecret struct {
    Name              string   `json:"name" yaml:"name" validate:"required,dns1123label"`
    Namespace         string   `json:"namespace" yaml:"namespace" validate:"required"`
    VaultPath         string   `json:"vaultPath" yaml:"vaultPath" validate:"required"`
    RequiredKeys      []string `json:"requiredKeys" yaml:"requiredKeys" validate:"required,min=1"`
    DestinationSecret string   `json:"destinationSecret" yaml:"destinationSecret" validate:"required"`
}

type PodEnvValidationConfig struct {
    Enabled bool              `json:"enabled" yaml:"enabled"`
    Pods    []PodValidation   `json:"pods" yaml:"pods,omitempty" validate:"required_if=Enabled true"`
}

type PodValidation struct {
    Name             string                    `json:"name" yaml:"name" validate:"required"`
    Containers       []string                  `json:"containers" yaml:"containers" validate:"required,min=1"`
    ExpectedEnvVars  []ExpectedEnvVar         `json:"expectedEnvVars" yaml:"expectedEnvVars" validate:"required,min=1"`
}

type ExpectedEnvVar struct {
    Name      string `json:"name" yaml:"name" validate:"required"`
    SecretRef string `json:"secretRef" yaml:"secretRef" validate:"required"`
    Key       string `json:"key" yaml:"key" validate:"required"`
}
```

**Validation Rules:**
- VaultStaticSecret names must be unique within namespace
- Pod environment variable mappings must reference valid secrets
- Vault authentication method must match cluster configuration
- Secret paths must follow organizational naming conventions

---

## ArgoCD Configuration

```go
type ArgoCDConfig struct {
    Address   string               `json:"address" yaml:"address" validate:"required,url"`
    Auth      ArgoCDAuthConfig     `json:"auth" yaml:"auth" validate:"required"`
    Selectors ArgoCDSelectors      `json:"selectors" yaml:"selectors" validate:"required"`
    Sources   []ArgoCDSource       `json:"sources" yaml:"sources,omitempty"`
}

type ArgoCDAuthConfig struct {
    Method    string `json:"method" yaml:"method" validate:"required,oneof=vault_token bearer_token"`
    VaultPath string `json:"vaultPath,omitempty" yaml:"vaultPath,omitempty" validate:"required_if=Method vault_token"`
    VaultKey  string `json:"vaultKey,omitempty" yaml:"vaultKey,omitempty" validate:"required_if=Method vault_token"`
    Token     string `json:"token,omitempty" yaml:"token,omitempty" validate:"required_if=Method bearer_token"`
}

type ArgoCDSelectors struct {
    ApplicationSets []string                    `json:"applicationSets" yaml:"applicationSets" validate:"required,min=1"`
    Applications    []ArgoCDApplicationSelector `json:"applications" yaml:"applications" validate:"required,min=1"`
    Project         string                      `json:"project" yaml:"project" validate:"required"`
}

type ArgoCDApplicationSelector struct {
    Pattern      string   `json:"pattern" yaml:"pattern" validate:"required"`
    Environments []string `json:"environments" yaml:"environments" validate:"required,min=1"`
}

type ArgoCDSource struct {
    Name    string `json:"name" yaml:"name" validate:"required"`
    RepoURL string `json:"repoURL" yaml:"repoURL" validate:"required,url"`
    Type    string `json:"type" yaml:"type" validate:"required,oneof=helm values kustomize"`
    Branch  string `json:"branch,omitempty" yaml:"branch,omitempty"`
}
```

**Validation Rules:**
- ArgoCD applications must match ApplicationSet templates
- Application patterns must be valid regex or glob patterns
- Project must exist in ArgoCD
- Multi-source configurations must have valid repository URLs

---

## Kubernetes Configuration (Multi-Cluster)

```go
type KubernetesConfig struct {
    Clusters      map[string]ClusterConfig   `json:"clusters" yaml:"clusters" validate:"required,min=1"`
    Resources     ResourceMonitoringConfig   `json:"resources" yaml:"resources" validate:"required"`
    ServiceMesh   *ServiceMeshConfig         `json:"serviceMesh,omitempty" yaml:"serviceMesh,omitempty"`
}

type ClusterConfig struct {
    Kubeconfig KubeconfigConfig `json:"kubeconfig" yaml:"kubeconfig" validate:"required"`
    Namespace  string           `json:"namespace" yaml:"namespace" validate:"required,dns1123label"`
}

type KubeconfigConfig struct {
    Path    string `json:"path" yaml:"path" validate:"required"`
    Context string `json:"context" yaml:"context" validate:"required"`
}

type ResourceMonitoringConfig struct {
    Workloads       []string                `json:"workloads" yaml:"workloads" validate:"required,min=1"`
    CustomResources []CustomResourceConfig  `json:"customResources,omitempty" yaml:"customResources,omitempty"`
}

type CustomResourceConfig struct {
    Group   string `json:"group" yaml:"group" validate:"required"`
    Version string `json:"version" yaml:"version" validate:"required"`
    Kind    string `json:"kind" yaml:"kind" validate:"required"`
}

type ServiceMeshConfig struct {
    Enabled   bool     `json:"enabled" yaml:"enabled"`
    Type      string   `json:"type" yaml:"type" validate:"required_if=Enabled true,oneof=istio linkerd"`
    Resources []string `json:"resources" yaml:"resources" validate:"required_if=Enabled true"`
}
```

**Validation Rules:**
- Each active environment must have corresponding cluster configuration
- Kubeconfig paths must exist and be accessible
- Namespace names must be valid per Kubernetes rules
- Custom resources must specify valid group/version/kind

---

## Git Configuration

```go
type GitConfig struct {
    Provider        string                  `json:"provider" yaml:"provider" validate:"required,oneof=github gitlab bitbucket"`
    Auth            GitAuthConfig           `json:"auth" yaml:"auth" validate:"required"`
    Repositories    []GitRepository         `json:"repositories" yaml:"repositories" validate:"required,min=1"`
    DriftDetection  DriftDetectionConfig    `json:"driftDetection" yaml:"driftDetection"`
}

type GitAuthConfig struct {
    Method    string `json:"method" yaml:"method" validate:"required,oneof=github_app pat ssh"`
    VaultPath string `json:"vaultPath" yaml:"vaultPath" validate:"required"`
    VaultKey  string `json:"vaultKey" yaml:"vaultKey" validate:"required"`
}

type GitRepository struct {
    Name        string           `json:"name" yaml:"name" validate:"required"`
    URL         string           `json:"url" yaml:"url" validate:"required,url"`
    Type        string           `json:"type" yaml:"type" validate:"required,oneof=source helm values"`
    Branches    []string         `json:"branches" yaml:"branches" validate:"required,min=1"`
    ValuesFiles *ValuesFilesConfig `json:"valuesFiles,omitempty" yaml:"valuesFiles,omitempty" validate:"required_if=Type values"`
}

type ValuesFilesConfig struct {
    Pattern string `json:"pattern" yaml:"pattern" validate:"required"`
}

type DriftDetectionConfig struct {
    Enabled          bool `json:"enabled" yaml:"enabled"`
    ScheduleMinutes  int  `json:"scheduleMinutes,omitempty" yaml:"scheduleMinutes,omitempty" validate:"required_if=Enabled true,min=5"`
    NotifyOnDrift    bool `json:"notifyOnDrift,omitempty" yaml:"notifyOnDrift,omitempty"`
}
```

**Validation Rules:**
- Git repositories must be accessible with provided credentials
- Values repositories must specify values file patterns
- Customer branches must be accessible if customer branch configuration is enabled
- Drift detection schedule must be reasonable (minimum 5 minutes)

---

## New Relic Configuration

```go
type NewRelicConfig struct {
    AccountID           int                            `json:"accountId" yaml:"accountId" validate:"required,min=1"`
    Region              string                         `json:"region" yaml:"region" validate:"required,oneof=US EU"`
    Auth                NewRelicAuthConfig             `json:"auth" yaml:"auth" validate:"required"`
    EntitySelector      NewRelicEntitySelector         `json:"entitySelector" yaml:"entitySelector" validate:"required"`
    EnvironmentMetrics  map[string][]string            `json:"environmentMetrics" yaml:"environmentMetrics,omitempty"`
}

type NewRelicAuthConfig struct {
    Method    string `json:"method" yaml:"method" validate:"required,eq=vault_api_key"`
    VaultPath string `json:"vaultPath" yaml:"vaultPath" validate:"required"`
    VaultKey  string `json:"vaultKey" yaml:"vaultKey" validate:"required"`
}

type NewRelicEntitySelector struct {
    TagFilters   []NewRelicTagFilter `json:"tagFilters" yaml:"tagFilters" validate:"required,min=1"`
    Environments []string            `json:"environments" yaml:"environments" validate:"required,min=1"`
}

type NewRelicTagFilter struct {
    Key   string `json:"key" yaml:"key" validate:"required"`
    Value string `json:"value" yaml:"value" validate:"required"`
}
```

**Validation Rules:**
- New Relic entities must exist for specified tag filters
- Account ID must be valid New Relic account
- Environment-specific metrics must map to active environments

---

## Policy and Governance Configuration

```go
type PolicyConfig struct {
    AllowedActions      map[string][]string    `json:"allowedActions" yaml:"allowedActions" validate:"required"`
    RequireMfaForActions []MfaRequirement      `json:"requireMfaForActions" yaml:"requireMfaForActions,omitempty"`
    CustomerPolicies    CustomerPolicyConfig   `json:"customerPolicies" yaml:"customerPolicies" validate:"required"`
    ResourceQuotas      map[string]ResourceQuota `json:"resourceQuotas" yaml:"resourceQuotas" validate:"required"`
}

type MfaRequirement struct {
    Action       string   `json:"action" yaml:"action" validate:"required"`
    Environments []string `json:"environments" yaml:"environments" validate:"required,min=1"`
}

type CustomerPolicyConfig struct {
    ConfigurationReview bool   `json:"configurationReview" yaml:"configurationReview"`
    SecretsCompliance   string `json:"secretsCompliance" yaml:"secretsCompliance" validate:"required"`
    RetentionDays       int    `json:"retentionDays" yaml:"retentionDays" validate:"required,min=1"`
}

type ResourceQuota struct {
    MaxReplicas int    `json:"maxReplicas" yaml:"maxReplicas" validate:"required,min=1"`
    MaxCPU      string `json:"maxCPU" yaml:"maxCPU" validate:"required"`
    MaxMemory   string `json:"maxMemory" yaml:"maxMemory" validate:"required"`
}
```

**Validation Rules:**
- MFA requirements must specify valid actions and environments
- Resource quotas must not exceed cluster limits
- Customer policies must include required compliance settings
- Actions must be from allowed list: refresh, validate, sync, inspect, rollback

---

## Validation Implementation

### Custom Validators

```go
// Custom validation tags
func RegisterCustomValidators(v *validator.Validate) {
    v.RegisterValidation("contextname", validateContextName)
    v.RegisterValidation("customer_branch_pattern", validateCustomerBranch)
    v.RegisterValidation("dns1123label", validateDNS1123Label)
    v.RegisterValidation("semver", validateSemanticVersion)
}

func validateContextName(fl validator.FieldLevel) bool {
    name := fl.Field().String()
    matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, name)
    return matched && len(name) >= 1 && len(name) <= 63
}

func validateCustomerBranch(fl validator.FieldLevel) bool {
    branch := fl.Field().String()
    if branch == "" {
        return true // Optional field
    }
    matched, _ := regexp.MatchString(`^customer/[a-z0-9-]+$`, branch)
    return matched
}

func validateDNS1123Label(fl validator.FieldLevel) bool {
    label := fl.Field().String()
    matched, _ := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, label)
    return matched && len(label) <= 63
}

func validateSemanticVersion(fl validator.FieldLevel) bool {
    version := fl.Field().String()
    matched, _ := regexp.MatchString(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`, version)
    return matched
}
```

### Cross-Reference Validation

```go
func ValidateGitOpsContext(ctx *Context) []ValidationError {
    var errors []ValidationError
    
    // Validate environment consistency
    activeEnvs := getActiveEnvironments(ctx.Spec.Application.Environments)
    
    // Check Helm values files
    for _, env := range activeEnvs {
        if _, exists := ctx.Spec.Helm.ValuesFiles[env]; !exists {
            errors = append(errors, ValidationError{
                Field: fmt.Sprintf("spec.helm.valuesFiles.%s", env),
                Message: fmt.Sprintf("Missing values file for active environment: %s", env),
            })
        }
    }
    
    // Check Kubernetes cluster configuration
    for _, env := range activeEnvs {
        if _, exists := ctx.Spec.Kubernetes.Clusters[env]; !exists {
            errors = append(errors, ValidationError{
                Field: fmt.Sprintf("spec.kubernetes.clusters.%s", env),
                Message: fmt.Sprintf("Missing cluster configuration for active environment: %s", env),
            })
        }
    }
    
    // Validate ArgoCD application patterns match ApplicationSets
    for _, appSelector := range ctx.Spec.ArgoCD.Selectors.Applications {
        for _, env := range appSelector.Environments {
            if !contains(activeEnvs, env) {
                errors = append(errors, ValidationError{
                    Field: "spec.argocd.selectors.applications",
                    Message: fmt.Sprintf("ArgoCD selector references inactive environment: %s", env),
                })
            }
        }
    }
    
    // Validate customer branch consistency
    if ctx.Spec.GitOps.CustomerBranch.Enabled {
        customerFromBranch := extractCustomerFromBranch(ctx.Spec.GitOps.CustomerBranch.Branch)
        customerFromLabel := ctx.Metadata.Labels["customer"]
        if customerFromBranch != customerFromLabel {
            errors = append(errors, ValidationError{
                Field: "spec.gitops.customerBranch.branch",
                Message: "Customer branch does not match customer label",
            })
        }
    }
    
    return errors
}

type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

---

## Example GitOps Context

```yaml
apiVersion: contextops/v1
kind: Context
metadata:
  name: webapp-dev
  labels:
    customer: "acme-corp"
    tier: "production"
    team: "platform"
spec:
  application:
    name: webapp
    environments:
      - name: dev
        active: true
      - name: qa
        active: true  
      - name: uat
        active: true
      - name: prod
        active: true

  gitops:
    bootstrapApplication:
      name: "bootstrap-app"
      namespace: "argocd"
      repoUrl: "https://github.com/acme-corp/gitops-config"
      path: "applications"
      branch: "main"
    
    applicationSets:
      - name: "webapp-appset"
        namespace: "argocd"
        generator:
          type: "git"
          directories: ["environments/*"]
        template:
          metadata:
            name: "webapp-{{path.basename}}"
          spec:
            project: "webapp"
    
    customerBranch:
      enabled: true
      branch: "customer/acme-corp"
      repository: "https://github.com/acme-corp/webapp-configs"

  helm:
    chart:
      name: "webapp"
      repository: "https://charts.acme-corp.com"
      version: "1.2.3"
    
    valuesFiles:
      dev: "values-dev.yaml"
      qa: "values-qa.yaml"
      uat: "values-uat.yaml"
      prod: "values-prod.yaml"
    
    dependencies:
      - name: "postgresql"
        version: "12.1.9"
        repository: "https://charts.bitnami.com/bitnami"
      - name: "redis"
        version: "17.3.7"
        repository: "https://charts.bitnami.com/bitnami"

  vault:
    address: "https://vault.acme-corp.com"
    namespace: "platform/acme-corp"
    auth:
      method: "kubernetes"
      kubernetes:
        role: "contextops-webapp"
        serviceAccount: "webapp-vault-reader"
    
    staticSecrets:
      - name: "webapp-db-creds"
        namespace: "webapp-dev"
        vaultPath: "kv/acme-corp/dev/webapp/database"
        requiredKeys: ["username", "password", "host"]
        destinationSecret: "webapp-db-secret"
      - name: "webapp-api-keys"
        namespace: "webapp-dev" 
        vaultPath: "kv/acme-corp/dev/webapp/api-keys"
        requiredKeys: ["stripe_key", "sendgrid_key"]
        destinationSecret: "webapp-api-secret"
    
    podEnvValidation:
      enabled: true
      pods:
        - name: "webapp-*"
          containers: ["webapp", "sidecar"]
          expectedEnvVars:
            - name: "DB_PASSWORD"
              secretRef: "webapp-db-secret"
              key: "password"
            - name: "STRIPE_API_KEY"
              secretRef: "webapp-api-secret"
              key: "stripe_key"

  argocd:
    address: "https://argocd.acme-corp.com"
    auth:
      method: "vault_token"
      vaultPath: "kv/platform/argocd"
      vaultKey: "token"
    
    selectors:
      applicationSets:
        - "webapp-appset"
      applications:
        - pattern: "webapp-*"
          environments: ["dev", "qa", "uat", "prod"]
      project: "webapp"
    
    sources:
      - name: "helm-charts"
        repoURL: "https://charts.acme-corp.com"
        type: "helm"
      - name: "config-values"
        repoURL: "https://github.com/acme-corp/webapp-configs"
        type: "values"
        branch: "customer/acme-corp"

  kubernetes:
    clusters:
      dev:
        kubeconfig:
          path: "~/.kube/dev-cluster-config"
          context: "dev-cluster"
        namespace: "webapp-dev"
      qa:
        kubeconfig:
          path: "~/.kube/qa-cluster-config" 
          context: "qa-cluster"
        namespace: "webapp-qa"
      uat:
        kubeconfig:
          path: "~/.kube/uat-cluster-config"
          context: "uat-cluster" 
        namespace: "webapp-uat"
      prod:
        kubeconfig:
          path: "~/.kube/prod-cluster-config"
          context: "prod-cluster"
        namespace: "webapp-prod"
    
    resources:
      workloads: ["deployments", "statefulsets", "services"]
      customResources:
        - group: "secrets.hashicorp.com"
          version: "v1beta1"
          kind: "VaultStaticSecret"
    
    serviceMesh:
      enabled: true
      type: "istio"
      resources: ["virtualservices", "destinationrules", "gateways"]

  git:
    provider: "github"
    auth:
      method: "github_app"
      vaultPath: "kv/platform/github"
      vaultKey: "token"
    
    repositories:
      - name: "webapp-code"
        url: "https://github.com/acme-corp/webapp"
        type: "source"
        branches: ["main", "develop"]
      - name: "webapp-charts"
        url: "https://github.com/acme-corp/webapp-helm"
        type: "helm"
        branches: ["main"]
      - name: "webapp-configs"
        url: "https://github.com/acme-corp/webapp-configs"
        type: "values"
        branches: ["main", "customer/acme-corp"]
        valuesFiles:
          pattern: "values-*.yaml"
    
    driftDetection:
      enabled: true
      scheduleMinutes: 15
      notifyOnDrift: true

  newrelic:
    accountId: 1234567
    region: "US"
    auth:
      method: "vault_api_key"
      vaultPath: "kv/platform/newrelic"
      vaultKey: "api_key"
    
    entitySelector:
      tagFilters:
        - key: "app"
          value: "webapp"
        - key: "customer"
          value: "acme-corp"
      environments: ["dev", "qa", "uat", "prod"]
    
    environmentMetrics:
      dev:
        - "apm.service.transaction.duration"
        - "apm.service.error.rate"
      prod:
        - "apm.service.transaction.duration"
        - "apm.service.error.rate"
        - "apm.service.throughput"
        - "infrastructure.cpu.utilization"

  policy:
    allowedActions:
      dev: ["refresh", "validate", "sync", "inspect", "rollback"]
      qa: ["refresh", "validate", "sync", "inspect"] 
      uat: ["refresh", "validate", "inspect"]
      prod: ["refresh", "validate", "inspect"]
    
    requireMfaForActions: 
      - action: "sync"
        environments: ["prod"]
      - action: "rollback"  
        environments: ["uat", "prod"]
    
    customerPolicies:
      configurationReview: true
      secretsCompliance: "pci-dss"
      retentionDays: 90
      
    resourceQuotas:
      dev:
        maxReplicas: 5
        maxCPU: "2000m"
        maxMemory: "4Gi" 
      prod:
        maxReplicas: 20
        maxCPU: "10000m"
        maxMemory: "20Gi"
```

---

## Related Documentation

- [GitOps Database Schema](./database-schema.md) - Database representation
- [GitOps API Schemas](./api-schemas.md) - REST API and message formats
- [Integration Results](./integration-results.md) - Service result structures
- [ADR-001: Event-driven GitOps workflows](../adr/ADR-001-event-driven-integration-workflows.md)