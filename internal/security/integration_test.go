package security

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/contextops/platformctl/internal/audit"
	"github.com/contextops/platformctl/internal/auth"
	"github.com/contextops/platformctl/internal/models"
	"github.com/contextops/platformctl/internal/testutil"
)

// MockAuditLogger for testing
type MockAuditLogger struct {
	events []audit.AuditEvent
}

func (m *MockAuditLogger) LogEvent(ctx context.Context, event *audit.AuditEvent) error {
	m.events = append(m.events, *event)
	return nil
}

func (m *MockAuditLogger) LogCRUDEvent(ctx context.Context, eventType audit.EventType, resourceType audit.ResourceType, resourceID, resourceName string, oldValues, newValues map[string]interface{}) error {
	auditCtx := audit.ExtractAuditContext(ctx, nil)
	event := audit.NewAuditEvent(auditCtx, eventType, resourceType, string(eventType), audit.OutcomeSuccess).
		WithResource(resourceID, resourceName).
		WithValues(oldValues, newValues)
	return m.LogEvent(ctx, event)
}

func (m *MockAuditLogger) LogAuthEvent(ctx context.Context, action string, outcome audit.Outcome, metadata map[string]interface{}) error {
	auditCtx := audit.ExtractAuditContext(ctx, nil)
	event := audit.NewAuditEvent(auditCtx, audit.EventTypeAuth, audit.ResourceTypeUser, action, outcome)
	for k, v := range metadata {
		event = event.WithMetadata(k, v)
	}
	return m.LogEvent(ctx, event)
}

func (m *MockAuditLogger) LogSystemEvent(ctx context.Context, action string, outcome audit.Outcome, metadata map[string]interface{}) error {
	auditCtx := audit.ExtractAuditContext(ctx, nil)
	event := audit.NewAuditEvent(auditCtx, audit.EventTypeSystem, audit.ResourceTypeSystem, action, outcome)
	for k, v := range metadata {
		event = event.WithMetadata(k, v)
	}
	return m.LogEvent(ctx, event)
}

func (m *MockAuditLogger) QueryEvents(ctx context.Context, filter *audit.EventFilter) ([]*audit.AuditEvent, error) {
	var results []*audit.AuditEvent
	for i := range m.events {
		results = append(results, &m.events[i])
	}
	return results, nil
}

func (m *MockAuditLogger) Close() error {
	return nil
}

func (m *MockAuditLogger) GetEvents() []audit.AuditEvent {
	return m.events
}

func (m *MockAuditLogger) Reset() {
	m.events = nil
}

// Test middleware stack integration
func TestSecurityMiddlewareStack(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Setup components
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	rateLimiter := NewRateLimiter(10, time.Minute)
	mockAuditor := &MockAuditLogger{}
	rbacManager := auth.NewRBACManager(testDB.DB.DB)

	// Create JWT manager
	jwtManager, err := auth.NewJWTManager(&auth.JWTConfig{
		RequireMFAForPrivileged: false, // Disable for testing
	})
	require.NoError(t, err)

	// Generate test token
	customer := &models.Customer{
		ID:       customerID,
		Username: "testuser",
	}
	tokenPair, err := jwtManager.GenerateTokenPair(customer, "test-session", []string{"app:read"}, []string{"customer-viewer"})
	require.NoError(t, err)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	// Setup middleware stack
	router := mux.NewRouter()
	
	// Apply middleware in order
	var handler http.Handler = testHandler
	
	// Audit middleware (innermost)
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	
	// RBAC middleware
	handler = auth.RBACMiddleware(rbacManager)(handler)
	
	// JWT auth middleware
	handler = auth.JWTMiddleware(jwtManager)(handler)
	
	// Security validation middleware
	handler = CreateSecurityMiddleware(validator)(handler)
	
	// Rate limiting middleware (outermost)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	router.Handle("/test", handler).Methods("GET")

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		expectedAudits int
		checkResponse  func(*testing.T, *http.Response, []audit.AuditEvent)
	}{
		{
			name: "successful authenticated request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedStatus: http.StatusOK,
			expectedAudits: 1,
			checkResponse: func(t *testing.T, resp *http.Response, events []audit.AuditEvent) {
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
				
				if len(events) > 0 {
					event := events[0]
					assert.Equal(t, audit.EventTypeRead, event.EventType)
					assert.Equal(t, audit.OutcomeSuccess, event.Outcome)
				}
			},
		},
		{
			name: "request without authorization",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedAudits: 1,
			checkResponse: func(t *testing.T, resp *http.Response, events []audit.AuditEvent) {
				if len(events) > 0 {
					event := events[0]
					assert.Equal(t, audit.EventTypeAuth, event.EventType)
					assert.Equal(t, audit.OutcomeFailure, event.Outcome)
				}
			},
		},
		{
			name: "request with invalid token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer invalid.token.here")
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedAudits: 1,
			checkResponse: func(t *testing.T, resp *http.Response, events []audit.AuditEvent) {
				if len(events) > 0 {
					event := events[0]
					assert.Equal(t, audit.EventTypeAuth, event.EventType)
					assert.Equal(t, audit.OutcomeFailure, event.Outcome)
				}
			},
		},
		{
			name: "request with security threat",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?param=<script>alert('xss')</script>", nil)
				req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedStatus: http.StatusBadRequest,
			expectedAudits: 1,
			checkResponse: func(t *testing.T, resp *http.Response, events []audit.AuditEvent) {
				if len(events) > 0 {
					event := events[0]
					assert.Contains(t, *event.ErrorMessage, "XSS")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuditor.Reset()
			
			req := tt.setupRequest()
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
			
			events := mockAuditor.GetEvents()
			assert.Len(t, events, tt.expectedAudits)

			if tt.checkResponse != nil {
				tt.checkResponse(t, recorder.Result(), events)
			}
		})
	}
}

