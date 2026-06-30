package storage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/storage"
	"github.com/kriipke/platformctl/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextStore_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	tests := []struct {
		name        string
		context     models.Context
		expectError bool
		errorType   error
	}{
		{
			name: "valid context creation",
			context: testutil.CreateTestContext("test-context-1", "test-app-1", 
				testutil.CreateTestContextDeployments("test-app-1", "dev-env", "prod-env")),
			expectError: false,
		},
		{
			name: "context with customer branch",
			context: testutil.CreateTestCustomerBranchContext("test-context-branch", "branch-app", "customer/enterprise-client"),
			expectError: false,
		},
		{
			name: "context with single deployment",
			context: testutil.CreateTestContext("test-context-single", "single-app", 
				testutil.CreateTestContextDeployments("single-app", "prod-env")),
			expectError: false,
		},
		{
			name: "context with multiple deployments",
			context: testutil.CreateTestContext("test-context-multi", "multi-app", 
				testutil.CreateTestContextDeployments("multi-app", "dev-env", "staging-env", "prod-env")),
			expectError: false,
		},
		{
			name:        "duplicate context creation should fail",
			context:     testutil.CreateTestContext("test-context-1", "test-app-1", testutil.CreateTestContextDeployments("test-app-1", "dev-env")),
			expectError: true,
			errorType:   storage.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Create(ctx, &tt.context, customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify timestamps are set
				assert.NotNil(t, tt.context.Metadata.CreatedAt)
				assert.NotNil(t, tt.context.Metadata.UpdatedAt)
			}
		})
	}
}

func TestContextStore_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test context
	originalContext := testutil.CreateTestContext("get-test-context", "get-app", 
		testutil.CreateTestContextDeployments("get-app", "dev-env", "prod-env"))
	err := store.Create(ctx, &originalContext, customerID)
	require.NoError(t, err)

	tests := []struct {
		name            string
		contextName     string
		customerID      string
		expectError     bool
		expectedContext *models.Context
		errorType       error
	}{
		{
			name:            "get existing context",
			contextName:     "get-test-context",
			customerID:      customerID,
			expectError:     false,
			expectedContext: &originalContext,
		},
		{
			name:        "get non-existent context",
			contextName: "non-existent-context",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "get context with wrong customer ID",
			contextName: "get-test-context",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextObj, err := store.Get(ctx, tt.contextName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, contextObj)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, contextObj)
				
				// Compare contexts (ignoring timestamps)
				testutil.AssertContextEqual(t, *tt.expectedContext, *contextObj)
			}
		})
	}
}

func TestContextStore_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test context
	originalContext := testutil.CreateTestContext("update-test-context", "update-app", 
		testutil.CreateTestContextDeployments("update-app", "dev-env", "prod-env"))
	err := store.Create(ctx, &originalContext, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setupFunc   func() models.Context
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name: "update existing context",
			setupFunc: func() models.Context {
				contextObj := originalContext
				// Enable customer branch
				contextObj.Spec.GitOps.CustomerBranch = models.CustomerBranchConfig{
					Enabled: true,
					Branch:  "customer/updated-client",
				}
				// Update monitoring settings
				contextObj.Spec.GitOps.Monitoring.VaultSecrets = false
				contextObj.Spec.GitOps.Monitoring.CrossEnvironmentDrift = false
				return contextObj
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update context deployments",
			setupFunc: func() models.Context {
				contextObj := originalContext
				// Change deployment states
				for i := range contextObj.Spec.Deployments {
					contextObj.Spec.Deployments[i].Active = i == 1 // Only second deployment active
				}
				// Add new deployment
				contextObj.Spec.Deployments = append(contextObj.Spec.Deployments, models.ContextDeployment{
					Environment:    "staging",
					AppRef:         "update-app",
					EnvironmentRef: "staging-env",
					Active:         false,
				})
				return contextObj
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update context app reference",
			setupFunc: func() models.Context {
				contextObj := originalContext
				contextObj.Spec.AppRef = "updated-app"
				// Update all deployment app refs to match
				for i := range contextObj.Spec.Deployments {
					contextObj.Spec.Deployments[i].AppRef = "updated-app"
				}
				return contextObj
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update non-existent context",
			setupFunc: func() models.Context {
				contextObj := testutil.CreateTestContext("non-existent-context", "some-app", 
					testutil.CreateTestContextDeployments("some-app", "dev-env"))
				return contextObj
			},
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name: "update context with wrong customer ID",
			setupFunc: func() models.Context {
				return originalContext
			},
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextObj := tt.setupFunc()
			originalUpdateTime := contextObj.Metadata.UpdatedAt
			
			err := store.Update(ctx, &contextObj, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify UpdatedAt timestamp is updated
				assert.NotNil(t, contextObj.Metadata.UpdatedAt)
				if originalUpdateTime != nil {
					assert.True(t, contextObj.Metadata.UpdatedAt.After(*originalUpdateTime))
				}
				
				// Verify the update by fetching the context
				updatedContext, err := store.Get(ctx, contextObj.Metadata.Name, tt.customerID)
				assert.NoError(t, err)
				testutil.AssertContextEqual(t, contextObj, *updatedContext)
			}
		})
	}
}

