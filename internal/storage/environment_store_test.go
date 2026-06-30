package storage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/contextops/platformctl/internal/models"
	"github.com/contextops/platformctl/internal/storage"
	"github.com/contextops/platformctl/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentStore_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	tests := []struct {
		name        string
		env         models.Environment
		expectError bool
		errorType   error
	}{
		{
			name:        "valid environment creation",
			env:         testutil.CreateTestEnvironment("test-env-1"),
			expectError: false,
		},
		{
			name:        "environment with token auth",
			env:         testutil.CreateTestEnvironmentWithTokenAuth("test-env-token"),
			expectError: false,
		},
		{
			name:        "minimal environment creation",
			env:         testutil.CreateTestEnvironmentMinimal("test-env-minimal"),
			expectError: false,
		},
		{
			name:        "duplicate environment creation should fail",
			env:         testutil.CreateTestEnvironment("test-env-1"), // Same name as first test
			expectError: true,
			errorType:   storage.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Create(ctx, &tt.env, customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify timestamps are set
				assert.NotNil(t, tt.env.Metadata.CreatedAt)
				assert.NotNil(t, tt.env.Metadata.UpdatedAt)
			}
		})
	}
}

func TestEnvironmentStore_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test environment
	originalEnv := testutil.CreateTestEnvironment("get-test-env")
	err := store.Create(ctx, &originalEnv, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		envName     string
		customerID  string
		expectError bool
		expectedEnv *models.Environment
		errorType   error
	}{
		{
			name:        "get existing environment",
			envName:     "get-test-env",
			customerID:  customerID,
			expectError: false,
			expectedEnv: &originalEnv,
		},
		{
			name:        "get non-existent environment",
			envName:     "non-existent-env",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "get environment with wrong customer ID",
			envName:     "get-test-env",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := store.Get(ctx, tt.envName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, env)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, env)
				
				// Compare environments (ignoring timestamps)
				testutil.AssertEnvironmentEqual(t, *tt.expectedEnv, *env)
			}
		})
	}
}

func TestEnvironmentStore_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test environment
	originalEnv := testutil.CreateTestEnvironment("update-test-env")
	err := store.Create(ctx, &originalEnv, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setupFunc   func() models.Environment
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name: "update existing environment",
			setupFunc: func() models.Environment {
				env := originalEnv
				env.Spec.Environment.Namespace = "updated-apps"
				env.Spec.Vault.Address = "https://updated-vault.example.com"
				return env
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update environment auth method",
			setupFunc: func() models.Environment {
				env := originalEnv
				env.Spec.Vault.Auth = models.VaultAuthConfig{
					Method: "token",
					Token:  "s.UPDATED_TOKEN_EXAMPLE",
				}
				return env
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update environment vault secrets",
			setupFunc: func() models.Environment {
				env := originalEnv
				env.Spec.VaultSecrets = append(env.Spec.VaultSecrets, models.VaultStaticSecret{
					Name:              "new-secrets",
					VaultPath:         "secret/new-app/updated",
					DestinationSecret: "new-secrets",
					RequiredKeys:      []string{"new-key"},
				})
				return env
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update non-existent environment",
			setupFunc: func() models.Environment {
				env := testutil.CreateTestEnvironment("non-existent-env")
				return env
			},
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name: "update environment with wrong customer ID",
			setupFunc: func() models.Environment {
				return originalEnv
			},
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := tt.setupFunc()
			originalUpdateTime := env.Metadata.UpdatedAt
			
			err := store.Update(ctx, &env, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify UpdatedAt timestamp is updated
				assert.NotNil(t, env.Metadata.UpdatedAt)
				if originalUpdateTime != nil {
					assert.True(t, env.Metadata.UpdatedAt.After(*originalUpdateTime))
				}
				
				// Verify the update by fetching the environment
				updatedEnv, err := store.Get(ctx, env.Metadata.Name, tt.customerID)
				assert.NoError(t, err)
				testutil.AssertEnvironmentEqual(t, env, *updatedEnv)
			}
		})
	}
}

