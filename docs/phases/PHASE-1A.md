# PHASE 1A: GitOps Core Foundation

**Duration:** 4-5 days  
**Prerequisites:** Go 1.21+, PostgreSQL 15+, Git, Kubernetes cluster access, ArgoCD instance  
**Deliverable:** GitOps-aware API Gateway with Context CRUD operations, multi-environment support, and GitOps validation

---

## Overview

Establish the foundational components of the GitOps-optimized ContextOps platform: App and Environment manifest models, Context pairing system, multi-environment database schema, API Gateway with GitOps awareness, and comprehensive CRUD operations supporting App+Environment deployments, ApplicationSet correlation, customer branches, and multi-environment configurations.

## Success Criteria

✅ App manifest model implemented with Helm sources and ApplicationSet metadata  
✅ Environment manifest model implemented with Vault datasources and cluster configuration  
✅ Context pairing system implemented linking Apps with Environments  
✅ Multi-environment PostgreSQL schema supporting App+Environment relationships  
✅ API Gateway serving App, Environment, and Context CRUD endpoints with customer isolation  
✅ Customer-based authentication and authorization middleware  
✅ App+Environment deployment validation logic  
✅ ApplicationSet correlation and customer branch validation  
✅ Vault datasource and VaultStaticSecret validation  
✅ Unit tests for App, Environment, and Context models  
✅ Integration test for complete App+Environment+Context CRUD flow  

---

## Implementation Tasks

### Task 1: GitOps Project Structure Setup

Create the foundational directory structure with GitOps components:

```bash
mkdir -p {cmd/gateway,internal/{contexts,gitops,storage,auth},pkg/{api,schemas,gitops}}
mkdir -p {internal/{helm,vault,argocd},pkg/validation}
cd /path/to/contextops
```

**App+Environment Manifest Files to create:**
- `go.mod` - Go module definition with GitOps dependencies
- `cmd/gateway/main.go` - GitOps-aware API Gateway entry point
- `internal/models/app.go` - App manifest domain models
- `internal/models/environment.go` - Environment manifest domain models  
- `internal/models/context.go` - Context pairing domain models
- `internal/validation/app.go` - App manifest validation logic
- `internal/validation/environment.go` - Environment manifest validation logic
- `internal/validation/context.go` - Context pairing validation logic
- `internal/gitops/applicationset.go` - ApplicationSet correlation models
- `internal/helm/sources.go` - Multi-source Helm chart models
- `internal/vault/datasources.go` - Vault datasource models
- `internal/storage/postgres.go` - Multi-manifest database connection
- `pkg/api/manifests.go` - App, Environment, Context API types
- `pkg/schemas/app_schema.go` - App manifest JSON schema validation
- `pkg/schemas/environment_schema.go` - Environment manifest JSON schema validation
- `pkg/schemas/context_schema.go` - Context pairing JSON schema validation
- `migrations/001_app_environment_schema.up.sql` - App+Environment+Context database schema
- `migrations/001_app_environment_schema.down.sql` - Schema rollback

### Task 2: App Manifest Domain Model

**File: `internal/models/app.go`**

Implement the App manifest model supporting multiple Helm sources and ApplicationSet metadata:

```go
// App manifest struct
type App struct {
    APIVersion string      `json:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string      `json:"kind" validate:"required,eq=App"`
    Metadata   AppMetadata `json:"metadata" validate:"required"`
    Spec       AppSpec     `json:"spec" validate:"required"`
}

type AppMetadata struct {
    Name        string            `json:"name" validate:"required,dns1123label"`
    Labels      map[string]string `json:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
    CreatedAt   *time.Time        `json:"createdAt,omitempty"`
    UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type AppSpec struct {
    Application  AppApplicationConfig `json:"application" validate:"required"`
    Helm         AppHelmConfig        `json:"helm" validate:"required"`
    ArgoCD       AppArgoCDConfig      `json:"argocd" validate:"required"`
    Environments []AppEnvironmentRef  `json:"environments" validate:"required,min=1"`
}

type AppApplicationConfig struct {
    Name       string `json:"name" validate:"required,dns1123label"`
    Version    string `json:"version" validate:"required,semver"`
    Maintainer string `json:"maintainer" validate:"required,email"`
}

type AppHelmConfig struct {
    Sources       []HelmSource `json:"sources" validate:"required,min=1"`
    DefaultSource int          `json:"defaultSource" validate:"gte=0"`
}

type HelmSource struct {
    Type       string `json:"type" validate:"required,oneof=helm-registry git oci"`
    Registry   string `json:"registry,omitempty"`
    Chart      string `json:"chart,omitempty" validate:"required"`
    Version    string `json:"version,omitempty"`
    Repository string `json:"repository,omitempty"`
    Path       string `json:"path,omitempty"`
    Ref        string `json:"ref,omitempty"`
}

type AppArgoCDConfig struct {
    ApplicationSets      []ApplicationSetConfig    `json:"applicationSets" validate:"required,min=1"`
    BootstrapApplication *BootstrapApplicationConfig `json:"bootstrapApplication,omitempty"`
}

type ApplicationSetConfig struct {
    Name      string                    `json:"name" validate:"required,dns1123label"`
    Namespace string                    `json:"namespace" validate:"required,dns1123label"`
    Generator ApplicationSetGenerator   `json:"generator" validate:"required"`
    Template  ApplicationSetTemplate    `json:"template" validate:"required"`
}

type ApplicationSetGenerator struct {
    Type string                      `json:"type" validate:"required,oneof=git clusters list"`
    Git  *GitGenerator               `json:"git,omitempty"`
    List *ListGenerator              `json:"list,omitempty"`
    Clusters *ClustersGenerator       `json:"clusters,omitempty"`
}

type GitGenerator struct {
    RepoURL     string                   `json:"repoURL" validate:"required,url"`
    Revision    string                   `json:"revision" validate:"required"`
    Directories []GitGeneratorDirectory  `json:"directories,omitempty"`
    Files       []GitGeneratorFile       `json:"files,omitempty"`
}

type GitGeneratorDirectory struct {
    Path    string `json:"path" validate:"required"`
    Exclude string `json:"exclude,omitempty"`
}

type ApplicationSetTemplate struct {
    Metadata ApplicationSetTemplateMetadata `json:"metadata" validate:"required"`
    Spec     ApplicationSetTemplateSpec     `json:"spec" validate:"required"`
}

type ApplicationSetTemplateMetadata struct {
    Name   string            `json:"name" validate:"required"`
    Labels map[string]string `json:"labels,omitempty"`
}

type ApplicationSetTemplateSpec struct {
    Source ApplicationSetTemplateSource `json:"source" validate:"required"`
}

type ApplicationSetTemplateSource struct {
    Helm *ApplicationSetTemplateHelm `json:"helm,omitempty"`
}

type ApplicationSetTemplateHelm struct {
    ValueFiles []string `json:"valueFiles,omitempty"`
}

type BootstrapApplicationConfig struct {
    Name      string `json:"name" validate:"required,dns1123label"`
    Namespace string `json:"namespace" validate:"required,dns1123label"`
}

type AppEnvironmentRef struct {
    Name           string `json:"name" validate:"required"`
    EnvironmentRef string `json:"environmentRef" validate:"required"`
}
```

### Task 2b: Environment Manifest Domain Model

**File: `internal/models/environment.go`**

```go
// Environment manifest struct  
type Environment struct {
    APIVersion string              `json:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string              `json:"kind" validate:"required,eq=Environment"`
    Metadata   EnvironmentMetadata `json:"metadata" validate:"required"`
    Spec       EnvironmentSpec     `json:"spec" validate:"required"`
}

