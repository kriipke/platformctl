-- Rollback App+Environment+Context database schema

-- Drop indexes first
DROP INDEX IF EXISTS idx_customer_branches_status;
DROP INDEX IF EXISTS idx_customer_branches_customer;
DROP INDEX IF EXISTS idx_pod_env_validations_status;
DROP INDEX IF EXISTS idx_pod_env_validations_context;
DROP INDEX IF EXISTS idx_context_deployments_status;
DROP INDEX IF EXISTS idx_context_deployments_environment;
DROP INDEX IF EXISTS idx_context_deployments_context;
DROP INDEX IF EXISTS idx_cluster_configs_status;
DROP INDEX IF EXISTS idx_cluster_configs_environment;
DROP INDEX IF EXISTS idx_vault_static_secrets_status;
DROP INDEX IF EXISTS idx_vault_static_secrets_environment;
DROP INDEX IF EXISTS idx_vault_sources_customer;
DROP INDEX IF EXISTS idx_vault_sources_environment;
DROP INDEX IF EXISTS idx_helm_sources_type;
DROP INDEX IF EXISTS idx_helm_sources_app;
DROP INDEX IF EXISTS idx_applicationsets_customer;
DROP INDEX IF EXISTS idx_applicationsets_status;
DROP INDEX IF EXISTS idx_applicationsets_app;
DROP INDEX IF EXISTS idx_contexts_updated_at;
DROP INDEX IF EXISTS idx_contexts_environment_ref;
DROP INDEX IF EXISTS idx_contexts_app_ref;
DROP INDEX IF EXISTS idx_contexts_customer;
DROP INDEX IF EXISTS idx_environments_updated_at;
DROP INDEX IF EXISTS idx_environments_customer;
DROP INDEX IF EXISTS idx_apps_updated_at;
DROP INDEX IF EXISTS idx_apps_customer;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS customer_branches;
DROP TABLE IF EXISTS pod_env_validations;
DROP TABLE IF EXISTS context_deployments;
DROP TABLE IF EXISTS cluster_configs;
DROP TABLE IF EXISTS vault_static_secrets;
DROP TABLE IF EXISTS vault_sources;
DROP TABLE IF EXISTS helm_sources;
DROP TABLE IF EXISTS applicationsets;
DROP TABLE IF EXISTS contexts;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS customers;