func TestRateLimitingIntegration(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	rateLimiter := NewRateLimiter(3, time.Second) // Very low limit for testing
	mockAuditor := &MockAuditLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Simple middleware stack for rate limiting test
	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = CreateSecurityMiddleware(validator)(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	server := httptest.NewServer(handler)
	defer server.Close()

	client := &http.Client{}

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// 4th request should be rate limited
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()

	// Check audit events
	events := mockAuditor.GetEvents()
	assert.True(t, len(events) >= 4) // At least 3 success + 1 rate limited

	// Find rate limited event
	rateLimitedEventFound := false
	for _, event := range events {
		if event.Outcome == audit.OutcomeFailure && strings.Contains(*event.ErrorMessage, "rate limit") {
			rateLimitedEventFound = true
			break
		}
	}
	assert.True(t, rateLimitedEventFound, "Should have audit event for rate limiting")
}

func TestSecurityValidationIntegration(t *testing.T) {
	config := &SecurityConfig{
		MaxStringLength:  100,
		MaxJSONSize:     1000,
		RequireHTTPS:    false,
		AllowPrivateIPs: true,
	}
	validator, err := NewValidator(config)
	require.NoError(t, err)

	mockAuditor := &MockAuditLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = CreateSecurityMiddleware(validator)(handler)

	tests := []struct {
		name           string
		requestBody    string
		contentType    string
		queryParams    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "clean request",
			requestBody:    `{"name": "test", "value": "clean"}`,
			contentType:    "application/json",
			queryParams:    "?param=safe_value",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "SQL injection in query",
			requestBody:    `{"name": "test"}`,
			contentType:    "application/json",
			queryParams:    "?param=' OR 1=1 --",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "SQL injection",
		},
		{
			name:           "XSS in request body",
			requestBody:    `{"content": "<script>alert('xss')</script>"}`,
			contentType:    "application/json",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "XSS",
		},
		{
			name:           "command injection in query",
			requestBody:    `{"name": "test"}`,
			contentType:    "application/json",
			queryParams:    "?cmd=file.txt; rm -rf /",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "command injection",
		},
		{
			name:           "path traversal attack",
			requestBody:    `{"path": "../../etc/passwd"}`,
			contentType:    "application/json",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "path traversal",
		},
		{
			name:           "oversized JSON body",
			requestBody:    `{"data": "` + strings.Repeat("a", 1000) + `"}`,
			contentType:    "application/json",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "JSON size",
		},
		{
			name:           "dangerous control characters",
			requestBody:    "{\x00\"name\": \"test\x01\"}",
			contentType:    "application/json",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "dangerous characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuditor.Reset()

			var body *bytes.Buffer
			if tt.requestBody != "" {
				body = bytes.NewBufferString(tt.requestBody)
			} else {
				body = bytes.NewBuffer(nil)
			}

			url := "/test" + tt.queryParams
			req := httptest.NewRequest("POST", url, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(recorder.Body.String()), strings.ToLower(tt.expectedError))
				
				// Check audit event
				events := mockAuditor.GetEvents()
				require.Len(t, events, 1)
				event := events[0]
				assert.Equal(t, audit.OutcomeFailure, event.Outcome)
				if event.ErrorMessage != nil {
					assert.Contains(t, strings.ToLower(*event.ErrorMessage), strings.ToLower(tt.expectedError))
				}
			}
		})
	}
}

