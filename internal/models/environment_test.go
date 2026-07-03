package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentMarshaling(t *testing.T) {
	tests := []struct {
		name        string
		environment Environment
		expectedErr bool
	}{
		{
			name: "valid environment manifest",
			environment: Environment{
				APIVersion: "platformctl/v1",
				Kind:       "Environment",
				Metadata: EnvironmentMetadata{
					Name: "dev-environment",
					Labels: map[string]string{
						"env":  "development",
						"team": "platform",
					},
					Annotations: map[string]string{
						"description": "Development environment configuration",
					},
				},
				Spec: EnvironmentSpec{
					Environment: EnvironmentConfig{
						Name: "development",
						Cluster: ClusterConfig{
							KubeconfigSecretRef: VaultSecretRef{
								Vault: "secret/kubernetes/dev-cluster",
								Key:   "kubeconfig",
							},
						},
						Namespace: "dev-apps",
					},
					Helm: EnvironmentHelmConfig{
						ValuesSource: HelmValuesSource{
							Type:       "git",
							Repository: "https://github.com/example/helm-values.git",
							Path:       "environments/dev",
							Branch:     "main",
						},
					},
					Vault: EnvironmentVaultConfig{
						Address:   "https://vault.example.com",
						Namespace: "dev",
						Auth: VaultAuthConfig{
							Method: "kubernetes",
							Kubernetes: &VaultKubernetesAuth{
								Role: "dev-role",
							},
						},
					},
					Datasources: map[string]VaultDatasource{
						"database": {
							Vault: "secret/database/dev",
							Keys:  []string{"username", "password", "host", "port"},
						},
						"redis": {
							Vault: "secret/redis/dev",
							Keys:  []string{"host", "port", "password"},
						},
					},
					VaultSecrets: []VaultStaticSecret{
						{
							Name:              "app-secrets",
							VaultPath:         "secret/app/dev",
							DestinationSecret: "app-secrets",
							RequiredKeys:      []string{"api-key", "database-url"},
						},
					},
					PodEnvValidation: PodEnvValidationConfig{
						Enabled: true,
						ExpectedEnvVars: []ExpectedEnvVar{
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
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.environment)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled Environment
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Verify the unmarshaled data matches original
			assert.Equal(t, tt.environment.APIVersion, unmarshaled.APIVersion)
			assert.Equal(t, tt.environment.Kind, unmarshaled.Kind)
			assert.Equal(t, tt.environment.Metadata.Name, unmarshaled.Metadata.Name)
			assert.Equal(t, tt.environment.Spec.Environment.Name, unmarshaled.Spec.Environment.Name)
		})
	}
}

func TestEnvironmentMetadataTimestamps(t *testing.T) {
	now := time.Now()
	metadata := EnvironmentMetadata{
		Name:      "test-env",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Test JSON marshaling with timestamps
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	var unmarshaled EnvironmentMetadata
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, metadata.Name, unmarshaled.Name)
	assert.True(t, metadata.CreatedAt.Equal(*unmarshaled.CreatedAt))
	assert.True(t, metadata.UpdatedAt.Equal(*unmarshaled.UpdatedAt))
}

func TestVaultAuthConfig(t *testing.T) {
	tests := []struct {
		name   string
		config VaultAuthConfig
	}{
		{
			name: "kubernetes auth",
			config: VaultAuthConfig{
				Method: "kubernetes",
				Kubernetes: &VaultKubernetesAuth{
					Role: "my-app-role",
				},
			},
		},
		{
			name: "token auth",
			config: VaultAuthConfig{
				Method: "token",
				Token:  "s.AAAAAAAAAAAAAAAAAAAAAA",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			require.NoError(t, err)

			var unmarshaled VaultAuthConfig
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.config.Method, unmarshaled.Method)
			assert.Equal(t, tt.config.Token, unmarshaled.Token)

			if tt.config.Kubernetes != nil {
				require.NotNil(t, unmarshaled.Kubernetes)
				assert.Equal(t, tt.config.Kubernetes.Role, unmarshaled.Kubernetes.Role)
			}
		})
	}
}

func TestVaultDatasource(t *testing.T) {
	datasource := VaultDatasource{
		Vault: "secret/database/production",
		Keys:  []string{"username", "password", "host", "port", "database"},
	}

	data, err := json.Marshal(datasource)
	require.NoError(t, err)

	var unmarshaled VaultDatasource
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, datasource.Vault, unmarshaled.Vault)
	assert.ElementsMatch(t, datasource.Keys, unmarshaled.Keys)
}

