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

func TestAppStore_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	tests := []struct {
		name        string
		app         models.App
		expectError bool
		errorType   error
	}{
		{
			name:        "valid app creation",
			app:         testutil.CreateTestApp("test-app-1"),
			expectError: false,
		},
		{
			name:        "app with git source",
			app:         testutil.CreateTestAppWithGitSource("test-app-git"),
			expectError: false,
		},
		{
			name:        "app with oci source",
			app:         testutil.CreateTestAppWithOCISource("test-app-oci"),
			expectError: false,
		},
		{
			name:        "app with multiple sources",
			app:         testutil.CreateTestAppWithMultipleSources("test-app-multi"),
			expectError: false,
		},
		{
			name:        "duplicate app creation should fail",
			app:         testutil.CreateTestApp("test-app-1"), // Same name as first test
			expectError: true,
			errorType:   storage.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Create(ctx, &tt.app, customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify timestamps are set
				assert.NotNil(t, tt.app.Metadata.CreatedAt)
				assert.NotNil(t, tt.app.Metadata.UpdatedAt)
			}
		})
	}
}

func TestAppStore_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test app
	originalApp := testutil.CreateTestApp("get-test-app")
	err := store.Create(ctx, &originalApp, customerID)
	require.NoError(t, err)

	tests := []struct {
		name         string
		appName      string
		customerID   string
		expectError  bool
		expectedApp  *models.App
		errorType    error
	}{
		{
			name:        "get existing app",
			appName:     "get-test-app",
			customerID:  customerID,
			expectError: false,
			expectedApp: &originalApp,
		},
		{
			name:        "get non-existent app",
			appName:     "non-existent-app",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "get app with wrong customer ID",
			appName:     "get-test-app",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := store.Get(ctx, tt.appName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, app)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, app)
				
				// Compare apps (ignoring timestamps)
				testutil.AssertAppEqual(t, *tt.expectedApp, *app)
			}
		})
	}
}

func TestAppStore_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test app
	originalApp := testutil.CreateTestApp("update-test-app")
	err := store.Create(ctx, &originalApp, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setupFunc   func() models.App
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name: "update existing app",
			setupFunc: func() models.App {
				app := originalApp
				app.Spec.Application.Version = "2.0.0"
				app.Spec.Application.Maintainer = "updated@example.com"
				return app
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update app helm sources",
			setupFunc: func() models.App {
				app := originalApp
				app.Spec.Helm.Sources = append(app.Spec.Helm.Sources, models.HelmSource{
					Type:       "git",
					Repository: "https://github.com/example/new-charts.git",
					Chart:      "new-chart",
				})
				return app
			},
			customerID:  customerID,
			expectError: false,
		},
		{
			name: "update non-existent app",
			setupFunc: func() models.App {
				app := testutil.CreateTestApp("non-existent-app")
				return app
			},
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name: "update app with wrong customer ID",
			setupFunc: func() models.App {
				return originalApp
			},
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.setupFunc()
			originalUpdateTime := app.Metadata.UpdatedAt
			
			err := store.Update(ctx, &app, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify UpdatedAt timestamp is updated
				assert.NotNil(t, app.Metadata.UpdatedAt)
				if originalUpdateTime != nil {
					assert.True(t, app.Metadata.UpdatedAt.After(*originalUpdateTime))
				}
				
				// Verify the update by fetching the app
				updatedApp, err := store.Get(ctx, app.Metadata.Name, tt.customerID)
				assert.NoError(t, err)
				testutil.AssertAppEqual(t, app, *updatedApp)
			}
		})
	}
}

