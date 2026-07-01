-- App+Environment+Context database schema for multi-tenant GitOps platform

-- Customers table - tenant/customer accounts. Referenced by FKs in later
-- migrations (002 security/audit schema) and queried by the auth service
-- (internal/auth/enhanced_middleware.go). Must exist before those FKs.
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL DEFAULT '',
    salt VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Apps table - shareable application manifests
CREATE TABLE apps (
    name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (name, customer_id),
    CONSTRAINT apps_name_check CHECK (name ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$')
);

-- Environments table - customer-specific environment configurations
CREATE TABLE environments (
    name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (name, customer_id),
    CONSTRAINT environments_name_check CHECK (name ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$')
);

-- Contexts table - pairing of Apps with Environments
CREATE TABLE contexts (
    name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    environment_reference VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (name, customer_id),
    FOREIGN KEY (app_reference, customer_id) REFERENCES apps(name, customer_id) ON DELETE CASCADE,
    FOREIGN KEY (environment_reference, customer_id) REFERENCES environments(name, customer_id) ON DELETE CASCADE,
    CONSTRAINT contexts_name_check CHECK (name ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$')
);

-- ApplicationSets tracking from App manifests
CREATE TABLE applicationsets (
    id SERIAL PRIMARY KEY,
    app_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    generator_type VARCHAR(50) NOT NULL,
    generator_config JSONB,
    template_config JSONB,
    status VARCHAR(50) DEFAULT 'unknown',
    last_sync TIMESTAMP WITH TIME ZONE,
    health VARCHAR(50) DEFAULT 'unknown',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (app_name, customer_id) REFERENCES apps(name, customer_id) ON DELETE CASCADE,
    UNIQUE(app_name, customer_id, name, namespace)
);

-- Helm sources tracking from App manifests
CREATE TABLE helm_sources (
    id SERIAL PRIMARY KEY,
    app_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN ('helm-registry', 'git', 'oci')),
    registry VARCHAR(500),
    chart VARCHAR(255) NOT NULL,
    version VARCHAR(100),
    repository VARCHAR(500),
    path VARCHAR(500),
    ref VARCHAR(100),
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (app_name, customer_id) REFERENCES apps(name, customer_id) ON DELETE CASCADE
);

-- Vault sources tracking from Environment manifests
CREATE TABLE vault_sources (
    id SERIAL PRIMARY KEY,
    environment_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    vault_address VARCHAR(500) NOT NULL,
    vault_namespace VARCHAR(255),
    auth_method VARCHAR(50) NOT NULL,
    auth_config JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (environment_name, customer_id) REFERENCES environments(name, customer_id) ON DELETE CASCADE
);

-- Vault static secrets from Environment manifests
CREATE TABLE vault_static_secrets (
    id SERIAL PRIMARY KEY,
    environment_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    vault_path VARCHAR(500) NOT NULL,
    destination_secret VARCHAR(255) NOT NULL,
    required_keys TEXT[] NOT NULL,
    validation_status VARCHAR(50) DEFAULT 'pending',
    last_validated TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (environment_name, customer_id) REFERENCES environments(name, customer_id) ON DELETE CASCADE
);

-- Cluster configurations from Environment manifests
CREATE TABLE cluster_configs (
    id SERIAL PRIMARY KEY,
    environment_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    cluster_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    kubeconfig_vault_path VARCHAR(500) NOT NULL,
    kubeconfig_vault_key VARCHAR(255) NOT NULL,
    connection_status VARCHAR(50) DEFAULT 'unknown',
    last_checked TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (environment_name, customer_id) REFERENCES environments(name, customer_id) ON DELETE CASCADE
);

-- Context deployment status tracking
CREATE TABLE context_deployments (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    environment VARCHAR(255) NOT NULL,
    deployment_status VARCHAR(50) DEFAULT 'unknown',
    sync_status VARCHAR(50) DEFAULT 'unknown',
    health_status VARCHAR(50) DEFAULT 'unknown',
    last_deployed TIMESTAMP WITH TIME ZONE,
    last_sync_time TIMESTAMP WITH TIME ZONE,
    git_commit VARCHAR(40),
    helm_revision VARCHAR(100),
    resource_count INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(context_name, customer_id, environment)
);

-- Pod environment variable validation tracking
CREATE TABLE pod_env_validations (
    id SERIAL PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    environment VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    env_var_name VARCHAR(255) NOT NULL,
    secret_ref VARCHAR(255),
    secret_key VARCHAR(255),
    expected_value_hash VARCHAR(64), -- SHA256 hash for comparison without storing actual values
    validation_status VARCHAR(50) DEFAULT 'pending',
    last_checked TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    error_message TEXT,
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE
);

-- Customer branch tracking for GitOps workflows
CREATE TABLE customer_branches (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    branch_name VARCHAR(255) NOT NULL,
    repository_url VARCHAR(500) NOT NULL,
    last_commit VARCHAR(40),
    last_commit_time TIMESTAMP WITH TIME ZONE,
    branch_status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(customer_id, branch_name, repository_url)
);

-- Indexes for efficient querying

-- Apps indexes
CREATE INDEX idx_apps_customer ON apps (customer_id);
CREATE INDEX idx_apps_updated_at ON apps (updated_at DESC);

-- Environments indexes
CREATE INDEX idx_environments_customer ON environments (customer_id);
CREATE INDEX idx_environments_updated_at ON environments (updated_at DESC);

-- Contexts indexes
CREATE INDEX idx_contexts_customer ON contexts (customer_id);
CREATE INDEX idx_contexts_app_ref ON contexts (app_reference, customer_id);
CREATE INDEX idx_contexts_environment_ref ON contexts (environment_reference, customer_id);
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at DESC);

-- ApplicationSets indexes
CREATE INDEX idx_applicationsets_app ON applicationsets (app_name, customer_id);
CREATE INDEX idx_applicationsets_status ON applicationsets (status, health);
CREATE INDEX idx_applicationsets_customer ON applicationsets (customer_id);

-- Helm sources indexes
CREATE INDEX idx_helm_sources_app ON helm_sources (app_name, customer_id);
CREATE INDEX idx_helm_sources_type ON helm_sources (source_type);

-- Vault sources indexes
CREATE INDEX idx_vault_sources_environment ON vault_sources (environment_name, customer_id);
CREATE INDEX idx_vault_sources_customer ON vault_sources (customer_id);

-- Vault static secrets indexes
CREATE INDEX idx_vault_static_secrets_environment ON vault_static_secrets (environment_name, customer_id);
CREATE INDEX idx_vault_static_secrets_status ON vault_static_secrets (validation_status);

-- Cluster configs indexes
CREATE INDEX idx_cluster_configs_environment ON cluster_configs (environment_name, customer_id);
CREATE INDEX idx_cluster_configs_status ON cluster_configs (connection_status);

-- Context deployments indexes
CREATE INDEX idx_context_deployments_context ON context_deployments (context_name, customer_id);
CREATE INDEX idx_context_deployments_environment ON context_deployments (environment);
CREATE INDEX idx_context_deployments_status ON context_deployments (deployment_status, sync_status, health_status);

-- Pod env validations indexes
CREATE INDEX idx_pod_env_validations_context ON pod_env_validations (context_name, customer_id, environment);
CREATE INDEX idx_pod_env_validations_status ON pod_env_validations (validation_status);

-- Customer branches indexes
CREATE INDEX idx_customer_branches_customer ON customer_branches (customer_id);
CREATE INDEX idx_customer_branches_status ON customer_branches (branch_status);