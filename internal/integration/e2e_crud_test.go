package integration

import (
	"context"
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/storage"
	"github.com/kriipke/platformctl/internal/testutil"
	"github.com/kriipke/platformctl/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_CompleteCRUDWorkflow tests the complete CRUD workflow for App, Environment, and Context manifests
// This test demonstrates the three-manifest architecture and their relationships
func TestE2E_CompleteCRUDWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	ctx := context.Background()
	customerID := "e2e-customer-123"

	// Initialize stores
	appStore := storage.NewAppStore(testDB.DB)
	envStore := storage.NewEnvironmentStore(testDB.DB)
	contextStore := storage.NewContextStore(testDB.DB)

	// ========================================
	// PHASE 1: CREATE MANIFESTS
	// ========================================

	t.Run("Phase1_CreateManifests", func(t *testing.T) {
		// 1.1: Create App manifest
		app := testutil.CreateTestAppWithMultipleSources("e2e-web-app")
		app.Spec.Environments = []models.AppEnvironmentRef{
			{Name: "dev", EnvironmentRef: "e2e-dev-env"},
			{Name: "staging", EnvironmentRef: "e2e-staging-env"},
			{Name: "prod", EnvironmentRef: "e2e-prod-env"},
		}

		// Validate app before creation
		err := validation.ValidateApp(&app)
		require.NoError(t, err, "App validation should pass")

		// Create app
		err = appStore.Create(ctx, &app, customerID)
		require.NoError(t, err, "App creation should succeed")

		t.Logf("✓ Created App manifest: %s", app.Metadata.Name)

		// 1.2: Create Environment manifests
		environments := []models.Environment{
			testutil.CreateTestEnvironment("e2e-dev-env"),
			testutil.CreateTestEnvironment("e2e-staging-env"),
			testutil.CreateTestEnvironment("e2e-prod-env"),
		}

		// Configure environments differently
		environments[0].Spec.Environment.Namespace = "dev-apps"
		environments[1].Spec.Environment.Namespace = "staging-apps"
		environments[1].Spec.Vault.Auth = models.VaultAuthConfig{
			Method: "token",
			Token:  "s.STAGING_TOKEN_EXAMPLE",
		}
		environments[2].Spec.Environment.Namespace = "prod-apps"
		environments[2].Spec.PodEnvValidation.Enabled = true
		environments[2].Spec.PodEnvValidation.ExpectedEnvVars = []models.ExpectedEnvVar{
			{Name: "DATABASE_URL", SecretRef: "app-secrets", Key: "database-url"},
			{Name: "REDIS_URL", SecretRef: "app-secrets", Key: "redis-url"},
		}

		for _, env := range environments {
			// Validate environment before creation
			err := validation.ValidateEnvironment(&env)
			require.NoError(t, err, "Environment validation should pass for %s", env.Metadata.Name)

			// Create environment
			err = envStore.Create(ctx, &env, customerID)
			require.NoError(t, err, "Environment creation should succeed for %s", env.Metadata.Name)

			t.Logf("✓ Created Environment manifest: %s", env.Metadata.Name)
		}

		// 1.3: Create Context manifest that pairs App and Environments
		contextDeployments := []models.ContextDeployment{
			{
				Environment:    "dev",
				AppRef:         "e2e-web-app",
				EnvironmentRef: "e2e-dev-env",
				Active:         true,
			},
			{
				Environment:    "staging",
				AppRef:         "e2e-web-app",
				EnvironmentRef: "e2e-staging-env",
				Active:         true,
			},
			{
				Environment:    "prod",
				AppRef:         "e2e-web-app",
				EnvironmentRef: "e2e-prod-env",
				Active:         false, // Not yet deployed to production
			},
		}

		contextObj := testutil.CreateTestContext("e2e-web-app-context", "e2e-web-app", contextDeployments)
		contextObj.Spec.GitOps.CustomerBranch = models.CustomerBranchConfig{
			Enabled: true,
			Branch:  "customer/e2e-test-client",
		}
		contextObj.Spec.GitOps.Monitoring = models.MonitoringConfig{
			ApplicationSets:       true,
			VaultSecrets:          true,
			HelmValues:            true,
			CrossEnvironmentDrift: true,
		}

		// Validate context before creation
		err = validation.ValidateContext(&contextObj)
		require.NoError(t, err, "Context validation should pass")

		// Create context
		err = contextStore.Create(ctx, &contextObj, customerID)
		require.NoError(t, err, "Context creation should succeed")

		t.Logf("✓ Created Context manifest: %s", contextObj.Metadata.Name)
	})

	// ========================================
	// PHASE 2: READ AND VERIFY RELATIONSHIPS
	// ========================================

	t.Run("Phase2_ReadAndVerifyRelationships", func(t *testing.T) {
		// 2.1: Read all manifests
		app, err := appStore.Get(ctx, "e2e-web-app", customerID)
		require.NoError(t, err)

		devEnv, err := envStore.Get(ctx, "e2e-dev-env", customerID)
		require.NoError(t, err)

		stagingEnv, err := envStore.Get(ctx, "e2e-staging-env", customerID)
		require.NoError(t, err)

		prodEnv, err := envStore.Get(ctx, "e2e-prod-env", customerID)
		require.NoError(t, err)

		contextObj, err := contextStore.Get(ctx, "e2e-web-app-context", customerID)
		require.NoError(t, err)

		// 2.2: Verify App-Environment relationships
		appEnvRefs := make(map[string]string)
		for _, envRef := range app.Spec.Environments {
			appEnvRefs[envRef.Name] = envRef.EnvironmentRef
		}

		assert.Equal(t, "e2e-dev-env", appEnvRefs["dev"])
		assert.Equal(t, "e2e-staging-env", appEnvRefs["staging"])
		assert.Equal(t, "e2e-prod-env", appEnvRefs["prod"])

		// 2.3: Verify Context-App-Environment relationships
		assert.Equal(t, "e2e-web-app", contextObj.Spec.AppRef)
		assert.Len(t, contextObj.Spec.Deployments, 3)

		for _, deployment := range contextObj.Spec.Deployments {
			assert.Equal(t, "e2e-web-app", deployment.AppRef, "All deployments should reference the same app")

			switch deployment.Environment {
			case "dev":
				assert.Equal(t, "e2e-dev-env", deployment.EnvironmentRef)
				assert.True(t, deployment.Active)
			case "staging":
				assert.Equal(t, "e2e-staging-env", deployment.EnvironmentRef)
				assert.True(t, deployment.Active)
			case "prod":
				assert.Equal(t, "e2e-prod-env", deployment.EnvironmentRef)
				assert.False(t, deployment.Active)
			}
		}

		// 2.4: Verify environment-specific configurations
		assert.Equal(t, "dev-apps", devEnv.Spec.Environment.Namespace)
		assert.Equal(t, "staging-apps", stagingEnv.Spec.Environment.Namespace)
		assert.Equal(t, "prod-apps", prodEnv.Spec.Environment.Namespace)

		assert.Equal(t, "kubernetes", devEnv.Spec.Vault.Auth.Method)
		assert.Equal(t, "token", stagingEnv.Spec.Vault.Auth.Method)
		assert.Equal(t, "kubernetes", prodEnv.Spec.Vault.Auth.Method)

		assert.False(t, devEnv.Spec.PodEnvValidation.Enabled)
		assert.False(t, stagingEnv.Spec.PodEnvValidation.Enabled)
		assert.True(t, prodEnv.Spec.PodEnvValidation.Enabled)
		assert.Len(t, prodEnv.Spec.PodEnvValidation.ExpectedEnvVars, 2)

		// 2.5: Verify context GitOps configuration
		assert.True(t, contextObj.Spec.GitOps.CustomerBranch.Enabled)
		assert.Equal(t, "customer/e2e-test-client", contextObj.Spec.GitOps.CustomerBranch.Branch)
		assert.True(t, contextObj.Spec.GitOps.Monitoring.ApplicationSets)
		assert.True(t, contextObj.Spec.GitOps.Monitoring.CrossEnvironmentDrift)

		t.Log("✓ Verified all manifest relationships and configurations")
	})

	// ========================================
	// PHASE 3: UPDATE MANIFESTS AND RELATIONSHIPS
	// ========================================

	t.Run("Phase3_UpdateManifestsAndRelationships", func(t *testing.T) {
		// 3.1: Update App manifest (add new environment)
		app, err := appStore.Get(ctx, "e2e-web-app", customerID)
		require.NoError(t, err)

		app.Spec.Application.Version = "2.0.0"
		app.Spec.Environments = append(app.Spec.Environments, models.AppEnvironmentRef{
			Name:           "canary",
			EnvironmentRef: "e2e-canary-env",
		})

		err = appStore.Update(ctx, app, customerID)
		require.NoError(t, err)

		t.Log("✓ Updated App manifest with new version and canary environment")

		// 3.2: Create new canary environment
		canaryEnv := testutil.CreateTestEnvironment("e2e-canary-env")
		canaryEnv.Spec.Environment.Namespace = "canary-apps"
		canaryEnv.Spec.Vault.Auth = models.VaultAuthConfig{
			Method: "token",
			Token:  "s.CANARY_TOKEN_EXAMPLE",
		}

		err = envStore.Create(ctx, &canaryEnv, customerID)
		require.NoError(t, err)

		t.Log("✓ Created new canary Environment manifest")

		// 3.3: Update Context manifest to include canary deployment
		contextObj, err := contextStore.Get(ctx, "e2e-web-app-context", customerID)
		require.NoError(t, err)

		// Activate production deployment and add canary
		for i := range contextObj.Spec.Deployments {
			if contextObj.Spec.Deployments[i].Environment == "prod" {
				contextObj.Spec.Deployments[i].Active = true
			}
		}

		contextObj.Spec.Deployments = append(contextObj.Spec.Deployments, models.ContextDeployment{
			Environment:    "canary",
			AppRef:         "e2e-web-app",
			EnvironmentRef: "e2e-canary-env",
			Active:         true,
		})

		// Update monitoring configuration
		contextObj.Spec.GitOps.Monitoring.HelmValues = false

		err = contextStore.Update(ctx, contextObj, customerID)
		require.NoError(t, err)

		t.Log("✓ Updated Context manifest with production activation and canary deployment")

		// 3.4: Verify updates
		updatedApp, err := appStore.Get(ctx, "e2e-web-app", customerID)
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", updatedApp.Spec.Application.Version)
		assert.Len(t, updatedApp.Spec.Environments, 4)

		updatedContext, err := contextStore.Get(ctx, "e2e-web-app-context", customerID)
		require.NoError(t, err)
		assert.Len(t, updatedContext.Spec.Deployments, 4)

		// Count active deployments
		activeCount := 0
		for _, deployment := range updatedContext.Spec.Deployments {
			if deployment.Active {
				activeCount++
			}
		}
		assert.Equal(t, 4, activeCount, "All deployments should now be active")
		assert.False(t, updatedContext.Spec.GitOps.Monitoring.HelmValues)

		t.Log("✓ Verified all updates were applied correctly")
	})

	// ========================================
	// PHASE 4: LIST AND QUERY OPERATIONS
	// ========================================

	t.Run("Phase4_ListAndQueryOperations", func(t *testing.T) {
		// 4.1: List all manifests for customer
		apps, err := appStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, apps, 1)
		assert.Equal(t, "e2e-web-app", apps[0].Metadata.Name)

		envs, err := envStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, envs, 4)

		// Verify environments are sorted
		envNames := make([]string, len(envs))
		for i, env := range envs {
			envNames[i] = env.Metadata.Name
		}
		assert.Equal(t, []string{"e2e-canary-env", "e2e-dev-env", "e2e-prod-env", "e2e-staging-env"}, envNames)

		contexts, err := contextStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, contexts, 1)
		assert.Equal(t, "e2e-web-app-context", contexts[0].Metadata.Name)

		// 4.2: Query contexts by app and environment references
		devContexts, err := contextStore.GetByAppAndEnvironment(ctx, "e2e-web-app", "e2e-dev-env", customerID)
		require.NoError(t, err)
		assert.Len(t, devContexts, 1)
		assert.Equal(t, "e2e-web-app-context", devContexts[0].Metadata.Name)

		// Query with non-existent references
		nonExistentContexts, err := contextStore.GetByAppAndEnvironment(ctx, "non-existent-app", "e2e-dev-env", customerID)
		require.NoError(t, err)
		assert.Len(t, nonExistentContexts, 0)

		t.Log("✓ Verified list and query operations")
	})

	// ========================================
	// PHASE 5: COMPLEX RELATIONSHIP SCENARIOS
	// ========================================

	t.Run("Phase5_ComplexRelationshipScenarios", func(t *testing.T) {
		// 5.1: Create second app that shares some environments
		app2 := testutil.CreateTestAppWithGitSource("e2e-api-service")
		app2.Spec.Environments = []models.AppEnvironmentRef{
			{Name: "dev", EnvironmentRef: "e2e-dev-env"},   // Shared environment
			{Name: "prod", EnvironmentRef: "e2e-prod-env"}, // Shared environment
		}

		err := appStore.Create(ctx, &app2, customerID)
		require.NoError(t, err)

		// 5.2: Create context for second app
		context2Deployments := []models.ContextDeployment{
			{
				Environment:    "dev",
				AppRef:         "e2e-api-service",
				EnvironmentRef: "e2e-dev-env",
				Active:         true,
			},
			{
				Environment:    "prod",
				AppRef:         "e2e-api-service",
				EnvironmentRef: "e2e-prod-env",
				Active:         false,
			},
		}

		context2 := testutil.CreateTestContext("e2e-api-service-context", "e2e-api-service", context2Deployments)
		context2.Spec.GitOps.CustomerBranch.Enabled = false // Different configuration

		err = contextStore.Create(ctx, &context2, customerID)
		require.NoError(t, err)

		// 5.3: Verify both contexts can reference the same environments
		devContexts, err := contextStore.GetByAppAndEnvironment(ctx, "e2e-web-app", "e2e-dev-env", customerID)
		require.NoError(t, err)
		assert.Len(t, devContexts, 1)

		devContextsApp2, err := contextStore.GetByAppAndEnvironment(ctx, "e2e-api-service", "e2e-dev-env", customerID)
		require.NoError(t, err)
		assert.Len(t, devContextsApp2, 1)

		// 5.4: Verify different apps can have different environment configurations
		allContexts, err := contextStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, allContexts, 2)

		// Find each context
		var webAppContext, apiServiceContext *models.Context
		for _, c := range allContexts {
			if c.Spec.AppRef == "e2e-web-app" {
				webAppContext = c
			} else if c.Spec.AppRef == "e2e-api-service" {
				apiServiceContext = c
			}
		}

		require.NotNil(t, webAppContext)
		require.NotNil(t, apiServiceContext)

		// Verify different configurations
		assert.True(t, webAppContext.Spec.GitOps.CustomerBranch.Enabled)
		assert.False(t, apiServiceContext.Spec.GitOps.CustomerBranch.Enabled)
		assert.Len(t, webAppContext.Spec.Deployments, 4)     // web-app has canary
		assert.Len(t, apiServiceContext.Spec.Deployments, 2) // api-service doesn't have canary

		t.Log("✓ Verified complex relationship scenarios with shared environments")
	})

	// ========================================
	// PHASE 6: DELETE OPERATIONS AND CLEANUP
	// ========================================

	t.Run("Phase6_DeleteOperationsAndCleanup", func(t *testing.T) {
		// 6.1: Delete contexts first (dependent on apps and environments)
		err := contextStore.Delete(ctx, "e2e-web-app-context", customerID)
		require.NoError(t, err)

		err = contextStore.Delete(ctx, "e2e-api-service-context", customerID)
		require.NoError(t, err)

		// Verify contexts are deleted
		contexts, err := contextStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, contexts, 0)

		// 6.2: Delete apps
		err = appStore.Delete(ctx, "e2e-web-app", customerID)
		require.NoError(t, err)

		err = appStore.Delete(ctx, "e2e-api-service", customerID)
		require.NoError(t, err)

		// Verify apps are deleted
		apps, err := appStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, apps, 0)

		// 6.3: Delete environments
		envNames := []string{"e2e-dev-env", "e2e-staging-env", "e2e-prod-env", "e2e-canary-env"}
		for _, envName := range envNames {
			err = envStore.Delete(ctx, envName, customerID)
			require.NoError(t, err)
		}

		// Verify environments are deleted
		envs, err := envStore.List(ctx, customerID)
		require.NoError(t, err)
		assert.Len(t, envs, 0)

		// 6.4: Verify all resources are cleaned up
		// Try to get deleted resources - should return not found
		_, err = appStore.Get(ctx, "e2e-web-app", customerID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = envStore.Get(ctx, "e2e-dev-env", customerID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		_, err = contextStore.Get(ctx, "e2e-web-app-context", customerID)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		t.Log("✓ Successfully cleaned up all resources")
	})
}

