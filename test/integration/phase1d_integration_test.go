package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/kriipke/platformctl/internal/aggregator"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/database"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/handlers"
	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/internal/readmodel"
	"github.com/kriipke/platformctl/pkg/api"
)

// Phase1DIntegrationTestSuite tests the complete Phase 1D GitOps aggregator and status API integration
type Phase1DIntegrationTestSuite struct {
	suite.Suite
	db            *sqlx.DB
	rabbitmq      *events.GitOpsRabbitMQ
	aggregator    *aggregator.GitOpsAggregator
	store         *readmodel.GitOpsStore
	statusHandler *handlers.GitOpsStatusHandler
	server        *httptest.Server
	customerUUID  uuid.UUID
	customerID    string
	correlationID string
	contextName   string
}

func (suite *Phase1DIntegrationTestSuite) SetupSuite() {
	// Load test configuration
	cfg := config.Load()
	cfg.DatabaseURL = os.Getenv("TEST_DATABASE_URL")
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "postgres://test:test@localhost:5432/platformctl_test?sslmode=disable"
	}
	cfg.RabbitMQURL = os.Getenv("TEST_RABBITMQ_URL")
	if cfg.RabbitMQURL == "" {
		cfg.RabbitMQURL = "amqp://test:test@localhost:5672/"
	}

	// Setup test database
	var err error
	suite.db, err = database.Connect(cfg.DatabaseURL)
	require.NoError(suite.T(), err)

	// Run migrations
	err = database.RunMigrations(cfg.DatabaseURL, "../../migrations")
	require.NoError(suite.T(), err)

	// Setup test RabbitMQ
	suite.rabbitmq, err = events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	require.NoError(suite.T(), err)

	// Initialize components
	logger := suite.createTestLogger()
	metrics := observability.NewMetrics(observability.MetricsConfig{
		ServiceName: "phase1d-integration-test",
		Namespace:   "platformctl",
	})

	suite.aggregator = aggregator.NewGitOpsAggregator(suite.db, logger, metrics)
	suite.store = readmodel.NewGitOpsStore(suite.db)
	suite.statusHandler = handlers.NewGitOpsStatusHandler(suite.store, logger)

	// Setup test data. Customer and correlation IDs must be UUIDs: the status
	// handlers key on models.Customer.ID and command_runs.correlation_id is a
	// UUID column.
	suite.customerUUID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	suite.customerID = suite.customerUUID.String()
	suite.correlationID = "22222222-2222-4222-8222-222222222222"
	suite.contextName = "test-context"

	// Setup test HTTP server
	suite.server = httptest.NewServer(suite.createTestRouter())

	suite.setupTestData()
}

func (suite *Phase1DIntegrationTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.rabbitmq != nil {
		suite.rabbitmq.Close()
	}
	if suite.db != nil {
		// Clean up test data
		suite.cleanupTestData()
		suite.db.Close()
	}
}

