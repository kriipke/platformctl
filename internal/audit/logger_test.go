package audit

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/contextops/platformctl/internal/models"
)

func TestNewAuditEvent(t *testing.T) {
	customerID := uuid.New()
	userID := "test-user"
	sessionID := "test-session"
	requestID := uuid.New()
	ip := net.ParseIP("192.168.1.100")
	userAgent := "test-agent"
	method := "POST"
	endpoint := "/api/test"

	auditCtx := &AuditContext{
		UserID:       &userID,
		CustomerID:   customerID,
		SessionID:    &sessionID,
		IPAddress:    &ip,
		UserAgent:    &userAgent,
		RequestID:    &requestID,
		HTTPMethod:   &method,
		HTTPEndpoint: &endpoint,
	}

	event := NewAuditEvent(
		auditCtx,
		EventTypeCreate,
		ResourceTypeApp,
		"create_app",
		OutcomeSuccess,
	)

	assert.NotNil(t, event)
	assert.NotEqual(t, uuid.Nil, event.EventID)
	assert.Equal(t, userID, *event.UserID)
	assert.Equal(t, customerID, event.CustomerID)
	assert.Equal(t, sessionID, *event.SessionID)
	assert.Equal(t, ip, *event.IPAddress)
	assert.Equal(t, userAgent, *event.UserAgent)
	assert.Equal(t, EventTypeCreate, event.EventType)
	assert.Equal(t, ResourceTypeApp, event.ResourceType)
	assert.Equal(t, "create_app", event.Action)
	assert.Equal(t, OutcomeSuccess, event.Outcome)
	assert.Equal(t, requestID, *event.RequestID)
	assert.Equal(t, method, *event.Method)
	assert.Equal(t, endpoint, *event.Endpoint)
	assert.NotNil(t, event.Metadata)
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
}

func TestAuditEventWithResource(t *testing.T) {
	auditCtx := &AuditContext{
		CustomerID: uuid.New(),
	}

	event := NewAuditEvent(auditCtx, EventTypeUpdate, ResourceTypeApp, "update", OutcomeSuccess)
	event = event.WithResource("app-123", "my-app")

	assert.Equal(t, "app-123", *event.ResourceID)
	assert.Equal(t, "my-app", *event.ResourceName)
}

func TestAuditEventWithError(t *testing.T) {
	auditCtx := &AuditContext{
		CustomerID: uuid.New(),
	}

	event := NewAuditEvent(auditCtx, EventTypeCreate, ResourceTypeApp, "create", OutcomeSuccess)
	event = event.WithError("VALIDATION_ERROR", "Invalid input data")

	assert.Equal(t, "VALIDATION_ERROR", *event.ErrorCode)
	assert.Equal(t, "Invalid input data", *event.ErrorMessage)
	assert.Equal(t, OutcomeError, event.Outcome)
}

func TestAuditEventWithValues(t *testing.T) {
	auditCtx := &AuditContext{
		CustomerID: uuid.New(),
	}

	oldValues := Metadata{
		"name":    "old-name",
		"version": "1.0.0",
	}

	newValues := Metadata{
		"name":    "new-name",
		"version": "1.1.0",
	}

	event := NewAuditEvent(auditCtx, EventTypeUpdate, ResourceTypeApp, "update", OutcomeSuccess)
	event = event.WithValues(oldValues, newValues)

	assert.Equal(t, oldValues, event.OldValues)
	assert.Equal(t, newValues, event.NewValues)
}

func TestAuditEventWithMetadata(t *testing.T) {
	auditCtx := &AuditContext{
		CustomerID: uuid.New(),
	}

	event := NewAuditEvent(auditCtx, EventTypeCreate, ResourceTypeApp, "create", OutcomeSuccess)
	event = event.WithMetadata("source", "api")
	event = event.WithMetadata("trace_id", "abc-123")

	assert.Equal(t, "api", event.Metadata["source"])
	assert.Equal(t, "abc-123", event.Metadata["trace_id"])
}

func TestAuditEventWithSensitive(t *testing.T) {
	auditCtx := &AuditContext{
		CustomerID: uuid.New(),
	}

	event := NewAuditEvent(auditCtx, EventTypeCreate, ResourceTypeApp, "create", OutcomeSuccess)
	event = event.WithSensitive(true)

	assert.True(t, event.IsSensitive)
}