func TestAppStore_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-123"

	// Create test apps
	app1 := testutil.CreateTestApp("delete-test-app-1")
	app2 := testutil.CreateTestApp("delete-test-app-2")
	
	err := store.Create(ctx, &app1, customerID)
	require.NoError(t, err)
	err = store.Create(ctx, &app2, customerID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		appName     string
		customerID  string
		expectError bool
		errorType   error
	}{
		{
			name:        "delete existing app",
			appName:     "delete-test-app-1",
			customerID:  customerID,
			expectError: false,
		},
		{
			name:        "delete non-existent app",
			appName:     "non-existent-app",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete app with wrong customer ID",
			appName:     "delete-test-app-2",
			customerID:  "wrong-customer",
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
		{
			name:        "delete already deleted app",
			appName:     "delete-test-app-1",
			customerID:  customerID,
			expectError: true,
			errorType:   storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(ctx, tt.appName, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify the app is deleted
				_, err := store.Get(ctx, tt.appName, tt.customerID)
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	}
}

func TestAppStore_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID1 := "test-customer-1"
	customerID2 := "test-customer-2"

	// Create test apps for customer 1
	customer1Apps := []models.App{
		testutil.CreateTestApp("customer1-app-1"),
		testutil.CreateTestApp("customer1-app-2"),
		testutil.CreateTestAppWithGitSource("customer1-app-3"),
	}
	
	for _, app := range customer1Apps {
		err := store.Create(ctx, &app, customerID1)
		require.NoError(t, err)
	}

	// Create test apps for customer 2
	customer2Apps := []models.App{
		testutil.CreateTestApp("customer2-app-1"),
	}
	
	for _, app := range customer2Apps {
		err := store.Create(ctx, &app, customerID2)
		require.NoError(t, err)
	}

	tests := []struct {
		name         string
		customerID   string
		expectedCount int
		expectError  bool
	}{
		{
			name:         "list apps for customer 1",
			customerID:   customerID1,
			expectedCount: 3,
			expectError:  false,
		},
		{
			name:         "list apps for customer 2",
			customerID:   customerID2,
			expectedCount: 1,
			expectError:  false,
		},
		{
			name:         "list apps for non-existent customer",
			customerID:   "non-existent-customer",
			expectedCount: 0,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps, err := store.List(ctx, tt.customerID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, apps, tt.expectedCount)
				
				// Verify all apps belong to the correct customer
				for _, app := range apps {
					assert.NotNil(t, app)
					assert.NotEmpty(t, app.Metadata.Name)
					assert.Equal(t, "contextops/v1", app.APIVersion)
					assert.Equal(t, "App", app.Kind)
				}
				
				// Verify apps are sorted by name
				if len(apps) > 1 {
					for i := 1; i < len(apps); i++ {
						assert.True(t, apps[i-1].Metadata.Name < apps[i].Metadata.Name,
							"Apps should be sorted by name: %s should come before %s",
							apps[i-1].Metadata.Name, apps[i].Metadata.Name)
					}
				}
			}
		})
	}
}