type EnvironmentMetadata struct {
    Name        string            `json:"name" validate:"required,dns1123label"`
    Labels      map[string]string `json:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
    CreatedAt   *time.Time        `json:"createdAt,omitempty"`
    UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type EnvironmentSpec struct {
    Environment  EnvironmentConfig    `json:"environment" validate:"required"`
    Helm         EnvironmentHelmConfig `json:"helm" validate:"required"`
    Datasources  map[string]VaultDatasource `json:"datasources" validate:"required"`
    VaultSecrets []VaultStaticSecret   `json:"vaultSecrets" validate:"required,min=1"`
    PodEnvValidation PodEnvValidationConfig `json:"podEnvValidation" validate:"required"`
}

type EnvironmentConfig struct {
    Name      string                 `json:"name" validate:"required"`
    Cluster   ClusterConfig          `json:"cluster" validate:"required"`
    Namespace string                 `json:"namespace" validate:"required,dns1123label"`
}

type ClusterConfig struct {
    KubeconfigSecretRef VaultSecretRef `json:"kubeconfigSecretRef" validate:"required"`
}

type VaultSecretRef struct {
    Vault string `json:"vault" validate:"required,vaultpath"`
    Key   string `json:"key" validate:"required"`
}

type EnvironmentHelmConfig struct {
    ValuesSource HelmValuesSource `json:"valuesSource" validate:"required"`
}

type HelmValuesSource struct {
    Type       string `json:"type" validate:"required,eq=git"`
    Repository string `json:"repository" validate:"required,url"`
    Path       string `json:"path" validate:"required"`
    Branch     string `json:"branch" validate:"required"`
}

type VaultDatasource struct {
    Vault string   `json:"vault" validate:"required,vaultpath"`
    Keys  []string `json:"keys" validate:"required,min=1"`
}

type VaultStaticSecret struct {
    Name              string   `json:"name" validate:"required,dns1123label"`
    VaultPath         string   `json:"vaultPath" validate:"required,vaultpath"`
    DestinationSecret string   `json:"destinationSecret" validate:"required,dns1123label"`
    RequiredKeys      []string `json:"requiredKeys" validate:"required,min=1"`
}

type PodEnvValidationConfig struct {
    Enabled         bool                  `json:"enabled"`
    ExpectedEnvVars []ExpectedEnvVar      `json:"expectedEnvVars,omitempty"`
}

type ExpectedEnvVar struct {
    Name      string `json:"name" validate:"required"`
    SecretRef string `json:"secretRef" validate:"required"`
    Key       string `json:"key" validate:"required"`
}
```

### Task 2c: Context Pairing Domain Model

**File: `internal/models/context.go`**

```go
// Context pairing struct
type Context struct {
    APIVersion string          `json:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string          `json:"kind" validate:"required,eq=Context"`
    Metadata   ContextMetadata `json:"metadata" validate:"required"`
    Spec       ContextSpec     `json:"spec" validate:"required"`
}

