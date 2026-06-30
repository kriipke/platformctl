package audit

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kriipke/platformctl/internal/testutil"
)

func TestNewPostgresLogger(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	assert.NoError(t, err)
	assert.NotNil(t, logger)

	err = logger.Close()
	assert.NoError(t, err)
}

func TestNewPostgresLoggerWithNilDB(t *testing.T) {
	logger, err := NewPostgresLogger(nil)
	assert.Error(t, err)
	assert.Nil(t, logger)
	assert.Contains(t, err.Error(), "database connection is required")
}

func TestPostgresLoggerLogEvent(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create a test customer first
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create test audit event
	ip := net.ParseIP("192.168.1.1")
	userAgent := "test-agent"
	userID := "test-user"
	sessionID := "test-session"
	requestID := uuid.New()
	method := "POST"
	endpoint := "/api/test"
	errorCode := "VALIDATION_ERROR"
	errorMessage := "Invalid input"

	event := &AuditEvent{
		EventID:       uuid.New(),
		Timestamp:     time.Now().UTC(),
		UserID:        &userID,
		CustomerID:    customerID,
		SessionID:     &sessionID,
		IPAddress:     &ip,
		UserAgent:     &userAgent,
		EventType:     EventTypeCreate,
		ResourceType:  ResourceTypeApp,
		ResourceID:    stringPtr("app-123"),
		ResourceName:  stringPtr("test-app"),
		Action:        "create_app",
		Outcome:       OutcomeError,
		ErrorCode:     &errorCode,
		ErrorMessage:  &errorMessage,
		RequestID:     &requestID,
		Method:        &method,
		Endpoint:      &endpoint,
		OldValues:     map[string]interface{}{"name": "old-name"},
		NewValues:     map[string]interface{}{"name": "new-name"},
		Metadata:      map[string]interface{}{"source": "api"},
		IsSensitive:   true,
	}

	// Log the event
	err = logger.LogEvent(context.Background(), event)
	assert.NoError(t, err)

	// Verify the event was logged
	var storedEvent AuditEvent
	var oldValuesJSON, newValuesJSON, metadataJSON sql.NullString
	
	err = testDB.QueryRowContext(context.Background(), `
		SELECT event_id, timestamp, user_id, customer_id, session_id, ip_address, user_agent,
			   event_type, resource_type, resource_id, resource_name, action, outcome,
			   error_code, error_message, request_id, method, endpoint,
			   old_values, new_values, metadata, is_sensitive
		FROM audit_logs WHERE event_id = $1`, event.EventID).Scan(
		&storedEvent.EventID,
		&storedEvent.Timestamp,
		&storedEvent.UserID,
		&storedEvent.CustomerID,
		&storedEvent.SessionID,
		&storedEvent.IPAddress,
		&storedEvent.UserAgent,
		&storedEvent.EventType,
		&storedEvent.ResourceType,
		&storedEvent.ResourceID,
		&storedEvent.ResourceName,
		&storedEvent.Action,
		&storedEvent.Outcome,
		&storedEvent.ErrorCode,
		&storedEvent.ErrorMessage,
		&storedEvent.RequestID,
		&storedEvent.Method,
		&storedEvent.Endpoint,
		&oldValuesJSON,
		&newValuesJSON,
		&metadataJSON,
		&storedEvent.IsSensitive,
	)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, event.EventID, storedEvent.EventID)
	assert.Equal(t, event.UserID, storedEvent.UserID)
	assert.Equal(t, event.CustomerID, storedEvent.CustomerID)
	assert.Equal(t, event.SessionID, storedEvent.SessionID)
	assert.Equal(t, event.IPAddress, storedEvent.IPAddress)
	assert.Equal(t, event.UserAgent, storedEvent.UserAgent)
	assert.Equal(t, event.EventType, storedEvent.EventType)
	assert.Equal(t, event.ResourceType, storedEvent.ResourceType)
	assert.Equal(t, event.ResourceID, storedEvent.ResourceID)
	assert.Equal(t, event.ResourceName, storedEvent.ResourceName)
	assert.Equal(t, event.Action, storedEvent.Action)
	assert.Equal(t, event.Outcome, storedEvent.Outcome)
	assert.Equal(t, event.ErrorCode, storedEvent.ErrorCode)
	assert.Equal(t, event.ErrorMessage, storedEvent.ErrorMessage)
	assert.Equal(t, event.RequestID, storedEvent.RequestID)
	assert.Equal(t, event.Method, storedEvent.Method)
	assert.Equal(t, event.Endpoint, storedEvent.Endpoint)
	assert.Equal(t, event.IsSensitive, storedEvent.IsSensitive)
	assert.WithinDuration(t, event.Timestamp, storedEvent.Timestamp, time.Second)

	// Verify JSON fields were stored correctly
	assert.True(t, oldValuesJSON.Valid)
	assert.Contains(t, oldValuesJSON.String, "old-name")
	assert.True(t, newValuesJSON.Valid)
	assert.Contains(t, newValuesJSON.String, "new-name")
	assert.True(t, metadataJSON.Valid)
	assert.Contains(t, metadataJSON.String, "api")
}

