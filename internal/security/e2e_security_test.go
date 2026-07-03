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

	"github.com/kriipke/platformctl/internal/audit"
	"github.com/kriipke/platformctl/internal/auth"
	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/testutil"
)

// E2ESecurityTestSuite provides a complete security testing environment
type E2ESecurityTestSuite struct {
	DB           *testutil.TestDB
	JWTManager   *auth.JWTManager
	RBACManager  *auth.RBACManager
	AuditLogger  audit.Logger
	Validator    *Validator
	RateLimiter  *RateLimiter
	CustomerID   uuid.UUID
	TestUser     *models.Customer
	AccessToken  string
	RefreshToken string
	MFAToken     string
}

func setupE2ESecuritySuite(t *testing.T) *E2ESecurityTestSuite {
	// Setup database
	testDB := testutil.NewTestDB(t)

	// Create test customer
	customerID := uuid.New()
	_, err := testDB.ExecContext(context.Background(),
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testuser", "test@example.com", true)
	require.NoError(t, err)

	// Setup security components
	jwtManager, err := auth.NewJWTManager(&auth.JWTConfig{
		AccessTokenExpiry:       30 * time.Minute,
		RefreshTokenExpiry:      24 * time.Hour,
		MFATokenExpiry:          5 * time.Minute,
		RequireMFAForPrivileged: true,
	})
	require.NoError(t, err)

	rbacManager := auth.NewRBACManager(testDB.DB.DB)
	auditLogger, err := audit.NewPostgresLogger(testDB.DB.DB)
	require.NoError(t, err)

	validator, err := NewValidator(DefaultSecurityConfig())
	require.NoError(t, err)

	rateLimiter := NewRateLimiter(100, time.Minute) // Generous limit for testing

	// Create test user
	testUser := &models.Customer{
		ID:       customerID,
		Username: "testuser",
	}

	// Generate tokens
	tokenPair, err := jwtManager.GenerateTokenPair(testUser, "test-session",
		[]string{"app:read", "app:create"}, []string{"customer-viewer"})
	require.NoError(t, err)

	mfaToken, err := jwtManager.GenerateMFAToken(testUser.Username, customerID, "test-session")
	require.NoError(t, err)

	return &E2ESecurityTestSuite{
		DB:           testDB,
		JWTManager:   jwtManager,
		RBACManager:  rbacManager,
		AuditLogger:  auditLogger,
		Validator:    validator,
		RateLimiter:  rateLimiter,
		CustomerID:   customerID,
		TestUser:     testUser,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		MFAToken:     mfaToken,
	}
}

func (suite *E2ESecurityTestSuite) Close() {
	suite.AuditLogger.Close()
	suite.DB.Close(&testing.T{})
}

func TestCompleteSecurityWorkflow(t *testing.T) {
	suite := setupE2ESecuritySuite(t)
	defer suite.Close()

	t.Run("authentication_flow", func(t *testing.T) {
		testAuthenticationFlow(t, suite)
	})

	t.Run("authorization_flow", func(t *testing.T) {
		testAuthorizationFlow(t, suite)
	})

	t.Run("mfa_protected_operations", func(t *testing.T) {
		testMFAProtectedOperations(t, suite)
	})

	t.Run("audit_trail_verification", func(t *testing.T) {
		testAuditTrailVerification(t, suite)
	})

	t.Run("security_incident_response", func(t *testing.T) {
		testSecurityIncidentResponse(t, suite)
	})

	t.Run("session_management", func(t *testing.T) {
		testSessionManagement(t, suite)
	})

	t.Run("rate_limiting_workflow", func(t *testing.T) {
		testRateLimitingWorkflow(t, suite)
	})

	t.Run("security_headers_and_validation", func(t *testing.T) {
		testSecurityHeadersAndValidation(t, suite)
	})
}

func testAuthenticationFlow(t *testing.T, suite *E2ESecurityTestSuite) {
	// Create test server with authentication middleware
	router := createSecureTestRouter(suite)

	tests := []struct {
		name            string
		token           string
		expectedStatus  int
		expectedInAudit string
	}{
		{
			name:            "valid token access",
			token:           suite.AccessToken,
			expectedStatus:  http.StatusOK,
			expectedInAudit: "success",
		},
		{
			name:            "no token provided",
			token:           "",
			expectedStatus:  http.StatusUnauthorized,
			expectedInAudit: "missing authorization header",
		},
		{
			name:            "invalid token format",
			token:           "invalid-token",
			expectedStatus:  http.StatusUnauthorized,
			expectedInAudit: "invalid token",
		},
		{
			name:            "malformed bearer token",
			token:           "Bearer",
			expectedStatus:  http.StatusUnauthorized,
			expectedInAudit: "invalid authorization format",
		},
		{
			name:            "expired token simulation",
			token:           "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDk0NTkyMDB9.expired",
			expectedStatus:  http.StatusUnauthorized,
			expectedInAudit: "token expired or invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/secure/apps", nil)
			if tt.token != "" && !strings.HasPrefix(tt.token, "Bearer") {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			} else if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Verify audit trail
			time.Sleep(100 * time.Millisecond) // Allow audit logging to complete
			events, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
				CustomerID: &suite.CustomerID,
				EventTypes: []audit.EventType{audit.EventTypeAuth, audit.EventTypeRead},
			})
			require.NoError(t, err)

			if tt.expectedStatus == http.StatusOK {
				// Should have successful auth and read events
				assert.True(t, len(events) >= 1)
				found := false
				for _, event := range events {
					if event.Outcome == audit.OutcomeSuccess {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have successful audit event")
			} else {
				// Should have failed auth event
				found := false
				for _, event := range events {
					if event.Outcome == audit.OutcomeFailure &&
						event.EventType == audit.EventTypeAuth {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have failed auth audit event")
			}
		})
	}
}

func testAuthorizationFlow(t *testing.T, suite *E2ESecurityTestSuite) {
	// Grant specific permissions
	err := suite.RBACManager.GrantPermission(
		suite.CustomerID, suite.TestUser.Username, "app", nil, "read", "admin", nil, nil)
	require.NoError(t, err)

	err = suite.RBACManager.GrantPermission(
		suite.CustomerID, suite.TestUser.Username, "app", stringPtr("app-123"), "update", "admin", nil, nil)
	require.NoError(t, err)

	// Create test server
	router := createSecureTestRouter(suite)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "allowed read operation",
			method:         "GET",
			path:           "/secure/apps",
			expectedStatus: http.StatusOK,
			description:    "Should allow read access to apps",
		},
		{
			name:           "allowed specific resource update",
			method:         "PUT",
			path:           "/secure/apps/app-123",
			expectedStatus: http.StatusOK,
			description:    "Should allow update of specific app",
		},
		{
			name:           "denied create operation",
			method:         "POST",
			path:           "/secure/apps",
			expectedStatus: http.StatusForbidden,
			description:    "Should deny create operation without permission",
		},
		{
			name:           "denied delete operation",
			method:         "DELETE",
			path:           "/secure/apps/app-123",
			expectedStatus: http.StatusForbidden,
			description:    "Should deny delete operation without permission",
		},
		{
			name:           "denied different resource update",
			method:         "PUT",
			path:           "/secure/apps/app-456",
			expectedStatus: http.StatusForbidden,
			description:    "Should deny update of different app without permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Buffer
			if tt.method == "POST" || tt.method == "PUT" {
				body = bytes.NewBufferString(`{"name": "test-app"}`)
			} else {
				body = bytes.NewBuffer(nil)
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code, tt.description)

			// Verify audit trail for authorization decisions
			time.Sleep(100 * time.Millisecond)
			events, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
				CustomerID: &suite.CustomerID,
			})
			require.NoError(t, err)
			assert.True(t, len(events) >= 1, "Should have audit events")
		})
	}
}

