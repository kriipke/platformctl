-- Migration 002: Security and Audit Schema
-- Enhances the database with comprehensive security and audit logging capabilities

-- Create audit_logs table for comprehensive event tracking
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- User and authentication context
    user_id VARCHAR(255), -- Can be null for system events
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    session_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    
    -- Event details
    event_type VARCHAR(100) NOT NULL, -- CREATE, UPDATE, DELETE, READ, AUTH, etc.
    resource_type VARCHAR(100) NOT NULL, -- app, environment, context, user, etc.
    resource_id VARCHAR(255), -- ID of the affected resource
    resource_name VARCHAR(255), -- Name of the affected resource
    
    -- Action and outcome
    action VARCHAR(255) NOT NULL, -- Specific action taken
    outcome VARCHAR(50) NOT NULL DEFAULT 'success', -- success, failure, error
    error_code VARCHAR(100), -- Error code if applicable
    error_message TEXT, -- Error details if applicable
    
    -- Additional context
    request_id UUID, -- Correlation ID for request tracing
    method VARCHAR(10), -- HTTP method for API calls
    endpoint VARCHAR(500), -- API endpoint called
    
    -- Data changes (for sensitive operations)
    old_values JSONB, -- Previous state (for updates/deletes)
    new_values JSONB, -- New state (for creates/updates)
    metadata JSONB, -- Additional context-specific data
    
    -- Compliance and retention
    retention_policy VARCHAR(50) DEFAULT 'standard', -- Retention classification
    is_sensitive BOOLEAN DEFAULT FALSE, -- Contains sensitive data flag
    
    CONSTRAINT valid_outcome CHECK (outcome IN ('success', 'failure', 'error', 'pending'))
);