func TestAuthenticationIntegration(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	mockAuditor := &MockAuditLogger{}
	rbacManager := auth.NewRBACManager(testDB.DB.DB)

	// Create JWT manager
	jwtManager, err := auth.NewJWTManager(&auth.JWTConfig{
		AccessTokenExpiry:       time.Minute,
		RequireMFAForPrivileged: true,
	})
	require.NoError(t, err)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected resource"))
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = auth.RBACMiddleware(rbacManager)(handler)
	handler = auth.JWTMiddleware(jwtManager)(handler)

	// Create tokens
	customer := &models.Customer{
		ID:       customerID,
		Username: "testuser",
	}

	normalTokenPair, err := jwtManager.GenerateTokenPair(customer, "session1", []string{"app:read"}, []string{"customer-viewer"})
	require.NoError(t, err)

	privilegedTokenPair, err := jwtManager.GenerateTokenPair(customer, "session2", []string{"*:*"}, []string{"customer-admin"})
	require.NoError(t, err)

	// Update privileged token with MFA
	updatedPrivilegedToken, err := jwtManager.UpdateMFAStatus(privilegedTokenPair.AccessToken, true)
	require.NoError(t, err)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		expectedAudits int
	}{
		{
			name:           "valid normal token",
			token:          normalTokenPair.AccessToken,
			expectedStatus: http.StatusOK,
			expectedAudits: 1,
		},
		{
			name:           "valid privileged token with MFA",
			token:          updatedPrivilegedToken,
			expectedStatus: http.StatusOK,
			expectedAudits: 1,
		},
		{
			name:           "privileged token without MFA",
			token:          privilegedTokenPair.AccessToken,
			expectedStatus: http.StatusForbidden,
			expectedAudits: 1,
		},
		{
			name:           "missing token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedAudits: 1,
		},
		{
			name:           "invalid token",
			token:          "invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectedAudits: 1,
		},
		{
			name:           "expired token",
			token:          "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDk0NTkyMDB9.expired",
			expectedStatus: http.StatusUnauthorized,
			expectedAudits: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuditor.Reset()

			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			events := mockAuditor.GetEvents()
			assert.Len(t, events, tt.expectedAudits)

			if len(events) > 0 {
				event := events[0]
				if tt.expectedStatus == http.StatusOK {
					assert.Equal(t, audit.OutcomeSuccess, event.Outcome)
				} else {
					assert.Equal(t, audit.OutcomeFailure, event.Outcome)
				}
			}
		})
	}
}

