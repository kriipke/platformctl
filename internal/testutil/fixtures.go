package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/storage"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestDB represents a test database instance
type TestDB struct {
	*storage.DB
	databaseName string
}

// NewTestDB creates a new test database for integration tests
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Get database URL from environment or use default
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/platformctl_test?sslmode=disable"
	}

	// Create unique database name for this test
	testDBName := fmt.Sprintf("test_%d_%s", time.Now().Unix(), t.Name())
	testDBName = sanitizeDBName(testDBName)

	// Connect to postgres database to create test database
	adminDB, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer adminDB.Close()

	// Create test database
	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(t, err)

	// Connect to test database
	testDBURL := fmt.Sprintf("%s_%s", dbURL[:len(dbURL)-len("?sslmode=disable")], testDBName) + "?sslmode=disable"
	db, err := storage.NewDB(testDBURL)
	require.NoError(t, err)

	// Run migrations
	err = db.RunMigrations("../../migrations")
	require.NoError(t, err)

	return &TestDB{
		DB:           db,
		databaseName: testDBName,
	}
}

// Close closes the test database and cleans up
func (tdb *TestDB) Close(t *testing.T) {
	t.Helper()

	// Close the database connection
	err := tdb.DB.Close()
	require.NoError(t, err)

	// Connect to postgres database to drop test database
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/platformctl_test?sslmode=disable"
	}

	adminDB, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	defer adminDB.Close()

	// Drop test database
	_, err = adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", tdb.databaseName))
	require.NoError(t, err)
}

// Cleanup removes all data from test database tables
func (tdb *TestDB) Cleanup(t *testing.T) {
	t.Helper()

	tables := []string{"applicationsets", "helm_sources", "contexts", "environments", "apps"}
	for _, table := range tables {
		_, err := tdb.ExecContext(context.Background(), fmt.Sprintf("DELETE FROM %s", table))
		require.NoError(t, err)
	}
}

// sanitizeDBName removes invalid characters from database name
func sanitizeDBName(name string) string {
	// Replace invalid characters with underscores
	sanitized := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sanitized += string(r)
		} else {
			sanitized += "_"
		}
	}
	return sanitized
}

// Test Fixtures

// CreateTestApp creates a valid test App manifest
func CreateTestApp(name string) models.App {
	now := time.Now()
	return models.App{
		APIVersion: "platformctl/v1",
		Kind:       "App",
		Metadata: models.AppMetadata{
			Name: name,
			Labels: map[string]string{
				"env":  "test",
				"team": "platform",
			},
			Annotations: map[string]string{
				"description": "Test application",
			},
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		Spec: models.AppSpec{
			Application: models.AppApplicationConfig{
				Name:       name,
				Version:    "1.0.0",
				Maintainer: "test@example.com",
			},
			Helm: models.AppHelmConfig{
				Sources: []models.HelmSource{
					{
						Type:     "helm-registry",
						Registry: "registry.example.com",
						Chart:    name + "-chart",
						Version:  "1.0.0",
					},
				},
				DefaultSource: 0,
			},
			ArgoCD: models.AppArgoCDConfig{
				ApplicationSets: []models.ApplicationSetConfig{
					{
						Name:      name + "-appset",
						Namespace: "argocd",
						Generator: models.ApplicationSetGenerator{
							Type: "git",
							Git: &models.GitGenerator{
								RepoURL:  "https://github.com/example/" + name + ".git",
								Revision: "main",
							},
						},
						Template: models.ApplicationSetTemplate{
							Metadata: models.ApplicationSetTemplateMetadata{
								Name: "{{name}}",
							},
							Spec: models.ApplicationSetTemplateSpec{
								Source: models.ApplicationSetTemplateSource{
									Helm: &models.ApplicationSetTemplateHelm{
										ValueFiles: []string{"values.yaml"},
									},
								},
							},
						},
					},
				},
			},
			Environments: []models.AppEnvironmentRef{
				{
					Name:           "dev",
					EnvironmentRef: "dev-" + name,
				},
				{
					Name:           "prod",
					EnvironmentRef: "prod-" + name,
				},
			},
		},
	}
}

// CreateTestEnvironment creates a valid test Environment manifest
func CreateTestEnvironment(name string) models.Environment {
	now := time.Now()
	return models.Environment{
		APIVersion: "platformctl/v1",
		Kind:       "Environment",
		Metadata: models.EnvironmentMetadata{
			Name: name,
			Labels: map[string]string{
				"env":  "test",
				"type": "development",
			},
			Annotations: map[string]string{
				"description": "Test environment",
			},
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		Spec: models.EnvironmentSpec{
			Environment: models.EnvironmentConfig{
				Name: name,
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/" + name,
						Key:   "kubeconfig",
					},
				},
				Namespace: name + "-apps",
			},
			Helm: models.EnvironmentHelmConfig{
				ValuesSource: models.HelmValuesSource{
					Type:       "git",
					Repository: "https://github.com/example/helm-values.git",
					Path:       "environments/" + name,
					Branch:     "main",
				},
			},
			Vault: models.EnvironmentVaultConfig{
				Address:   "https://vault.example.com",
				Namespace: name,
				Auth: models.VaultAuthConfig{
					Method: "kubernetes",
					Kubernetes: &models.VaultKubernetesAuth{
						Role: name + "-role",
					},
				},
			},
			Datasources: map[string]models.VaultDatasource{
				"database": {
					Vault: "secret/database/" + name,
					Keys:  []string{"username", "password", "host", "port"},
				},
				"redis": {
					Vault: "secret/redis/" + name,
					Keys:  []string{"host", "port", "password"},
				},
			},
			VaultSecrets: []models.VaultStaticSecret{
				{
					Name:              "app-secrets",
					VaultPath:         "secret/app/" + name,
					DestinationSecret: "app-secrets",
					RequiredKeys:      []string{"api-key", "jwt-secret"},
				},
			},
			PodEnvValidation: models.PodEnvValidationConfig{
				Enabled: true,
				ExpectedEnvVars: []models.ExpectedEnvVar{
					{
						Name:      "DATABASE_URL",
						SecretRef: "app-secrets",
						Key:       "database-url",
					},
					{
						Name:      "API_KEY",
						SecretRef: "app-secrets",
						Key:       "api-key",
					},
				},
			},
		},
	}
}