func (suite *Phase1DIntegrationTestSuite) TestCompleteGitOpsWorkflow() {
	t := suite.T()

	// Test 1: Process App Manifest Result
	appResult := suite.createTestAppManifestResult()
	err := suite.aggregator.ProcessResultMessage(context.Background(), appResult)
	require.NoError(t, err, "Should process app manifest result without error")

	// Verify app manifest was stored
	appStatus, err := suite.store.GetAppManifestStatus(context.Background(), suite.customerID, suite.contextName, "test-app")
	require.NoError(t, err, "Should retrieve app manifest status")
	assert.Equal(t, "synced", appStatus.SyncStatus, "App sync status should be synced")
	assert.Equal(t, "healthy", appStatus.HealthStatus, "App health status should be healthy")
	assert.Equal(t, 3, appStatus.ApplicationCount, "Should have 3 generated applications")

	// Test 2: Process Environment Manifest Result
	envResult := suite.createTestEnvironmentManifestResult()
	err = suite.aggregator.ProcessResultMessage(context.Background(), envResult)
	require.NoError(t, err, "Should process environment manifest result without error")

	// Verify environment manifest was stored
	envStatus, err := suite.store.GetEnvironmentManifestStatus(context.Background(), suite.customerID, suite.contextName, "production")
	require.NoError(t, err, "Should retrieve environment manifest status")
	assert.Equal(t, "valid", envStatus.VaultValidationStatus, "Vault validation status should be valid")
	assert.Equal(t, "connected", envStatus.ClusterValidationStatus, "Cluster validation status should be connected")

	// Test 3: Process Context Pairing Result
	contextResult := suite.createTestContextPairingResult()
	err = suite.aggregator.ProcessResultMessage(context.Background(), contextResult)
	require.NoError(t, err, "Should process context pairing result without error")

	// Verify context pairing was stored
	contextStatus, err := suite.store.GetContextStatus(context.Background(), suite.customerID, suite.contextName)
	require.NoError(t, err, "Should retrieve context status")
	assert.Equal(t, "valid", contextStatus.PairingStatus, "Context pairing status should be valid")
	assert.Equal(t, "synced", contextStatus.SyncStatus, "Context sync status should be synced")
	assert.Equal(t, "healthy", contextStatus.HealthStatus, "Context health status should be healthy")

	// Test 4: Multi-environment correlation
	err = suite.createMultiEnvironmentData()
	require.NoError(t, err, "Should create multi-environment test data")

	multiEnvStatus, err := suite.store.GetMultiEnvironmentAppStatus(context.Background(), suite.customerID, suite.contextName, "test-app")
	require.NoError(t, err, "Should retrieve multi-environment app status")
	assert.Len(t, multiEnvStatus, 3, "Should have status for 3 environments (dev, staging, prod)")

	// Test 5: Vault validation details
	vaultDetails, err := suite.store.GetVaultValidationDetails(context.Background(), suite.customerID, suite.contextName, "production")
	require.NoError(t, err, "Should retrieve vault validation details")
	assert.Greater(t, len(vaultDetails), 0, "Should have vault validation details")
}