func TestExtractAuditContext(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		setupContext   func() context.Context
		expectedResult func(*testing.T, *AuditContext)
	}{
		{
			name: "complete context with customer",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("POST", "/api/apps", nil)
				req.RemoteAddr = "192.168.1.100:12345"
				req.Header.Set("User-Agent", "test-client/1.0")
				req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.100")
				req.Header.Set("X-Real-IP", "203.0.113.2")
				return req
			},
			setupContext: func() context.Context {
				ctx := context.Background()
				customer := &models.Customer{
					ID: uuid.New(),
				}
				ctx = context.WithValue(ctx, "customer", customer)
				ctx = context.WithValue(ctx, "user_id", "test-user")
				ctx = context.WithValue(ctx, "session_id", "test-session")
				ctx = context.WithValue(ctx, "request_id", uuid.New())
				return ctx
			},
			expectedResult: func(t *testing.T, auditCtx *AuditContext) {
				assert.NotNil(t, auditCtx.UserID)
				assert.Equal(t, "test-user", *auditCtx.UserID)
				assert.NotEqual(t, uuid.Nil, auditCtx.CustomerID)
				assert.NotNil(t, auditCtx.SessionID)
				assert.Equal(t, "test-session", *auditCtx.SessionID)
				assert.NotNil(t, auditCtx.IPAddress)
				assert.Equal(t, "203.0.113.1", auditCtx.IPAddress.String()) // First IP from X-Forwarded-For
				assert.NotNil(t, auditCtx.UserAgent)
				assert.Equal(t, "test-client/1.0", *auditCtx.UserAgent)
				assert.NotNil(t, auditCtx.HTTPMethod)
				assert.Equal(t, "POST", *auditCtx.HTTPMethod)
				assert.NotNil(t, auditCtx.HTTPEndpoint)
				assert.Equal(t, "/api/apps", *auditCtx.HTTPEndpoint)
				assert.NotNil(t, auditCtx.RequestID)
			},
		},
		{
			name: "context without customer generates request ID",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "/health", nil)
				return req
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedResult: func(t *testing.T, auditCtx *AuditContext) {
				assert.NotNil(t, auditCtx.RequestID)
				assert.NotEqual(t, uuid.Nil, *auditCtx.RequestID)
				assert.Nil(t, auditCtx.UserID)
				assert.Equal(t, uuid.Nil, auditCtx.CustomerID)
			},
		},
		{
			name: "nil request",
			setupRequest: func() *http.Request {
				return nil
			},
			setupContext: func() context.Context {
				customer := &models.Customer{ID: uuid.New()}
				ctx := context.WithValue(context.Background(), "customer", customer)
				return ctx
			},
			expectedResult: func(t *testing.T, auditCtx *AuditContext) {
				assert.Nil(t, auditCtx.IPAddress)
				assert.Nil(t, auditCtx.UserAgent)
				assert.Nil(t, auditCtx.HTTPMethod)
				assert.Nil(t, auditCtx.HTTPEndpoint)
				assert.NotEqual(t, uuid.Nil, auditCtx.CustomerID)
			},
		},
		{
			name: "X-Real-IP header",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "203.0.113.3")
				return req
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedResult: func(t *testing.T, auditCtx *AuditContext) {
				assert.NotNil(t, auditCtx.IPAddress)
				assert.Equal(t, "203.0.113.3", auditCtx.IPAddress.String())
			},
		},
		{
			name: "RemoteAddr fallback",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "203.0.113.4:8080"
				return req
			},
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedResult: func(t *testing.T, auditCtx *AuditContext) {
				assert.NotNil(t, auditCtx.IPAddress)
				assert.Equal(t, "203.0.113.4", auditCtx.IPAddress.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			ctx := tt.setupContext()

			auditCtx := ExtractAuditContext(ctx, req)

			require.NotNil(t, auditCtx)
			tt.expectedResult(t, auditCtx)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		request  func() *http.Request
		expected string
	}{
		{
			name: "X-Forwarded-For single IP",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.1")
				return req
			},
			expected: "203.0.113.1",
		},
		{
			name: "X-Forwarded-For multiple IPs",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.1, 10.0.0.1")
				return req
			},
			expected: "203.0.113.1", // First IP
		},
		{
			name: "X-Forwarded-For with spaces",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "  203.0.113.2  ,  192.168.1.2  ")
				return req
			},
			expected: "203.0.113.2",
		},
		{
			name: "X-Real-IP",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "203.0.113.3")
				return req
			},
			expected: "203.0.113.3",
		},
		{
			name: "RemoteAddr with port",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "203.0.113.4:8080"
				return req
			},
			expected: "203.0.113.4",
		},
		{
			name: "RemoteAddr without port",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "203.0.113.5"
				return req
			},
			expected: "",
		},
		{
			name: "IPv6 address",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "2001:db8::1")
				return req
			},
			expected: "2001:db8::1",
		},
		{
			name: "invalid IP in X-Forwarded-For",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "invalid-ip, 203.0.113.6")
				return req
			},
			expected: "203.0.113.6", // Second valid IP
		},
		{
			name: "no IP headers",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				return req
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.request()
			ip := getClientIP(req)

			if tt.expected == "" {
				assert.Nil(t, ip)
			} else {
				require.NotNil(t, ip)
				assert.Equal(t, tt.expected, ip.String())
			}
		})
	}
}

