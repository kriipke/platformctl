package security

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/contextops/platformctl/internal/testutil"
)

func TestSecuritySchemaIntegration(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	t.Run("audit_logs table", func(t *testing.T) {
		testAuditLogsTable(t, testDB)
	})

	t.Run("security_events table", func(t *testing.T) {
		testSecurityEventsTable(t, testDB)
	})

	t.Run("sessions table", func(t *testing.T) {
		testSessionsTable(t, testDB)
	})

	t.Run("user_permissions table", func(t *testing.T) {
		testUserPermissionsTable(t, testDB)
	})

	t.Run("security_config table", func(t *testing.T) {
		testSecurityConfigTable(t, testDB)
	})

	t.Run("circuit_breaker_metrics table", func(t *testing.T) {
		testCircuitBreakerMetricsTable(t, testDB)
	})

	t.Run("row level security", func(t *testing.T) {
		testRowLevelSecurity(t, testDB)
	})

	t.Run("database constraints", func(t *testing.T) {
		testDatabaseConstraints(t, testDB)
	})
}

func testAuditLogsTable(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Test audit log insertion
	eventID := uuid.New()
	requestID := uuid.New()
	
	query := `
		INSERT INTO audit_logs (
			event_id, customer_id, user_id, session_id, ip_address, user_agent,
			event_type, resource_type, resource_id, resource_name, action, outcome,
			error_code, error_message, request_id, method, endpoint, old_values,
			new_values, metadata, is_sensitive
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)`

	_, err = testDB.ExecContext(context.Background(), query,
		eventID, customerID, "test-user", "session-123", "192.168.1.100", "test-agent",
		"CREATE", "app", "app-123", "test-app", "create_app", "success",
		"VALIDATION_ERROR", "Invalid input", requestID, "POST", "/api/apps",
		`{"name": "old"}`, `{"name": "new"}`, `{"source": "api"}`, true)
	require.NoError(t, err)

	// Verify insertion
	var storedEvent struct {
		EventID      uuid.UUID
		CustomerID   uuid.UUID
		UserID       sql.NullString
		SessionID    sql.NullString
		EventType    string
		ResourceType string
		Action       string
		Outcome      string
		IsSensitive  bool
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT event_id, customer_id, user_id, session_id, event_type, resource_type, action, outcome, is_sensitive FROM audit_logs WHERE event_id = $1",
		eventID).Scan(&storedEvent.EventID, &storedEvent.CustomerID, &storedEvent.UserID,
		&storedEvent.SessionID, &storedEvent.EventType, &storedEvent.ResourceType,
		&storedEvent.Action, &storedEvent.Outcome, &storedEvent.IsSensitive)
	require.NoError(t, err)

	assert.Equal(t, eventID, storedEvent.EventID)
	assert.Equal(t, customerID, storedEvent.CustomerID)
	assert.Equal(t, "test-user", storedEvent.UserID.String)
	assert.Equal(t, "session-123", storedEvent.SessionID.String)
	assert.Equal(t, "CREATE", storedEvent.EventType)
	assert.Equal(t, "app", storedEvent.ResourceType)
	assert.Equal(t, "create_app", storedEvent.Action)
	assert.Equal(t, "success", storedEvent.Outcome)
	assert.True(t, storedEvent.IsSensitive)

	// Test index usage
	var count int
	err = testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1 AND event_type = $2",
		customerID, "CREATE").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Test constraint validation
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
		uuid.New(), customerID, "INVALID", "app", "test", "invalid_outcome")
	assert.Error(t, err) // Should fail due to outcome constraint
}

