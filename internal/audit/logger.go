package audit

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kriipke/platformctl/internal/models"
)

// EventType represents the type of audit event
type EventType string

const (
	EventTypeCreate EventType = "CREATE"
	EventTypeRead   EventType = "READ"
	EventTypeUpdate EventType = "UPDATE"
	EventTypeDelete EventType = "DELETE"
	EventTypeAuth   EventType = "AUTH"
	EventTypeSystem EventType = "SYSTEM"
	EventTypeAdmin  EventType = "ADMIN"
)

// ResourceType represents the type of resource being audited
type ResourceType string

const (
	ResourceTypeApp         ResourceType = "app"
	ResourceTypeEnvironment ResourceType = "environment"
	ResourceTypeContext     ResourceType = "context"
	ResourceTypeUser        ResourceType = "user"
	ResourceTypeSession     ResourceType = "session"
	ResourceTypeSystem      ResourceType = "system"
)

// Outcome represents the result of an audited operation
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeError   Outcome = "error"
	OutcomePending Outcome = "pending"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	ID            int64                  `json:"id" db:"id"`
	EventID       uuid.UUID              `json:"event_id" db:"event_id"`
	Timestamp     time.Time              `json:"timestamp" db:"timestamp"`
	UserID        *string                `json:"user_id,omitempty" db:"user_id"`
	CustomerID    uuid.UUID              `json:"customer_id" db:"customer_id"`
	SessionID     *string                `json:"session_id,omitempty" db:"session_id"`
	IPAddress     *net.IP                `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent     *string                `json:"user_agent,omitempty" db:"user_agent"`
	EventType     EventType              `json:"event_type" db:"event_type"`
	ResourceType  ResourceType           `json:"resource_type" db:"resource_type"`
	ResourceID    *string                `json:"resource_id,omitempty" db:"resource_id"`
	ResourceName  *string                `json:"resource_name,omitempty" db:"resource_name"`
	Action        string                 `json:"action" db:"action"`
	Outcome       Outcome                `json:"outcome" db:"outcome"`
	ErrorCode     *string                `json:"error_code,omitempty" db:"error_code"`
	ErrorMessage  *string                `json:"error_message,omitempty" db:"error_message"`
	RequestID     *uuid.UUID             `json:"request_id,omitempty" db:"request_id"`
	Method        *string                `json:"method,omitempty" db:"method"`
	Endpoint      *string                `json:"endpoint,omitempty" db:"endpoint"`
	OldValues     Metadata `json:"old_values,omitempty" db:"old_values"`
	NewValues     Metadata `json:"new_values,omitempty" db:"new_values"`
	Metadata      Metadata `json:"metadata,omitempty" db:"metadata"`
	IsSensitive   bool                   `json:"is_sensitive" db:"is_sensitive"`
}

// AuditContext contains information about the context of an audit event
type AuditContext struct {
	UserID      *string
	CustomerID  uuid.UUID
	SessionID   *string
	IPAddress   *net.IP
	UserAgent   *string
	RequestID   *uuid.UUID
	HTTPMethod  *string
	HTTPEndpoint *string
}

// Logger defines the interface for audit logging
type Logger interface {
	LogEvent(ctx context.Context, event *AuditEvent) error
	LogCRUDEvent(ctx context.Context, eventType EventType, resourceType ResourceType, resourceID, resourceName string, oldValues, newValues map[string]interface{}) error
	LogAuthEvent(ctx context.Context, action string, outcome Outcome, metadata map[string]interface{}) error
	LogSystemEvent(ctx context.Context, action string, outcome Outcome, metadata map[string]interface{}) error
	QueryEvents(ctx context.Context, filter *EventFilter) ([]*AuditEvent, error)
	Close() error
}

// EventFilter represents filter criteria for querying audit events
type EventFilter struct {
	CustomerID    *uuid.UUID     `json:"customer_id,omitempty"`
	UserID        *string        `json:"user_id,omitempty"`
	EventTypes    []EventType    `json:"event_types,omitempty"`
	ResourceTypes []ResourceType `json:"resource_types,omitempty"`
	ResourceID    *string        `json:"resource_id,omitempty"`
	Outcomes      []Outcome      `json:"outcomes,omitempty"`
	StartTime     *time.Time     `json:"start_time,omitempty"`
	EndTime       *time.Time     `json:"end_time,omitempty"`
	Limit         int            `json:"limit,omitempty"`
	Offset        int            `json:"offset,omitempty"`
}

// ExtractAuditContext extracts audit context from HTTP request and application context
func ExtractAuditContext(ctx context.Context, r *http.Request) *AuditContext {
	auditCtx := &AuditContext{}

	// Extract customer from context (set by auth middleware)
	if customer, ok := ctx.Value("customer").(*models.Customer); ok {
		auditCtx.CustomerID = customer.ID
	}

	// Extract user information from context
	if userID, ok := ctx.Value("user_id").(string); ok {
		auditCtx.UserID = &userID
	}

	// Extract session information
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		auditCtx.SessionID = &sessionID
	}

	// Extract request ID for correlation
	if requestID, ok := ctx.Value("request_id").(uuid.UUID); ok {
		auditCtx.RequestID = &requestID
	} else {
		// Generate a new request ID if not present
		newID := uuid.New()
		auditCtx.RequestID = &newID
	}

	if r != nil {
		// Extract IP address
		if ip := getClientIP(r); ip != nil {
			auditCtx.IPAddress = ip
		}

		// Extract user agent
		if ua := r.UserAgent(); ua != "" {
			auditCtx.UserAgent = &ua
		}

		// Extract HTTP method and endpoint
		method := r.Method
		auditCtx.HTTPMethod = &method
		
		endpoint := r.URL.Path
		auditCtx.HTTPEndpoint = &endpoint
	}

	return auditCtx
}

// getClientIP attempts to extract the real client IP address
func getClientIP(r *http.Request) *net.IP {
	// Check X-Forwarded-For header
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := parseXForwardedFor(xForwardedFor)
		if len(ips) > 0 {
			return &ips[0]
		}
	}

	// Check X-Real-IP header
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		if ip := net.ParseIP(xRealIP); ip != nil {
			return &ip
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	
	if ip := net.ParseIP(host); ip != nil {
		return &ip
	}

	return nil
}

// parseXForwardedFor parses the X-Forwarded-For header and returns a list of IP addresses
func parseXForwardedFor(header string) []net.IP {
	var ips []net.IP
	
	// Split by comma and parse each IP
	for _, ipStr := range splitAndTrim(header, ",") {
		if ip := net.ParseIP(ipStr); ip != nil {
			ips = append(ips, ip)
		}
	}
	
	return ips
}

// splitAndTrim splits a string by delimiter and trims whitespace
func splitAndTrim(s, delimiter string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, delimiter) {
		trimmed := trimWhitespace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits a string by delimiter
func splitString(s, delimiter string) []string {
	if s == "" {
		return []string{}
	}
	
	var parts []string
	start := 0
	delimLen := len(delimiter)
	
	for i := 0; i <= len(s)-delimLen; i++ {
		if s[i:i+delimLen] == delimiter {
			parts = append(parts, s[start:i])
			start = i + delimLen
			i += delimLen - 1
		}
	}
	
	parts = append(parts, s[start:])
	return parts
}

// trimWhitespace removes leading and trailing whitespace
func trimWhitespace(s string) string {
	start := 0
	end := len(s)
	
	// Trim leading whitespace
	for start < end && isWhitespace(s[start]) {
		start++
	}
	
	// Trim trailing whitespace
	for end > start && isWhitespace(s[end-1]) {
		end--
	}
	
	return s[start:end]
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// Metadata represents audit event metadata
type Metadata map[string]interface{}

// Value implements the driver.Valuer interface for database storage
func (m Metadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface for database retrieval
func (m *Metadata) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Metadata", value)
	}

	return json.Unmarshal(bytes, m)
}

// NewAuditEvent creates a new audit event with the given context
func NewAuditEvent(auditCtx *AuditContext, eventType EventType, resourceType ResourceType, action string, outcome Outcome) *AuditEvent {
	return &AuditEvent{
		EventID:      uuid.New(),
		Timestamp:    time.Now().UTC(),
		UserID:       auditCtx.UserID,
		CustomerID:   auditCtx.CustomerID,
		SessionID:    auditCtx.SessionID,
		IPAddress:    auditCtx.IPAddress,
		UserAgent:    auditCtx.UserAgent,
		EventType:    eventType,
		ResourceType: resourceType,
		Action:       action,
		Outcome:      outcome,
		RequestID:    auditCtx.RequestID,
		Method:       auditCtx.HTTPMethod,
		Endpoint:     auditCtx.HTTPEndpoint,
		Metadata:     make(map[string]interface{}),
	}
}

// WithResource sets resource information on the audit event
func (e *AuditEvent) WithResource(resourceID, resourceName string) *AuditEvent {
	e.ResourceID = &resourceID
	e.ResourceName = &resourceName
	return e
}

// WithError sets error information on the audit event
func (e *AuditEvent) WithError(errorCode string, errorMessage string) *AuditEvent {
	e.ErrorCode = &errorCode
	e.ErrorMessage = &errorMessage
	if e.Outcome == OutcomeSuccess {
		e.Outcome = OutcomeError
	}
	return e
}

// WithValues sets old and new values for the audit event
func (e *AuditEvent) WithValues(oldValues, newValues map[string]interface{}) *AuditEvent {
	e.OldValues = oldValues
	e.NewValues = newValues
	return e
}

// WithMetadata adds metadata to the audit event
func (e *AuditEvent) WithMetadata(key string, value interface{}) *AuditEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// WithSensitive marks the audit event as containing sensitive data
func (e *AuditEvent) WithSensitive(sensitive bool) *AuditEvent {
	e.IsSensitive = sensitive
	return e
}