// TestE2E_ManifestValidationWorkflow tests the validation workflow throughout the CRUD lifecycle
func TestE2E_ManifestValidationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end validation tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	ctx := context.Background()
	customerID := "validation-customer-456"

	appStore := storage.NewAppStore(testDB.DB)
	envStore := storage.NewEnvironmentStore(testDB.DB)
	contextStore := storage.NewContextStore(testDB.DB)

	t.Run("ValidationWorkflow", func(t *testing.T) {
		// Test invalid manifests are rejected
		t.Run("RejectInvalidManifests", func(t *testing.T) {
			// Invalid app
			invalidApp := testutil.CreateTestApp("invalid-app")
			invalidApp.Spec.Application.Version = "invalid-version"  // Not semver
			invalidApp.Spec.Application.Maintainer = "invalid-email" // Not email

			err := validation.ValidateApp(&invalidApp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "version must be a valid semantic version")

			// Invalid environment
			invalidEnv := testutil.CreateTestEnvironment("invalid-env")
			invalidEnv.Spec.Vault.Auth.Method = "invalid-method"

			err = validation.ValidateEnvironment(&invalidEnv)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "method must be one of: kubernetes, token")

			// Invalid context
			invalidContext := testutil.CreateTestContext("invalid-context", "some-app", []models.ContextDeployment{
				{
					Environment:    "invalid-env-type",
					AppRef:         "some-app",
					EnvironmentRef: "some-env",
					Active:         false, // No active deployments
				},
			})

			err = validation.ValidateContext(&invalidContext)
			assert.Error(t, err)
			// Should fail due to both invalid environment name and no active deployments
		})

		// Test valid manifests are accepted
		t.Run("AcceptValidManifests", func(t *testing.T) {
			// Valid app
			validApp := testutil.CreateTestApp("valid-app")
			err := validation.ValidateApp(&validApp)
			assert.NoError(t, err)

			err = appStore.Create(ctx, &validApp, customerID)
			assert.NoError(t, err)

			// Valid environment
			validEnv := testutil.CreateTestEnvironment("valid-env")
			err = validation.ValidateEnvironment(&validEnv)
			assert.NoError(t, err)

			err = envStore.Create(ctx, &validEnv, customerID)
			assert.NoError(t, err)

			// Valid context
			validContext := testutil.CreateTestContext("valid-context", "valid-app",
				testutil.CreateTestContextDeployments("valid-app", "valid-env"))
			err = validation.ValidateContext(&validContext)
			assert.NoError(t, err)

			err = contextStore.Create(ctx, &validContext, customerID)
			assert.NoError(t, err)

			t.Log("✓ All valid manifests were accepted and stored")
		})
	})
}