func TestParseXForwardedFor(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []string
	}{
		{
			name:     "single IP",
			header:   "203.0.113.1",
			expected: []string{"203.0.113.1"},
		},
		{
			name:     "multiple IPs",
			header:   "203.0.113.1, 192.168.1.1, 10.0.0.1",
			expected: []string{"203.0.113.1", "192.168.1.1", "10.0.0.1"},
		},
		{
			name:     "IPs with extra spaces",
			header:   "  203.0.113.1  ,  192.168.1.1  ,  10.0.0.1  ",
			expected: []string{"203.0.113.1", "192.168.1.1", "10.0.0.1"},
		},
		{
			name:     "mixed valid and invalid IPs",
			header:   "203.0.113.1, invalid-ip, 192.168.1.1",
			expected: []string{"203.0.113.1", "192.168.1.1"},
		},
		{
			name:     "IPv6 addresses",
			header:   "2001:db8::1, 2001:db8::2",
			expected: []string{"2001:db8::1", "2001:db8::2"},
		},
		{
			name:     "empty header",
			header:   "",
			expected: []string{},
		},
		{
			name:     "only invalid IPs",
			header:   "invalid-ip, another-invalid",
			expected: []string{},
		},
		{
			name:     "empty segments",
			header:   "203.0.113.1, , 192.168.1.1, ,",
			expected: []string{"203.0.113.1", "192.168.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips := parseXForwardedFor(tt.header)

			assert.Len(t, ips, len(tt.expected))
			for i, expectedIP := range tt.expected {
				assert.Equal(t, expectedIP, ips[i].String())
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		expected  []string
	}{
		{
			name:      "normal case",
			input:     "a,b,c",
			delimiter: ",",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "with spaces",
			input:     "  a  ,  b  ,  c  ",
			delimiter: ",",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "empty segments",
			input:     "a,,b,",
			delimiter: ",",
			expected:  []string{"a", "b"},
		},
		{
			name:      "empty string",
			input:     "",
			delimiter: ",",
			expected:  []string{},
		},
		{
			name:      "single item",
			input:     "single",
			delimiter: ",",
			expected:  []string{"single"},
		},
		{
			name:      "only whitespace",
			input:     "  ,  ,  ",
			delimiter: ",",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input, tt.delimiter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrimWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no whitespace",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "leading whitespace",
			input:    "  hello",
			expected: "hello",
		},
		{
			name:     "trailing whitespace",
			input:    "hello  ",
			expected: "hello",
		},
		{
			name:     "both sides",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "mixed whitespace",
			input:    "\t\n  hello \r\n\t",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimWhitespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapValueScan(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "nil value",
			value:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "byte slice JSON",
			value: []byte(`{"key": "value", "number": 42}`),
			expected: map[string]interface{}{
				"key":    "value",
				"number": float64(42), // JSON numbers become float64
			},
			wantErr: false,
		},
		{
			name:  "string JSON",
			value: `{"key": "value", "nested": {"inner": true}}`,
			expected: map[string]interface{}{
				"key": "value",
				"nested": map[string]interface{}{
					"inner": true,
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid JSON",
			value:    []byte(`{"invalid": json}`),
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "unsupported type",
			value:    123,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Metadata
			err := (&m).Scan(tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, map[string]interface{}(m))
			}
		})
	}
}

func TestMapValueDriverValue(t *testing.T) {
	tests := []struct {
		name     string
		input    Metadata
		expected string
		wantErr  bool
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: "",
			wantErr:  false,
		},
		{
			name: "simple map",
			input: map[string]interface{}{
				"key": "value",
			},
			expected: `{"key":"value"}`,
			wantErr:  false,
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"key": "value",
				"nested": map[string]interface{}{
					"inner": 42,
				},
			},
			expected: `{"key":"value","nested":{"inner":42}}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.input.Value()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected == "" {
					assert.Nil(t, value)
				} else {
					assert.Equal(t, tt.expected, string(value.([]byte)))
				}
			}
		})
	}
}

func TestEventTypeConstants(t *testing.T) {
	assert.Equal(t, EventType("CREATE"), EventTypeCreate)
	assert.Equal(t, EventType("READ"), EventTypeRead)
	assert.Equal(t, EventType("UPDATE"), EventTypeUpdate)
	assert.Equal(t, EventType("DELETE"), EventTypeDelete)
	assert.Equal(t, EventType("AUTH"), EventTypeAuth)
	assert.Equal(t, EventType("SYSTEM"), EventTypeSystem)
	assert.Equal(t, EventType("ADMIN"), EventTypeAdmin)
}

func TestResourceTypeConstants(t *testing.T) {
	assert.Equal(t, ResourceType("app"), ResourceTypeApp)
	assert.Equal(t, ResourceType("environment"), ResourceTypeEnvironment)
	assert.Equal(t, ResourceType("context"), ResourceTypeContext)
	assert.Equal(t, ResourceType("user"), ResourceTypeUser)
	assert.Equal(t, ResourceType("session"), ResourceTypeSession)
	assert.Equal(t, ResourceType("system"), ResourceTypeSystem)
}

func TestOutcomeConstants(t *testing.T) {
	assert.Equal(t, Outcome("success"), OutcomeSuccess)
	assert.Equal(t, Outcome("failure"), OutcomeFailure)
	assert.Equal(t, Outcome("error"), OutcomeError)
	assert.Equal(t, Outcome("pending"), OutcomePending)
}