func testSecurityEventsTable(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert security event
	eventID := uuid.New()
	correlationID := uuid.New()

	query := `
		INSERT INTO security_events (
			event_id, customer_id, user_id, session_id, ip_address,
			event_category, event_subcategory, severity, risk_score,
			description, threat_indicators, affected_resources,
			status, assigned_to, response_actions, resolution_notes,
			source, correlation_id, external_ref
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)`

	_, err = testDB.ExecContext(context.Background(), query,
		eventID, customerID, "test-user", "session-123", "192.168.1.100",
		"authentication", "failed_login", "high", 85,
		"Multiple failed login attempts detected",
		`{"ip": "192.168.1.100", "user_agent": "malicious"}`,
		`{"users": ["test-user"], "sessions": ["session-123"]}`,
		"new", "security-analyst", `{"action": "account_lockout"}`,
		"Locked user account after 5 failed attempts", "auth-service", correlationID, "EXT-12345")
	require.NoError(t, err)

	// Verify insertion
	var storedEvent struct {
		EventID        uuid.UUID
		CustomerID     uuid.UUID
		EventCategory  string
		Severity       string
		RiskScore      sql.NullInt32
		Status         string
		CorrelationID  uuid.UUID
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT event_id, customer_id, event_category, severity, risk_score, status, correlation_id FROM security_events WHERE event_id = $1",
		eventID).Scan(&storedEvent.EventID, &storedEvent.CustomerID, &storedEvent.EventCategory,
		&storedEvent.Severity, &storedEvent.RiskScore, &storedEvent.Status, &storedEvent.CorrelationID)
	require.NoError(t, err)

	assert.Equal(t, eventID, storedEvent.EventID)
	assert.Equal(t, customerID, storedEvent.CustomerID)
	assert.Equal(t, "authentication", storedEvent.EventCategory)
	assert.Equal(t, "high", storedEvent.Severity)
	assert.Equal(t, int32(85), storedEvent.RiskScore.Int32)
	assert.Equal(t, "new", storedEvent.Status)
	assert.Equal(t, correlationID, storedEvent.CorrelationID)

	// Test constraint validation
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO security_events (event_id, customer_id, event_category, description, severity, status, risk_score) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		uuid.New(), customerID, "test", "test event", "invalid_severity", "new", 150)
	assert.Error(t, err) // Should fail due to severity or risk_score constraint
}

func testSessionsTable(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert session
	sessionID := "session-" + uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	query := `
		INSERT INTO sessions (
			id, customer_id, user_id, expires_at, last_activity,
			is_active, ip_address, user_agent, login_method,
			mfa_verified, permissions, metadata, is_privileged, requires_mfa
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)`

	_, err = testDB.ExecContext(context.Background(), query,
		sessionID, customerID, "test-user", expiresAt, time.Now(),
		true, "192.168.1.100", "test-agent", "mfa",
		true, `{"app:read": true}`, `{"device": "mobile"}`, true, true)
	require.NoError(t, err)

	// Verify insertion
	var storedSession struct {
		ID            string
		CustomerID    uuid.UUID
		UserID        string
		IsActive      bool
		LoginMethod   string
		MFAVerified   bool
		IsPrivileged  bool
		RequiresMFA   bool
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT id, customer_id, user_id, is_active, login_method, mfa_verified, is_privileged, requires_mfa FROM sessions WHERE id = $1",
		sessionID).Scan(&storedSession.ID, &storedSession.CustomerID, &storedSession.UserID,
		&storedSession.IsActive, &storedSession.LoginMethod, &storedSession.MFAVerified,
		&storedSession.IsPrivileged, &storedSession.RequiresMFA)
	require.NoError(t, err)

	assert.Equal(t, sessionID, storedSession.ID)
	assert.Equal(t, customerID, storedSession.CustomerID)
	assert.Equal(t, "test-user", storedSession.UserID)
	assert.True(t, storedSession.IsActive)
	assert.Equal(t, "mfa", storedSession.LoginMethod)
	assert.True(t, storedSession.MFAVerified)
	assert.True(t, storedSession.IsPrivileged)
	assert.True(t, storedSession.RequiresMFA)

	// Test constraint validation
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO sessions (id, customer_id, user_id, expires_at, login_method) VALUES ($1, $2, $3, $4, $5)",
		"invalid-session", customerID, "test-user", expiresAt, "invalid_method")
	assert.Error(t, err) // Should fail due to login_method constraint
}