func TestEnvironmentStore_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test environments
	env1 := testutil.CreateTestEnvironment("delete-test-env-1")
	env2 := testutil.CreateTestEnvironment("delete-test-env-2")
	
	err := store.Create(ctx, &env1, customerID)
	require.NoError(t, err)
	err = store.Create(ctx, &env2, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		envName     string
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name:        "delete existing environment",
			envName:     "delete-test-env-1",
			customerID:  customerID,
			expectError: false,
		},
		{
			name:        "delete non-existent environment",
			envName:     "non-existent-env",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete environment with wrong customer ID",
			envName:     "delete-test-env-2",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete already deleted environment",
			envName:     "delete-test-env-1",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(ctx, tt.envName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify the environment is deleted
				_, err := store.Get(ctx, tt.envName, tt.customerID)
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	}
}

func TestEnvironmentStore_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID1 := "test-customer-1"
	customerID2 := "test-customer-2"

	// Create test environments for customer 1
	customer1Envs := []models.Environment{
		testutil.CreateTestEnvironment("customer1-env-1"),
		testutil.CreateTestEnvironment("customer1-env-2"),
		testutil.CreateTestEnvironmentWithTokenAuth("customer1-env-3"),
	}
	
	for _, env := range customer1Envs {
		err := store.Create(ctx, &env, customerID1)
		require.NoError(t, err)
	}

	// Create test environments for customer 2
	customer2Envs := []models.Environment{
		testutil.CreateTestEnvironment("customer2-env-1"),
	}
	
	for _, env := range customer2Envs {
		err := store.Create(ctx, &env, customerID2)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		customerID    string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "list environments for customer 1",
			customerID:    customerID1,
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "list environments for customer 2",
			customerID:    customerID2,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "list environments for non-existent customer",
			customerID:    "non-existent-customer",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envs, err := store.List(ctx, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, envs, tt.expectedCount)
				
				// Verify all environments belong to the correct customer
				for _, env := range envs {
					assert.NotNil(t, env)
					assert.NotEmpty(t, env.Metadata.Name)
					assert.Equal(t, "contextops/v1", env.APIVersion)
					assert.Equal(t, "Environment", env.Kind)
				}
				
				// Verify environments are sorted by name
				if len(envs) > 1 {
					for i := 1; i < len(envs); i++ {
						assert.True(t, envs[i-1].Metadata.Name < envs[i].Metadata.Name,
							"Environments should be sorted by name: %s should come before %s",
							envs[i-1].Metadata.Name, envs[i].Metadata.Name)
					}
				}
			}
		})
	}
}