// CreateTestContext creates a valid test Context manifest
func CreateTestContext(name, appRef string, deployments []models.ContextDeployment) models.Context {
	now := time.Now()
	return models.Context{
		APIVersion: "platformctl/v1",
		Kind:       "Context",
		Metadata: models.ContextMetadata{
			Name: name,
			Labels: map[string]string{
				"app":  appRef,
				"team": "platform",
			},
			Annotations: map[string]string{
				"description": "Test context pairing",
			},
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		Spec: models.ContextSpec{
			AppRef:      appRef,
			Deployments: deployments,
			GitOps: models.ContextGitOpsConfig{
				CustomerBranch: models.CustomerBranchConfig{
					Enabled: false,
				},
				Monitoring: models.MonitoringConfig{
					ApplicationSets:       true,
					VaultSecrets:          true,
					HelmValues:            true,
					CrossEnvironmentDrift: true,
				},
			},
		},
	}
}

// CreateTestContextDeployments creates test deployments for a context
func CreateTestContextDeployments(appRef string, environmentRefs ...string) []models.ContextDeployment {
	deployments := make([]models.ContextDeployment, len(environmentRefs))
	environments := []string{"dev", "staging", "prod"}

	for i, envRef := range environmentRefs {
		env := "dev"
		if i < len(environments) {
			env = environments[i]
		}

		deployments[i] = models.ContextDeployment{
			Environment:    env,
			AppRef:         appRef,
			EnvironmentRef: envRef,
			Active:         i == 0, // First deployment is active
		}
	}

	return deployments
}

// CreateTestCustomerBranchContext creates a test context with customer branch enabled
func CreateTestCustomerBranchContext(name, appRef, customerBranch string) models.Context {
	context := CreateTestContext(name, appRef, CreateTestContextDeployments(appRef, "dev-env", "prod-env"))
	context.Spec.GitOps.CustomerBranch = models.CustomerBranchConfig{
		Enabled: true,
		Branch:  customerBranch,
	}
	return context
}

// App variations for testing

// CreateTestAppWithGitSource creates an app with git Helm source
func CreateTestAppWithGitSource(name string) models.App {
	app := CreateTestApp(name)
	app.Spec.Helm.Sources = []models.HelmSource{
		{
			Type:       "git",
			Repository: "https://github.com/example/charts.git",
			Chart:      name + "-chart",
			Path:       "charts/" + name,
			Ref:        "main",
		},
	}
	return app
}

// CreateTestAppWithOCISource creates an app with OCI Helm source
func CreateTestAppWithOCISource(name string) models.App {
	app := CreateTestApp(name)
	app.Spec.Helm.Sources = []models.HelmSource{
		{
			Type:     "oci",
			Registry: "oci://registry.example.com",
			Chart:    name + "-chart",
			Version:  "1.0.0",
		},
	}
	return app
}

// CreateTestAppWithMultipleSources creates an app with multiple Helm sources
func CreateTestAppWithMultipleSources(name string) models.App {
	app := CreateTestApp(name)
	app.Spec.Helm.Sources = []models.HelmSource{
		{
			Type:     "helm-registry",
			Registry: "registry.example.com",
			Chart:    name + "-chart",
			Version:  "1.0.0",
		},
		{
			Type:       "git",
			Repository: "https://github.com/example/charts.git",
			Chart:      name + "-chart-git",
			Path:       "charts/" + name,
			Ref:        "main",
		},
		{
			Type:     "oci",
			Registry: "oci://registry.example.com",
			Chart:    name + "-chart-oci",
			Version:  "1.0.0",
		},
	}
	app.Spec.Helm.DefaultSource = 1 // Use git source as default
	return app
}

// Environment variations for testing

// CreateTestEnvironmentWithTokenAuth creates an environment with Vault token auth
func CreateTestEnvironmentWithTokenAuth(name string) models.Environment {
	env := CreateTestEnvironment(name)
	env.Spec.Vault.Auth = models.VaultAuthConfig{
		Method: "token",
		Token:  "s.AAAAAAAAAAAAAAAAAAAAAA",
	}
	return env
}

// CreateTestEnvironmentMinimal creates a minimal valid environment
func CreateTestEnvironmentMinimal(name string) models.Environment {
	now := time.Now()
	return models.Environment{
		APIVersion: "platformctl/v1",
		Kind:       "Environment",
		Metadata: models.EnvironmentMetadata{
			Name:      name,
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		Spec: models.EnvironmentSpec{
			Environment: models.EnvironmentConfig{
				Name: name,
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/" + name,
						Key:   "kubeconfig",
					},
				},
				Namespace: name + "-apps",
			},
			Helm: models.EnvironmentHelmConfig{
				ValuesSource: models.HelmValuesSource{
					Type:       "git",
					Repository: "https://github.com/example/helm-values.git",
					Path:       "environments/" + name,
					Branch:     "main",
				},
			},
			Vault: models.EnvironmentVaultConfig{
				Address: "https://vault.example.com",
				Auth: models.VaultAuthConfig{
					Method: "kubernetes",
					Kubernetes: &models.VaultKubernetesAuth{
						Role: name + "-role",
					},
				},
			},
			Datasources: map[string]models.VaultDatasource{
				"database": {
					Vault: "secret/database/" + name,
					Keys:  []string{"username", "password"},
				},
			},
			VaultSecrets: []models.VaultStaticSecret{
				{
					Name:              "minimal-secrets",
					VaultPath:         "secret/app/" + name,
					DestinationSecret: "minimal-secrets",
					RequiredKeys:      []string{"api-key"},
				},
			},
			PodEnvValidation: models.PodEnvValidationConfig{
				Enabled: false,
			},
		},
	}
}