func TestVaultStaticSecret(t *testing.T) {
	secret := VaultStaticSecret{
		Name:              "app-config",
		VaultPath:         "secret/app/production/config",
		DestinationSecret: "app-config-secret",
		RequiredKeys:      []string{"api-key", "webhook-secret", "database-url"},
	}

	data, err := json.Marshal(secret)
	require.NoError(t, err)

	var unmarshaled VaultStaticSecret
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, secret.Name, unmarshaled.Name)
	assert.Equal(t, secret.VaultPath, unmarshaled.VaultPath)
	assert.Equal(t, secret.DestinationSecret, unmarshaled.DestinationSecret)
	assert.ElementsMatch(t, secret.RequiredKeys, unmarshaled.RequiredKeys)
}

func TestPodEnvValidationConfig(t *testing.T) {
	tests := []struct {
		name   string
		config PodEnvValidationConfig
	}{
		{
			name: "enabled with expected env vars",
			config: PodEnvValidationConfig{
				Enabled: true,
				ExpectedEnvVars: []ExpectedEnvVar{
					{
						Name:      "DATABASE_URL",
						SecretRef: "database-secrets",
						Key:       "url",
					},
					{
						Name:      "API_KEY",
						SecretRef: "api-secrets",
						Key:       "key",
					},
				},
			},
		},
		{
			name: "disabled",
			config: PodEnvValidationConfig{
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			require.NoError(t, err)

			var unmarshaled PodEnvValidationConfig
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.config.Enabled, unmarshaled.Enabled)
			assert.Len(t, unmarshaled.ExpectedEnvVars, len(tt.config.ExpectedEnvVars))

			for i, expected := range tt.config.ExpectedEnvVars {
				if i < len(unmarshaled.ExpectedEnvVars) {
					assert.Equal(t, expected.Name, unmarshaled.ExpectedEnvVars[i].Name)
					assert.Equal(t, expected.SecretRef, unmarshaled.ExpectedEnvVars[i].SecretRef)
					assert.Equal(t, expected.Key, unmarshaled.ExpectedEnvVars[i].Key)
				}
			}
		})
	}
}

func TestHelmValuesSource(t *testing.T) {
	source := HelmValuesSource{
		Type:       "git",
		Repository: "https://github.com/example/helm-values.git",
		Path:       "environments/production",
		Branch:     "release-v1.0",
	}

	data, err := json.Marshal(source)
	require.NoError(t, err)

	var unmarshaled HelmValuesSource
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, source.Type, unmarshaled.Type)
	assert.Equal(t, source.Repository, unmarshaled.Repository)
	assert.Equal(t, source.Path, unmarshaled.Path)
	assert.Equal(t, source.Branch, unmarshaled.Branch)
}

func TestClusterConfig(t *testing.T) {
	cluster := ClusterConfig{
		KubeconfigSecretRef: VaultSecretRef{
			Vault: "secret/kubernetes/production-cluster",
			Key:   "kubeconfig",
		},
	}

	data, err := json.Marshal(cluster)
	require.NoError(t, err)

	var unmarshaled ClusterConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, cluster.KubeconfigSecretRef.Vault, unmarshaled.KubeconfigSecretRef.Vault)
	assert.Equal(t, cluster.KubeconfigSecretRef.Key, unmarshaled.KubeconfigSecretRef.Key)
}

func TestVaultSecretRef(t *testing.T) {
	secretRef := VaultSecretRef{
		Vault: "secret/path/to/secret",
		Key:   "specific-key",
	}

	data, err := json.Marshal(secretRef)
	require.NoError(t, err)

	var unmarshaled VaultSecretRef
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, secretRef.Vault, unmarshaled.Vault)
	assert.Equal(t, secretRef.Key, unmarshaled.Key)
}