func testMFAProtectedOperations(t *testing.T, suite *E2ESecurityTestSuite) {
	// Create privileged token that requires MFA
	privilegedTokenPair, err := suite.JWTManager.GenerateTokenPair(
		suite.TestUser, "privileged-session",
		[]string{"*:*"}, []string{"customer-admin"})
	require.NoError(t, err)

	// Update token with MFA verified
	mfaVerifiedToken, err := suite.JWTManager.UpdateMFAStatus(privilegedTokenPair.AccessToken, true)
	require.NoError(t, err)

	router := createSecureTestRouter(suite)

	tests := []struct {
		name           string
		token          string
		path           string
		method         string
		expectedStatus int
		description    string
	}{
		{
			name:           "privileged operation without MFA",
			token:          privilegedTokenPair.AccessToken,
			path:           "/secure/admin/users",
			method:         "DELETE",
			expectedStatus: http.StatusForbidden,
			description:    "Should deny privileged operation without MFA",
		},
		{
			name:           "privileged operation with MFA",
			token:          mfaVerifiedToken,
			path:           "/secure/admin/users",
			method:         "DELETE",
			expectedStatus: http.StatusOK,
			description:    "Should allow privileged operation with MFA",
		},
		{
			name:           "non-privileged operation without MFA",
			token:          suite.AccessToken,
			path:           "/secure/apps",
			method:         "GET",
			expectedStatus: http.StatusOK,
			description:    "Should allow non-privileged operation without MFA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code, tt.description)
		})
	}
}