func TestContextStore_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test contexts
	context1 := testutil.CreateTestContext("delete-test-context-1", "delete-app-1", 
		testutil.CreateTestContextDeployments("delete-app-1", "dev-env"))
	context2 := testutil.CreateTestContext("delete-test-context-2", "delete-app-2", 
		testutil.CreateTestContextDeployments("delete-app-2", "prod-env"))
	
	err := store.Create(ctx, &context1, customerID)
	require.NoError(t, err)
	err = store.Create(ctx, &context2, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		contextName string
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name:        "delete existing context",
			contextName: "delete-test-context-1",
			customerID:  customerID,
			expectError: false,
		},
		{
			name:        "delete non-existent context",
			contextName: "non-existent-context",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete context with wrong customer ID",
			contextName: "delete-test-context-2",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete already deleted context",
			contextName: "delete-test-context-1",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(ctx, tt.contextName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify the context is deleted
				_, err := store.Get(ctx, tt.contextName, tt.customerID)
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	}
}

func TestContextStore_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID1 := "test-customer-1"
	customerID2 := "test-customer-2"

	// Create test contexts for customer 1
	customer1Contexts := []models.Context{
		testutil.CreateTestContext("customer1-context-1", "app-1", 
			testutil.CreateTestContextDeployments("app-1", "dev-env")),
		testutil.CreateTestContext("customer1-context-2", "app-2", 
			testutil.CreateTestContextDeployments("app-2", "prod-env")),
		testutil.CreateTestCustomerBranchContext("customer1-context-3", "app-3", "customer/client-1"),
	}
	
	for _, contextObj := range customer1Contexts {
		err := store.Create(ctx, &contextObj, customerID1)
		require.NoError(t, err)
	}

	// Create test contexts for customer 2
	customer2Contexts := []models.Context{
		testutil.CreateTestContext("customer2-context-1", "app-1", 
			testutil.CreateTestContextDeployments("app-1", "staging-env")),
	}
	
	for _, contextObj := range customer2Contexts {
		err := store.Create(ctx, &contextObj, customerID2)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		customerID    string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "list contexts for customer 1",
			customerID:    customerID1,
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "list contexts for customer 2",
			customerID:    customerID2,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "list contexts for non-existent customer",
			customerID:    "non-existent-customer",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contexts, err := store.List(ctx, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, contexts, tt.expectedCount)
				
				// Verify all contexts belong to the correct customer
				for _, contextObj := range contexts {
					assert.NotNil(t, contextObj)
					assert.NotEmpty(t, contextObj.Metadata.Name)
					assert.Equal(t, "contextops/v1", contextObj.APIVersion)
					assert.Equal(t, "Context", contextObj.Kind)
				}
				
				// Verify contexts are sorted by name
				if len(contexts) > 1 {
					for i := 1; i < len(contexts); i++ {
						assert.True(t, contexts[i-1].Metadata.Name < contexts[i].Metadata.Name,
							"Contexts should be sorted by name: %s should come before %s",
							contexts[i-1].Metadata.Name, contexts[i].Metadata.Name)
					}
				}
			}
		})
	}
}

func TestContextStore_GetByAppAndEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test contexts
	context1 := testutil.CreateTestContext("context-app1-dev", "app-1", 
		testutil.CreateTestContextDeployments("app-1", "dev-env"))
	context2 := testutil.CreateTestContext("context-app1-prod", "app-1", 
		testutil.CreateTestContextDeployments("app-1", "prod-env"))
	context3 := testutil.CreateTestContext("context-app2-dev", "app-2", 
		testutil.CreateTestContextDeployments("app-2", "dev-env"))

	contexts := []models.Context{context1, context2, context3}
	for _, contextObj := range contexts {
		err := store.Create(ctx, &contextObj, customerID)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		appRef        string
		envRef        string
		customerID    string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "get contexts for app-1 and dev-env",
			appRef:        "app-1",
			envRef:        "dev-env",
			customerID:    customerID,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "get contexts for app-1 (any environment)",
			appRef:        "app-1",
			envRef:        "prod-env",
			customerID:    customerID,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "get contexts for non-existent app",
			appRef:        "non-existent-app",
			envRef:        "dev-env",
			customerID:    customerID,
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "get contexts for non-existent environment",
			appRef:        "app-1",
			envRef:        "non-existent-env",
			customerID:    customerID,
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "get contexts with wrong customer ID",
			appRef:        "app-1",
			envRef:        "dev-env",
			customerID:    "wrong-customer",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contexts, err := store.GetByAppAndEnvironment(ctx, tt.appRef, tt.envRef, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, contexts, tt.expectedCount)
				
				// Verify all contexts match the query
				for _, contextObj := range contexts {
					assert.Equal(t, tt.appRef, contextObj.Spec.AppRef)
					// Note: The current implementation stores app_reference and environment_reference 
					// from the first deployment, so we check the first deployment
					if len(contextObj.Spec.Deployments) > 0 {
						assert.Equal(t, tt.envRef, contextObj.Spec.Deployments[0].EnvironmentRef)
					}
				}
			}
		})
	}
}