type ContextMetadata struct {
    Name        string            `json:"name" validate:"required,dns1123label"`
    Labels      map[string]string `json:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
    CreatedAt   *time.Time        `json:"createdAt,omitempty"`
    UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type ContextSpec struct {
    AppRef      string                `json:"appRef" validate:"required"`
    Deployments []ContextDeployment   `json:"deployments" validate:"required,min=1"`
    GitOps      ContextGitOpsConfig   `json:"gitops" validate:"required"`
}

type ContextDeployment struct {
    Environment    string `json:"environment" validate:"required"`
    AppRef         string `json:"appRef" validate:"required"`
    EnvironmentRef string `json:"environmentRef" validate:"required"`
    Active         bool   `json:"active"`
}

type ContextGitOpsConfig struct {
    CustomerBranch CustomerBranchConfig `json:"customerBranch" validate:"required"`
    Monitoring     MonitoringConfig     `json:"monitoring" validate:"required"`
}

type CustomerBranchConfig struct {
    Enabled bool   `json:"enabled"`
    Branch  string `json:"branch" validate:"required_if=Enabled true,customer_branch"`
}

type MonitoringConfig struct {
    ApplicationSets          bool `json:"applicationSets"`
    VaultSecrets            bool `json:"vaultSecrets"`
    HelmValues              bool `json:"helmValues"`
    CrossEnvironmentDrift   bool `json:"crossEnvironmentDrift"`
}
```

**Key Implementation Points:**
- **App Manifests**: Support multiple Helm sources (registry, Git, OCI) with ApplicationSet metadata
- **Environment Manifests**: Customer-specific with Vault datasources and cluster kubeconfig references
- **Context Pairing**: Links Apps with Environments, enabling flexible deployment combinations
- **Customer Isolation**: Environment manifests are customer-specific while Apps can be shared
- **ApplicationSet Correlation**: Apps include ApplicationSet metadata for UI correlation
- **Vault Integration**: Direct vault path references for all credentials and configuration sources
    ApplicationSets      []ApplicationSetConfig `json:"applicationSets" validate:"required,min=1"`
    CustomerBranch       CustomerBranchConfig   `json:"customerBranch"`
}

// Vault-secrets-operator Configuration
type VaultConfig struct {
    Address           string                  `json:"address" validate:"required,url"`
    Namespace         string                  `json:"namespace,omitempty"`
    Auth              VaultAuthConfig         `json:"auth" validate:"required"`
    StaticSecrets     []VaultStaticSecret     `json:"staticSecrets" validate:"required,min=1"`
    PodEnvValidation  PodEnvValidationConfig  `json:"podEnvValidation"`
}

type VaultStaticSecret struct {
    Name              string   `json:"name" validate:"required,dns1123label"`
    Namespace         string   `json:"namespace" validate:"required"`
    VaultPath         string   `json:"vaultPath" validate:"required"`
    RequiredKeys      []string `json:"requiredKeys" validate:"required,min=1"`
    DestinationSecret string   `json:"destinationSecret" validate:"required"`
}

// Multi-Cluster Kubernetes Configuration
type KubernetesConfig struct {
    Clusters      map[string]ClusterConfig   `json:"clusters" validate:"required,min=1"`
    Resources     ResourceMonitoringConfig   `json:"resources" validate:"required"`
    ServiceMesh   *ServiceMeshConfig         `json:"serviceMesh,omitempty"`
}

type ClusterConfig struct {
    Kubeconfig KubeconfigConfig `json:"kubeconfig" validate:"required"`
    Namespace  string           `json:"namespace" validate:"required,dns1123label"`
}
```

**Key GitOps Implementation Points:**
- Multi-environment support with dev/qa/uat/prod validation
- ApplicationSet configuration models for ArgoCD integration
- Customer branch validation following `customer/{customer-name}` pattern
- VaultStaticSecret models for Vault-secrets-operator integration
- Multi-cluster Kubernetes configuration support
- Comprehensive validation tags for GitOps-specific requirements
- Customer label requirement for multi-tenancy support

### Task 3: GitOps Database Schema and Multi-Environment Persistence

**File: `migrations/001_gitops_schema.up.sql`**

```sql
-- GitOps Contexts table with multi-environment support
CREATE TABLE contexts (
    name VARCHAR(255) PRIMARY KEY CHECK (name ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$'),
    customer_id VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ApplicationSets tracking table
CREATE TABLE applicationsets (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) REFERENCES contexts(name) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'unknown',
    last_sync TIMESTAMP WITH TIME ZONE,
    health VARCHAR(50) DEFAULT 'unknown',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(context_name, name, cluster)
);

-- Environment-specific status tracking
CREATE TABLE environment_status (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) REFERENCES contexts(name) ON DELETE CASCADE,
    environment VARCHAR(20) NOT NULL CHECK (environment IN ('dev', 'qa', 'uat', 'prod')),
    application_name VARCHAR(255) NOT NULL,
    cluster VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    sync_status VARCHAR(50) DEFAULT 'unknown',
    health_status VARCHAR(50) DEFAULT 'unknown',
    last_deployed TIMESTAMP WITH TIME ZONE,
    helm_revision VARCHAR(100),
    git_commit VARCHAR(40),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(context_name, environment, application_name)
);

-- Vault secrets correlation table
CREATE TABLE vault_secrets (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) REFERENCES contexts(name) ON DELETE CASCADE,
    secret_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    environment VARCHAR(20) NOT NULL,
    vault_path VARCHAR(500) NOT NULL,
    destination_secret VARCHAR(255) NOT NULL,
    required_keys TEXT[] NOT NULL,
    last_validated TIMESTAMP WITH TIME ZONE,
    validation_status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Pod environment variable validation tracking
CREATE TABLE pod_env_validations (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) REFERENCES contexts(name) ON DELETE CASCADE,
    environment VARCHAR(20) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    env_var_name VARCHAR(255) NOT NULL,
    secret_ref VARCHAR(255),
    secret_key VARCHAR(255),
    validation_status VARCHAR(50) DEFAULT 'pending',
    last_checked TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    error_message TEXT
);

-- Indexes for GitOps query patterns
CREATE INDEX idx_contexts_customer ON contexts (customer_id);
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at);
CREATE INDEX idx_applicationsets_context ON applicationsets (context_name);
CREATE INDEX idx_applicationsets_status ON applicationsets (status, health);
CREATE INDEX idx_environment_status_context_env ON environment_status (context_name, environment);
CREATE INDEX idx_vault_secrets_context ON vault_secrets (context_name, environment);
CREATE INDEX idx_pod_env_validations_status ON pod_env_validations (validation_status, last_checked);