func testAuditTrailVerification(t *testing.T, suite *E2ESecurityTestSuite) {
	router := createSecureTestRouter(suite)

	// Perform various operations
	operations := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/secure/apps", ""},
		{"POST", "/secure/apps", `{"name": "audit-test-app"}`},
		{"PUT", "/secure/apps/app-123", `{"name": "updated-app"}`},
		{"DELETE", "/secure/apps/app-456", ""},
	}

	for _, op := range operations {
		var body *bytes.Buffer
		if op.body != "" {
			body = bytes.NewBufferString(op.body)
		} else {
			body = bytes.NewBuffer(nil)
		}

		req := httptest.NewRequest(op.method, op.path, body)
		req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("User-Agent", "e2e-test-client/1.0")

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		time.Sleep(50 * time.Millisecond) // Allow audit logging to complete
	}

	// Verify comprehensive audit trail
	events, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
		CustomerID: &suite.CustomerID,
	})
	require.NoError(t, err)
	assert.True(t, len(events) >= len(operations), "Should have audit events for all operations")

	// Verify audit event details
	for _, event := range events {
		assert.Equal(t, suite.CustomerID, event.CustomerID)
		assert.NotNil(t, event.Timestamp)
		assert.NotEqual(t, uuid.Nil, event.EventID)

		if event.Method != nil {
			assert.Contains(t, []string{"GET", "POST", "PUT", "DELETE"}, *event.Method)
		}

		if event.IPAddress != nil {
			assert.Equal(t, "192.168.1.100", event.IPAddress.String())
		}

		if event.UserAgent != nil {
			assert.Equal(t, "e2e-test-client/1.0", *event.UserAgent)
		}
	}

	// Test audit event querying and filtering
	readEvents, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
		CustomerID: &suite.CustomerID,
		EventTypes: []audit.EventType{audit.EventTypeRead},
	})
	require.NoError(t, err)
	assert.True(t, len(readEvents) > 0, "Should have read events")

	// Test time-based filtering
	now := time.Now()
	recentEvents, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
		CustomerID: &suite.CustomerID,
		StartTime:  timePtr(now.Add(-time.Hour)),
		EndTime:    timePtr(now.Add(time.Hour)),
	})
	require.NoError(t, err)
	assert.True(t, len(recentEvents) > 0, "Should have recent events")
}

func testSecurityIncidentResponse(t *testing.T, suite *E2ESecurityTestSuite) {
	router := createSecureTestRouter(suite)

	// Simulate security incidents
	incidents := []struct {
		name         string
		request      func() *http.Request
		expectedCode int
		incidentType string
	}{
		{
			name: "SQL injection attempt",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/secure/apps?name=' OR 1=1 --", nil)
				req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
				return req
			},
			expectedCode: http.StatusBadRequest,
			incidentType: "sql_injection",
		},
		{
			name: "XSS attempt",
			request: func() *http.Request {
				body := bytes.NewBufferString(`{"name": "<script>alert('xss')</script>"}`)
				req := httptest.NewRequest("POST", "/secure/apps", body)
				req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedCode: http.StatusBadRequest,
			incidentType: "xss_attempt",
		},
		{
			name: "Command injection attempt",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/secure/apps?cmd=ls; rm -rf /", nil)
				req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
				return req
			},
			expectedCode: http.StatusBadRequest,
			incidentType: "command_injection",
		},
		{
			name: "Path traversal attempt",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/secure/apps/../../../etc/passwd", nil)
				req.Header.Set("Authorization", "Bearer "+suite.AccessToken)
				return req
			},
			expectedCode: http.StatusBadRequest,
			incidentType: "path_traversal",
		},
	}

	for _, incident := range incidents {
		t.Run(incident.name, func(t *testing.T) {
			req := incident.request()
			req.RemoteAddr = "192.168.1.200:12345" // Suspicious IP
			req.Header.Set("User-Agent", "AttackBot/1.0")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, incident.expectedCode, recorder.Code)

			// Verify security incident was logged
			time.Sleep(100 * time.Millisecond)
			events, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
				CustomerID: &suite.CustomerID,
				Outcomes:   []audit.Outcome{audit.OutcomeFailure},
			})
			require.NoError(t, err)

			found := false
			for _, event := range events {
				if event.ErrorMessage != nil &&
					(strings.Contains(strings.ToLower(*event.ErrorMessage), "injection") ||
						strings.Contains(strings.ToLower(*event.ErrorMessage), "xss") ||
						strings.Contains(strings.ToLower(*event.ErrorMessage), "traversal")) {
					found = true
					assert.Equal(t, "192.168.1.200", event.IPAddress.String())
					assert.Equal(t, "AttackBot/1.0", *event.UserAgent)
					break
				}
			}
			assert.True(t, found, "Should have logged security incident for "+incident.name)
		})
	}
}

