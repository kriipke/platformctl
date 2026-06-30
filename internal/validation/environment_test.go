package validation

import (
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		environment models.Environment
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid environment manifest",
			environment: models.Environment{
				APIVersion: "platformctl/v1",
				Kind:       "Environment",
				Metadata: models.EnvironmentMetadata{
					Name: "valid-env",
				},
				Spec: models.EnvironmentSpec{
					Environment: models.EnvironmentConfig{
						Name: "development",
						Cluster: models.ClusterConfig{
							KubeconfigSecretRef: models.VaultSecretRef{
								Vault: "secret/kubernetes/dev",
								Key:   "kubeconfig",
							},
						},
						Namespace: "dev-apps",
					},
					Helm: models.EnvironmentHelmConfig{
						ValuesSource: models.HelmValuesSource{
							Type:       "git",
							Repository: "https://github.com/example/helm-values.git",
							Path:       "environments/dev",
							Branch:     "main",
						},
					},
					Vault: models.EnvironmentVaultConfig{
						Address: "https://vault.example.com",
						Auth: models.VaultAuthConfig{
							Method: "kubernetes",
							Kubernetes: &models.VaultKubernetesAuth{
								Role: "dev-role",
							},
						},
					},
					Datasources: map[string]models.VaultDatasource{
						"database": {
							Vault: "secret/database/dev",
							Keys:  []string{"username", "password"},
						},
					},
					VaultSecrets: []models.VaultStaticSecret{
						{
							Name:              "app-secrets",
							VaultPath:         "secret/app/dev",
							DestinationSecret: "app-secrets",
							RequiredKeys:      []string{"api-key"},
						},
					},
					PodEnvValidation: models.PodEnvValidationConfig{
						Enabled: false,
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid metadata - empty name",
			environment: models.Environment{
				Metadata: models.EnvironmentMetadata{
					Name: "",
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid metadata - invalid DNS name",
			environment: models.Environment{
				Metadata: models.EnvironmentMetadata{
					Name: "Invalid_Name",
				},
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvironment(&tt.environment)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironmentMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    models.EnvironmentMetadata
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid metadata",
			metadata: models.EnvironmentMetadata{
				Name: "dev-environment",
			},
			expectError: false,
		},
		{
			name: "empty name",
			metadata: models.EnvironmentMetadata{
				Name: "",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid DNS name",
			metadata: models.EnvironmentMetadata{
				Name: "Invalid_Environment_Name",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentMetadata(&tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironmentConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.EnvironmentConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid environment config",
			config: models.EnvironmentConfig{
				Name: "development",
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/dev",
						Key:   "kubeconfig",
					},
				},
				Namespace: "dev-apps",
			},
			expectError: false,
		},
		{
			name: "empty name",
			config: models.EnvironmentConfig{
				Name: "",
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/dev",
						Key:   "kubeconfig",
					},
				},
				Namespace: "dev-apps",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "empty namespace",
			config: models.EnvironmentConfig{
				Name: "development",
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/dev",
						Key:   "kubeconfig",
					},
				},
				Namespace: "",
			},
			expectError: true,
			errorMsg:    "namespace is required",
		},
		{
			name: "invalid namespace DNS name",
			config: models.EnvironmentConfig{
				Name: "development",
				Cluster: models.ClusterConfig{
					KubeconfigSecretRef: models.VaultSecretRef{
						Vault: "secret/kubernetes/dev",
						Key:   "kubeconfig",
					},
				},
				Namespace: "Invalid_Namespace",
			},
			expectError: true,
			errorMsg:    "namespace must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateClusterConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.ClusterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid cluster config",
			config: models.ClusterConfig{
				KubeconfigSecretRef: models.VaultSecretRef{
					Vault: "secret/kubernetes/production",
					Key:   "kubeconfig",
				},
			},
			expectError: false,
		},
		{
			name: "invalid vault secret ref",
			config: models.ClusterConfig{
				KubeconfigSecretRef: models.VaultSecretRef{
					Vault: "", // Invalid empty vault path
					Key:   "kubeconfig",
				},
			},
			expectError: true,
			errorMsg:    "vault path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClusterConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVaultSecretRef(t *testing.T) {
	tests := []struct {
		name        string
		ref         models.VaultSecretRef
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid vault secret ref",
			ref: models.VaultSecretRef{
				Vault: "secret/path/to/secret",
				Key:   "my-key",
			},
			expectError: false,
		},
		{
			name: "empty vault path",
			ref: models.VaultSecretRef{
				Vault: "",
				Key:   "my-key",
			},
			expectError: true,
			errorMsg:    "vault path is required",
		},
		{
			name: "invalid vault path - starts with /",
			ref: models.VaultSecretRef{
				Vault: "/secret/path",
				Key:   "my-key",
			},
			expectError: true,
			errorMsg:    "vault path must be a valid vault path",
		},
		{
			name: "invalid vault path - no slash",
			ref: models.VaultSecretRef{
				Vault: "secretpath",
				Key:   "my-key",
			},
			expectError: true,
			errorMsg:    "vault path must be a valid vault path",
		},
		{
			name: "empty key",
			ref: models.VaultSecretRef{
				Vault: "secret/path/to/secret",
				Key:   "",
			},
			expectError: true,
			errorMsg:    "key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultSecretRef(&tt.ref)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateHelmValuesSource(t *testing.T) {
	tests := []struct {
		name        string
		source      models.HelmValuesSource
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid helm values source",
			source: models.HelmValuesSource{
				Type:       "git",
				Repository: "https://github.com/example/helm-values.git",
				Path:       "environments/prod",
				Branch:     "main",
			},
			expectError: false,
		},
		{
			name: "invalid type",
			source: models.HelmValuesSource{
				Type:       "s3",
				Repository: "https://github.com/example/helm-values.git",
				Path:       "environments/prod",
				Branch:     "main",
			},
			expectError: true,
			errorMsg:    "type must be 'git'",
		},
		{
			name: "empty repository",
			source: models.HelmValuesSource{
				Type:       "git",
				Repository: "",
				Path:       "environments/prod",
				Branch:     "main",
			},
			expectError: true,
			errorMsg:    "repository is required",
		},
		{
			name: "invalid repository URL",
			source: models.HelmValuesSource{
				Type:       "git",
				Repository: "invalid-url",
				Path:       "environments/prod",
				Branch:     "main",
			},
			expectError: true,
			errorMsg:    "repository must be a valid URL",
		},
		{
			name: "empty path",
			source: models.HelmValuesSource{
				Type:       "git",
				Repository: "https://github.com/example/helm-values.git",
				Path:       "",
				Branch:     "main",
			},
			expectError: true,
			errorMsg:    "path is required",
		},
		{
			name: "empty branch",
			source: models.HelmValuesSource{
				Type:       "git",
				Repository: "https://github.com/example/helm-values.git",
				Path:       "environments/prod",
				Branch:     "",
			},
			expectError: true,
			errorMsg:    "branch is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHelmValuesSource(&tt.source)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironmentVaultConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.EnvironmentVaultConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid vault config with kubernetes auth",
			config: models.EnvironmentVaultConfig{
				Address: "https://vault.example.com",
				Auth: models.VaultAuthConfig{
					Method: "kubernetes",
					Kubernetes: &models.VaultKubernetesAuth{
						Role: "my-role",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid vault config with token auth",
			config: models.EnvironmentVaultConfig{
				Address: "https://vault.example.com",
				Auth: models.VaultAuthConfig{
					Method: "token",
					Token:  "s.AAAAAAAAAAAAAAAAAAAAAA",
				},
			},
			expectError: false,
		},
		{
			name: "empty address",
			config: models.EnvironmentVaultConfig{
				Address: "",
				Auth: models.VaultAuthConfig{
					Method: "kubernetes",
					Kubernetes: &models.VaultKubernetesAuth{
						Role: "my-role",
					},
				},
			},
			expectError: true,
			errorMsg:    "address is required",
		},
		{
			name: "invalid address URL",
			config: models.EnvironmentVaultConfig{
				Address: "invalid-url",
				Auth: models.VaultAuthConfig{
					Method: "kubernetes",
					Kubernetes: &models.VaultKubernetesAuth{
						Role: "my-role",
					},
				},
			},
			expectError: true,
			errorMsg:    "address must be a valid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentVaultConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVaultAuthConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.VaultAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid kubernetes auth",
			config: models.VaultAuthConfig{
				Method: "kubernetes",
				Kubernetes: &models.VaultKubernetesAuth{
					Role: "my-app-role",
				},
			},
			expectError: false,
		},
		{
			name: "valid token auth",
			config: models.VaultAuthConfig{
				Method: "token",
				Token:  "s.AAAAAAAAAAAAAAAAAAAAAA",
			},
			expectError: false,
		},
		{
			name: "invalid method",
			config: models.VaultAuthConfig{
				Method: "ldap",
			},
			expectError: true,
			errorMsg:    "method must be one of: kubernetes, token",
		},
		{
			name: "token auth without token",
			config: models.VaultAuthConfig{
				Method: "token",
				Token:  "",
			},
			expectError: true,
			errorMsg:    "token is required for token auth method",
		},
		{
			name: "kubernetes auth without kubernetes config",
			config: models.VaultAuthConfig{
				Method: "kubernetes",
			},
			expectError: true,
			errorMsg:    "kubernetes configuration is required for kubernetes auth method",
		},
		{
			name: "kubernetes auth without role",
			config: models.VaultAuthConfig{
				Method: "kubernetes",
				Kubernetes: &models.VaultKubernetesAuth{
					Role: "",
				},
			},
			expectError: true,
			errorMsg:    "kubernetes.role is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultAuthConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVaultDatasource(t *testing.T) {
	tests := []struct {
		name        string
		datasource  models.VaultDatasource
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid vault datasource",
			datasource: models.VaultDatasource{
				Vault: "secret/database/production",
				Keys:  []string{"username", "password", "host", "port"},
			},
			expectError: false,
		},
		{
			name: "empty vault path",
			datasource: models.VaultDatasource{
				Vault: "",
				Keys:  []string{"username", "password"},
			},
			expectError: true,
			errorMsg:    "vault path is required",
		},
		{
			name: "invalid vault path",
			datasource: models.VaultDatasource{
				Vault: "/invalid/path",
				Keys:  []string{"username", "password"},
			},
			expectError: true,
			errorMsg:    "vault path must be a valid vault path",
		},
		{
			name: "no keys",
			datasource: models.VaultDatasource{
				Vault: "secret/database/production",
				Keys:  []string{},
			},
			expectError: true,
			errorMsg:    "at least one key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultDatasource(&tt.datasource)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVaultStaticSecret(t *testing.T) {
	tests := []struct {
		name        string
		secret      models.VaultStaticSecret
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid vault static secret",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "secret/app/production",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"api-key", "jwt-secret"},
			},
			expectError: false,
		},
		{
			name: "empty name",
			secret: models.VaultStaticSecret{
				Name:              "",
				VaultPath:         "secret/app/production",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid name DNS",
			secret: models.VaultStaticSecret{
				Name:              "Invalid_Name",
				VaultPath:         "secret/app/production",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "empty vault path",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "vaultPath is required",
		},
		{
			name: "invalid vault path",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "/invalid/vault/path",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "vaultPath must be a valid vault path",
		},
		{
			name: "empty destination secret",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "secret/app/production",
				DestinationSecret: "",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "destinationSecret is required",
		},
		{
			name: "invalid destination secret DNS",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "secret/app/production",
				DestinationSecret: "Invalid_Secret_Name",
				RequiredKeys:      []string{"api-key"},
			},
			expectError: true,
			errorMsg:    "destinationSecret must be a valid DNS-1123 label",
		},
		{
			name: "no required keys",
			secret: models.VaultStaticSecret{
				Name:              "app-secrets",
				VaultPath:         "secret/app/production",
				DestinationSecret: "app-secrets",
				RequiredKeys:      []string{},
			},
			expectError: true,
			errorMsg:    "at least one required key is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultStaticSecret(&tt.secret)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePodEnvValidationConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.PodEnvValidationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "disabled validation",
			config: models.PodEnvValidationConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "enabled validation with valid env vars",
			config: models.PodEnvValidationConfig{
				Enabled: true,
				ExpectedEnvVars: []models.ExpectedEnvVar{
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
			expectError: false,
		},
		{
			name: "enabled validation with invalid env var",
			config: models.PodEnvValidationConfig{
				Enabled: true,
				ExpectedEnvVars: []models.ExpectedEnvVar{
					{
						Name:      "", // Invalid empty name
						SecretRef: "database-secrets",
						Key:       "url",
					},
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePodEnvValidationConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExpectedEnvVar(t *testing.T) {
	tests := []struct {
		name        string
		envVar      models.ExpectedEnvVar
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid expected env var",
			envVar: models.ExpectedEnvVar{
				Name:      "DATABASE_URL",
				SecretRef: "database-secrets",
				Key:       "url",
			},
			expectError: false,
		},
		{
			name: "empty name",
			envVar: models.ExpectedEnvVar{
				Name:      "",
				SecretRef: "database-secrets",
				Key:       "url",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "empty secret ref",
			envVar: models.ExpectedEnvVar{
				Name:      "DATABASE_URL",
				SecretRef: "",
				Key:       "url",
			},
			expectError: true,
			errorMsg:    "secretRef is required",
		},
		{
			name: "empty key",
			envVar: models.ExpectedEnvVar{
				Name:      "DATABASE_URL",
				SecretRef: "database-secrets",
				Key:       "",
			},
			expectError: true,
			errorMsg:    "key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExpectedEnvVar(&tt.envVar)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironmentSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        models.EnvironmentSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid environment spec",
			spec: models.EnvironmentSpec{
				Environment: models.EnvironmentConfig{
					Name: "production",
					Cluster: models.ClusterConfig{
						KubeconfigSecretRef: models.VaultSecretRef{
							Vault: "secret/kubernetes/prod",
							Key:   "kubeconfig",
						},
					},
					Namespace: "prod-apps",
				},
				Helm: models.EnvironmentHelmConfig{
					ValuesSource: models.HelmValuesSource{
						Type:       "git",
						Repository: "https://github.com/example/helm-values.git",
						Path:       "environments/prod",
						Branch:     "main",
					},
				},
				Vault: models.EnvironmentVaultConfig{
					Address: "https://vault.example.com",
					Auth: models.VaultAuthConfig{
						Method: "kubernetes",
						Kubernetes: &models.VaultKubernetesAuth{
							Role: "prod-role",
						},
					},
				},
				Datasources: map[string]models.VaultDatasource{
					"database": {
						Vault: "secret/database/prod",
						Keys:  []string{"username", "password"},
					},
				},
				VaultSecrets: []models.VaultStaticSecret{
					{
						Name:              "app-secrets",
						VaultPath:         "secret/app/prod",
						DestinationSecret: "app-secrets",
						RequiredKeys:      []string{"api-key"},
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
					},
				},
			},
			expectError: false,
		},
		{
			name: "no datasources",
			spec: models.EnvironmentSpec{
				Environment: models.EnvironmentConfig{
					Name: "production",
					Cluster: models.ClusterConfig{
						KubeconfigSecretRef: models.VaultSecretRef{
							Vault: "secret/kubernetes/prod",
							Key:   "kubeconfig",
						},
					},
					Namespace: "prod-apps",
				},
				Helm: models.EnvironmentHelmConfig{
					ValuesSource: models.HelmValuesSource{
						Type:       "git",
						Repository: "https://github.com/example/helm-values.git",
						Path:       "environments/prod",
						Branch:     "main",
					},
				},
				Vault: models.EnvironmentVaultConfig{
					Address: "https://vault.example.com",
					Auth: models.VaultAuthConfig{
						Method: "kubernetes",
						Kubernetes: &models.VaultKubernetesAuth{
							Role: "prod-role",
						},
					},
				},
				Datasources: map[string]models.VaultDatasource{}, // Empty
				VaultSecrets: []models.VaultStaticSecret{
					{
						Name:              "app-secrets",
						VaultPath:         "secret/app/prod",
						DestinationSecret: "app-secrets",
						RequiredKeys:      []string{"api-key"},
					},
				},
				PodEnvValidation: models.PodEnvValidationConfig{
					Enabled: false,
				},
			},
			expectError: true,
			errorMsg:    "at least one datasource is required",
		},
		{
			name: "no vault secrets",
			spec: models.EnvironmentSpec{
				Environment: models.EnvironmentConfig{
					Name: "production",
					Cluster: models.ClusterConfig{
						KubeconfigSecretRef: models.VaultSecretRef{
							Vault: "secret/kubernetes/prod",
							Key:   "kubeconfig",
						},
					},
					Namespace: "prod-apps",
				},
				Helm: models.EnvironmentHelmConfig{
					ValuesSource: models.HelmValuesSource{
						Type:       "git",
						Repository: "https://github.com/example/helm-values.git",
						Path:       "environments/prod",
						Branch:     "main",
					},
				},
				Vault: models.EnvironmentVaultConfig{
					Address: "https://vault.example.com",
					Auth: models.VaultAuthConfig{
						Method: "kubernetes",
						Kubernetes: &models.VaultKubernetesAuth{
							Role: "prod-role",
						},
					},
				},
				Datasources: map[string]models.VaultDatasource{
					"database": {
						Vault: "secret/database/prod",
						Keys:  []string{"username", "password"},
					},
				},
				VaultSecrets:     []models.VaultStaticSecret{}, // Empty
				PodEnvValidation: models.PodEnvValidationConfig{Enabled: false},
			},
			expectError: true,
			errorMsg:    "at least one vault secret is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvironmentSpec(&tt.spec)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidVaultPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"valid secret path", "secret/myapp/config", true},
		{"valid kv-v2 path", "kv-v2/data/myapp", true},
		{"valid with multiple segments", "secret/team/app/environment", true},
		{"empty path", "", false},
		{"path starting with slash", "/secret/myapp", false},
		{"path without slash", "secretpath", false},
		{"just slash", "/", false},
		{"single segment", "secret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidVaultPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}