func (suite *Phase1DIntegrationTestSuite) TestStatusAPIEndpoints() {
	t := suite.T()

	// Setup test authentication header
	authHeader := suite.createTestAuthHeader()

	// Test 1: Get Context Status API
	resp, err := suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/"+suite.contextName+"/status", nil, authHeader)
	require.NoError(t, err, "Should make context status request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var contextStatus readmodel.ContextStatus
	err = json.NewDecoder(resp.Body).Decode(&contextStatus)
	require.NoError(t, err, "Should decode context status response")
	assert.Equal(t, suite.contextName, contextStatus.ContextName, "Should return correct context name")

	// Test 2: List Context Statuses API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/status", nil, authHeader)
	require.NoError(t, err, "Should make list contexts request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var listResponse struct {
		Contexts []readmodel.ContextStatus `json:"contexts"`
		Count    int                       `json:"count"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResponse)
	require.NoError(t, err, "Should decode list contexts response")
	assert.Greater(t, listResponse.Count, 0, "Should have at least one context")

	// Test 3: Get App Manifest Status API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/"+suite.contextName+"/apps/test-app/status", nil, authHeader)
	require.NoError(t, err, "Should make app status request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var appStatus readmodel.AppManifestStatus
	err = json.NewDecoder(resp.Body).Decode(&appStatus)
	require.NoError(t, err, "Should decode app status response")
	assert.Equal(t, "test-app", appStatus.AppName, "Should return correct app name")

	// Test 4: Get Environment Manifest Status API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/"+suite.contextName+"/environments/production/status", nil, authHeader)
	require.NoError(t, err, "Should make environment status request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var envStatus readmodel.EnvironmentManifestStatus
	err = json.NewDecoder(resp.Body).Decode(&envStatus)
	require.NoError(t, err, "Should decode environment status response")
	assert.Equal(t, "production", envStatus.EnvironmentName, "Should return correct environment name")

	// Test 5: Get Multi-Environment App Status API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/"+suite.contextName+"/apps/test-app/environments/status", nil, authHeader)
	require.NoError(t, err, "Should make multi-environment status request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var multiEnvResponse struct {
		Environments []readmodel.MultiEnvironmentAppStatus `json:"environments"`
		Count        int                                   `json:"count"`
	}
	err = json.NewDecoder(resp.Body).Decode(&multiEnvResponse)
	require.NoError(t, err, "Should decode multi-environment response")
	assert.Greater(t, multiEnvResponse.Count, 0, "Should have at least one environment")

	// Test 6: Get Vault Validation Details API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/"+suite.contextName+"/environments/production/vault/status", nil, authHeader)
	require.NoError(t, err, "Should make vault validation request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var vaultResponse struct {
		Validations []readmodel.VaultValidationDetail `json:"validations"`
		Count       int                               `json:"count"`
	}
	err = json.NewDecoder(resp.Body).Decode(&vaultResponse)
	require.NoError(t, err, "Should decode vault validation response")

	// Test 7: Get System Health Overview API
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/health/overview", nil, authHeader)
	require.NoError(t, err, "Should make health overview request")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return OK status")

	var healthOverview map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthOverview)
	require.NoError(t, err, "Should decode health overview response")
	assert.Contains(t, healthOverview, "summary", "Should contain summary section")
	assert.Contains(t, healthOverview, "health_distribution", "Should contain health distribution")
}

func (suite *Phase1DIntegrationTestSuite) TestErrorHandling() {
	t := suite.T()

	// Test processing invalid result message
	invalidResult := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			MessageID:     "invalid-msg-id",
			CorrelationID: "invalid-correlation-id",
			CustomerID:    suite.customerID,
			ContextName:   suite.contextName,
			ManifestType:  "invalid-type",
		},
		ServiceName: "test-service",
		Status:      "error",
		CompletedAt: time.Now(),
	}

	err := suite.aggregator.ProcessResultMessage(context.Background(), invalidResult)
	assert.Error(t, err, "Should return error for invalid manifest type")

	// Test API error responses
	authHeader := suite.createTestAuthHeader()

	// Test 404 for non-existent context
	resp, err := suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts/non-existent-context/status", nil, authHeader)
	require.NoError(t, err, "Should make request")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Should return 404 for non-existent context")

	// Test 400 for missing parameters
	resp, err = suite.makeAuthenticatedRequest("GET", "/api/v1/gitops/contexts//status", nil, authHeader)
	require.NoError(t, err, "Should make request")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Should return 404 for empty context name")
}

// Helper methods for test setup and utilities

func (suite *Phase1DIntegrationTestSuite) setupTestData() {
	// Insert the app and environment the context references (FK constraints)
	_, err := suite.db.Exec(`
		INSERT INTO apps (name, customer_id, spec)
		VALUES ($1, $2, $3)
		ON CONFLICT (name, customer_id) DO NOTHING
	`, "test-app", suite.customerID, `{"kind": "App"}`)
	require.NoError(suite.T(), err)

	_, err = suite.db.Exec(`
		INSERT INTO environments (name, customer_id, spec)
		VALUES ($1, $2, $3)
		ON CONFLICT (name, customer_id) DO NOTHING
	`, "production", suite.customerID, `{"kind": "Environment"}`)
	require.NoError(suite.T(), err)

	// Insert test context
	_, err = suite.db.Exec(`
		INSERT INTO contexts (name, customer_id, app_reference, environment_reference, spec)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name, customer_id) DO NOTHING
	`, suite.contextName, suite.customerID, "test-app", "production", `{"kind": "Context"}`)
	require.NoError(suite.T(), err)

	// Insert test command run for correlation
	_, err = suite.db.Exec(`
		INSERT INTO command_runs (correlation_id, context_name, customer_id, action, manifest_type, requested_by, requested_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (correlation_id) DO NOTHING
	`, suite.correlationID, suite.contextName, suite.customerID, "sync-app", "app", "test-user", time.Now())
	require.NoError(suite.T(), err)
}

func (suite *Phase1DIntegrationTestSuite) cleanupTestData() {
	// Clean up in reverse dependency order
	tables := []string{
		"customer_git_branch_correlation",
		"multi_environment_app_status",
		"gitops_vault_validation_status",
		"context_pairing_operations",
		"environment_manifest_validation",
		"app_manifest_correlation",
		"context_pairing_status",
		"result_events",
		"command_runs",
		"contexts",
		"apps",
		"environments",
	}

	for _, table := range tables {
		_, err := suite.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE customer_id = $1", table), suite.customerID)
		if err != nil {
			suite.T().Logf("Warning: Failed to clean up table %s: %v", table, err)
		}
	}
}

func (suite *Phase1DIntegrationTestSuite) createTestAppManifestResult() *api.GitOpsResultMessage {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			MessageID:     "test-app-msg-id",
			CorrelationID: suite.correlationID,
			CustomerID:    suite.customerID,
			ContextName:   suite.contextName,
			ManifestType:  "app",
			RequestedAt:   time.Now().Add(-1 * time.Minute),
		},
		ServiceName: "app-sync-service",
		Status:      "healthy",
		CompletedAt: time.Now(),
		AppManifestData: &api.AppManifestResult{
			AppName:            "test-app",
			ApplicationSetName: "test-applicationset",
			Namespace:          "argocd",
			SyncStatus:         "synced",
			HealthStatus:       "healthy",
			HelmSources: []api.HelmSourceStatus{
				{
					Name:    "backend-chart",
					Type:    "helm-registry",
					URL:     "https://charts.example.com",
					Version: "1.0.0",
					Status:  "available",
				},
			},
			GitSources: []api.GitSourceStatus{
				{
					URL:      "https://github.com/example/manifests",
					Path:     "apps/backend",
					Revision: "main",
					Status:   "available",
				},
			},
			Applications: []api.ApplicationStatus{
				{
					Name:         "backend-dev",
					Environment:  "dev",
					Cluster:      "dev-cluster",
					Namespace:    "backend-dev",
					SyncStatus:   "synced",
					HealthStatus: "healthy",
				},
				{
					Name:         "backend-staging",
					Environment:  "staging",
					Cluster:      "staging-cluster",
					Namespace:    "backend-staging",
					SyncStatus:   "synced",
					HealthStatus: "healthy",
				},
				{
					Name:         "backend-prod",
					Environment:  "production",
					Cluster:      "prod-cluster",
					Namespace:    "backend-prod",
					SyncStatus:   "synced",
					HealthStatus: "healthy",
				},
			},
			Generator: api.ApplicationSetGenerator{
				Type: "git",
				Parameters: map[string]interface{}{
					"repoURL": "https://github.com/example/manifests",
					"path":    "environments",
				},
			},
		},
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: 1500,
			ApiCallsCount:    5,
			CacheHitRate:     0.8,
		},
	}
}

func (suite *Phase1DIntegrationTestSuite) createTestEnvironmentManifestResult() *api.GitOpsResultMessage {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			MessageID:       "test-env-msg-id",
			CorrelationID:   suite.correlationID,
			CustomerID:      suite.customerID,
			ContextName:     suite.contextName,
			EnvironmentName: "production",
			ManifestType:    "environment",
			RequestedAt:     time.Now().Add(-1 * time.Minute),
		},
		ServiceName: "environment-validator",
		Status:      "healthy",
		CompletedAt: time.Now(),
		EnvironmentManifestData: &api.EnvironmentManifestResult{
			EnvironmentName: "production",
			VaultValidations: []api.VaultValidationResult{
				{
					VaultPath:        "/secret/backend/prod",
					SecretName:       "backend-secrets",
					ValidationStatus: "valid",
					PodEnvValidations: []api.PodEnvValidationResult{
						{
							PodName:          "backend-pod-1",
							Namespace:        "backend-prod",
							EnvVarName:       "DATABASE_PASSWORD",
							ValidationStatus: "match",
						},
					},
					LastValidated: time.Now(),
				},
			},
			ClusterValidations: []api.ClusterValidationResult{
				{
					ClusterName:      "prod-cluster",
					Server:           "https://prod-k8s.example.com",
					Namespace:        "backend-prod",
					ConnectionStatus: "connected",
					LastChecked:      time.Now(),
				},
			},
			ValuesFileStatus: []api.ValuesFileStatus{
				{
					FilePath:     "values-prod.yaml",
					Status:       "available",
					LastModified: &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
					Size:         1024,
				},
			},
			LastValidated: time.Now(),
		},
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: 2500,
			ApiCallsCount:    8,
			CacheHitRate:     0.6,
		},
	}
}

func (suite *Phase1DIntegrationTestSuite) createTestContextPairingResult() *api.GitOpsResultMessage {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			MessageID:     "test-context-msg-id",
			CorrelationID: suite.correlationID,
			CustomerID:    suite.customerID,
			ContextName:   suite.contextName,
			ManifestType:  "context",
			RequestedAt:   time.Now().Add(-1 * time.Minute),
		},
		ServiceName: "context-correlator",
		Status:      "healthy",
		CompletedAt: time.Now(),
		ContextPairingData: &api.ContextPairingResult{
			ContextName:          suite.contextName,
			AppReference:         "test-app",
			EnvironmentReference: "production",
			PairingStatus:        "valid",
			SyncStatus:           "synced",
			HealthStatus:         "healthy",
			CorrelationData: map[string]interface{}{
				"app_exists":         true,
				"environment_exists": true,
				"validation_passed":  true,
			},
			ResourceCount:      15,
			LastDeploymentTime: &[]time.Time{time.Now().Add(-30 * time.Minute)}[0],
		},
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: 800,
			ApiCallsCount:    3,
			CacheHitRate:     0.9,
		},
	}
}

func (suite *Phase1DIntegrationTestSuite) createMultiEnvironmentData() error {
	environments := []string{"dev", "staging", "production"}

	for _, env := range environments {
		_, err := suite.db.Exec(`
			INSERT INTO multi_environment_app_status (
				customer_id, context_name, app_name, environment, cluster_name, namespace,
				sync_status, health_status, deployment_status, helm_revision, git_commit,
				image_tags, resource_versions, last_deployed, last_checked,
				performance_metrics
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			ON CONFLICT (customer_id, context_name, app_name, environment, cluster_name, namespace) 
			DO UPDATE SET last_updated = NOW()
		`,
			suite.customerID, suite.contextName, "test-app", env, env+"-cluster", "backend-"+env,
			"synced", "healthy", "deployed", "1.0.0", "abc123",
			`{"backend": "v1.0.0", "frontend": "v2.1.0"}`,
			`{"deployment": "apps/v1", "service": "v1"}`,
			time.Now().Add(-1*time.Hour), time.Now(),
			`{"processing_time_ms": 1000, "api_calls_count": 2}`,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (suite *Phase1DIntegrationTestSuite) createTestLogger() zerolog.Logger {
	return zerolog.New(os.Stdout).With().Timestamp().Str("service", "phase1d-integration-test").Logger()
}

func (suite *Phase1DIntegrationTestSuite) createTestRouter() http.Handler {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Simulate customer authentication: the status handlers read a
	// *models.Customer from the gin context under the "customer" key.
	router.Use(func(c *gin.Context) {
		c.Set("customer", &models.Customer{
			ID:       suite.customerUUID,
			Name:     "Test Customer",
			Username: "test-user",
			Active:   true,
		})
		c.Next()
	})

	// Mirror the gateway's GitOps status routes (cmd/gateway/main.go)
	gitopsGroup := router.Group("/api/v1/gitops")
	gitopsGroup.GET("/contexts/status", suite.statusHandler.ListContextStatuses)
	gitopsGroup.GET("/contexts/:contextName/status", suite.statusHandler.GetContextStatus)
	gitopsGroup.GET("/contexts/:contextName/health", suite.statusHandler.GetContextHealth)
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/status", suite.statusHandler.GetAppManifestStatus)
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/environments/status", suite.statusHandler.GetMultiEnvironmentAppStatus)
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/status", suite.statusHandler.GetEnvironmentManifestStatus)
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/vault/status", suite.statusHandler.GetVaultValidationDetails)
	gitopsGroup.GET("/health/overview", suite.statusHandler.GetSystemHealthOverview)

	return router
}

func (suite *Phase1DIntegrationTestSuite) createTestAuthHeader() map[string]string {
	// Simulate basic auth header - in real implementation, this would be proper auth
	return map[string]string{
		"Authorization": "Bearer test-token-for-" + suite.customerID,
	}
}

func (suite *Phase1DIntegrationTestSuite) makeAuthenticatedRequest(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	var reqBody *bytes.Buffer
	if body != nil {
		reqBody = bytes.NewBuffer(body)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequest(method, suite.server.URL+path, reqBody)
	} else {
		req, err = http.NewRequest(method, suite.server.URL+path, nil)
	}
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// Test runner
func TestPhase1DIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Phase 1D integration tests in short mode")
	}

	// Check if required test dependencies are available
	if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("SKIP_DB_TESTS") != "true" {
		t.Skip("Skipping Phase 1D integration tests: TEST_DATABASE_URL not set")
	}
	if os.Getenv("TEST_RABBITMQ_URL") == "" && os.Getenv("SKIP_RABBITMQ_TESTS") != "true" {
		t.Skip("Skipping Phase 1D integration tests: TEST_RABBITMQ_URL not set")
	}

	suite.Run(t, new(Phase1DIntegrationTestSuite))
}