-- GIN indexes for JSONB GitOps queries
CREATE INDEX idx_contexts_gitops ON contexts USING GIN ((spec->'gitops'));
CREATE INDEX idx_contexts_application ON contexts USING GIN ((spec->'application'));
CREATE INDEX idx_contexts_vault ON contexts USING GIN ((spec->'vault'));

-- Trigger function for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at triggers to relevant tables
CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE ON contexts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    
CREATE TRIGGER update_environment_status_updated_at BEFORE UPDATE ON environment_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**File: `internal/storage/postgres.go`**

```go
type GitOpsContextStore struct {
    db *sql.DB
}

func NewGitOpsContextStore(db *sql.DB) *GitOpsContextStore {
    return &GitOpsContextStore{db: db}
}

// GitOps Context CRUD operations
func (s *GitOpsContextStore) Create(ctx context.Context, context *contexts.Context) error
func (s *GitOpsContextStore) Get(ctx context.Context, name, customerID string) (*contexts.Context, error)
func (s *GitOpsContextStore) List(ctx context.Context, customerID string) ([]*contexts.Context, error)
func (s *GitOpsContextStore) Update(ctx context.Context, context *contexts.Context) error
func (s *GitOpsContextStore) Delete(ctx context.Context, name, customerID string) error

// ApplicationSet operations
func (s *GitOpsContextStore) CreateApplicationSet(ctx context.Context, appSet *ApplicationSetStatus) error
func (s *GitOpsContextStore) UpdateApplicationSetStatus(ctx context.Context, appSet *ApplicationSetStatus) error
func (s *GitOpsContextStore) GetApplicationSets(ctx context.Context, contextName string) ([]*ApplicationSetStatus, error)

// Environment status operations
func (s *GitOpsContextStore) UpdateEnvironmentStatus(ctx context.Context, status *EnvironmentStatus) error
func (s *GitOpsContextStore) GetEnvironmentStatus(ctx context.Context, contextName, environment string) (*EnvironmentStatus, error)
func (s *GitOpsContextStore) ListEnvironmentStatuses(ctx context.Context, contextName string) ([]*EnvironmentStatus, error)

// Vault secrets correlation operations
func (s *GitOpsContextStore) CreateVaultSecret(ctx context.Context, vaultSecret *VaultSecretStatus) error
func (s *GitOpsContextStore) UpdateVaultSecretValidation(ctx context.Context, vaultSecret *VaultSecretStatus) error
func (s *GitOpsContextStore) GetVaultSecrets(ctx context.Context, contextName, environment string) ([]*VaultSecretStatus, error)

// Pod environment variable validation operations
func (s *GitOpsContextStore) CreatePodEnvValidation(ctx context.Context, validation *PodEnvValidation) error
func (s *GitOpsContextStore) UpdatePodEnvValidationStatus(ctx context.Context, validation *PodEnvValidation) error
func (s *GitOpsContextStore) GetPodEnvValidations(ctx context.Context, contextName, environment string) ([]*PodEnvValidation, error)
```

**GitOps-Enhanced Implementation Notes:**
- Multi-tenant isolation using customer_id filtering in all queries
- JSONB for GitOps context specs with specialized GIN indexes for ApplicationSet, Vault, and Helm queries  
- Separate tables for ApplicationSet status, environment-specific deployments, Vault secret validation, and pod environment correlation
- Transaction support for multi-table GitOps operations (e.g., creating context + ApplicationSets + Vault secrets atomically)
- Connection pooling with customer-aware resource limits
- Context timeout handling with GitOps operation awareness

### Task 4: GitOps Context Validation

**File: `internal/contexts/validation.go`**

Implement comprehensive GitOps-aware validation:

```go
func ValidateGitOpsContext(ctx *Context) error {
    // 1. Struct tag validation using validator package with GitOps-specific rules
    if err := validator.New().Struct(ctx); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // 2. GitOps-specific business rule validation
    if err := validateGitOpsBusinessRules(ctx); err != nil {
        return fmt.Errorf("GitOps business rule validation failed: %w", err)
    }
    
    // 3. ApplicationSet configuration validation
    if err := validateApplicationSets(ctx.Spec.GitOps.ApplicationSets); err != nil {
        return fmt.Errorf("ApplicationSet validation failed: %w", err)
    }
    
    // 4. Customer branch validation
    if err := validateCustomerBranch(ctx.Spec.GitOps.CustomerBranch); err != nil {
        return fmt.Errorf("customer branch validation failed: %w", err)
    }
    
    // 5. Vault-secrets-operator validation
    if err := validateVaultStaticSecrets(ctx.Spec.Vault.StaticSecrets); err != nil {
        return fmt.Errorf("Vault static secrets validation failed: %w", err)
    }
    
    // 6. Multi-environment consistency validation
    if err := validateEnvironmentConsistency(ctx.Spec.Application.Environments); err != nil {
        return fmt.Errorf("environment consistency validation failed: %w", err)
    }
    
    // 7. Helm values correlation validation
    if err := validateHelmValues(ctx.Spec.Helm); err != nil {
        return fmt.Errorf("Helm values validation failed: %w", err)
    }
    
    return nil
}

// GitOps-specific validation functions
func validateContextName(name string) bool {
    // Context names must follow Kubernetes DNS-1123 label convention
    matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`, name)
    return matched && len(name) <= 63
}

func validateApplicationSets(appSets []ApplicationSetConfig) error {
    for _, appSet := range appSets {
        if appSet.Name == "" {
            return errors.New("ApplicationSet name cannot be empty")
        }
        if appSet.Namespace == "" {
            return errors.New("ApplicationSet namespace cannot be empty")  
        }
        if appSet.Generator.Type == "" {
            return errors.New("ApplicationSet generator type is required")
        }
    }
    return nil
}