func TestEnvironmentSpecComplexStructure(t *testing.T) {
	spec := EnvironmentSpec{
		Environment: EnvironmentConfig{
			Name: "staging",
			Cluster: ClusterConfig{
				KubeconfigSecretRef: VaultSecretRef{
					Vault: "secret/kubernetes/staging-cluster",
					Key:   "kubeconfig",
				},
			},
			Namespace: "staging-apps",
		},
		Helm: EnvironmentHelmConfig{
			ValuesSource: HelmValuesSource{
				Type:       "git",
				Repository: "https://github.com/example/helm-values.git",
				Path:       "environments/staging",
				Branch:     "main",
			},
		},
		Vault: EnvironmentVaultConfig{
			Address:   "https://vault.staging.example.com",
			Namespace: "staging",
			Auth: VaultAuthConfig{
				Method: "kubernetes",
				Kubernetes: &VaultKubernetesAuth{
					Role: "staging-apps-role",
				},
			},
		},
		Datasources: map[string]VaultDatasource{
			"postgresql": {
				Vault: "secret/database/staging/postgresql",
				Keys:  []string{"username", "password", "host", "port", "database"},
			},
			"redis": {
				Vault: "secret/cache/staging/redis",
				Keys:  []string{"host", "port", "password"},
			},
			"s3": {
				Vault: "secret/storage/staging/s3",
				Keys:  []string{"access-key", "secret-key", "bucket", "region"},
			},
		},
		VaultSecrets: []VaultStaticSecret{
			{
				Name:              "app-secrets",
				VaultPath:         "secret/app/staging",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"jwt-secret", "api-key"},
			},
			{
				Name:              "external-api-keys",
				VaultPath:         "secret/external-services/staging",
				DestinationSecret: "external-secrets",
				RequiredKeys:      []string{"stripe-key", "sendgrid-key", "auth0-secret"},
			},
		},
		PodEnvValidation: PodEnvValidationConfig{
			Enabled: true,
			ExpectedEnvVars: []ExpectedEnvVar{
				{
					Name:      "DATABASE_URL",
					SecretRef: "app-secrets",
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
			},
		},
	}

	// Test marshaling and unmarshaling
	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var unmarshaled EnvironmentSpec
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, spec.Environment.Name, unmarshaled.Environment.Name)
	assert.Equal(t, spec.Environment.Namespace, unmarshaled.Environment.Namespace)
	assert.Equal(t, spec.Vault.Address, unmarshaled.Vault.Address)
	assert.Equal(t, spec.Vault.Auth.Method, unmarshaled.Vault.Auth.Method)
	assert.Equal(t, spec.Helm.ValuesSource.Repository, unmarshaled.Helm.ValuesSource.Repository)

	// Verify collections
	assert.Len(t, unmarshaled.Datasources, 3)
	assert.Len(t, unmarshaled.VaultSecrets, 2)
	assert.Len(t, unmarshaled.PodEnvValidation.ExpectedEnvVars, 3)

	// Verify specific nested values
	assert.Contains(t, unmarshaled.Datasources, "postgresql")
	assert.Equal(t, "secret/database/staging/postgresql", unmarshaled.Datasources["postgresql"].Vault)
	assert.Equal(t, "app-secrets", unmarshaled.VaultSecrets[0].Name)
	assert.True(t, unmarshaled.PodEnvValidation.Enabled)
}

func TestExpectedEnvVar(t *testing.T) {
	envVar := ExpectedEnvVar{
		Name:      "DATABASE_URL",
		SecretRef: "database-secrets",
		Key:       "url",
	}

	data, err := json.Marshal(envVar)
	require.NoError(t, err)

	var unmarshaled ExpectedEnvVar
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, envVar.Name, unmarshaled.Name)
	assert.Equal(t, envVar.SecretRef, unmarshaled.SecretRef)
	assert.Equal(t, envVar.Key, unmarshaled.Key)
}