// TestE2E_ErrorHandlingAndRecovery tests error scenarios and recovery mechanisms
func TestE2E_ErrorHandlingAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end error handling tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	ctx := context.Background()
	customerID := "error-customer-789"

	appStore := storage.NewAppStore(testDB.DB)
	envStore := storage.NewEnvironmentStore(testDB.DB)
	contextStore := storage.NewContextStore(testDB.DB)

	t.Run("ErrorHandlingWorkflow", func(t *testing.T) {
		// Test duplicate creation errors
		t.Run("DuplicateCreationErrors", func(t *testing.T) {
			app := testutil.CreateTestApp("duplicate-app")

			err := appStore.Create(ctx, &app, customerID)
			assert.NoError(t, err, "First creation should succeed")

			err = appStore.Create(ctx, &app, customerID)
			assert.Error(t, err, "Duplicate creation should fail")
			assert.ErrorIs(t, err, storage.ErrConflict)
		})

		// Test not found errors
		t.Run("NotFoundErrors", func(t *testing.T) {
			_, err := appStore.Get(ctx, "non-existent-app", customerID)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			_, err = envStore.Get(ctx, "non-existent-env", customerID)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			_, err = contextStore.Get(ctx, "non-existent-context", customerID)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Update non-existent resources
			app := testutil.CreateTestApp("non-existent-app")
			err = appStore.Update(ctx, &app, customerID)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Delete non-existent resources
			err = appStore.Delete(ctx, "non-existent-app", customerID)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		// Test cross-customer isolation
		t.Run("CrossCustomerIsolation", func(t *testing.T) {
			customer1 := "customer-1"
			customer2 := "customer-2"

			// Create app for customer 1
			app := testutil.CreateTestApp("isolated-app")
			err := appStore.Create(ctx, &app, customer1)
			assert.NoError(t, err)

			// Customer 2 should not see customer 1's app
			_, err = appStore.Get(ctx, "isolated-app", customer2)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Customer 2 should not be able to update customer 1's app
			err = appStore.Update(ctx, &app, customer2)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			// Customer 2 should not be able to delete customer 1's app
			err = appStore.Delete(ctx, "isolated-app", customer2)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			t.Log("✓ Verified cross-customer isolation")
		})
	})
}