func TestAppStore_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-concurrent"

	// Test concurrent creations
	t.Run("concurrent_creates", func(t *testing.T) {
		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				app := testutil.CreateTestApp(fmt.Sprintf("concurrent-app-%d", id))
				err := store.Create(ctx, &app, customerID)
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

		// Verify all apps were created
		apps, err := store.List(ctx, customerID)
		assert.NoError(t, err)
		assert.Len(t, apps, numGoroutines)
	})

	// Clean up for next test
	testDB.Cleanup(t)

	// Test concurrent reads of the same app
	t.Run("concurrent_reads", func(t *testing.T) {
		// Create a test app first
		app := testutil.CreateTestApp("concurrent-read-app")
		err := store.Create(ctx, &app, customerID)
		require.NoError(t, err)

		numGoroutines := 10
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := store.Get(ctx, app.Metadata.Name, customerID)
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

func TestAppStore_ComplexHelmSources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-helm"

	// Create app with complex Helm configuration
	app := testutil.CreateTestApp("complex-helm-app")
	app.Spec.Helm.Sources = []models.HelmSource{
		{
			Type:       "helm-registry",
			Registry:   "registry.example.com",
			Chart:      "primary-chart",
			Version:    "1.0.0",
		},
		{
			Type:       "git",
			Repository: "https://github.com/example/charts.git",
			Chart:      "secondary-chart",
			Path:       "charts/secondary",
			Ref:        "v1.2.3",
		},
		{
			Type:     "oci",
			Registry: "oci://ghcr.io/example",
			Chart:    "tertiary-chart",
			Version:  "2.0.0",
		},
	}
	app.Spec.Helm.DefaultSource = 2 // Use OCI source as default

	// Test create
	err := store.Create(ctx, &app, customerID)
	assert.NoError(t, err)

	// Test get and verify all Helm sources are preserved
	retrievedApp, err := store.Get(ctx, app.Metadata.Name, customerID)
	assert.NoError(t, err)
	assert.Len(t, retrievedApp.Spec.Helm.Sources, 3)
	assert.Equal(t, 2, retrievedApp.Spec.Helm.DefaultSource)

	// Verify each source type is correctly preserved
	sources := retrievedApp.Spec.Helm.Sources
	assert.Equal(t, "helm-registry", sources[0].Type)
	assert.Equal(t, "registry.example.com", sources[0].Registry)
	assert.Equal(t, "git", sources[1].Type)
	assert.Equal(t, "https://github.com/example/charts.git", sources[1].Repository)
	assert.Equal(t, "oci", sources[2].Type)
	assert.Equal(t, "oci://ghcr.io/example", sources[2].Registry)

	// Test update with modified Helm sources
	app.Spec.Helm.Sources = []models.HelmSource{
		{
			Type:     "helm-registry",
			Registry: "updated-registry.example.com",
			Chart:    "updated-chart",
			Version:  "2.0.0",
		},
	}
	app.Spec.Helm.DefaultSource = 0

	err = store.Update(ctx, &app, customerID)
	assert.NoError(t, err)

	// Verify update removed old sources and added new ones
	updatedApp, err := store.Get(ctx, app.Metadata.Name, customerID)
	assert.NoError(t, err)
	assert.Len(t, updatedApp.Spec.Helm.Sources, 1)
	assert.Equal(t, 0, updatedApp.Spec.Helm.DefaultSource)
	assert.Equal(t, "updated-registry.example.com", updatedApp.Spec.Helm.Sources[0].Registry)
}

func TestAppStore_ComplexApplicationSets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	store := storage.NewAppStore(testDB.DB)
	ctx := context.Background()
	customerID := "test-customer-appsets"

	// Create app with complex ApplicationSet configuration
	app := testutil.CreateTestApp("complex-appset-app")
	app.Spec.ArgoCD.ApplicationSets = []models.ApplicationSetConfig{
		{
			Name:      "git-generator-appset",
			Namespace: "argocd",
			Generator: models.ApplicationSetGenerator{
				Type: "git",
				Git: &models.GitGenerator{
					RepoURL:  "https://github.com/example/configs.git",
					Revision: "main",
					Directories: []models.GitGeneratorDirectory{
						{Path: "environments/*/", Exclude: "environments/template"},
					},
					Files: []models.GitGeneratorFile{
						{Path: "environments/*/config.json"},
					},
				},
			},
			Template: models.ApplicationSetTemplate{
				Metadata: models.ApplicationSetTemplateMetadata{
					Name: "{{path.basename}}-app",
					Labels: map[string]string{
						"environment": "{{path.basename}}",
					},
				},
				Spec: models.ApplicationSetTemplateSpec{
					Source: models.ApplicationSetTemplateSource{
						Helm: &models.ApplicationSetTemplateHelm{
							ValueFiles: []string{"values-{{path.basename}}.yaml"},
						},
					},
				},
			},
		},
		{
			Name:      "list-generator-appset",
			Namespace: "argocd",
			Generator: models.ApplicationSetGenerator{
				Type: "list",
				List: &models.ListGenerator{
					Elements: []map[string]interface{}{
						{"cluster": "dev", "url": "https://kubernetes.dev.example.com"},
						{"cluster": "prod", "url": "https://kubernetes.prod.example.com"},
					},
				},
			},
			Template: models.ApplicationSetTemplate{
				Metadata: models.ApplicationSetTemplateMetadata{
					Name: "{{cluster}}-deployment",
				},
				Spec: models.ApplicationSetTemplateSpec{
					Source: models.ApplicationSetTemplateSource{},
				},
			},
		},
	}

	// Test create
	err := store.Create(ctx, &app, customerID)
	assert.NoError(t, err)

	// Test get and verify all ApplicationSets are preserved
	retrievedApp, err := store.Get(ctx, app.Metadata.Name, customerID)
	assert.NoError(t, err)
	assert.Len(t, retrievedApp.Spec.ArgoCD.ApplicationSets, 2)

	// Verify git generator ApplicationSet
	gitAppSet := retrievedApp.Spec.ArgoCD.ApplicationSets[0]
	assert.Equal(t, "git-generator-appset", gitAppSet.Name)
	assert.Equal(t, "git", gitAppSet.Generator.Type)
	require.NotNil(t, gitAppSet.Generator.Git)
	assert.Equal(t, "https://github.com/example/configs.git", gitAppSet.Generator.Git.RepoURL)
	assert.Len(t, gitAppSet.Generator.Git.Directories, 1)
	assert.Len(t, gitAppSet.Generator.Git.Files, 1)

	// Verify list generator ApplicationSet
	listAppSet := retrievedApp.Spec.ArgoCD.ApplicationSets[1]
	assert.Equal(t, "list-generator-appset", listAppSet.Name)
	assert.Equal(t, "list", listAppSet.Generator.Type)
	require.NotNil(t, listAppSet.Generator.List)
	assert.Len(t, listAppSet.Generator.List.Elements, 2)
}