func testSessionManagement(t *testing.T, suite *E2ESecurityTestSuite) {
	// Test token refresh
	t.Run("token_refresh", func(t *testing.T) {
		newTokenPair, err := suite.JWTManager.RefreshTokens(
			suite.RefreshToken,
			[]string{"app:read", "app:write"},
			[]string{"customer-operator"})
		require.NoError(t, err)

		// Verify new tokens are different
		assert.NotEqual(t, suite.AccessToken, newTokenPair.AccessToken)
		assert.NotEqual(t, suite.RefreshToken, newTokenPair.RefreshToken)

		// Verify new token works
		router := createSecureTestRouter(suite)
		req := httptest.NewRequest("GET", "/secure/apps", nil)
		req.Header.Set("Authorization", "Bearer "+newTokenPair.AccessToken)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("token_expiration", func(t *testing.T) {
		// Create short-lived token
		shortLivedManager, err := auth.NewJWTManager(&auth.JWTConfig{
			AccessTokenExpiry: time.Millisecond, // Very short
		})
		require.NoError(t, err)

		tokenPair, err := shortLivedManager.GenerateTokenPair(
			suite.TestUser, "short-session", []string{"app:read"}, []string{"customer-viewer"})
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Try to use expired token
		_, err = shortLivedManager.ValidateToken(tokenPair.AccessToken)
		assert.Equal(t, auth.ErrTokenExpired, err)
	})

	t.Run("mfa_token_lifecycle", func(t *testing.T) {
		// Generate MFA token
		mfaToken, err := suite.JWTManager.GenerateMFAToken(
			suite.TestUser.Username, suite.CustomerID, "mfa-session")
		require.NoError(t, err)

		// Verify MFA token
		claims, err := suite.JWTManager.ValidateToken(mfaToken)
		require.NoError(t, err)
		assert.Equal(t, auth.TokenTypeMFA, claims.TokenType)
		assert.False(t, claims.MFAVerified) // MFA tokens themselves don't have MFAVerified=true

		// Use MFA token to update access token
		updatedAccessToken, err := suite.JWTManager.UpdateMFAStatus(suite.AccessToken, true)
		require.NoError(t, err)

		// Verify MFA status is updated
		updatedClaims, err := suite.JWTManager.ValidateToken(updatedAccessToken)
		require.NoError(t, err)
		assert.True(t, updatedClaims.MFAVerified)
	})
}

func testRateLimitingWorkflow(t *testing.T, suite *E2ESecurityTestSuite) {
	// Create restrictive rate limiter for testing
	testRateLimiter := NewRateLimiter(3, time.Second)

	router := mux.NewRouter()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	var handler http.Handler = testHandler
	handler = audit.LoggingMiddleware(suite.AuditLogger)(handler)
	handler = RateLimitMiddleware(testRateLimiter)(handler)

	router.Handle("/test", handler)

	// Test rate limiting
	clientIP := "192.168.1.50"
	successCount := 0
	rateLimitedCount := 0

	// Make requests beyond rate limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP + ":12345"

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		if recorder.Code == http.StatusOK {
			successCount++
		} else if recorder.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	assert.Equal(t, 3, successCount, "Should allow 3 requests")
	assert.Equal(t, 2, rateLimitedCount, "Should rate limit 2 requests")

	// Verify rate limiting audit events
	time.Sleep(100 * time.Millisecond)
	events, err := suite.AuditLogger.QueryEvents(context.Background(), &audit.EventFilter{
		Outcomes: []audit.Outcome{audit.OutcomeFailure},
	})
	require.NoError(t, err)

	rateLimitEventFound := false
	for _, event := range events {
		if event.ErrorMessage != nil && strings.Contains(*event.ErrorMessage, "rate limit") {
			rateLimitEventFound = true
			break
		}
	}
	assert.True(t, rateLimitEventFound, "Should have rate limit audit events")

	// Test rate limit recovery
	time.Sleep(1100 * time.Millisecond) // Wait for rate limit to reset

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP + ":12345"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code, "Should allow requests after rate limit reset")
}

func testSecurityHeadersAndValidation(t *testing.T, suite *E2ESecurityTestSuite) {
	// Create middleware with security headers
	middleware, err := NewSecurityMiddleware(suite.Validator, DefaultMiddlewareConfig())
	require.NoError(t, err)

	router := mux.NewRouter()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	var handler http.Handler = testHandler
	handler = middleware.ContentSecurityMiddleware()(handler)
	handler = middleware.SecurityMiddleware()(handler)

	router.Handle("/secure", handler)

	// Test security headers
	req := httptest.NewRequest("GET", "/secure", nil)
	req.Header.Set("User-Agent", "test-client")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	// Verify security headers
	headers := recorder.Header()
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'self'", headers.Get("Content-Security-Policy"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", headers.Get("Strict-Transport-Security"))

	// Verify sensitive headers are removed
	assert.Empty(t, headers.Get("Server"))
	assert.Empty(t, headers.Get("X-Powered-By"))

	// Test input validation
	req = httptest.NewRequest("POST", "/secure",
		bytes.NewBufferString(`{"data": "clean input"}`))
	req.Header.Set("User-Agent", "test-client")
	req.Header.Set("Content-Type", "application/json")

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	// Test malicious input rejection
	req = httptest.NewRequest("POST", "/secure",
		bytes.NewBufferString(`{"data": "<script>alert('xss')</script>"}`))
	req.Header.Set("User-Agent", "test-client")
	req.Header.Set("Content-Type", "application/json")

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "XSS")
}