func TestEnvironmentStore_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-concurrent"

	// Test concurrent creations
	t.Run("concurrent_creates", func(t *testing.T) {
		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				env := testutil.CreateTestEnvironment(fmt.Sprintf("concurrent-env-%d", id))
				err := store.Create(ctx, &env, customerID)
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

		// Verify all environments were created
		envs, err := store.List(ctx, customerID)
		assert.NoError(t, err)
		assert.Len(t, envs, numGoroutines)
	})

	// Clean up for next test
	testDB.Cleanup(t)

	// Test concurrent reads of the same environment
	t.Run("concurrent_reads", func(t *testing.T) {
		// Create a test environment first
		env := testutil.CreateTestEnvironment("concurrent-read-env")
		err := store.Create(ctx, &env, customerID)
		require.NoError(t, err)

		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := store.Get(ctx, env.Metadata.Name, customerID)
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

func TestEnvironmentStore_ComplexVaultConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-vault"

	// Create environment with complex Vault configuration
	env := testutil.CreateTestEnvironment("complex-vault-env")
	
	// Add multiple datasources
	env.Spec.Datasources = map[string]models.VaultDatasource{
		"postgresql": {
			Vault: "secret/database/production/postgresql",
			Keys:  []string{"username", "password", "host", "port", "database"},
		},
		"redis": {
			Vault: "secret/cache/production/redis",
			Keys:  []string{"host", "port", "password", "ssl"},
		},
		"s3": {
			Vault: "secret/storage/production/s3",
			Keys:  []string{"access-key", "secret-key", "bucket", "region"},
		},
		"external-api": {
			Vault: "secret/external/production/api-keys",
			Keys:  []string{"stripe-key", "sendgrid-key", "auth0-secret"},
		},
	}

	// Add multiple vault secrets
	env.Spec.VaultSecrets = []models.VaultStaticSecret{
		{
			Name:              "app-secrets",
			VaultPath:         "secret/app/production",
			DestinationSecret: "app-secrets",
			RequiredKeys:      []string{"jwt-secret", "api-key", "webhook-secret"},
		},
		{
			Name:              "database-secrets",
			VaultPath:         "secret/database/production/credentials",
			DestinationSecret: "database-secrets",
			RequiredKeys:      []string{"master-password", "read-replica-password"},
		},
		{
			Name:              "monitoring-secrets",
			VaultPath:         "secret/monitoring/production",
			DestinationSecret: "monitoring-secrets",
			RequiredKeys:      []string{"prometheus-token", "grafana-admin-password"},
		},
	}

	// Set complex pod environment validation
	env.Spec.PodEnvValidation = models.PodEnvValidationConfig{
		Enabled: true,
		ExpectedEnvVars: []models.ExpectedEnvVar{
			{
				Name:      "DATABASE_URL",
				SecretRef: "database-secrets",
				Key:       "database-url",
			},
			{
				Name:      "REDIS_URL",
				SecretRef: "app-secrets",
				Key:       "redis-url",
			},
			{
				Name:      "JWT_SECRET",
				SecretRef: "app-secrets",
				Key:       "jwt-secret",
			},
			{
				Name:      "MONITORING_TOKEN",
				SecretRef: "monitoring-secrets",
				Key:       "prometheus-token",
			},
		},
	}

	// Test create
	err := store.Create(ctx, &env, customerID)
	assert.NoError(t, err)

	// Test get and verify all configuration is preserved
	retrievedEnv, err := store.Get(ctx, env.Metadata.Name, customerID)
	assert.NoError(t, err)

	// Verify datasources
	assert.Len(t, retrievedEnv.Spec.Datasources, 4)
	assert.Contains(t, retrievedEnv.Spec.Datasources, "postgresql")
	assert.Contains(t, retrievedEnv.Spec.Datasources, "redis")
	assert.Contains(t, retrievedEnv.Spec.Datasources, "s3")
	assert.Contains(t, retrievedEnv.Spec.Datasources, "external-api")

	// Verify vault secrets
	assert.Len(t, retrievedEnv.Spec.VaultSecrets, 3)
	secretNames := make([]string, len(retrievedEnv.Spec.VaultSecrets))
	for i, secret := range retrievedEnv.Spec.VaultSecrets {
		secretNames[i] = secret.Name
	}
	assert.Contains(t, secretNames, "app-secrets")
	assert.Contains(t, secretNames, "database-secrets")
	assert.Contains(t, secretNames, "monitoring-secrets")

	// Verify pod environment validation
	assert.True(t, retrievedEnv.Spec.PodEnvValidation.Enabled)
	assert.Len(t, retrievedEnv.Spec.PodEnvValidation.ExpectedEnvVars, 4)

	// Test update with modified configuration
	env.Spec.Datasources = map[string]models.VaultDatasource{
		"postgresql": {
			Vault: "secret/database/updated/postgresql",
			Keys:  []string{"username", "password", "host", "port"},
		},
	}
	env.Spec.VaultSecrets = []models.VaultStaticSecret{
		{
			Name:              "updated-secrets",
			VaultPath:         "secret/app/updated",
			DestinationSecret: "updated-secrets",
			RequiredKeys:      []string{"updated-key"},
		},
	}

	err = store.Update(ctx, &env, customerID)
	assert.NoError(t, err)

	// Verify update removed old configuration and added new
	updatedEnv, err := store.Get(ctx, env.Metadata.Name, customerID)
	assert.NoError(t, err)
	assert.Len(t, updatedEnv.Spec.Datasources, 1)
	assert.Contains(t, updatedEnv.Spec.Datasources, "postgresql")
	assert.Equal(t, "secret/database/updated/postgresql", updatedEnv.Spec.Datasources["postgresql"].Vault)
	assert.Len(t, updatedEnv.Spec.VaultSecrets, 1)
	assert.Equal(t, "updated-secrets", updatedEnv.Spec.VaultSecrets[0].Name)
}

func TestEnvironmentStore_DifferentAuthMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-auth"

	tests := []struct {
		name       string
		authConfig models.VaultAuthConfig
	}{
		{
			name: "kubernetes auth",
			authConfig: models.VaultAuthConfig{
				Method: "kubernetes",
				Kubernetes: &models.VaultKubernetesAuth{
					Role: "production-role",
				},
			},
		},
		{
			name: "token auth",
			authConfig: models.VaultAuthConfig{
				Method: "token",
				Token:  "s.EXAMPLE_VAULT_TOKEN_HERE",
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testutil.CreateTestEnvironment(fmt.Sprintf("auth-test-env-%d", i))
			env.Spec.Vault.Auth = tt.authConfig

			// Test create
			err := store.Create(ctx, &env, customerID)
			assert.NoError(t, err)

			// Test get and verify auth configuration is preserved
			retrievedEnv, err := store.Get(ctx, env.Metadata.Name, customerID)
			assert.NoError(t, err)

			assert.Equal(t, tt.authConfig.Method, retrievedEnv.Spec.Vault.Auth.Method)
			assert.Equal(t, tt.authConfig.Token, retrievedEnv.Spec.Vault.Auth.Token)
			
			if tt.authConfig.Kubernetes != nil {
				require.NotNil(t, retrievedEnv.Spec.Vault.Auth.Kubernetes)
				assert.Equal(t, tt.authConfig.Kubernetes.Role, retrievedEnv.Spec.Vault.Auth.Kubernetes.Role)
			} else {
				assert.Nil(t, retrievedEnv.Spec.Vault.Auth.Kubernetes)
			}
		})
	}
}

