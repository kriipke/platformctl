CREATE TABLE contexts (
    name VARCHAR(255) PRIMARY KEY CHECK (name ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$'),
    customer_id VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

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

CREATE INDEX idx_contexts_customer ON contexts (customer_id);
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at);
CREATE INDEX idx_applicationsets_context ON applicationsets (context_name);
CREATE INDEX idx_applicationsets_status ON applicationsets (status, health);
CREATE INDEX idx_environment_status_context_env ON environment_status (context_name, environment);
CREATE INDEX idx_vault_secrets_context ON vault_secrets (context_name, environment);
CREATE INDEX idx_pod_env_validations_status ON pod_env_validations (validation_status, last_checked);

CREATE INDEX idx_contexts_gitops ON contexts USING GIN ((spec->'gitops'));
CREATE INDEX idx_contexts_application ON contexts USING GIN ((spec->'application'));
CREATE INDEX idx_contexts_vault ON contexts USING GIN ((spec->'vault'));

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE ON contexts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_environment_status_updated_at BEFORE UPDATE ON environment_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