// Helper functions

func createSecureTestRouter(suite *E2ESecurityTestSuite) *mux.Router {
	router := mux.NewRouter()

	// Create handlers for different operations
	appHandler := createAppHandler(suite)
	adminHandler := createAdminHandler(suite)

	// Apply security middleware stack
	var secureHandler http.Handler = appHandler
	secureHandler = audit.LoggingMiddleware(suite.AuditLogger)(secureHandler)
	secureHandler = auth.RBACMiddleware(suite.RBACManager)(secureHandler)
	secureHandler = auth.JWTMiddleware(suite.JWTManager)(secureHandler)
	secureHandler = CreateSecurityMiddleware(suite.Validator)(secureHandler)
	secureHandler = RateLimitMiddleware(suite.RateLimiter)(secureHandler)

	var secureAdminHandler http.Handler = adminHandler
	secureAdminHandler = audit.LoggingMiddleware(suite.AuditLogger)(secureAdminHandler)
	secureAdminHandler = auth.RBACMiddleware(suite.RBACManager)(secureAdminHandler)
	secureAdminHandler = auth.JWTMiddleware(suite.JWTManager)(secureAdminHandler)
	secureAdminHandler = CreateSecurityMiddleware(suite.Validator)(secureAdminHandler)
	secureAdminHandler = RateLimitMiddleware(suite.RateLimiter)(secureAdminHandler)

	// Register routes
	router.PathPrefix("/secure/apps").Handler(secureHandler)
	router.PathPrefix("/secure/admin").Handler(secureAdminHandler)

	return router
}

func createAppHandler(suite *E2ESecurityTestSuite) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleGetApps(w, r, suite)
		case "POST":
			handleCreateApp(w, r, suite)
		case "PUT":
			handleUpdateApp(w, r, suite)
		case "DELETE":
			handleDeleteApp(w, r, suite)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func createAdminHandler(suite *E2ESecurityTestSuite) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Admin operations require privileged access
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "admin operation completed"})
	})
}

func handleGetApps(w http.ResponseWriter, r *http.Request, suite *E2ESecurityTestSuite) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apps": []map[string]string{
			{"id": "app-123", "name": "test-app"},
			{"id": "app-456", "name": "another-app"},
		},
	})
}

func handleCreateApp(w http.ResponseWriter, r *http.Request, suite *E2ESecurityTestSuite) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     "app-789",
		"name":   "new-app",
		"status": "created",
	})
}

func handleUpdateApp(w http.ResponseWriter, r *http.Request, suite *E2ESecurityTestSuite) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     "app-123",
		"name":   "updated-app",
		"status": "updated",
	})
}

func handleDeleteApp(w http.ResponseWriter, r *http.Request, suite *E2ESecurityTestSuite) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     "app-123",
		"status": "deleted",
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