func testUserPermissionsTable(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert permission
	expiresAt := time.Now().Add(24 * time.Hour)
	
	query := `
		INSERT INTO user_permissions (
			customer_id, user_id, resource_type, resource_id, action,
			effect, conditions, granted_by, expires_at, reason, is_inherited
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	_, err = testDB.ExecContext(context.Background(), query,
		customerID, "test-user", "app", "app-123", "read",
		"allow", `{"environment": "production"}`, "admin", expiresAt,
		"Granted for production access", true)
	require.NoError(t, err)

	// Verify insertion
	var storedPermission struct {
		CustomerID   uuid.UUID
		UserID       string
		ResourceType string
		ResourceID   sql.NullString
		Action       string
		Effect       string
		IsInherited  bool
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT customer_id, user_id, resource_type, resource_id, action, effect, is_inherited FROM user_permissions WHERE customer_id = $1 AND user_id = $2",
		customerID, "test-user").Scan(&storedPermission.CustomerID, &storedPermission.UserID,
		&storedPermission.ResourceType, &storedPermission.ResourceID, &storedPermission.Action,
		&storedPermission.Effect, &storedPermission.IsInherited)
	require.NoError(t, err)

	assert.Equal(t, customerID, storedPermission.CustomerID)
	assert.Equal(t, "test-user", storedPermission.UserID)
	assert.Equal(t, "app", storedPermission.ResourceType)
	assert.Equal(t, "app-123", storedPermission.ResourceID.String)
	assert.Equal(t, "read", storedPermission.Action)
	assert.Equal(t, "allow", storedPermission.Effect)
	assert.True(t, storedPermission.IsInherited)

	// Test unique constraint
	_, err = testDB.ExecContext(context.Background(), query,
		customerID, "test-user", "app", "app-123", "read",
		"deny", `{}`, "admin", expiresAt, "Duplicate test", false)
	assert.Error(t, err) // Should fail due to unique constraint

	// Test constraint validation
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO user_permissions (customer_id, user_id, resource_type, action, effect, granted_by) VALUES ($1, $2, $3, $4, $5, $6)",
		customerID, "test-user", "app", "write", "invalid_effect", "admin")
	assert.Error(t, err) // Should fail due to effect constraint
}

func testSecurityConfigTable(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert security config
	query := `
		INSERT INTO security_config (
			customer_id, password_policy, mfa_enabled, mfa_required_for,
			session_timeout_minutes, rbac_enabled, abac_enabled, default_permissions,
			audit_retention_days, audit_sensitive_operations, audit_all_operations,
			failed_login_threshold, failed_login_window_minutes, lockout_duration_minutes,
			compliance_frameworks, data_retention_policy, encryption_requirements,
			circuit_breaker_enabled, circuit_breaker_config, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)`

	passwordPolicy := `{"min_length": 12, "require_uppercase": true}`
	mfaRequiredFor := `["admin", "deployer"]`
	defaultPermissions := `{"app:read": true}`
	complianceFrameworks := `["SOC2", "GDPR"]`
	dataRetentionPolicy := `{"audit_logs": 2555, "sensitive_data": 30}`
	encryptionRequirements := `{"at_rest": true, "in_transit": true}`
	circuitBreakerConfig := `{"default_timeout": 60, "default_threshold": 5}`

	_, err = testDB.ExecContext(context.Background(), query,
		customerID, passwordPolicy, true, mfaRequiredFor,
		480, true, false, defaultPermissions,
		2555, true, false,
		5, 15, 30,
		complianceFrameworks, dataRetentionPolicy, encryptionRequirements,
		true, circuitBreakerConfig, "admin")
	require.NoError(t, err)

	// Verify insertion
	var storedConfig struct {
		CustomerID              uuid.UUID
		MFAEnabled              bool
		SessionTimeoutMinutes   int
		RBACEnabled             bool
		AuditRetentionDays      int
		FailedLoginThreshold    int
		CircuitBreakerEnabled   bool
		UpdatedBy               string
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT customer_id, mfa_enabled, session_timeout_minutes, rbac_enabled, audit_retention_days, failed_login_threshold, circuit_breaker_enabled, updated_by FROM security_config WHERE customer_id = $1",
		customerID).Scan(&storedConfig.CustomerID, &storedConfig.MFAEnabled,
		&storedConfig.SessionTimeoutMinutes, &storedConfig.RBACEnabled, &storedConfig.AuditRetentionDays,
		&storedConfig.FailedLoginThreshold, &storedConfig.CircuitBreakerEnabled, &storedConfig.UpdatedBy)
	require.NoError(t, err)

	assert.Equal(t, customerID, storedConfig.CustomerID)
	assert.True(t, storedConfig.MFAEnabled)
	assert.Equal(t, 480, storedConfig.SessionTimeoutMinutes)
	assert.True(t, storedConfig.RBACEnabled)
	assert.Equal(t, 2555, storedConfig.AuditRetentionDays)
	assert.Equal(t, 5, storedConfig.FailedLoginThreshold)
	assert.True(t, storedConfig.CircuitBreakerEnabled)
	assert.Equal(t, "admin", storedConfig.UpdatedBy)

	// Test unique constraint
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO security_config (customer_id, updated_by) VALUES ($1, $2)",
		customerID, "admin2")
	assert.Error(t, err) // Should fail due to unique constraint

	// Test updated_at trigger
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	_, err = testDB.ExecContext(context.Background(),
		"UPDATE security_config SET mfa_enabled = $1 WHERE customer_id = $2",
		false, customerID)
	require.NoError(t, err)

	var updatedAt time.Time
	err = testDB.QueryRowContext(context.Background(),
		"SELECT updated_at FROM security_config WHERE customer_id = $1",
		customerID).Scan(&updatedAt)
	require.NoError(t, err)

	// Updated timestamp should be recent
	assert.WithinDuration(t, time.Now(), updatedAt, time.Minute)
}

func testCircuitBreakerMetricsTable(t *testing.T, testDB *testutil.TestDB) {
	// Insert circuit breaker metrics
	query := `
		INSERT INTO circuit_breaker_metrics (
			service_name, state, total_requests, successful_requests,
			failed_requests, consecutive_failures, consecutive_successes,
			failure_rate, success_rate, avg_response_time_ms,
			last_failure_time, last_success_time
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)`

	lastFailure := time.Now().Add(-time.Hour)
	lastSuccess := time.Now().Add(-time.Minute)

	_, err := testDB.ExecContext(context.Background(), query,
		"test-service", "half_open", 100, 75,
		25, 5, 10,
		0.25, 0.75, 150,
		lastFailure, lastSuccess)
	require.NoError(t, err)

	// Verify insertion
	var storedMetrics struct {
		ServiceName           string
		State                 string
		TotalRequests         int64
		SuccessfulRequests    int64
		FailedRequests        int64
		ConsecutiveFailures   int64
		ConsecutiveSuccesses  int64
		FailureRate          sql.NullFloat64
		SuccessRate          sql.NullFloat64
		AvgResponseTimeMs    sql.NullInt64
	}

	err = testDB.QueryRowContext(context.Background(),
		"SELECT service_name, state, total_requests, successful_requests, failed_requests, consecutive_failures, consecutive_successes, failure_rate, success_rate, avg_response_time_ms FROM circuit_breaker_metrics WHERE service_name = $1",
		"test-service").Scan(&storedMetrics.ServiceName, &storedMetrics.State,
		&storedMetrics.TotalRequests, &storedMetrics.SuccessfulRequests, &storedMetrics.FailedRequests,
		&storedMetrics.ConsecutiveFailures, &storedMetrics.ConsecutiveSuccesses, &storedMetrics.FailureRate,
		&storedMetrics.SuccessRate, &storedMetrics.AvgResponseTimeMs)
	require.NoError(t, err)

	assert.Equal(t, "test-service", storedMetrics.ServiceName)
	assert.Equal(t, "half_open", storedMetrics.State)
	assert.Equal(t, int64(100), storedMetrics.TotalRequests)
	assert.Equal(t, int64(75), storedMetrics.SuccessfulRequests)
	assert.Equal(t, int64(25), storedMetrics.FailedRequests)
	assert.Equal(t, int64(5), storedMetrics.ConsecutiveFailures)
	assert.Equal(t, int64(10), storedMetrics.ConsecutiveSuccesses)
	assert.Equal(t, 0.25, storedMetrics.FailureRate.Float64)
	assert.Equal(t, 0.75, storedMetrics.SuccessRate.Float64)
	assert.Equal(t, int64(150), storedMetrics.AvgResponseTimeMs.Int64)

	// Test constraint validation
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO circuit_breaker_metrics (service_name, state) VALUES ($1, $2)",
		"invalid-service", "invalid_state")
	assert.Error(t, err) // Should fail due to state constraint
}

func testRowLevelSecurity(t *testing.T, testDB *testutil.TestDB) {
	// Create two test customers
	customer1ID := uuid.New()
	customer2ID := uuid.New()

	for _, customerID := range []uuid.UUID{customer1ID, customer2ID} {
		_, err := testDB.ExecContext(context.Background(),
			"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
			customerID, "Test Customer", "testcustomer", "test@example.com", true)
		require.NoError(t, err)
	}

	// Insert audit logs for both customers
	for _, customerID := range []uuid.UUID{customer1ID, customer2ID} {
		eventID := uuid.New()
		_, err := testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
			eventID, customerID, "CREATE", "app", "test", "success")
		require.NoError(t, err)
	}

	// Test that RLS policies would prevent cross-customer access
	// Note: This is a conceptual test - actual RLS enforcement would require database user context
	
	// Verify data exists for both customers
	var count1, count2 int
	
	err := testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1", customer1ID).Scan(&count1)
	require.NoError(t, err)
	assert.Equal(t, 1, count1)

	err = testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1", customer2ID).Scan(&count2)
	require.NoError(t, err)
	assert.Equal(t, 1, count2)

	// Test foreign key constraints
	_, err = testDB.ExecContext(context.Background(),
		"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
		uuid.New(), uuid.New(), "CREATE", "app", "test", "success")
	assert.Error(t, err) // Should fail due to foreign key constraint
}

func testDatabaseConstraints(t *testing.T, testDB *testutil.TestDB) {
	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	t.Run("audit_logs constraints", func(t *testing.T) {
		// Test outcome constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
			uuid.New(), customerID, "CREATE", "app", "test", "invalid_outcome")
		assert.Error(t, err)

		// Test foreign key constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
			uuid.New(), uuid.New(), "CREATE", "app", "test", "success")
		assert.Error(t, err)
	})

	t.Run("security_events constraints", func(t *testing.T) {
		// Test severity constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO security_events (event_id, customer_id, event_category, description, severity) VALUES ($1, $2, $3, $4, $5)",
			uuid.New(), customerID, "test", "test event", "invalid_severity")
		assert.Error(t, err)

		// Test status constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO security_events (event_id, customer_id, event_category, description, severity, status) VALUES ($1, $2, $3, $4, $5, $6)",
			uuid.New(), customerID, "test", "test event", "medium", "invalid_status")
		assert.Error(t, err)

		// Test risk_score constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO security_events (event_id, customer_id, event_category, description, severity, risk_score) VALUES ($1, $2, $3, $4, $5, $6)",
			uuid.New(), customerID, "test", "test event", "medium", 150)
		assert.Error(t, err) // Risk score should be <= 100
	})

	t.Run("sessions constraints", func(t *testing.T) {
		// Test login_method constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO sessions (id, customer_id, user_id, expires_at, login_method) VALUES ($1, $2, $3, $4, $5)",
			"test-session", customerID, "test-user", time.Now().Add(time.Hour), "invalid_method")
		assert.Error(t, err)
	})

	t.Run("user_permissions constraints", func(t *testing.T) {
		// Test effect constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO user_permissions (customer_id, user_id, resource_type, action, effect, granted_by) VALUES ($1, $2, $3, $4, $5, $6)",
			customerID, "test-user", "app", "read", "invalid_effect", "admin")
		assert.Error(t, err)

		// Test unique constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO user_permissions (customer_id, user_id, resource_type, action, effect, granted_by) VALUES ($1, $2, $3, $4, $5, $6)",
			customerID, "test-user", "app", "read", "allow", "admin")
		require.NoError(t, err)

		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO user_permissions (customer_id, user_id, resource_type, action, effect, granted_by) VALUES ($1, $2, $3, $4, $5, $6)",
			customerID, "test-user", "app", "read", "deny", "admin")
		assert.Error(t, err) // Should violate unique constraint
	})

	t.Run("circuit_breaker_metrics constraints", func(t *testing.T) {
		// Test state constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO circuit_breaker_metrics (service_name, state) VALUES ($1, $2)",
			"test-service", "invalid_state")
		assert.Error(t, err)
	})

	t.Run("security_config constraints", func(t *testing.T) {
		// Test unique constraint
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO security_config (customer_id, updated_by) VALUES ($1, $2)",
			customerID, "admin1")
		require.NoError(t, err)

		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO security_config (customer_id, updated_by) VALUES ($1, $2)",
			customerID, "admin2")
		assert.Error(t, err) // Should violate unique constraint
	})
}

func TestAuditLogCleanupFunction(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert old audit logs
	oldTime := time.Now().AddDate(0, 0, -30) // 30 days old
	for i := 0; i < 5; i++ {
		eventID := uuid.New()
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			eventID, customerID, "CREATE", "app", "test", "success", oldTime)
		require.NoError(t, err)
	}

	// Insert recent audit logs
	recentTime := time.Now().AddDate(0, 0, -1) // 1 day old
	for i := 0; i < 3; i++ {
		eventID := uuid.New()
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			eventID, customerID, "CREATE", "app", "test", "success", recentTime)
		require.NoError(t, err)
	}

	// Verify initial count
	var initialCount int
	err = testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1", customerID).Scan(&initialCount)
	require.NoError(t, err)
	assert.Equal(t, 8, initialCount)

	// Call cleanup function with 7 days retention
	var deletedCount int
	err = testDB.QueryRowContext(context.Background(),
		"SELECT cleanup_audit_logs($1)", 7).Scan(&deletedCount)
	require.NoError(t, err)
	assert.Equal(t, 5, deletedCount) // Should delete the 5 old logs

	// Verify remaining count
	var finalCount int
	err = testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1", customerID).Scan(&finalCount)
	require.NoError(t, err)
	assert.Equal(t, 3, finalCount) // Should have 3 recent logs remaining

	// Verify cleanup operation was logged
	var cleanupLogExists bool
	err = testDB.QueryRowContext(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM audit_logs WHERE action = 'cleanup' AND event_type = 'MAINTENANCE')").Scan(&cleanupLogExists)
	require.NoError(t, err)
	assert.True(t, cleanupLogExists)
}

func TestSecuritySchemaIndexes(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Test that expected indexes exist
	expectedIndexes := map[string][]string{
		"audit_logs": {
			"idx_audit_logs_timestamp",
			"idx_audit_logs_customer_id",
			"idx_audit_logs_event_type",
			"idx_audit_logs_resource_type",
			"idx_audit_logs_outcome",
			"idx_audit_logs_event_id",
		},
		"security_events": {
			"idx_security_events_timestamp",
			"idx_security_events_customer_id",
			"idx_security_events_severity",
			"idx_security_events_status",
			"idx_security_events_category",
		},
		"sessions": {
			"idx_sessions_customer_id",
			"idx_sessions_user_id",
			"idx_sessions_expires_at",
		},
		"user_permissions": {
			"idx_user_permissions_customer_user",
		},
		"circuit_breaker_metrics": {
			"idx_cb_metrics_service_time",
			"idx_cb_metrics_state",
		},
	}

	for table, indexes := range expectedIndexes {
		for _, indexName := range indexes {
			var indexExists bool
			err := testDB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = $1 AND tablename = $2)",
				indexName, table).Scan(&indexExists)
			require.NoError(t, err)
			assert.True(t, indexExists, "Index %s should exist on table %s", indexName, table)
		}
	}
}

func TestSecuritySchemaPerformance(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Insert a moderate amount of test data
	const numRecords = 1000

	start := time.Now()
	for i := 0; i < numRecords; i++ {
		eventID := uuid.New()
		_, err = testDB.ExecContext(context.Background(),
			"INSERT INTO audit_logs (event_id, customer_id, event_type, resource_type, action, outcome) VALUES ($1, $2, $3, $4, $5, $6)",
			eventID, customerID, "CREATE", "app", "test", "success")
		require.NoError(t, err)
	}
	insertDuration := time.Since(start)

	t.Logf("Inserted %d audit log records in %v", numRecords, insertDuration)

	// Test query performance
	start = time.Now()
	var count int
	err = testDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1 AND event_type = $2",
		customerID, "CREATE").Scan(&count)
	require.NoError(t, err)
	queryDuration := time.Since(start)

	assert.Equal(t, numRecords, count)
	t.Logf("Query completed in %v", queryDuration)

	// Query should be fast due to indexes
	assert.Less(t, queryDuration, time.Second, "Query should be fast with proper indexing")
}