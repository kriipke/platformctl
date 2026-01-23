-- GitOps Read Model Schema for Multi-Environment Application Monitoring
-- Phase 1D: Aggregated status views optimized for GitOps dashboard queries

-- Context pairing status - aggregated view of app+environment combinations
CREATE TABLE context_pairing_status (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    environment_reference VARCHAR(255) NOT NULL,
    pairing_status VARCHAR(50) DEFAULT 'unknown',  -- valid, invalid, missing_app, missing_environment
    sync_status VARCHAR(50) DEFAULT 'unknown',      -- synced, out_of_sync, syncing, failed
    health_status VARCHAR(50) DEFAULT 'unknown',    -- healthy, degraded, unhealthy, unknown
    resource_count INTEGER DEFAULT 0,
    last_sync_time TIMESTAMP WITH TIME ZONE,
    last_deployment_time TIMESTAMP WITH TIME ZONE,
    correlation_data JSONB,
    validation_errors TEXT[],
    git_commit VARCHAR(40),
    helm_revision VARCHAR(100),
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, app_reference, environment_reference)
);

-- App manifest correlation - tracks ApplicationSets and their generated applications
CREATE TABLE app_manifest_correlation (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    app_name VARCHAR(255) NOT NULL,
    applicationset_name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    generator_type VARCHAR(50),  -- git, clusters, list
    generator_config JSONB,
    sync_status VARCHAR(50) DEFAULT 'unknown',
    health_status VARCHAR(50) DEFAULT 'unknown',
    application_count INTEGER DEFAULT 0,
    helm_sources JSONB,  -- Array of HelmSourceStatus
    git_sources JSONB,   -- Array of GitSourceStatus
    generated_applications JSONB,  -- Array of ApplicationStatus
    bootstrap_correlation JSONB,  -- Bootstrap Application correlation data
    last_sync_time TIMESTAMP WITH TIME ZONE,
    performance_metrics JSONB,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, app_name, applicationset_name, namespace)
);

-- Environment manifest validation - aggregated vault and cluster validation results
CREATE TABLE environment_manifest_validation (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    environment_name VARCHAR(255) NOT NULL,
    vault_validation_status VARCHAR(50) DEFAULT 'unknown',  -- valid, invalid, partial, error
    cluster_validation_status VARCHAR(50) DEFAULT 'unknown',  -- connected, disconnected, partial, error
    values_file_status VARCHAR(50) DEFAULT 'unknown',  -- available, missing, partial, error
    vault_validations JSONB,  -- Array of VaultValidationResult
    cluster_validations JSONB,  -- Array of ClusterValidationResult
    values_file_validations JSONB,  -- Array of ValuesFileStatus
    pod_env_validations JSONB,  -- Array of PodEnvValidationResult
    secret_correlations JSONB,  -- Vault secret to K8s secret correlations
    validation_summary JSONB,  -- Summary statistics
    last_validated TIMESTAMP WITH TIME ZONE,
    performance_metrics JSONB,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, environment_name)
);

-- Context pairing operations - tracks correlation between app and environment manifests
CREATE TABLE context_pairing_operations (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    operation_type VARCHAR(50) NOT NULL,  -- sync, validate, correlate, inspect
    operation_status VARCHAR(50) DEFAULT 'pending',  -- pending, running, completed, failed
    app_manifest_data JSONB,  -- AppManifestResult
    environment_manifest_data JSONB,  -- EnvironmentManifestResult
    correlation_results JSONB,  -- Cross-manifest correlation data
    multi_env_correlation JSONB,  -- Multi-environment correlation data
    customer_branch_data JSONB,  -- Customer Git branch correlation
    error_details TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    performance_metrics JSONB,
    correlation_id UUID,  -- Links to command_runs table
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    FOREIGN KEY (correlation_id) REFERENCES command_runs(correlation_id) ON DELETE SET NULL
);

-- GitOps vault validation status - detailed Vault secret validation tracking
CREATE TABLE gitops_vault_validation_status (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    environment_name VARCHAR(255) NOT NULL,
    vault_path VARCHAR(500) NOT NULL,
    secret_name VARCHAR(255) NOT NULL,
    validation_status VARCHAR(50) DEFAULT 'pending',  -- valid, invalid, missing, error
    required_keys TEXT[] NOT NULL,
    missing_keys TEXT[],
    extra_keys TEXT[],
    pod_correlations JSONB,  -- Pod environment variable correlations
    kubernetes_secret_name VARCHAR(255),  -- Corresponding K8s secret
    kubernetes_namespace VARCHAR(255),
    last_validated TIMESTAMP WITH TIME ZONE,
    validation_error TEXT,
    performance_metrics JSONB,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, environment_name, vault_path, secret_name)
);

-- Multi-environment application status - consolidated view across environments
CREATE TABLE multi_environment_app_status (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    app_name VARCHAR(255) NOT NULL,
    environment VARCHAR(255) NOT NULL,  -- dev, staging, prod, etc.
    cluster_name VARCHAR(255),
    namespace VARCHAR(255) NOT NULL,
    sync_status VARCHAR(50) DEFAULT 'unknown',
    health_status VARCHAR(50) DEFAULT 'unknown',
    deployment_status VARCHAR(50) DEFAULT 'unknown',  -- deployed, deploying, failed, pending
    helm_revision VARCHAR(100),
    git_commit VARCHAR(40),
    image_tags JSONB,  -- Container image tags
    resource_versions JSONB,  -- Kubernetes resource versions
    last_deployed TIMESTAMP WITH TIME ZONE,
    last_checked TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    performance_metrics JSONB,
    error_message TEXT,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, app_name, environment, cluster_name, namespace)
);