func validateCustomerBranch(branch CustomerBranchConfig) error {
    if branch.Enabled {
        if branch.Branch == "" {
            return errors.New("customer branch name is required when enabled")
        }
        // Validate customer branch pattern: customer/{customer-name}
        matched, _ := regexp.MatchString(`^customer/[a-z0-9][a-z0-9-]*[a-z0-9]$`, branch.Branch)
        if !matched {
            return fmt.Errorf("customer branch must follow pattern 'customer/{customer-name}': %s", branch.Branch)
        }
    }
    return nil
}

func validateVaultStaticSecrets(secrets []VaultStaticSecret) error {
    for _, secret := range secrets {
        if secret.Name == "" {
            return errors.New("VaultStaticSecret name cannot be empty")
        }
        if secret.VaultPath == "" {
            return errors.New("VaultStaticSecret vaultPath cannot be empty")
        }
        if len(secret.RequiredKeys) == 0 {
            return errors.New("VaultStaticSecret must specify at least one required key")
        }
        if secret.DestinationSecret == "" {
            return errors.New("VaultStaticSecret destinationSecret cannot be empty")
        }
    }
    return nil
}

func validateEnvironmentConsistency(environments []EnvironmentConfig) error {
    if len(environments) == 0 {
        return errors.New("at least one environment must be specified")
    }
    
    validEnvs := map[string]bool{"dev": true, "qa": true, "uat": true, "prod": true}
    seenEnvs := make(map[string]bool)
    
    for _, env := range environments {
        if !validEnvs[env.Name] {
            return fmt.Errorf("invalid environment name: %s. Must be one of: dev, qa, uat, prod", env.Name)
        }
        if seenEnvs[env.Name] {
            return fmt.Errorf("duplicate environment: %s", env.Name)
        }
        seenEnvs[env.Name] = true
    }
    
    return nil
}

func validateHelmValues(helm HelmConfig) error {
    if helm.ChartName == "" {
        return errors.New("Helm chart name cannot be empty")
    }
    if helm.ChartVersion == "" {
        return errors.New("Helm chart version cannot be empty")
    }
    if len(helm.ValuesFiles) == 0 {
        return errors.New("at least one Helm values file must be specified")
    }
    
    // Validate environment-specific values files exist
    for _, valuesFile := range helm.ValuesFiles {
        if valuesFile.Environment == "" {
            return errors.New("Helm values file must specify environment")
        }
        if valuesFile.Path == "" {
            return errors.New("Helm values file path cannot be empty")
        }
        // Validate path follows values-{env}.yaml pattern
        expectedPattern := fmt.Sprintf("values-%s.yaml", valuesFile.Environment)
        if !strings.Contains(valuesFile.Path, expectedPattern) {
            return fmt.Errorf("Helm values file should follow pattern 'values-%s.yaml', got: %s", 
                valuesFile.Environment, valuesFile.Path)
        }
    }
    
    return nil
}

func validateSecretReferences(ctx *Context) error {
    // Prevent inline secrets in any configuration
    specJSON, _ := json.Marshal(ctx.Spec)
    suspiciousPatterns := []string{
        "password", "secret", "key", "token", "credential", 
        "-----BEGIN", "-----END", "base64:",
    }
    
    specStr := strings.ToLower(string(specJSON))
    for _, pattern := range suspiciousPatterns {
        if strings.Contains(specStr, pattern) {
            return fmt.Errorf("potential inline secret detected. Use Vault references instead: %s", pattern)
        }
    }
    
    return nil
}

func validateKubernetesPolicy(policy *PolicyConfig) error {
    if policy.RBAC.Enabled {
        if len(policy.RBAC.Roles) == 0 {
            return errors.New("RBAC is enabled but no roles specified")
        }
    }
    
    if policy.NetworkPolicy.Enabled {
        if len(policy.NetworkPolicy.Ingress) == 0 && len(policy.NetworkPolicy.Egress) == 0 {
            return errors.New("NetworkPolicy is enabled but no ingress/egress rules specified")
        }
    }
    
    return nil
}
```

### Task 5: GitOps-Aware API Gateway Foundation

**File: `cmd/gateway/main.go`**

```go
func main() {
    // Configuration loading with GitOps-specific settings
    cfg := loadConfig()
    
    // Database connection with multi-tenant support
    db := setupDatabase(cfg.DatabaseURL)
    defer db.Close()
    
    // GitOps service dependencies
    gitOpsContextStore := storage.NewGitOpsContextStore(db)
    contextValidator := validation.NewGitOpsValidator()
    customerAuth := auth.NewCustomerAuthenticator(cfg)
    
    // Handler dependencies
    contextHandler := handlers.NewGitOpsContextHandler(gitOpsContextStore, contextValidator)
    applicationSetHandler := handlers.NewApplicationSetHandler(gitOpsContextStore)
    environmentHandler := handlers.NewEnvironmentHandler(gitOpsContextStore)
    vaultHandler := handlers.NewVaultValidationHandler(gitOpsContextStore)
    
    // Router setup with GitOps endpoints
    router := setupGitOpsRouter(contextHandler, applicationSetHandler, environmentHandler, vaultHandler, customerAuth)
    
    // Server startup with graceful shutdown
    server := &http.Server{
        Addr:         cfg.Port,
        Handler:      router,
        ReadTimeout:  cfg.ReadTimeout,
        WriteTimeout: cfg.WriteTimeout,
    }
    
    log.Printf("Starting GitOps API Gateway on %s", cfg.Port)
    log.Fatal(server.ListenAndServe())
}
```

**GitOps-Enhanced API Endpoints:**

```go
// File: internal/handlers/gitops_context.go
type GitOpsContextHandler struct {
    store     *storage.GitOpsContextStore
    validator *validation.GitOpsValidator
}

// Core GitOps Context operations
func (h *GitOpsContextHandler) CreateContext(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) GetContext(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) ListContexts(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) UpdateContext(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request)