func TestPostgresLoggerLogCRUDEvent(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create a test customer first
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create context with audit information
	userID := "test-user"
	ctx := context.WithValue(context.Background(), "user_id", userID)
	ctx = context.WithValue(ctx, "customer_id", customerID)

	oldValues := map[string]interface{}{"version": "1.0.0"}
	newValues := map[string]interface{}{"version": "1.1.0"}

	err = logger.LogCRUDEvent(
		ctx,
		EventTypeUpdate,
		ResourceTypeApp,
		"app-123",
		"test-app",
		oldValues,
		newValues,
	)
	assert.NoError(t, err)

	// Verify the event was logged
	var count int
	err = testDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM audit_logs 
		WHERE event_type = $1 AND resource_type = $2 AND resource_id = $3`,
		EventTypeUpdate, ResourceTypeApp, "app-123").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPostgresLoggerLogAuthEvent(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create a test customer first
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create context with audit information
	ctx := context.WithValue(context.Background(), "customer_id", customerID)
	ctx = context.WithValue(ctx, "user_id", "test-user")

	metadata := map[string]interface{}{
		"login_method": "password",
		"ip_address":   "192.168.1.1",
	}

	err = logger.LogAuthEvent(ctx, "login", OutcomeSuccess, metadata)
	assert.NoError(t, err)

	// Verify the event was logged
	var count int
	err = testDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM audit_logs 
		WHERE event_type = $1 AND action = $2 AND outcome = $3`,
		EventTypeAuth, "login", OutcomeSuccess).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPostgresLoggerLogSystemEvent(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create a test customer first (system events still need a customer context)
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "customer_id", customerID)

	metadata := map[string]interface{}{
		"component": "database",
		"version":   "1.0.0",
	}

	err = logger.LogSystemEvent(ctx, "startup", OutcomeSuccess, metadata)
	assert.NoError(t, err)

	// Verify the event was logged
	var count int
	err = testDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM audit_logs 
		WHERE event_type = $1 AND action = $2`,
		EventTypeSystem, "startup").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPostgresLoggerQueryEvents(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create test customers
	customer1 := uuid.New()
	customer2 := uuid.New()
	
	for _, id := range []uuid.UUID{customer1, customer2} {
		_, err = testDB.ExecContext(context.Background(), 
			"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
			id, "Test Customer", "testcustomer", "test@example.com", true)
		require.NoError(t, err)
	}

	// Create test events
	now := time.Now().UTC()
	events := []*AuditEvent{
		{
			EventID:      uuid.New(),
			Timestamp:    now.Add(-2 * time.Hour),
			CustomerID:   customer1,
			EventType:    EventTypeCreate,
			ResourceType: ResourceTypeApp,
			Action:       "create",
			Outcome:      OutcomeSuccess,
		},
		{
			EventID:      uuid.New(),
			Timestamp:    now.Add(-1 * time.Hour),
			CustomerID:   customer1,
			EventType:    EventTypeUpdate,
			ResourceType: ResourceTypeApp,
			Action:       "update",
			Outcome:      OutcomeSuccess,
		},
		{
			EventID:      uuid.New(),
			Timestamp:    now,
			CustomerID:   customer2,
			EventType:    EventTypeDelete,
			ResourceType: ResourceTypeEnvironment,
			Action:       "delete",
			Outcome:      OutcomeFailure,
		},
	}

	// Insert test events
	for _, event := range events {
		err = logger.LogEvent(context.Background(), event)
		require.NoError(t, err)
	}

	tests := []struct {
		name     string
		filter   *EventFilter
		expected int
	}{
		{
			name:     "no filter",
			filter:   &EventFilter{},
			expected: 3,
		},
		{
			name: "filter by customer",
			filter: &EventFilter{
				CustomerID: &customer1,
			},
			expected: 2,
		},
		{
			name: "filter by event type",
			filter: &EventFilter{
				EventTypes: []EventType{EventTypeCreate, EventTypeDelete},
			},
			expected: 2,
		},
		{
			name: "filter by resource type",
			filter: &EventFilter{
				ResourceTypes: []ResourceType{ResourceTypeApp},
			},
			expected: 2,
		},
		{
			name: "filter by outcome",
			filter: &EventFilter{
				Outcomes: []Outcome{OutcomeSuccess},
			},
			expected: 2,
		},
		{
			name: "filter by time range",
			filter: &EventFilter{
				StartTime: timePtr(now.Add(-90 * time.Minute)),
				EndTime:   timePtr(now.Add(-30 * time.Minute)),
			},
			expected: 1,
		},
		{
			name: "filter with limit",
			filter: &EventFilter{
				Limit: 2,
			},
			expected: 2,
		},
		{
			name: "filter with offset",
			filter: &EventFilter{
				Offset: 1,
			},
			expected: 2,
		},
		{
			name: "combined filters",
			filter: &EventFilter{
				CustomerID:    &customer1,
				EventTypes:    []EventType{EventTypeCreate, EventTypeUpdate},
				ResourceTypes: []ResourceType{ResourceTypeApp},
				Outcomes:      []Outcome{OutcomeSuccess},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := logger.QueryEvents(context.Background(), tt.filter)
			assert.NoError(t, err)
			assert.Len(t, results, tt.expected)

			// Verify results are ordered by timestamp desc
			for i := 1; i < len(results); i++ {
				assert.True(t, 
					results[i-1].Timestamp.After(results[i].Timestamp) || 
					results[i-1].Timestamp.Equal(results[i].Timestamp),
					"Results should be ordered by timestamp DESC")
			}
		})
	}
}

func TestPostgresLoggerQueryEventsWithPagination(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create test customer
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create 10 test events
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		event := &AuditEvent{
			EventID:      uuid.New(),
			Timestamp:    now.Add(-time.Duration(i) * time.Minute),
			CustomerID:   customerID,
			EventType:    EventTypeCreate,
			ResourceType: ResourceTypeApp,
			Action:       "create",
			Outcome:      OutcomeSuccess,
		}
		err = logger.LogEvent(context.Background(), event)
		require.NoError(t, err)
	}

	// Test pagination
	page1, err := logger.QueryEvents(context.Background(), &EventFilter{
		CustomerID: &customerID,
		Limit:      3,
		Offset:     0,
	})
	assert.NoError(t, err)
	assert.Len(t, page1, 3)

	page2, err := logger.QueryEvents(context.Background(), &EventFilter{
		CustomerID: &customerID,
		Limit:      3,
		Offset:     3,
	})
	assert.NoError(t, err)
	assert.Len(t, page2, 3)

	// Verify no overlap between pages
	for _, e1 := range page1 {
		for _, e2 := range page2 {
			assert.NotEqual(t, e1.EventID, e2.EventID, "Pages should not overlap")
		}
	}
}

func TestPostgresLoggerQueryEventsError(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Close the database to force an error
	logger.Close()

	_, err = logger.QueryEvents(context.Background(), &EventFilter{})
	assert.Error(t, err)
}

func TestPostgresLoggerConcurrentWrites(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create test customer
	customerID := uuid.New()
	_, err = testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	const numGoroutines = 10
	const eventsPerGoroutine = 5

	// Use channels to coordinate concurrent writes
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &AuditEvent{
					EventID:      uuid.New(),
					Timestamp:    time.Now().UTC(),
					CustomerID:   customerID,
					EventType:    EventTypeCreate,
					ResourceType: ResourceTypeApp,
					Action:       "create",
					Outcome:      OutcomeSuccess,
				}
				
				if err := logger.LogEvent(context.Background(), event); err != nil {
					errCh <- err
					return
				}
			}
			errCh <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}

	// Verify all events were written
	var count int
	err = testDB.QueryRowContext(context.Background(), 
		"SELECT COUNT(*) FROM audit_logs WHERE customer_id = $1", 
		customerID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*eventsPerGoroutine, count)
}

func TestPostgresLoggerTransactionRollback(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	logger, err := NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)
	defer logger.Close()

	// Create invalid event (missing customer_id)
	event := &AuditEvent{
		EventID:      uuid.New(),
		Timestamp:    time.Now().UTC(),
		CustomerID:   uuid.Nil, // Invalid - should cause foreign key error
		EventType:    EventTypeCreate,
		ResourceType: ResourceTypeApp,
		Action:       "create",
		Outcome:      OutcomeSuccess,
	}

	// This should fail due to foreign key constraint
	err = logger.LogEvent(context.Background(), event)
	assert.Error(t, err)

	// Verify no partial data was written
	var count int
	err = testDB.QueryRowContext(context.Background(), 
		"SELECT COUNT(*) FROM audit_logs WHERE event_id = $1", 
		event.EventID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}