// Test helper functions

// AssertAppEqual compares two App manifests for equality (ignoring timestamps)
func AssertAppEqual(t *testing.T, expected, actual models.App) {
	t.Helper()

	// Compare basic fields
	require.Equal(t, expected.APIVersion, actual.APIVersion)
	require.Equal(t, expected.Kind, actual.Kind)
	require.Equal(t, expected.Metadata.Name, actual.Metadata.Name)
	require.Equal(t, expected.Metadata.Labels, actual.Metadata.Labels)
	require.Equal(t, expected.Metadata.Annotations, actual.Metadata.Annotations)

	// Compare spec
	require.Equal(t, expected.Spec.Application, actual.Spec.Application)
	require.Equal(t, expected.Spec.Helm, actual.Spec.Helm)
	require.Equal(t, expected.Spec.ArgoCD, actual.Spec.ArgoCD)
	require.Equal(t, expected.Spec.Environments, actual.Spec.Environments)
}

// AssertEnvironmentEqual compares two Environment manifests for equality (ignoring timestamps)
func AssertEnvironmentEqual(t *testing.T, expected, actual models.Environment) {
	t.Helper()

	// Compare basic fields
	require.Equal(t, expected.APIVersion, actual.APIVersion)
	require.Equal(t, expected.Kind, actual.Kind)
	require.Equal(t, expected.Metadata.Name, actual.Metadata.Name)
	require.Equal(t, expected.Metadata.Labels, actual.Metadata.Labels)
	require.Equal(t, expected.Metadata.Annotations, actual.Metadata.Annotations)

	// Compare spec
	require.Equal(t, expected.Spec, actual.Spec)
}

// AssertContextEqual compares two Context manifests for equality (ignoring timestamps)
func AssertContextEqual(t *testing.T, expected, actual models.Context) {
	t.Helper()

	// Compare basic fields
	require.Equal(t, expected.APIVersion, actual.APIVersion)
	require.Equal(t, expected.Kind, actual.Kind)
	require.Equal(t, expected.Metadata.Name, actual.Metadata.Name)
	require.Equal(t, expected.Metadata.Labels, actual.Metadata.Labels)
	require.Equal(t, expected.Metadata.Annotations, actual.Metadata.Annotations)

	// Compare spec
	require.Equal(t, expected.Spec, actual.Spec)
}