// GitOps-specific operations
func (h *GitOpsContextHandler) ValidateContextForGitOps(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) GetContextStatus(w http.ResponseWriter, r *http.Request)
func (h *GitOpsContextHandler) GetContextEnvironments(w http.ResponseWriter, r *http.Request)

// File: internal/handlers/applicationset.go  
func (h *ApplicationSetHandler) GetApplicationSets(w http.ResponseWriter, r *http.Request)
func (h *ApplicationSetHandler) GetApplicationSetStatus(w http.ResponseWriter, r *http.Request)
func (h *ApplicationSetHandler) UpdateApplicationSetStatus(w http.ResponseWriter, r *http.Request)

// File: internal/handlers/environment.go
func (h *EnvironmentHandler) GetEnvironmentStatus(w http.ResponseWriter, r *http.Request)
func (h *EnvironmentHandler) ListEnvironmentStatuses(w http.ResponseWriter, r *http.Request)
func (h *EnvironmentHandler) UpdateEnvironmentStatus(w http.ResponseWriter, r *http.Request)

// File: internal/handlers/vault.go
func (h *VaultValidationHandler) ValidateSecrets(w http.ResponseWriter, r *http.Request)
func (h *VaultValidationHandler) GetSecretValidationStatus(w http.ResponseWriter, r *http.Request)
func (h *VaultValidationHandler) ValidatePodEnvVars(w http.ResponseWriter, r *http.Request)
```

**GitOps-Enhanced Routes:**

**Context Management:**
- `POST /api/v1/contexts` → CreateContext (with GitOps validation)
- `GET /api/v1/contexts` → ListContexts (customer-filtered)
- `GET /api/v1/contexts/{name}` → GetContext
- `PUT /api/v1/contexts/{name}` → UpdateContext (with GitOps re-validation)
- `DELETE /api/v1/contexts/{name}` → DeleteContext
- `POST /api/v1/contexts/{name}/validate` → ValidateContextForGitOps
- `GET /api/v1/contexts/{name}/status` → GetContextStatus (aggregated GitOps status)

**ApplicationSet Management:**
- `GET /api/v1/contexts/{name}/applicationsets` → GetApplicationSets
- `GET /api/v1/contexts/{name}/applicationsets/{appset}` → GetApplicationSetStatus
- `PUT /api/v1/contexts/{name}/applicationsets/{appset}/status` → UpdateApplicationSetStatus

**Environment Management:**
- `GET /api/v1/contexts/{name}/environments` → ListEnvironmentStatuses
- `GET /api/v1/contexts/{name}/environments/{env}` → GetEnvironmentStatus  
- `PUT /api/v1/contexts/{name}/environments/{env}/status` → UpdateEnvironmentStatus

**Vault Integration:**
- `POST /api/v1/contexts/{name}/vault/validate` → ValidateSecrets
- `GET /api/v1/contexts/{name}/vault/secrets` → GetSecretValidationStatus
- `POST /api/v1/contexts/{name}/vault/pod-env-validation` → ValidatePodEnvVars

### Task 6: Customer-Aware Authentication Middleware

**File: `internal/auth/middleware.go`**

Implement customer-based authentication and multi-tenant isolation:

```go
type CustomerContext struct {
    CustomerID   string   `json:"customer_id"`
    CustomerName string   `json:"customer_name"`
    UserID       string   `json:"user_id"`
    Permissions  []string `json:"permissions"`
}

func CustomerAuthMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // For Phase 1A: Extract customer from header or default to system
            customerID := r.Header.Get("X-Customer-ID")
            if customerID == "" {
                customerID = "system" // Default for Phase 1A
            }
            
            // Extract user information (placeholder for Phase 1A)
            userID := r.Header.Get("X-User-ID")
            if userID == "" {
                userID = "system-user"
            }
            
            // Create customer context
            customerCtx := &CustomerContext{
                CustomerID:   customerID,
                CustomerName: customerID, // Simplified for Phase 1A
                UserID:       userID,
                Permissions:  []string{"context:read", "context:write"}, // Default permissions
            }
            
            // Set customer context for downstream handlers
            ctx := r.Context()
            ctx = context.WithValue(ctx, "customer", customerCtx)
            
            // TODO Phase 1B: Implement proper JWT/OAuth2 validation
            // TODO Phase 1B: Add customer permission validation
            // TODO Phase 1B: Add audit logging
            
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GetCustomerFromContext(ctx context.Context) (*CustomerContext, error) {
    customer, ok := ctx.Value("customer").(*CustomerContext)
    if !ok {
        return nil, errors.New("no customer context found")
    }
    return customer, nil
}