-- Customer git branch correlation - tracks customer branch patterns and compliance
CREATE TABLE customer_git_branch_correlation (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    repository_url VARCHAR(500) NOT NULL,
    customer_branch VARCHAR(255) NOT NULL,  -- Expected: customer/{customer_id}
    branch_compliance VARCHAR(50) DEFAULT 'unknown',  -- compliant, non_compliant, missing
    last_commit VARCHAR(40),
    last_commit_time TIMESTAMP WITH TIME ZONE,
    branch_protection JSONB,  -- Branch protection rules
    manifest_files JSONB,  -- List of manifest files in branch
    values_files JSONB,  -- Environment-specific values files
    drift_detection JSONB,  -- Configuration drift from main branch
    last_validated TIMESTAMP WITH TIME ZONE,
    performance_metrics JSONB,
    validation_error TEXT,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE,
    UNIQUE(customer_id, context_name, repository_url, customer_branch)
);

-- Indexes for efficient GitOps dashboard queries

-- Context pairing status indexes
CREATE INDEX idx_context_pairing_status_customer ON context_pairing_status (customer_id);
CREATE INDEX idx_context_pairing_status_context ON context_pairing_status (context_name, customer_id);
CREATE INDEX idx_context_pairing_status_health ON context_pairing_status (pairing_status, sync_status, health_status);
CREATE INDEX idx_context_pairing_status_updated ON context_pairing_status (last_updated DESC);

-- App manifest correlation indexes
CREATE INDEX idx_app_manifest_correlation_customer ON app_manifest_correlation (customer_id);
CREATE INDEX idx_app_manifest_correlation_context ON app_manifest_correlation (context_name, customer_id);
CREATE INDEX idx_app_manifest_correlation_app ON app_manifest_correlation (app_name, customer_id);
CREATE INDEX idx_app_manifest_correlation_status ON app_manifest_correlation (sync_status, health_status);
CREATE INDEX idx_app_manifest_correlation_updated ON app_manifest_correlation (last_updated DESC);

-- Environment manifest validation indexes
CREATE INDEX idx_environment_manifest_validation_customer ON environment_manifest_validation (customer_id);
CREATE INDEX idx_environment_manifest_validation_context ON environment_manifest_validation (context_name, customer_id);
CREATE INDEX idx_environment_manifest_validation_env ON environment_manifest_validation (environment_name, customer_id);
CREATE INDEX idx_environment_manifest_validation_status ON environment_manifest_validation (vault_validation_status, cluster_validation_status);
CREATE INDEX idx_environment_manifest_validation_updated ON environment_manifest_validation (last_updated DESC);

-- Context pairing operations indexes
CREATE INDEX idx_context_pairing_operations_customer ON context_pairing_operations (customer_id);
CREATE INDEX idx_context_pairing_operations_context ON context_pairing_operations (context_name, customer_id);
CREATE INDEX idx_context_pairing_operations_correlation ON context_pairing_operations (correlation_id);
CREATE INDEX idx_context_pairing_operations_status ON context_pairing_operations (operation_status);
CREATE INDEX idx_context_pairing_operations_started ON context_pairing_operations (started_at DESC);

-- GitOps vault validation status indexes
CREATE INDEX idx_gitops_vault_validation_customer ON gitops_vault_validation_status (customer_id);
CREATE INDEX idx_gitops_vault_validation_context ON gitops_vault_validation_status (context_name, customer_id);
CREATE INDEX idx_gitops_vault_validation_env ON gitops_vault_validation_status (environment_name, customer_id);
CREATE INDEX idx_gitops_vault_validation_status ON gitops_vault_validation_status (validation_status);
CREATE INDEX idx_gitops_vault_validation_path ON gitops_vault_validation_status (vault_path);

-- Multi-environment app status indexes
CREATE INDEX idx_multi_environment_app_customer ON multi_environment_app_status (customer_id);
CREATE INDEX idx_multi_environment_app_context ON multi_environment_app_status (context_name, customer_id);
CREATE INDEX idx_multi_environment_app_name ON multi_environment_app_status (app_name, customer_id);
CREATE INDEX idx_multi_environment_app_env ON multi_environment_app_status (environment);
CREATE INDEX idx_multi_environment_app_status ON multi_environment_app_status (sync_status, health_status, deployment_status);
CREATE INDEX idx_multi_environment_app_updated ON multi_environment_app_status (last_updated DESC);

-- Customer git branch correlation indexes
CREATE INDEX idx_customer_git_branch_customer ON customer_git_branch_correlation (customer_id);
CREATE INDEX idx_customer_git_branch_context ON customer_git_branch_correlation (context_name, customer_id);
CREATE INDEX idx_customer_git_branch_compliance ON customer_git_branch_correlation (branch_compliance);
CREATE INDEX idx_customer_git_branch_repo ON customer_git_branch_correlation (repository_url);

-- Trigger functions for updated_at columns
CREATE TRIGGER update_context_pairing_status_updated_at BEFORE UPDATE ON context_pairing_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_app_manifest_correlation_updated_at BEFORE UPDATE ON app_manifest_correlation
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_environment_manifest_validation_updated_at BEFORE UPDATE ON environment_manifest_validation
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_context_pairing_operations_updated_at BEFORE UPDATE ON context_pairing_operations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_gitops_vault_validation_status_updated_at BEFORE UPDATE ON gitops_vault_validation_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_multi_environment_app_status_updated_at BEFORE UPDATE ON multi_environment_app_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_customer_git_branch_correlation_updated_at BEFORE UPDATE ON customer_git_branch_correlation
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();