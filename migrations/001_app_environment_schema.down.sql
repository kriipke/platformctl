DROP TRIGGER IF EXISTS update_environment_status_updated_at ON environment_status;
DROP TRIGGER IF EXISTS update_contexts_updated_at ON contexts;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP INDEX IF EXISTS idx_contexts_vault;
DROP INDEX IF EXISTS idx_contexts_application;
DROP INDEX IF EXISTS idx_contexts_gitops;
DROP INDEX IF EXISTS idx_pod_env_validations_status;
DROP INDEX IF EXISTS idx_vault_secrets_context;
DROP INDEX IF EXISTS idx_environment_status_context_env;
DROP INDEX IF EXISTS idx_applicationsets_status;
DROP INDEX IF EXISTS idx_applicationsets_context;
DROP INDEX IF EXISTS idx_contexts_updated_at;
DROP INDEX IF EXISTS idx_contexts_customer;

DROP TABLE IF EXISTS pod_env_validations;
DROP TABLE IF EXISTS vault_secrets;
DROP TABLE IF EXISTS environment_status;
DROP TABLE IF EXISTS applicationsets;
DROP TABLE IF EXISTS contexts;