func TestContextStore_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-concurrent"

	// Test concurrent creations
	t.Run("concurrent_creates", func(t *testing.T) {
		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				contextObj := testutil.CreateTestContext(fmt.Sprintf("concurrent-context-%d", id), 
					fmt.Sprintf("concurrent-app-%d", id), 
					testutil.CreateTestContextDeployments(fmt.Sprintf("concurrent-app-%d", id), "dev-env"))
				err := store.Create(ctx, &contextObj, customerID)
				done <- err
			}(i)
		}

		// Collect results
		var errors []error
		for i := 0; i < numGoroutines; i++ {
			if err := <-done; err != nil {
				errors = append(errors, err)
			}
		}

		// All creates should succeed since they have different names
		assert.Empty(t, errors)

		// Verify all contexts were created
		contexts, err := store.List(ctx, customerID)
		assert.NoError(t, err)
		assert.Len(t, contexts, numGoroutines)
	})

	// Clean up for next test
	testDB.Cleanup(t)

	// Test concurrent reads of the same context
	t.Run("concurrent_reads", func(t *testing.T) {
		// Create a test context first
		contextObj := testutil.CreateTestContext("concurrent-read-context", "read-app", 
			testutil.CreateTestContextDeployments("read-app", "dev-env"))
		err := store.Create(ctx, &contextObj, customerID)
		require.NoError(t, err)

		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := store.Get(ctx, contextObj.Metadata.Name, customerID)
				done <- err
			}()
		}

		// Collect results
		var errors []error
		for i := 0; i < numGoroutines; i++ {
			if err := <-done; err != nil {
				errors = append(errors, err)
			}
		}

		// All reads should succeed
		assert.Empty(t, errors)
	})
}

func TestContextStore_ComplexDeploymentScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-complex"

	// Create context with complex deployment scenario
	contextObj := testutil.CreateTestContext("complex-deployment-context", "complex-app", []models.ContextDeployment{
		{
			Environment:    "development",
			AppRef:         "complex-app",
			EnvironmentRef: "dev-env",
			Active:         true, // Active development
		},
		{
			Environment:    "testing",
			AppRef:         "complex-app",
			EnvironmentRef: "test-env",
			Active:         true, // Active testing
		},
		{
			Environment:    "staging",
			AppRef:         "complex-app",
			EnvironmentRef: "staging-env",
			Active:         false, // Prepared but not active
		},
		{
			Environment:    "production",
			AppRef:         "complex-app",
			EnvironmentRef: "prod-env",
			Active:         true, // Active production
		},
	})

	// Configure monitoring for all aspects
	contextObj.Spec.GitOps.Monitoring = models.MonitoringConfig{
		ApplicationSets:       true,
		VaultSecrets:         true,
		HelmValues:           true,
		CrossEnvironmentDrift: true,
	}

	// Test create
	err := store.Create(ctx, &contextObj, customerID)
	assert.NoError(t, err)

	// Test get and verify all deployments are preserved
	retrievedContext, err := store.Get(ctx, contextObj.Metadata.Name, customerID)
	assert.NoError(t, err)

	assert.Len(t, retrievedContext.Spec.Deployments, 4)
	assert.Equal(t, "complex-app", retrievedContext.Spec.AppRef)

	// Verify deployment states
	activeCount := 0
	for _, deployment := range retrievedContext.Spec.Deployments {
		assert.Equal(t, "complex-app", deployment.AppRef)
		assert.NotEmpty(t, deployment.Environment)
		assert.NotEmpty(t, deployment.EnvironmentRef)
		if deployment.Active {
			activeCount++
		}
	}
	assert.Equal(t, 3, activeCount, "Should have 3 active deployments")

	// Verify monitoring configuration
	assert.True(t, retrievedContext.Spec.GitOps.Monitoring.ApplicationSets)
	assert.True(t, retrievedContext.Spec.GitOps.Monitoring.VaultSecrets)
	assert.True(t, retrievedContext.Spec.GitOps.Monitoring.HelmValues)
	assert.True(t, retrievedContext.Spec.GitOps.Monitoring.CrossEnvironmentDrift)

	// Test update by changing deployment states
	contextObj.Spec.Deployments[2].Active = true  // Activate staging
	contextObj.Spec.Deployments[3].Active = false // Deactivate production

	err = store.Update(ctx, &contextObj, customerID)
	assert.NoError(t, err)

	// Verify deployment state changes
	updatedContext, err := store.Get(ctx, contextObj.Metadata.Name, customerID)
	assert.NoError(t, err)

	activeCount = 0
	for _, deployment := range updatedContext.Spec.Deployments {
		if deployment.Active {
			activeCount++
		}
	}
	assert.Equal(t, 3, activeCount, "Should still have 3 active deployments after update")

	// Find specific deployments to verify state changes
	var stagingDeployment, prodDeployment *models.ContextDeployment
	for i, deployment := range updatedContext.Spec.Deployments {
		if deployment.Environment == "staging" {
			stagingDeployment = &updatedContext.Spec.Deployments[i]
		}
		if deployment.Environment == "production" {
			prodDeployment = &updatedContext.Spec.Deployments[i]
		}
	}
	
	require.NotNil(t, stagingDeployment)
	require.NotNil(t, prodDeployment)
	assert.True(t, stagingDeployment.Active, "Staging should be active after update")
	assert.False(t, prodDeployment.Active, "Production should be inactive after update")
}