-- Create indexes for audit_logs
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_customer_id ON audit_logs(customer_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX idx_audit_logs_outcome ON audit_logs(outcome);
CREATE INDEX idx_audit_logs_session_id ON audit_logs(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_audit_logs_request_id ON audit_logs(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX idx_audit_logs_event_id ON audit_logs(event_id);

-- Composite indexes for common queries
CREATE INDEX idx_audit_logs_customer_resource ON audit_logs(customer_id, resource_type, timestamp DESC);
CREATE INDEX idx_audit_logs_user_event ON audit_logs(user_id, event_type, timestamp DESC) WHERE user_id IS NOT NULL;

-- Create security_events table for security-specific logging
CREATE TABLE IF NOT EXISTS security_events (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL DEFAULT gen_random_uuid(),
    audit_log_id BIGINT REFERENCES audit_logs(id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Security context
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    user_id VARCHAR(255),
    session_id VARCHAR(255),
    ip_address INET,
    
    -- Security event classification
    event_category VARCHAR(100) NOT NULL, -- authentication, authorization, data_access, etc.
    event_subcategory VARCHAR(100), -- login, logout, permission_denied, etc.
    severity VARCHAR(20) NOT NULL DEFAULT 'medium', -- low, medium, high, critical
    risk_score INTEGER CHECK (risk_score >= 0 AND risk_score <= 100),
    
    -- Event details
    description TEXT NOT NULL,
    threat_indicators JSONB, -- IOCs, patterns, etc.
    affected_resources JSONB, -- Resources involved in the security event
    
    -- Response and investigation
    status VARCHAR(50) DEFAULT 'new', -- new, investigating, resolved, false_positive
    assigned_to VARCHAR(255), -- Security analyst assigned
    response_actions JSONB, -- Actions taken in response
    resolution_notes TEXT,
    resolved_at TIMESTAMPTZ,
    
    -- Additional context
    source VARCHAR(100), -- Source system/component that detected the event
    correlation_id UUID, -- For grouping related events
    external_ref VARCHAR(255), -- Reference to external security tools
    
    CONSTRAINT valid_severity CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT valid_status CHECK (status IN ('new', 'investigating', 'resolved', 'false_positive'))
);

-- Create indexes for security_events
CREATE INDEX idx_security_events_timestamp ON security_events(timestamp DESC);
CREATE INDEX idx_security_events_customer_id ON security_events(customer_id);
CREATE INDEX idx_security_events_user_id ON security_events(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_security_events_severity ON security_events(severity);
CREATE INDEX idx_security_events_status ON security_events(status);
CREATE INDEX idx_security_events_category ON security_events(event_category);
CREATE INDEX idx_security_events_correlation ON security_events(correlation_id) WHERE correlation_id IS NOT NULL;
CREATE INDEX idx_security_events_risk_score ON security_events(risk_score) WHERE risk_score IS NOT NULL;

-- Create sessions table for session management
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    
    -- Session details
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_activity TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,
    
    -- Security context
    ip_address INET,
    user_agent TEXT,
    login_method VARCHAR(50), -- password, mfa, sso, etc.
    mfa_verified BOOLEAN DEFAULT FALSE,
    
    -- Session data
    permissions JSONB, -- Cached permissions for performance
    metadata JSONB, -- Additional session context
    
    -- Security flags
    is_privileged BOOLEAN DEFAULT FALSE, -- High privilege session
    requires_mfa BOOLEAN DEFAULT FALSE, -- MFA required for sensitive operations
    
    CONSTRAINT valid_login_method CHECK (login_method IN ('password', 'mfa', 'sso', 'api_key', 'service_account'))
);

-- Create indexes for sessions
CREATE INDEX idx_sessions_customer_id ON sessions(customer_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_is_active ON sessions(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_sessions_last_activity ON sessions(last_activity DESC);

-- Create user_permissions table for fine-grained access control
CREATE TABLE IF NOT EXISTS user_permissions (
    id BIGSERIAL PRIMARY KEY,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    
    -- Permission details
    resource_type VARCHAR(100) NOT NULL, -- app, environment, context, system
    resource_id VARCHAR(255), -- Specific resource ID (NULL for all resources of type)
    action VARCHAR(100) NOT NULL, -- create, read, update, delete, deploy, etc.
    effect VARCHAR(10) NOT NULL DEFAULT 'allow', -- allow, deny
    
    -- Conditions and constraints
    conditions JSONB, -- Additional conditions (environment, time, etc.)
    granted_by VARCHAR(255), -- Who granted this permission
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ, -- Optional expiration
    
    -- Metadata
    reason TEXT, -- Justification for the permission
    is_inherited BOOLEAN DEFAULT FALSE, -- Inherited from role/group
    
    CONSTRAINT valid_effect CHECK (effect IN ('allow', 'deny')),
    UNIQUE(customer_id, user_id, resource_type, resource_id, action)
);

-- Create indexes for user_permissions
CREATE INDEX idx_user_permissions_customer_user ON user_permissions(customer_id, user_id);
CREATE INDEX idx_user_permissions_resource ON user_permissions(resource_type, resource_id) WHERE resource_id IS NOT NULL;
CREATE INDEX idx_user_permissions_expires ON user_permissions(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_user_permissions_effect ON user_permissions(effect);

-- Create circuit_breaker_metrics table for monitoring resilience patterns
CREATE TABLE IF NOT EXISTS circuit_breaker_metrics (
    id BIGSERIAL PRIMARY KEY,
    service_name VARCHAR(100) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Circuit breaker state
    state VARCHAR(20) NOT NULL, -- closed, open, half_open
    
    -- Metrics
    total_requests BIGINT DEFAULT 0,
    successful_requests BIGINT DEFAULT 0,
    failed_requests BIGINT DEFAULT 0,
    consecutive_failures BIGINT DEFAULT 0,
    consecutive_successes BIGINT DEFAULT 0,
    
    -- Rates
    failure_rate DECIMAL(5,4), -- 0.0000 to 1.0000
    success_rate DECIMAL(5,4), -- 0.0000 to 1.0000
    
    -- Timing
    avg_response_time_ms BIGINT,
    last_failure_time TIMESTAMPTZ,
    last_success_time TIMESTAMPTZ,
    
    CONSTRAINT valid_cb_state CHECK (state IN ('closed', 'open', 'half_open'))
);

-- Create indexes for circuit_breaker_metrics
CREATE INDEX idx_cb_metrics_service_time ON circuit_breaker_metrics(service_name, timestamp DESC);
CREATE INDEX idx_cb_metrics_state ON circuit_breaker_metrics(state);
CREATE INDEX idx_cb_metrics_failure_rate ON circuit_breaker_metrics(failure_rate DESC) WHERE failure_rate IS NOT NULL;

-- Create configuration table for security settings
CREATE TABLE IF NOT EXISTS security_config (
    id BIGSERIAL PRIMARY KEY,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    
    -- Authentication settings
    password_policy JSONB, -- Password requirements
    mfa_enabled BOOLEAN DEFAULT TRUE,
    mfa_required_for JSONB, -- List of operations requiring MFA
    session_timeout_minutes INTEGER DEFAULT 480, -- 8 hours
    
    -- Authorization settings
    rbac_enabled BOOLEAN DEFAULT TRUE,
    abac_enabled BOOLEAN DEFAULT FALSE,
    default_permissions JSONB, -- Default permissions for new users
    
    -- Audit settings
    audit_retention_days INTEGER DEFAULT 2555, -- 7 years for compliance
    audit_sensitive_operations BOOLEAN DEFAULT TRUE,
    audit_all_operations BOOLEAN DEFAULT FALSE,
    
    -- Security monitoring
    failed_login_threshold INTEGER DEFAULT 5,
    failed_login_window_minutes INTEGER DEFAULT 15,
    lockout_duration_minutes INTEGER DEFAULT 30,
    
    -- Compliance settings
    compliance_frameworks JSONB, -- SOC2, GDPR, etc.
    data_retention_policy JSONB,
    encryption_requirements JSONB,
    
    -- Circuit breaker settings
    circuit_breaker_enabled BOOLEAN DEFAULT TRUE,
    circuit_breaker_config JSONB, -- Per-service CB configuration
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    
    UNIQUE(customer_id)
);

-- Create index for security_config
CREATE INDEX idx_security_config_customer ON security_config(customer_id);

-- Enable Row Level Security (RLS) for multi-tenant isolation
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_config ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for audit_logs
CREATE POLICY audit_logs_customer_isolation ON audit_logs
    FOR ALL
    TO authenticated_users
    USING (customer_id = current_setting('app.current_customer_id')::UUID);

-- Create RLS policies for security_events
CREATE POLICY security_events_customer_isolation ON security_events
    FOR ALL
    TO authenticated_users
    USING (customer_id = current_setting('app.current_customer_id')::UUID);

-- Create RLS policies for sessions
CREATE POLICY sessions_customer_isolation ON sessions
    FOR ALL
    TO authenticated_users
    USING (customer_id = current_setting('app.current_customer_id')::UUID);

-- Create RLS policies for user_permissions
CREATE POLICY user_permissions_customer_isolation ON user_permissions
    FOR ALL
    TO authenticated_users
    USING (customer_id = current_setting('app.current_customer_id')::UUID);

-- Create RLS policies for security_config
CREATE POLICY security_config_customer_isolation ON security_config
    FOR ALL
    TO authenticated_users
    USING (customer_id = current_setting('app.current_customer_id')::UUID);

-- Create a function to automatically update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add trigger to automatically update updated_at for security_config
CREATE TRIGGER update_security_config_updated_at
    BEFORE UPDATE ON security_config
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Create function for audit log cleanup (for compliance/retention)
CREATE OR REPLACE FUNCTION cleanup_audit_logs(retention_days INTEGER DEFAULT 2555)
RETURNS BIGINT AS $$
DECLARE
    deleted_count BIGINT;
BEGIN
    DELETE FROM audit_logs 
    WHERE timestamp < NOW() - (retention_days || ' days')::INTERVAL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    -- Log the cleanup operation
    INSERT INTO audit_logs (
        customer_id, 
        event_type, 
        resource_type, 
        action, 
        outcome,
        metadata
    ) VALUES (
        '00000000-0000-0000-0000-000000000000'::UUID, -- System customer ID
        'MAINTENANCE',
        'audit_log',
        'cleanup',
        'success',
        jsonb_build_object('deleted_count', deleted_count, 'retention_days', retention_days)
    );
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;