package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PostgresLogger implements the Logger interface using PostgreSQL
type PostgresLogger struct {
	db *sql.DB
}

// NewPostgresLogger creates a new PostgreSQL audit logger
func NewPostgresLogger(db *sql.DB) *PostgresLogger {
	return &PostgresLogger{db: db}
}

// LogEvent logs an audit event to the database
func (l *PostgresLogger) LogEvent(ctx context.Context, event *AuditEvent) error {
	query := `
		INSERT INTO audit_logs (
			event_id, timestamp, user_id, customer_id, session_id, ip_address, user_agent,
			event_type, resource_type, resource_id, resource_name, action, outcome,
			error_code, error_message, request_id, method, endpoint, old_values, new_values,
			metadata, is_sensitive
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING id`

	err := l.db.QueryRowContext(ctx, query,
		event.EventID,
		event.Timestamp,
		event.UserID,
		event.CustomerID,
		event.SessionID,
		event.IPAddress,
		event.UserAgent,
		event.EventType,
		event.ResourceType,
		event.ResourceID,
		event.ResourceName,
		event.Action,
		event.Outcome,
		event.ErrorCode,
		event.ErrorMessage,
		event.RequestID,
		event.Method,
		event.Endpoint,
		event.OldValues,
		event.NewValues,
		event.Metadata,
		event.IsSensitive,
	).Scan(&event.ID)

	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

// LogCRUDEvent logs a CRUD operation audit event
func (l *PostgresLogger) LogCRUDEvent(ctx context.Context, eventType EventType, resourceType ResourceType, resourceID, resourceName string, oldValues, newValues map[string]interface{}) error {
	auditCtx := ExtractAuditContext(ctx, nil)
	
	event := NewAuditEvent(auditCtx, eventType, resourceType, string(eventType), OutcomeSuccess)
	event.WithResource(resourceID, resourceName)
	event.WithValues(oldValues, newValues)

	// Mark as sensitive if dealing with credentials, tokens, etc.
	if containsSensitiveData(oldValues) || containsSensitiveData(newValues) {
		event.WithSensitive(true)
	}

	return l.LogEvent(ctx, event)
}

// LogAuthEvent logs an authentication-related audit event
func (l *PostgresLogger) LogAuthEvent(ctx context.Context, action string, outcome Outcome, metadata map[string]interface{}) error {
	auditCtx := ExtractAuditContext(ctx, nil)
	
	event := NewAuditEvent(auditCtx, EventTypeAuth, ResourceTypeUser, action, outcome)
	
	// Add metadata
	for key, value := range metadata {
		event.WithMetadata(key, value)
	}

	// Authentication events are typically sensitive
	event.WithSensitive(true)

	return l.LogEvent(ctx, event)
}

// LogSystemEvent logs a system-related audit event
func (l *PostgresLogger) LogSystemEvent(ctx context.Context, action string, outcome Outcome, metadata map[string]interface{}) error {
	auditCtx := ExtractAuditContext(ctx, nil)
	
	event := NewAuditEvent(auditCtx, EventTypeSystem, ResourceTypeSystem, action, outcome)
	
	// Add metadata
	for key, value := range metadata {
		event.WithMetadata(key, value)
	}

	return l.LogEvent(ctx, event)
}

// QueryEvents queries audit events based on filter criteria
func (l *PostgresLogger) QueryEvents(ctx context.Context, filter *EventFilter) ([]*AuditEvent, error) {
	query, args := l.buildQueryWithFilter(filter)
	
	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer rows.Close()

	var events []*AuditEvent
	for rows.Next() {
		event := &AuditEvent{}
		err := rows.Scan(
			&event.ID,
			&event.EventID,
			&event.Timestamp,
			&event.UserID,
			&event.CustomerID,
			&event.SessionID,
			&event.IPAddress,
			&event.UserAgent,
			&event.EventType,
			&event.ResourceType,
			&event.ResourceID,
			&event.ResourceName,
			&event.Action,
			&event.Outcome,
			&event.ErrorCode,
			&event.ErrorMessage,
			&event.RequestID,
			&event.Method,
			&event.Endpoint,
			&event.OldValues,
			&event.NewValues,
			&event.Metadata,
			&event.IsSensitive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit events: %w", err)
	}

	return events, nil
}

// buildQueryWithFilter builds a SQL query with WHERE conditions based on the filter
func (l *PostgresLogger) buildQueryWithFilter(filter *EventFilter) (string, []interface{}) {
	baseQuery := `
		SELECT id, event_id, timestamp, user_id, customer_id, session_id, ip_address, user_agent,
		       event_type, resource_type, resource_id, resource_name, action, outcome,
		       error_code, error_message, request_id, method, endpoint, old_values, new_values,
		       metadata, is_sensitive
		FROM audit_logs`

	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter == nil {
		return baseQuery + " ORDER BY timestamp DESC LIMIT 100", args
	}

	// Add WHERE conditions based on filter
	if filter.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIndex))
		args = append(args, *filter.CustomerID)
		argIndex++
	}

	if filter.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, *filter.UserID)
		argIndex++
	}

	if len(filter.EventTypes) > 0 {
		eventTypes := make([]string, len(filter.EventTypes))
		for i, et := range filter.EventTypes {
			eventTypes[i] = string(et)
		}
		conditions = append(conditions, fmt.Sprintf("event_type = ANY($%d)", argIndex))
		args = append(args, pq.Array(eventTypes))
		argIndex++
	}

	if len(filter.ResourceTypes) > 0 {
		resourceTypes := make([]string, len(filter.ResourceTypes))
		for i, rt := range filter.ResourceTypes {
			resourceTypes[i] = string(rt)
		}
		conditions = append(conditions, fmt.Sprintf("resource_type = ANY($%d)", argIndex))
		args = append(args, pq.Array(resourceTypes))
		argIndex++
	}

	if filter.ResourceID != nil {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argIndex))
		args = append(args, *filter.ResourceID)
		argIndex++
	}

	if len(filter.Outcomes) > 0 {
		outcomes := make([]string, len(filter.Outcomes))
		for i, o := range filter.Outcomes {
			outcomes[i] = string(o)
		}
		conditions = append(conditions, fmt.Sprintf("outcome = ANY($%d)", argIndex))
		args = append(args, pq.Array(outcomes))
		argIndex++
	}

	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIndex))
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIndex))
		args = append(args, *filter.EndTime)
		argIndex++
	}

	// Build the complete query
	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ordering
	baseQuery += " ORDER BY timestamp DESC"

	// Add pagination
	limit := 100
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	baseQuery += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)
	argIndex++

	if filter.Offset > 0 {
		baseQuery += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	return baseQuery, args
}

