package validation

import (
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateApp(t *testing.T) {
	tests := []struct {
		name        string
		app         models.App
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid app manifest",
			app: models.App{
				APIVersion: "platformctl/v1",
				Kind:       "App",
				Metadata: models.AppMetadata{
					Name: "valid-app",
				},
				Spec: models.AppSpec{
					Application: models.AppApplicationConfig{
						Name:       "valid-app",
						Version:    "1.0.0",
						Maintainer: "test@example.com",
					},
					Helm: models.AppHelmConfig{
						Sources: []models.HelmSource{
							{
								Type:     "helm-registry",
								Registry: "registry.example.com",
								Chart:    "test-chart",
							},
						},
						DefaultSource: 0,
					},
					ArgoCD: models.AppArgoCDConfig{
						ApplicationSets: []models.ApplicationSetConfig{
							{
								Name:      "test-appset",
								Namespace: "argocd",
								Generator: models.ApplicationSetGenerator{
									Type: "git",
									Git: &models.GitGenerator{
										RepoURL:  "https://github.com/example/repo.git",
										Revision: "main",
									},
								},
								Template: models.ApplicationSetTemplate{
									Metadata: models.ApplicationSetTemplateMetadata{
										Name: "test-template",
									},
									Spec: models.ApplicationSetTemplateSpec{
										Source: models.ApplicationSetTemplateSource{},
									},
								},
							},
						},
					},
					Environments: []models.AppEnvironmentRef{
						{
							Name:           "dev",
							EnvironmentRef: "dev-env",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid metadata - empty name",
			app: models.App{
				Metadata: models.AppMetadata{
					Name: "",
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid metadata - invalid DNS name",
			app: models.App{
				Metadata: models.AppMetadata{
					Name: "Invalid_Name_With_Underscores",
				},
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateApp(&tt.app)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAppMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    models.AppMetadata
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid metadata",
			metadata: models.AppMetadata{
				Name: "valid-app-name",
			},
			expectError: false,
		},
		{
			name: "empty name",
			metadata: models.AppMetadata{
				Name: "",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid DNS name - uppercase",
			metadata: models.AppMetadata{
				Name: "InvalidName",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "invalid DNS name - underscores",
			metadata: models.AppMetadata{
				Name: "invalid_name",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "invalid DNS name - too long",
			metadata: models.AppMetadata{
				Name: "this-is-a-very-long-name-that-exceeds-the-maximum-allowed-length-for-dns-labels",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "valid DNS name - with hyphens",
			metadata: models.AppMetadata{
				Name: "valid-app-name-with-hyphens",
			},
			expectError: false,
		},
		{
			name: "valid DNS name - with numbers",
			metadata: models.AppMetadata{
				Name: "app123",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppMetadata(&tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAppApplicationConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.AppApplicationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid application config",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "1.2.3",
				Maintainer: "test@example.com",
			},
			expectError: false,
		},
		{
			name: "empty name",
			config: models.AppApplicationConfig{
				Name:       "",
				Version:    "1.0.0",
				Maintainer: "test@example.com",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid DNS name",
			config: models.AppApplicationConfig{
				Name:       "Invalid_Name",
				Version:    "1.0.0",
				Maintainer: "test@example.com",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "empty version",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "",
				Maintainer: "test@example.com",
			},
			expectError: true,
			errorMsg:    "version is required",
		},
		{
			name: "invalid semver",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "invalid-version",
				Maintainer: "test@example.com",
			},
			expectError: true,
			errorMsg:    "version must be a valid semantic version",
		},
		{
			name: "valid semver with v prefix",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "v1.2.3",
				Maintainer: "test@example.com",
			},
			expectError: false,
		},
		{
			name: "valid semver with prerelease",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "1.2.3-alpha.1",
				Maintainer: "test@example.com",
			},
			expectError: false,
		},
		{
			name: "empty maintainer",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "1.0.0",
				Maintainer: "",
			},
			expectError: true,
			errorMsg:    "maintainer is required",
		},
		{
			name: "invalid email",
			config: models.AppApplicationConfig{
				Name:       "test-app",
				Version:    "1.0.0",
				Maintainer: "invalid-email",
			},
			expectError: true,
			errorMsg:    "maintainer must be a valid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppApplicationConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAppHelmConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.AppHelmConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid helm config",
			config: models.AppHelmConfig{
				Sources: []models.HelmSource{
					{
						Type:     "helm-registry",
						Registry: "registry.example.com",
						Chart:    "test-chart",
					},
				},
				DefaultSource: 0,
			},
			expectError: false,
		},
		{
			name: "no sources",
			config: models.AppHelmConfig{
				Sources:       []models.HelmSource{},
				DefaultSource: 0,
			},
			expectError: true,
			errorMsg:    "at least one Helm source is required",
		},
		{
			name: "default source out of range",
			config: models.AppHelmConfig{
				Sources: []models.HelmSource{
					{
						Type:     "helm-registry",
						Registry: "registry.example.com",
						Chart:    "test-chart",
					},
				},
				DefaultSource: 1, // Only one source at index 0
			},
			expectError: true,
			errorMsg:    "defaultSource index is out of range",
		},
		{
			name: "multiple sources with valid default",
			config: models.AppHelmConfig{
				Sources: []models.HelmSource{
					{
						Type:     "helm-registry",
						Registry: "registry.example.com",
						Chart:    "test-chart",
					},
					{
						Type:       "git",
						Repository: "https://github.com/example/charts.git",
						Chart:      "another-chart",
					},
				},
				DefaultSource: 1,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppHelmConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateHelmSource(t *testing.T) {
	tests := []struct {
		name        string
		source      models.HelmSource
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid helm-registry source",
			source: models.HelmSource{
				Type:     "helm-registry",
				Registry: "registry.example.com",
				Chart:    "test-chart",
			},
			expectError: false,
		},
		{
			name: "valid git source",
			source: models.HelmSource{
				Type:       "git",
				Repository: "https://github.com/example/charts.git",
				Chart:      "test-chart",
			},
			expectError: false,
		},
		{
			name: "valid oci source",
			source: models.HelmSource{
				Type:     "oci",
				Registry: "oci://registry.example.com",
				Chart:    "test-chart",
			},
			expectError: false,
		},
		{
			name: "invalid type",
			source: models.HelmSource{
				Type:  "invalid-type",
				Chart: "test-chart",
			},
			expectError: true,
			errorMsg:    "type must be one of: helm-registry, git, oci",
		},
		{
			name: "empty chart",
			source: models.HelmSource{
				Type:     "helm-registry",
				Registry: "registry.example.com",
				Chart:    "",
			},
			expectError: true,
			errorMsg:    "chart is required",
		},
		{
			name: "helm-registry without registry",
			source: models.HelmSource{
				Type:  "helm-registry",
				Chart: "test-chart",
			},
			expectError: true,
			errorMsg:    "registry is required for helm-registry type",
		},
		{
			name: "git without repository",
			source: models.HelmSource{
				Type:  "git",
				Chart: "test-chart",
			},
			expectError: true,
			errorMsg:    "repository is required for git type",
		},
		{
			name: "git with invalid repository URL",
			source: models.HelmSource{
				Type:       "git",
				Repository: "invalid-url",
				Chart:      "test-chart",
			},
			expectError: true,
			errorMsg:    "repository must be a valid URL",
		},
		{
			name: "oci without registry",
			source: models.HelmSource{
				Type:  "oci",
				Chart: "test-chart",
			},
			expectError: true,
			errorMsg:    "registry is required for oci type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHelmSource(&tt.source)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAppArgoCDConfig(t *testing.T) {
	validApplicationSet := models.ApplicationSetConfig{
		Name:      "test-appset",
		Namespace: "argocd",
		Generator: models.ApplicationSetGenerator{
			Type: "git",
			Git: &models.GitGenerator{
				RepoURL:  "https://github.com/example/repo.git",
				Revision: "main",
			},
		},
		Template: models.ApplicationSetTemplate{
			Metadata: models.ApplicationSetTemplateMetadata{
				Name: "test-template",
			},
			Spec: models.ApplicationSetTemplateSpec{
				Source: models.ApplicationSetTemplateSource{},
			},
		},
	}

	tests := []struct {
		name        string
		config      models.AppArgoCDConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid argocd config",
			config: models.AppArgoCDConfig{
				ApplicationSets: []models.ApplicationSetConfig{validApplicationSet},
			},
			expectError: false,
		},
		{
			name: "no application sets",
			config: models.AppArgoCDConfig{
				ApplicationSets: []models.ApplicationSetConfig{},
			},
			expectError: true,
			errorMsg:    "at least one ApplicationSet is required",
		},
		{
			name: "valid with bootstrap application",
			config: models.AppArgoCDConfig{
				ApplicationSets: []models.ApplicationSetConfig{validApplicationSet},
				BootstrapApplication: &models.BootstrapApplicationConfig{
					Name:      "bootstrap-app",
					Namespace: "argocd",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppArgoCDConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateApplicationSetConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.ApplicationSetConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid application set config",
			config: models.ApplicationSetConfig{
				Name:      "test-appset",
				Namespace: "argocd",
				Generator: models.ApplicationSetGenerator{
					Type: "git",
					Git: &models.GitGenerator{
						RepoURL:  "https://github.com/example/repo.git",
						Revision: "main",
					},
				},
				Template: models.ApplicationSetTemplate{
					Metadata: models.ApplicationSetTemplateMetadata{
						Name: "test-template",
					},
					Spec: models.ApplicationSetTemplateSpec{
						Source: models.ApplicationSetTemplateSource{},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty name",
			config: models.ApplicationSetConfig{
				Name:      "",
				Namespace: "argocd",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid DNS name",
			config: models.ApplicationSetConfig{
				Name:      "Invalid_Name",
				Namespace: "argocd",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
		{
			name: "empty namespace",
			config: models.ApplicationSetConfig{
				Name:      "test-appset",
				Namespace: "",
			},
			expectError: true,
			errorMsg:    "namespace is required",
		},
		{
			name: "invalid namespace DNS name",
			config: models.ApplicationSetConfig{
				Name:      "test-appset",
				Namespace: "Invalid_Namespace",
			},
			expectError: true,
			errorMsg:    "namespace must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplicationSetConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateApplicationSetGenerator(t *testing.T) {
	tests := []struct {
		name        string
		generator   models.ApplicationSetGenerator
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid git generator",
			generator: models.ApplicationSetGenerator{
				Type: "git",
				Git: &models.GitGenerator{
					RepoURL:  "https://github.com/example/repo.git",
					Revision: "main",
				},
			},
			expectError: false,
		},
		{
			name: "valid list generator",
			generator: models.ApplicationSetGenerator{
				Type: "list",
				List: &models.ListGenerator{
					Elements: []map[string]interface{}{
						{"env": "dev"},
						{"env": "prod"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid clusters generator",
			generator: models.ApplicationSetGenerator{
				Type: "clusters",
				Clusters: &models.ClustersGenerator{
					Selector: map[string]string{"env": "production"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid type",
			generator: models.ApplicationSetGenerator{
				Type: "invalid-type",
			},
			expectError: true,
			errorMsg:    "type must be one of: git, clusters, list",
		},
		{
			name: "git type without git config",
			generator: models.ApplicationSetGenerator{
				Type: "git",
			},
			expectError: true,
			errorMsg:    "git generator configuration is required",
		},
		{
			name: "list type without list config",
			generator: models.ApplicationSetGenerator{
				Type: "list",
			},
			expectError: true,
			errorMsg:    "list generator configuration is required",
		},
		{
			name: "list type with empty elements",
			generator: models.ApplicationSetGenerator{
				Type: "list",
				List: &models.ListGenerator{
					Elements: []map[string]interface{}{},
				},
			},
			expectError: true,
			errorMsg:    "list generator must have at least one element",
		},
		{
			name: "clusters type without clusters config",
			generator: models.ApplicationSetGenerator{
				Type: "clusters",
			},
			expectError: true,
			errorMsg:    "clusters generator configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplicationSetGenerator(&tt.generator)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGitGenerator(t *testing.T) {
	tests := []struct {
		name        string
		generator   models.GitGenerator
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid git generator",
			generator: models.GitGenerator{
				RepoURL:  "https://github.com/example/repo.git",
				Revision: "main",
			},
			expectError: false,
		},
		{
			name: "empty repoURL",
			generator: models.GitGenerator{
				RepoURL:  "",
				Revision: "main",
			},
			expectError: true,
			errorMsg:    "repoURL is required",
		},
		{
			name: "invalid repoURL",
			generator: models.GitGenerator{
				RepoURL:  "invalid-url",
				Revision: "main",
			},
			expectError: true,
			errorMsg:    "repoURL must be a valid URL",
		},
		{
			name: "empty revision",
			generator: models.GitGenerator{
				RepoURL:  "https://github.com/example/repo.git",
				Revision: "",
			},
			expectError: true,
			errorMsg:    "revision is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitGenerator(&tt.generator)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAppEnvironmentRef(t *testing.T) {
	tests := []struct {
		name        string
		envRef      models.AppEnvironmentRef
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid environment ref",
			envRef: models.AppEnvironmentRef{
				Name:           "production",
				EnvironmentRef: "prod-env",
			},
			expectError: false,
		},
		{
			name: "empty name",
			envRef: models.AppEnvironmentRef{
				Name:           "",
				EnvironmentRef: "prod-env",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "empty environment ref",
			envRef: models.AppEnvironmentRef{
				Name:           "production",
				EnvironmentRef: "",
			},
			expectError: true,
			errorMsg:    "environmentRef is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppEnvironmentRef(&tt.envRef)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationHelperFunctions(t *testing.T) {
	t.Run("isValidDNSName", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{"valid simple name", "test", true},
			{"valid with hyphens", "test-app", true},
			{"valid with numbers", "app123", true},
			{"valid long name", "very-long-but-valid-dns-name", true},
			{"empty string", "", false},
			{"too long", "this-name-is-way-too-long-to-be-a-valid-dns-1123-label-according-to-kubernetes", false},
			{"uppercase", "TestApp", false},
			{"underscores", "test_app", false},
			{"starts with hyphen", "-testapp", false},
			{"ends with hyphen", "testapp-", false},
			{"special characters", "test@app", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isValidDNSName(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isValidSemver", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{"simple version", "1.0.0", true},
			{"with v prefix", "v1.2.3", true},
			{"with prerelease", "1.2.3-alpha", true},
			{"with prerelease and build", "1.2.3-alpha.1+build.123", true},
			{"patch version only", "1.0", false},
			{"major version only", "1", false},
			{"invalid format", "1.2.3.4", false},
			{"empty string", "", false},
			{"text version", "latest", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isValidSemver(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isValidEmail", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{"simple email", "test@example.com", true},
			{"with subdomain", "user@mail.example.com", true},
			{"with plus", "user+tag@example.com", true},
			{"with numbers", "user123@example123.com", true},
			{"no @ symbol", "testexample.com", false},
			{"no domain", "test@", false},
			{"no local part", "@example.com", false},
			{"invalid domain", "test@", false},
			{"empty string", "", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isValidEmail(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isValidURL", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{"https URL", "https://example.com", true},
			{"http URL", "http://example.com", true},
			{"git URL", "git://github.com/user/repo.git", true},
			{"no protocol", "example.com", false},
			{"ftp protocol", "ftp://example.com", false},
			{"empty string", "", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isValidURL(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestValidateAppSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        models.AppSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid spec",
			spec: models.AppSpec{
				Application: models.AppApplicationConfig{
					Name:       "test-app",
					Version:    "1.0.0",
					Maintainer: "test@example.com",
				},
				Helm: models.AppHelmConfig{
					Sources: []models.HelmSource{
						{
							Type:     "helm-registry",
							Registry: "registry.example.com",
							Chart:    "test-chart",
						},
					},
					DefaultSource: 0,
				},
				ArgoCD: models.AppArgoCDConfig{
					ApplicationSets: []models.ApplicationSetConfig{
						{
							Name:      "test-appset",
							Namespace: "argocd",
							Generator: models.ApplicationSetGenerator{
								Type: "git",
								Git: &models.GitGenerator{
									RepoURL:  "https://github.com/example/repo.git",
									Revision: "main",
								},
							},
							Template: models.ApplicationSetTemplate{
								Metadata: models.ApplicationSetTemplateMetadata{
									Name: "test-template",
								},
								Spec: models.ApplicationSetTemplateSpec{
									Source: models.ApplicationSetTemplateSource{},
								},
							},
						},
					},
				},
				Environments: []models.AppEnvironmentRef{
					{
						Name:           "dev",
						EnvironmentRef: "dev-env",
					},
				},
			},
			expectError: false,
		},
		{
			name: "no environments",
			spec: models.AppSpec{
				Application: models.AppApplicationConfig{
					Name:       "test-app",
					Version:    "1.0.0",
					Maintainer: "test@example.com",
				},
				Helm: models.AppHelmConfig{
					Sources: []models.HelmSource{
						{
							Type:     "helm-registry",
							Registry: "registry.example.com",
							Chart:    "test-chart",
						},
					},
					DefaultSource: 0,
				},
				ArgoCD: models.AppArgoCDConfig{
					ApplicationSets: []models.ApplicationSetConfig{
						{
							Name:      "test-appset",
							Namespace: "argocd",
							Generator: models.ApplicationSetGenerator{
								Type: "git",
								Git: &models.GitGenerator{
									RepoURL:  "https://github.com/example/repo.git",
									Revision: "main",
								},
							},
							Template: models.ApplicationSetTemplate{
								Metadata: models.ApplicationSetTemplateMetadata{
									Name: "test-template",
								},
								Spec: models.ApplicationSetTemplateSpec{
									Source: models.ApplicationSetTemplateSource{},
								},
							},
						},
					},
				},
				Environments: []models.AppEnvironmentRef{},
			},
			expectError: true,
			errorMsg:    "at least one environment reference is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppSpec(&tt.spec)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
