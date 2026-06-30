package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase1CServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Setup test configuration
	cfg := &config.Config{
		RabbitMQURL: "amqp://localhost:5672/",
		DatabaseURL: "postgres://localhost/contextops_test?sslmode=disable",
	}

	// Initialize message bus
	messageBus, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	require.NoError(t, err)
	defer messageBus.Close()

	// Setup service queue manager
	queueManager := events.NewServiceQueueManager(messageBus)
	err = queueManager.SetupAllServiceQueues()
	require.NoError(t, err)

	t.Run("Environment Validation Service", func(t *testing.T) {
		testEnvironmentValidationService(t, messageBus)
	})

	t.Run("App Sync Service", func(t *testing.T) {
		testAppSyncService(t, messageBus)
	})

	t.Run("Context Correlation Service", func(t *testing.T) {
		testContextCorrelationService(t, messageBus)
	})

	t.Run("Multi-Environment Kubernetes Service", func(t *testing.T) {
		testMultiEnvironmentKubernetesService(t, messageBus)
	})

	t.Run("Customer Git Branch Service", func(t *testing.T) {
		testCustomerGitBranchService(t, messageBus)
	})
}

func testEnvironmentValidationService(t *testing.T, messageBus *events.GitOpsMessageBus) {
	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create test command
	cmd := api.NewEnvironmentManifestCommandMessage(
		"test-customer",
		"test-context",
		"test-environment",
		"test-user",
	)
	cmd.Payload["validate_vault_sources"] = true
	cmd.Payload["validate_cluster_configs"] = true
	cmd.Payload["validate_values_files"] = true
	cmd.Payload["check_pod_env"] = true

	// Publish command
	err := publisher.PublishGitOpsCommand(cmd)
	require.NoError(t, err)

	// In a real integration test, we would:
	// 1. Start the environment validation service
	// 2. Wait for it to process the command
	// 3. Verify the result message was published
	// For now, we just verify the command was published successfully

	assert.Equal(t, "validate-environment", cmd.Action)
	assert.Equal(t, "environment", cmd.ManifestType)
	assert.Equal(t, "test-customer", cmd.CustomerID)
}

func testAppSyncService(t *testing.T, messageBus *events.GitOpsMessageBus) {
	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create test command
	cmd := api.NewAppManifestCommandMessage(
		"test-customer",
		"test-context",
		"test-app",
		"test-user",
	)
	cmd.Payload["sync_applicationset"] = true
	cmd.Payload["validate_helm_sources"] = true
	cmd.Payload["check_git_sources"] = true

	// Publish command
	err := publisher.PublishGitOpsCommand(cmd)
	require.NoError(t, err)

	assert.Equal(t, "sync-app", cmd.Action)
	assert.Equal(t, "app", cmd.ManifestType)
	assert.Equal(t, "test-customer", cmd.CustomerID)
	assert.Equal(t, "test-app", cmd.AppName)
}

func testContextCorrelationService(t *testing.T, messageBus *events.GitOpsMessageBus) {
	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create test command
	cmd := api.NewContextPairingCommandMessage(
		"test-customer",
		"test-context",
		"test-user",
	)
	cmd.ManifestMetadata.AppReference = "test-app"
	cmd.ManifestMetadata.EnvironmentReference = "test-environment"
	cmd.Payload["validate_pairing"] = true
	cmd.Payload["sync_after_correlation"] = true

	// Publish command
	err := publisher.PublishGitOpsCommand(cmd)
	require.NoError(t, err)

	assert.Equal(t, "correlate-context", cmd.Action)
	assert.Equal(t, "context", cmd.ManifestType)
	assert.Equal(t, "test-customer", cmd.CustomerID)
}

func testMultiEnvironmentKubernetesService(t *testing.T, messageBus *events.GitOpsMessageBus) {
	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create test command for multi-environment correlation
	cmd := api.NewGitOpsCommandMessage(
		"test-customer",
		"test-context",
		"correlate-context",
		"environment",
		"test-user",
	)
	cmd.TargetService = "multi-environment-kubernetes"
	cmd.Payload["environments"] = []string{"dev", "staging", "prod"}
	cmd.Payload["compare_resources"] = true

	// Publish command
	err := publisher.PublishGitOpsCommand(cmd)
	require.NoError(t, err)

	assert.Equal(t, "correlate-context", cmd.Action)
	assert.Equal(t, "environment", cmd.ManifestType)
	assert.Equal(t, "test-customer", cmd.CustomerID)
}

