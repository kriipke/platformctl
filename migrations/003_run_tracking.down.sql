-- Drop indexes
DROP INDEX IF EXISTS idx_result_events_completed_at;
DROP INDEX IF EXISTS idx_result_events_status;
DROP INDEX IF EXISTS idx_result_events_customer_context;
DROP INDEX IF EXISTS idx_result_events_service;
DROP INDEX IF EXISTS idx_result_events_correlation;

DROP INDEX IF EXISTS idx_command_runs_manifest_type;
DROP INDEX IF EXISTS idx_command_runs_requested_at;
DROP INDEX IF EXISTS idx_command_runs_status;
DROP INDEX IF EXISTS idx_command_runs_customer_action;
DROP INDEX IF EXISTS idx_command_runs_context;

-- Drop trigger
DROP TRIGGER IF EXISTS update_command_runs_updated_at ON command_runs;

-- Drop tables
DROP TABLE IF EXISTS result_events;
DROP TABLE IF EXISTS command_runs;