func TestAuditingIntegration(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Use real PostgreSQL audit logger
	auditLogger := audit.NewPostgresLogger(testDB.DB.DB)
	defer auditLogger.Close()

	jwtManager, err := auth.NewJWTManager(&auth.JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       customerID,
		Username: "testuser",
	}
	tokenPair, err := jwtManager.GenerateTokenPair(customer, "session", []string{"app:create"}, []string{"customer-operator"})
	require.NoError(t, err)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate CRUD operation
		ctx := r.Context()
		
		// Log a CRUD event
		auditLogger.LogCRUDEvent(
			ctx,
			audit.EventTypeCreate,
			audit.ResourceTypeApp,
			"app-123",
			"test-app",
			nil,
			map[string]interface{}{"name": "test-app", "version": "1.0.0"},
		)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "app-123", "name": "test-app"}`))
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(auditLogger)(handler)
	handler = auth.JWTMiddleware(jwtManager)(handler)

	req := httptest.NewRequest("POST", "/apps", strings.NewReader(`{"name": "test-app"}`))
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("User-Agent", "test-client/1.0")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)

	// Wait a moment for audit logging to complete
	time.Sleep(100 * time.Millisecond)

	// Query audit events
	events, err := auditLogger.QueryEvents(context.Background(), &audit.EventFilter{
		CustomerID: &customerID,
	})
	require.NoError(t, err)
	assert.True(t, len(events) >= 2) // HTTP request + CRUD event

	// Check for HTTP request audit
	httpEventFound := false
	crudEventFound := false

	for _, event := range events {
		if event.Method != nil && *event.Method == "POST" {
			httpEventFound = true
			assert.Equal(t, audit.EventTypeCreate, event.EventType)
			assert.Equal(t, customerID, event.CustomerID)
			assert.NotNil(t, event.IPAddress)
			assert.Equal(t, "192.168.1.100", event.IPAddress.String())
			assert.NotNil(t, event.UserAgent)
			assert.Equal(t, "test-client/1.0", *event.UserAgent)
		}

		if event.ResourceType == audit.ResourceTypeApp && event.ResourceID != nil && *event.ResourceID == "app-123" {
			crudEventFound = true
			assert.Equal(t, audit.EventTypeCreate, event.EventType)
			assert.Equal(t, "test-app", *event.ResourceName)
			assert.NotNil(t, event.NewValues)
			assert.Equal(t, "test-app", event.NewValues["name"])
		}
	}

	assert.True(t, httpEventFound, "Should have HTTP request audit event")
	assert.True(t, crudEventFound, "Should have CRUD operation audit event")
}

func TestConcurrentSecurityMiddleware(t *testing.T) {
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	rateLimiter := NewRateLimiter(100, time.Second) // Higher limit for concurrent test
	mockAuditor := &MockAuditLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = CreateSecurityMiddleware(validator)(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	server := httptest.NewServer(handler)
	defer server.Close()

	const numGoroutines = 10
	const requestsPerGoroutine = 5

	results := make(chan int, numGoroutines*requestsPerGoroutine)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			client := &http.Client{}
			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := client.Get(server.URL + "?test=clean_param")
				if err != nil {
					results <- 500
					continue
				}
				results <- resp.StatusCode
				resp.Body.Close()
			}
		}(i)
	}

	// Collect results
	statusCounts := make(map[int]int)
	for i := 0; i < numGoroutines*requestsPerGoroutine; i++ {
		status := <-results
		statusCounts[status]++
	}

	// All requests should succeed (no security violations, within rate limit)
	assert.Equal(t, numGoroutines*requestsPerGoroutine, statusCounts[200])

	// Check audit events
	events := mockAuditor.GetEvents()
	assert.Equal(t, numGoroutines*requestsPerGoroutine, len(events))

	for _, event := range events {
		assert.Equal(t, audit.OutcomeSuccess, event.Outcome)
	}
}

func TestSecurityMiddlewareErrorHandling(t *testing.T) {
	// Test middleware behavior with various error conditions
	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	mockAuditor := &MockAuditLogger{}

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	var handler http.Handler = panicHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = CreateSecurityMiddleware(validator)(handler)

	req := httptest.NewRequest("GET", "/test?param=safe", nil)
	recorder := httptest.NewRecorder()

	// Should not panic, should return 500
	assert.NotPanics(t, func() {
		handler.ServeHTTP(recorder, req)
	})

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	// Check audit event for panic
	events := mockAuditor.GetEvents()
	require.Len(t, events, 1)
	event := events[0]
	assert.Equal(t, audit.OutcomeError, event.Outcome)
	assert.Contains(t, *event.ErrorMessage, "panic")
}

func TestMiddlewareChainOrdering(t *testing.T) {
	// Test that middleware is applied in the correct order
	var executionOrder []string

	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	rateLimiter := NewRateLimiter(10, time.Minute)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder = append(executionOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})

	// Custom middleware to track execution order
	orderTracker := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, name+"-before")
				next.ServeHTTP(w, r)
				executionOrder = append(executionOrder, name+"-after")
			})
		}
	}

	// Apply middleware in specific order
	var handler http.Handler = testHandler
	handler = orderTracker("inner")(handler)
	handler = CreateSecurityMiddleware(validator)(handler)
	handler = orderTracker("middle")(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)
	handler = orderTracker("outer")(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	executionOrder = nil
	handler.ServeHTTP(recorder, req)

	expectedOrder := []string{
		"outer-before",
		"middle-before",
		"inner-before",
		"handler",
		"inner-after",
		"middle-after",
		"outer-after",
	}

	assert.Equal(t, expectedOrder, executionOrder)
}

// Benchmark integration tests
func BenchmarkSecurityMiddlewareStack(b *testing.B) {
	validator, _ := NewValidator(DefaultSecurityConfig())
	rateLimiter := NewRateLimiter(100000, time.Second) // High limit
	mockAuditor := &MockAuditLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(mockAuditor)(handler)
	handler = CreateSecurityMiddleware(validator)(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	req := httptest.NewRequest("GET", "/test?param=safe_value", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}