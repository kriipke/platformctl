-- Table to track command runs and their correlation IDs
CREATE TABLE command_runs (
    correlation_id UUID PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    manifest_type VARCHAR(50) NOT NULL,
    app_name VARCHAR(255),
    environment_name VARCHAR(255),
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- pending, in_progress, completed, failed
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    result_payload JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (context_name, customer_id) REFERENCES contexts(name, customer_id) ON DELETE CASCADE
);

-- Table to track result events from GitOps services
CREATE TABLE result_events (
    id SERIAL PRIMARY KEY,
    correlation_id UUID REFERENCES command_runs(correlation_id) ON DELETE CASCADE,
    service_name VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    manifest_type VARCHAR(50) NOT NULL,
    app_manifest_data JSONB,
    environment_manifest_data JSONB,
    context_pairing_data JSONB,
    performance_metrics JSONB,
    error_message TEXT,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_command_runs_context ON command_runs (context_name, customer_id);
CREATE INDEX idx_command_runs_customer_action ON command_runs (customer_id, action);
CREATE INDEX idx_command_runs_status ON command_runs (status);
CREATE INDEX idx_command_runs_requested_at ON command_runs (requested_at DESC);
CREATE INDEX idx_command_runs_manifest_type ON command_runs (manifest_type);

CREATE INDEX idx_result_events_correlation ON result_events (correlation_id);
CREATE INDEX idx_result_events_service ON result_events (service_name);
CREATE INDEX idx_result_events_customer_context ON result_events (customer_id, context_name);
CREATE INDEX idx_result_events_status ON result_events (status);
CREATE INDEX idx_result_events_completed_at ON result_events (completed_at DESC);

-- Trigger function for updated_at on command_runs
CREATE TRIGGER update_command_runs_updated_at BEFORE UPDATE ON command_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();