func TestContextStore_CustomerBranchScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-branch"

	tests := []struct {
		name           string
		customerBranch models.CustomerBranchConfig
	}{
		{
			name: "disabled customer branch",
			customerBranch: models.CustomerBranchConfig{
				Enabled: false,
			},
		},
		{
			name: "enabled customer branch",
			customerBranch: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/enterprise-corp",
			},
		},
		{
			name: "customer branch with hyphens",
			customerBranch: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/big-client-company",
			},
		},
		{
			name: "customer branch with numbers",
			customerBranch: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/client123",
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextObj := testutil.CreateTestContext(fmt.Sprintf("branch-test-context-%d", i), 
				fmt.Sprintf("branch-app-%d", i), 
				testutil.CreateTestContextDeployments(fmt.Sprintf("branch-app-%d", i), "dev-env"))
			contextObj.Spec.GitOps.CustomerBranch = tt.customerBranch

			// Test create
			err := store.Create(ctx, &contextObj, customerID)
			assert.NoError(t, err)

			// Test get and verify customer branch configuration is preserved
			retrievedContext, err := store.Get(ctx, contextObj.Metadata.Name, customerID)
			assert.NoError(t, err)

			assert.Equal(t, tt.customerBranch.Enabled, retrievedContext.Spec.GitOps.CustomerBranch.Enabled)
			assert.Equal(t, tt.customerBranch.Branch, retrievedContext.Spec.GitOps.CustomerBranch.Branch)
		})
	}
}

func TestContextStore_MonitoringConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewContextStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-monitoring"

	tests := []struct {
		name       string
		monitoring models.MonitoringConfig
	}{
		{
			name: "all monitoring disabled",
			monitoring: models.MonitoringConfig{
				ApplicationSets:       false,
				VaultSecrets:         false,
				HelmValues:           false,
				CrossEnvironmentDrift: false,
			},
		},
		{
			name: "all monitoring enabled",
			monitoring: models.MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         true,
				HelmValues:           true,
				CrossEnvironmentDrift: true,
			},
		},
		{
			name: "selective monitoring",
			monitoring: models.MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         false,
				HelmValues:           true,
				CrossEnvironmentDrift: false,
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextObj := testutil.CreateTestContext(fmt.Sprintf("monitoring-test-context-%d", i), 
				fmt.Sprintf("monitoring-app-%d", i), 
				testutil.CreateTestContextDeployments(fmt.Sprintf("monitoring-app-%d", i), "prod-env"))
			contextObj.Spec.GitOps.Monitoring = tt.monitoring

			// Test create
			err := store.Create(ctx, &contextObj, customerID)
			assert.NoError(t, err)

			// Test get and verify monitoring configuration is preserved
			retrievedContext, err := store.Get(ctx, contextObj.Metadata.Name, customerID)
			assert.NoError(t, err)

			monitoring := retrievedContext.Spec.GitOps.Monitoring
			assert.Equal(t, tt.monitoring.ApplicationSets, monitoring.ApplicationSets)
			assert.Equal(t, tt.monitoring.VaultSecrets, monitoring.VaultSecrets)
			assert.Equal(t, tt.monitoring.HelmValues, monitoring.HelmValues)
			assert.Equal(t, tt.monitoring.CrossEnvironmentDrift, monitoring.CrossEnvironmentDrift)
		})
	}
}