// Close closes the database connection
func (l *PostgresLogger) Close() error {
	return l.db.Close()
}

// containsSensitiveData checks if a map contains potentially sensitive information
func containsSensitiveData(data map[string]interface{}) bool {
	if data == nil {
		return false
	}

	sensitiveKeys := []string{
		"password", "token", "secret", "key", "credential", "auth",
		"private", "confidential", "sensitive", "vault", "cert",
		"certificate", "private_key", "access_token", "refresh_token",
		"api_key", "session_token", "bearer_token", "jwt",
	}

	for key := range data {
		lowerKey := strings.ToLower(key)
		for _, sensitiveKey := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitiveKey) {
				return true
			}
		}
	}

	return false
}

// GetEventCountByCustomer returns the count of audit events for a customer within a time range
func (l *PostgresLogger) GetEventCountByCustomer(ctx context.Context, customerID uuid.UUID, startTime, endTime time.Time) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM audit_logs 
		WHERE customer_id = $1 AND timestamp >= $2 AND timestamp <= $3`

	var count int64
	err := l.db.QueryRowContext(ctx, query, customerID, startTime, endTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get event count: %w", err)
	}

	return count, nil
}

// GetFailedEventsCount returns the count of failed events for a customer within a time range
func (l *PostgresLogger) GetFailedEventsCount(ctx context.Context, customerID uuid.UUID, startTime, endTime time.Time) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM audit_logs 
		WHERE customer_id = $1 AND outcome IN ('failure', 'error') AND timestamp >= $2 AND timestamp <= $3`

	var count int64
	err := l.db.QueryRowContext(ctx, query, customerID, startTime, endTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get failed events count: %w", err)
	}

	return count, nil
}

// GetEventsByUser returns audit events for a specific user within a time range
func (l *PostgresLogger) GetEventsByUser(ctx context.Context, customerID uuid.UUID, userID string, startTime, endTime time.Time, limit int) ([]*AuditEvent, error) {
	filter := &EventFilter{
		CustomerID: &customerID,
		UserID:     &userID,
		StartTime:  &startTime,
		EndTime:    &endTime,
		Limit:      limit,
	}

	return l.QueryEvents(ctx, filter)
}

// GetSecurityEvents returns security-related audit events (auth failures, permission denials, etc.)
func (l *PostgresLogger) GetSecurityEvents(ctx context.Context, customerID uuid.UUID, startTime, endTime time.Time) ([]*AuditEvent, error) {
	filter := &EventFilter{
		CustomerID: &customerID,
		EventTypes: []EventType{EventTypeAuth, EventTypeAdmin},
		Outcomes:   []Outcome{OutcomeFailure, OutcomeError},
		StartTime:  &startTime,
		EndTime:    &endTime,
		Limit:      500,
	}

	return l.QueryEvents(ctx, filter)
}