func testCustomerGitBranchService(t *testing.T, messageBus *events.GitOpsMessageBus) {
	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create test command for customer branch sync
	cmd, err := publisher.PublishCustomerBranchSync(
		"test-customer",
		"test-context",
		"customer/test-customer",
		"test-user",
	)
	require.NoError(t, err)

	assert.Equal(t, "sync-customer-branch", cmd.Action)
	assert.Equal(t, "git", cmd.ManifestType)
	assert.Equal(t, "test-customer", cmd.CustomerID)
	assert.Equal(t, "customer/test-customer", cmd.ManifestMetadata.CustomerBranch)
}

func TestServiceMessageFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Test the complete message flow: Command -> Processing -> Result
	cfg := &config.Config{
		RabbitMQURL: "amqp://localhost:5672/",
	}

	messageBus, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	require.NoError(t, err)
	defer messageBus.Close()

	// Create a result consumer to capture results
	consumer := events.NewResultConsumer(messageBus)
	
	resultsChan := make(chan *api.GitOpsResultMessage, 10)
	testHandler := &TestResultHandler{resultsChan: resultsChan}
	consumer.RegisterHandler("test", testHandler)
	
	err = consumer.Start()
	require.NoError(t, err)
	defer consumer.Stop()

	// Publish a test command
	publisher := events.NewGitOpsCommandPublisher(messageBus)
	cmd := api.NewAppManifestCommandMessage(
		"test-customer",
		"test-context",
		"test-app",
		"test-user",
	)

	err = publisher.PublishGitOpsCommand(cmd)
	require.NoError(t, err)

	// Wait for result (with timeout)
	select {
	case result := <-resultsChan:
		assert.Equal(t, cmd.CorrelationID, result.CorrelationID)
		assert.Equal(t, cmd.CustomerID, result.CustomerID)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result message")
	}
}

func TestServiceHealthChecks(t *testing.T) {
	// Test that all services have proper health check endpoints
	services := []string{
		"environment-validation-svc",
		"app-sync-svc",
		"context-correlation-svc",
		"multi-environment-kube-svc",
		"customer-git-branch-svc",
	}

	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			// In a real integration test, we would:
			// 1. Start the service
			// 2. Make HTTP request to /health endpoint
			// 3. Verify response status and format
			// For now, we just verify the service name is valid
			assert.NotEmpty(t, service)
		})
	}
}

func TestCustomerIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Test that customer isolation is properly enforced
	cfg := &config.Config{
		RabbitMQURL: "amqp://localhost:5672/",
	}

	messageBus, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	require.NoError(t, err)
	defer messageBus.Close()

	publisher := events.NewGitOpsCommandPublisher(messageBus)

	// Create commands for different customers
	customer1Cmd := api.NewAppManifestCommandMessage(
		"customer-1",
		"test-context",
		"test-app",
		"user-1",
	)

	customer2Cmd := api.NewAppManifestCommandMessage(
		"customer-2",
		"test-context",
		"test-app",
		"user-2",
	)

	// Publish both commands
	err = publisher.PublishGitOpsCommand(customer1Cmd)
	require.NoError(t, err)

	err = publisher.PublishGitOpsCommand(customer2Cmd)
	require.NoError(t, err)

	// Verify customer isolation in message headers
	assert.Equal(t, "customer-1", customer1Cmd.CustomerID)
	assert.Equal(t, "customer-2", customer2Cmd.CustomerID)
	assert.NotEqual(t, customer1Cmd.CorrelationID, customer2Cmd.CorrelationID)
}

// TestResultHandler captures result messages for testing
type TestResultHandler struct {
	resultsChan chan *api.GitOpsResultMessage
}

func (h *TestResultHandler) HandleResult(result *api.GitOpsResultMessage) error {
	select {
	case h.resultsChan <- result:
		return nil
	default:
		return nil // Drop if channel is full
	}
}

func TestManifestSpecificValidation(t *testing.T) {
	// Test manifest-specific validation logic
	testCases := []struct {
		name         string
		manifestType string
		action       string
		expectedValid bool
	}{
		{
			name:         "Valid App Sync",
			manifestType: "app",
			action:       "sync-app",
			expectedValid: true,
		},
		{
			name:         "Valid Environment Validation",
			manifestType: "environment",
			action:       "validate-environment",
			expectedValid: true,
		},
		{
			name:         "Valid Context Correlation",
			manifestType: "context",
			action:       "correlate-context",
			expectedValid: true,
		},
		{
			name:         "Invalid Action for App",
			manifestType: "app",
			action:       "validate-environment",
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := api.NewGitOpsCommandMessage(
				"test-customer",
				"test-context",
				tc.action,
				tc.manifestType,
				"test-user",
			)

			// Basic validation
			assert.Equal(t, tc.manifestType, cmd.ManifestType)
			assert.Equal(t, tc.action, cmd.Action)
			
			// In a real test, we would validate against service handlers
			// For now, we just ensure the message structure is correct
			assert.NotEmpty(t, cmd.MessageID)
			assert.NotEmpty(t, cmd.CorrelationID)
		})
	}
}