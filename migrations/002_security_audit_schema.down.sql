-- Migration 002 Rollback: Remove Security and Audit Schema

-- Drop the cleanup function
DROP FUNCTION IF EXISTS cleanup_audit_logs(INTEGER);

-- Drop the trigger and function for updated_at
DROP TRIGGER IF EXISTS update_security_config_updated_at ON security_config;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop RLS policies
DROP POLICY IF EXISTS audit_logs_customer_isolation ON audit_logs;
DROP POLICY IF EXISTS security_events_customer_isolation ON security_events;
DROP POLICY IF EXISTS sessions_customer_isolation ON sessions;
DROP POLICY IF EXISTS user_permissions_customer_isolation ON user_permissions;
DROP POLICY IF EXISTS security_config_customer_isolation ON security_config;

-- Disable Row Level Security
ALTER TABLE IF EXISTS audit_logs DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS security_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS sessions DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS user_permissions DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS security_config DISABLE ROW LEVEL SECURITY;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS security_config;
DROP TABLE IF EXISTS circuit_breaker_metrics;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS audit_logs;