func TestEnvironmentStore_ClusterConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewEnvironmentStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-cluster"

	// Create environment with specific cluster configuration
	env := testutil.CreateTestEnvironment("cluster-test-env")
	env.Spec.Environment.Name = "production-cluster"
	env.Spec.Environment.Namespace = "prod-applications"
	env.Spec.Environment.Cluster.KubeconfigSecretRef = models.VaultSecretRef{
		Vault: "secret/kubernetes/production-cluster",
		Key:   "kubeconfig-admin",
	}

	// Test create
	err := store.Create(ctx, &env, customerID)
	assert.NoError(t, err)

	// Test get and verify cluster configuration is preserved
	retrievedEnv, err := store.Get(ctx, env.Metadata.Name, customerID)
	assert.NoError(t, err)

	assert.Equal(t, "production-cluster", retrievedEnv.Spec.Environment.Name)
	assert.Equal(t, "prod-applications", retrievedEnv.Spec.Environment.Namespace)
	assert.Equal(t, "secret/kubernetes/production-cluster", retrievedEnv.Spec.Environment.Cluster.KubeconfigSecretRef.Vault)
	assert.Equal(t, "kubeconfig-admin", retrievedEnv.Spec.Environment.Cluster.KubeconfigSecretRef.Key)

	// Test update with modified cluster configuration
	env.Spec.Environment.Name = "updated-cluster"
	env.Spec.Environment.Namespace = "updated-namespace"
	env.Spec.Environment.Cluster.KubeconfigSecretRef.Vault = "secret/kubernetes/updated-cluster"
	env.Spec.Environment.Cluster.KubeconfigSecretRef.Key = "kubeconfig-updated"

	err = store.Update(ctx, &env, customerID)
	assert.NoError(t, err)

	// Verify cluster configuration was updated
	updatedEnv, err := store.Get(ctx, env.Metadata.Name, customerID)
	assert.NoError(t, err)

	assert.Equal(t, "updated-cluster", updatedEnv.Spec.Environment.Name)
	assert.Equal(t, "updated-namespace", updatedEnv.Spec.Environment.Namespace)
	assert.Equal(t, "secret/kubernetes/updated-cluster", updatedEnv.Spec.Environment.Cluster.KubeconfigSecretRef.Vault)
	assert.Equal(t, "kubeconfig-updated", updatedEnv.Spec.Environment.Cluster.KubeconfigSecretRef.Key)
}