func RequirePermission(permission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            customer, err := GetCustomerFromContext(r.Context())
            if err != nil {
                http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
                return
            }
            
            // Check if customer has required permission
            hasPermission := false
            for _, perm := range customer.Permissions {
                if perm == permission || perm == "*" {
                    hasPermission = true
                    break
                }
            }
            
            if !hasPermission {
                http.Error(w, fmt.Sprintf("Forbidden: missing permission %s", permission), http.StatusForbidden)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

func CustomerIsolationMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Ensure all database queries are scoped to customer
            customer, err := GetCustomerFromContext(r.Context())
            if err != nil {
                http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
                return
            }
            
            // Add customer isolation headers for database layer
            r.Header.Set("X-Database-Customer-ID", customer.CustomerID)
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### Task 7: GitOps Configuration Management

**File: `internal/config/config.go`**

```go
import (
    "time"
    "github.com/caarlos0/env/v9"
    "github.com/joho/godotenv"
)

type Config struct {
    // Server configuration
    Port         string        `env:"PORT" envDefault:":8080"`
    ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
    WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
    LogLevel     string        `env:"LOG_LEVEL" envDefault:"info"`
    
    // Database configuration
    DatabaseURL      string `env:"DATABASE_URL" envDefault:"postgres://localhost/contextops?sslmode=disable"`
    MaxDBConnections int    `env:"MAX_DB_CONNECTIONS" envDefault:"25"`
    DBTimeout        time.Duration `env:"DB_TIMEOUT" envDefault:"10s"`
    
    // GitOps integration configuration
    ArgoCD   ArgoCDConfig   `envPrefix:"ARGOCD_"`
    Vault    VaultConfig    `envPrefix:"VAULT_"`
    Helm     HelmConfig     `envPrefix:"HELM_"`
    
    // Multi-tenant configuration
    MultiTenant MultiTenantConfig `envPrefix:"TENANT_"`
    
    // Development configuration
    DevMode bool `env:"DEV_MODE" envDefault:"false"`
}

type ArgoCDConfig struct {
    Enabled    bool   `env:"ENABLED" envDefault:"true"`
    ServerURL  string `env:"SERVER_URL" envDefault:"https://argocd.example.com"`
    AuthToken  string `env:"AUTH_TOKEN"`
    Namespace  string `env:"NAMESPACE" envDefault:"argocd"`
    Insecure   bool   `env:"INSECURE" envDefault:"false"`
}

type VaultConfig struct {
    Enabled   bool   `env:"ENABLED" envDefault:"true"`
    Address   string `env:"ADDRESS" envDefault:"https://vault.example.com"`
    AuthPath  string `env:"AUTH_PATH" envDefault:"kubernetes"`
    Role      string `env:"ROLE" envDefault:"contextops"`
    Namespace string `env:"NAMESPACE"`
}

type HelmConfig struct {
    Enabled     bool   `env:"ENABLED" envDefault:"true"`
    RegistryURL string `env:"REGISTRY_URL" envDefault:"https://charts.example.com"`
    Username    string `env:"USERNAME"`
    Password    string `env:"PASSWORD"`
}

type MultiTenantConfig struct {
    Enabled           bool          `env:"ENABLED" envDefault:"true"`
    DefaultCustomerID string        `env:"DEFAULT_CUSTOMER_ID" envDefault:"system"`
    IsolationMode     string        `env:"ISOLATION_MODE" envDefault:"strict"` // strict, permissive
    CustomerHeader    string        `env:"CUSTOMER_HEADER" envDefault:"X-Customer-ID"`
    UserHeader        string        `env:"USER_HEADER" envDefault:"X-User-ID"`
    SessionTimeout    time.Duration `env:"SESSION_TIMEOUT" envDefault:"24h"`
}

func Load() *Config {
    // Load .env file if it exists (for development)
    _ = godotenv.Load()
    
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        log.Fatal("failed to parse config: ", err)
    }
    
    // Validate critical GitOps configuration
    if err := validateGitOpsConfig(cfg); err != nil {
        log.Fatal("invalid GitOps configuration: ", err)
    }
    
    return cfg
}

func validateGitOpsConfig(cfg *Config) error {
    if cfg.ArgoCD.Enabled && cfg.ArgoCD.ServerURL == "" {
        return errors.New("ArgoCD server URL is required when ArgoCD is enabled")
    }
    
    if cfg.Vault.Enabled && cfg.Vault.Address == "" {
        return errors.New("Vault address is required when Vault is enabled")
    }
    
    if cfg.MultiTenant.Enabled && cfg.MultiTenant.IsolationMode != "strict" && cfg.MultiTenant.IsolationMode != "permissive" {
        return errors.New("tenant isolation mode must be 'strict' or 'permissive'")
    }
    
    return nil
}

func (cfg *Config) IsDevelopment() bool {
    return cfg.DevMode || cfg.LogLevel == "debug"
}

func (cfg *Config) GetDatabaseConfig() string {
    return fmt.Sprintf("%s?max_connections=%d&connect_timeout=%ds", 
        cfg.DatabaseURL, cfg.MaxDBConnections, int(cfg.DBTimeout.Seconds()))
}
```

---

## GitOps Testing Requirements

### Unit Tests

**File: `internal/contexts/gitops_models_test.go`**
- Test GitOps Context struct marshaling/unmarshaling with all nested GitOps types
- Test ApplicationSet configuration validation
- Test VaultStaticSecret models and validation
- Test multi-environment configuration structures
- Test customer branch configuration validation

**File: `internal/contexts/validation_test.go`**
- Test GitOps-specific validation scenarios (valid/invalid contexts)
- Test ApplicationSet configuration validation (generators, templates)
- Test customer branch pattern validation (`customer/{customer-name}`)
- Test Vault-secrets-operator configuration validation
- Test multi-environment consistency validation
- Test Helm values file correlation validation
- Test inline secret detection and prevention
- Test edge cases and boundary conditions for GitOps workflows

**File: `internal/storage/postgres_test.go`**
- Test all GitOps Context CRUD operations with customer isolation
- Test ApplicationSet status operations
- Test environment-specific status tracking
- Test Vault secret validation operations
- Test pod environment variable validation tracking
- Test multi-tenant data isolation
- Test error conditions (duplicate names, unauthorized access, etc.)
- Test database connection handling with customer scoping

**File: `internal/auth/middleware_test.go`**
- Test customer context extraction from headers
- Test customer isolation middleware functionality
- Test permission-based access control
- Test multi-tenant security boundaries
- Test authentication middleware chain

### Integration Tests

**File: `cmd/gateway/gitops_integration_test.go`**
- Test complete GitOps Context CRUD flow via HTTP endpoints
- Test ApplicationSet management endpoints
- Test environment status tracking endpoints
- Test Vault secret validation endpoints
- Test customer isolation across all endpoints
- Test GitOps validation during context creation/updates
- Test multi-environment status correlation
- Test JSON request/response formatting with GitOps fields
- Test error responses and status codes for GitOps operations
- Test concurrent access scenarios with customer isolation

**File: `internal/gitops/applicationset_test.go`**
- Test ApplicationSet configuration parsing
- Test ApplicationSet generator validation
- Test ApplicationSet status correlation with ArgoCD

**File: `internal/vault/validation_test.go`**
- Test VaultStaticSecret configuration validation
- Test secret-to-pod environment variable correlation
- Test Vault path validation and format checking

---

## GitOps Dependencies

Add to `go.mod`:
```
require (
    // Core web framework and routing
    github.com/gorilla/mux v1.8.0
    github.com/gorilla/handlers v1.5.1
    
    // Database and migrations
    github.com/lib/pq v1.10.9
    github.com/golang-migrate/migrate/v4 v4.16.2
    github.com/jmoiron/sqlx v1.3.5
    
    // Validation and configuration
    github.com/go-playground/validator/v10 v10.15.5
    github.com/caarlos0/env/v9 v9.0.0
    github.com/joho/godotenv v1.4.0
    
    // GitOps integrations
    github.com/argoproj/argo-cd/v2 v2.8.4
    github.com/hashicorp/vault/api v1.10.0
    helm.sh/helm/v3 v3.12.3
    
    // Kubernetes client libraries
    k8s.io/client-go v0.28.2
    k8s.io/api v0.28.2
    k8s.io/apimachinery v0.28.2
    sigs.k8s.io/controller-runtime v0.16.2
    
    // JSON/YAML processing for GitOps configurations
    github.com/ghodss/yaml v1.0.0
    github.com/tidwall/gjson v1.16.0
    
    // Logging and observability
    github.com/sirupsen/logrus v1.9.3
    github.com/prometheus/client_golang v1.17.0
    
    // Testing and development
    github.com/stretchr/testify v1.8.4
    github.com/testcontainers/testcontainers-go v0.24.1
    github.com/DATA-DOG/go-sqlmock v1.5.0
)
```

**Key GitOps-specific dependencies:**
- **ArgoCD client libraries**: For ApplicationSet monitoring and status retrieval
- **Vault API client**: For Vault-secrets-operator integration and secret validation
- **Helm client**: For chart analysis and values file correlation
- **Kubernetes clients**: For multi-cluster resource monitoring and pod environment validation
- **Controller-runtime**: For custom resource handling and status tracking

---

## GitOps Validation Checklist

Before marking Phase 1A complete:

**Compilation and Build:**
- [ ] `go build ./cmd/gateway` compiles successfully with all GitOps dependencies
- [ ] Database migrations run without errors for all GitOps tables
- [ ] No compilation errors in GitOps integration modules

**Testing:**
- [ ] All unit tests pass: `go test ./internal/...`
- [ ] GitOps integration tests pass: `go test ./cmd/gateway/...`
- [ ] ApplicationSet configuration tests pass
- [ ] Vault-secrets-operator validation tests pass
- [ ] Multi-environment validation tests pass

**Core GitOps Context Operations:**
- [ ] Can create GitOps contexts with ApplicationSet configuration via API endpoints
- [ ] Can create GitOps contexts with Vault-secrets-operator configuration
- [ ] Can create GitOps contexts with multi-environment support (dev/qa/uat/prod)
- [ ] Can create GitOps contexts with customer branch configuration
- [ ] Can read, update, delete GitOps contexts via API endpoints
- [ ] Customer isolation works correctly (customers cannot see each other's contexts)

**GitOps-Specific Validation:**
- [ ] ApplicationSet configuration validation works (name, namespace, generators)
- [ ] Customer branch pattern validation works (`customer/{customer-name}`)
- [ ] VaultStaticSecret validation works (vault path, required keys, destination)
- [ ] Multi-environment consistency validation works (no duplicate environments)
- [ ] Helm values file correlation validation works (values-{env}.yaml pattern)
- [ ] Invalid GitOps context specs are properly rejected with meaningful errors
- [ ] Inline secret detection prevents secrets in context specifications

**Database and Storage:**
- [ ] GitOps contexts table stores contexts with customer_id isolation
- [ ] ApplicationSets table tracks ApplicationSet status correctly
- [ ] Environment status table tracks per-environment deployment status
- [ ] Vault secrets table tracks secret validation status
- [ ] Pod environment validation table tracks correlation status
- [ ] Database indexes support efficient GitOps queries
- [ ] Multi-tenant data isolation is enforced at database level

**API and Authentication:**
- [ ] All GitOps API endpoints return proper HTTP status codes
- [ ] JSON responses include GitOps-specific fields and are well-formatted
- [ ] Customer authentication middleware extracts customer context correctly
- [ ] Customer isolation middleware prevents cross-tenant data access
- [ ] Permission-based access control works for GitOps operations

**Configuration:**
- [ ] GitOps configuration loads correctly (ArgoCD, Vault, Helm settings)
- [ ] Multi-tenant configuration is properly validated
- [ ] Development and production configuration modes work correctly

---

## GitOps Next Steps

Upon completion, Phase 1A provides:
- **GitOps-aware Context CRUD API** with ApplicationSet, Vault, and multi-environment support
- **Comprehensive GitOps validation** including ApplicationSet configuration, customer branches, and Vault-secrets-operator integration
- **Multi-tenant database persistence** with customer isolation and GitOps-specific tracking tables
- **Customer-based authentication** and multi-tenant security boundaries
- **Foundation for GitOps monitoring** (Phase 1B will add real-time ApplicationSet and environment status synchronization)

**Handoff to Phase 1B:** The API Gateway can accept and validate GitOps contexts with ApplicationSet configurations, Vault secret references, and multi-environment specifications. Next phase will add RabbitMQ integration for GitOps command publishing, ArgoCD ApplicationSet monitoring, Vault secret validation events, and multi-environment status correlation.