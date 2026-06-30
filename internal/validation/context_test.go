package validation

import (
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateContext(t *testing.T) {
	tests := []struct {
		name        string
		context     models.Context
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid context manifest",
			context: models.Context{
				APIVersion: "contextops/v1",
				Kind:       "Context",
				Metadata: models.ContextMetadata{
					Name: "valid-context",
				},
				Spec: models.ContextSpec{
					AppRef: "my-app",
					Deployments: []models.ContextDeployment{
						{
							Environment:    "dev",
							AppRef:         "my-app",
							EnvironmentRef: "dev-environment",
							Active:         true,
						},
						{
							Environment:    "prod",
							AppRef:         "my-app",
							EnvironmentRef: "prod-environment",
							Active:         false,
						},
					},
					GitOps: models.ContextGitOpsConfig{
						CustomerBranch: models.CustomerBranchConfig{
							Enabled: false,
						},
						Monitoring: models.MonitoringConfig{
							ApplicationSets:       true,
							VaultSecrets:         true,
							HelmValues:           true,
							CrossEnvironmentDrift: false,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid metadata - empty name",
			context: models.Context{
				Metadata: models.ContextMetadata{
					Name: "",
				},
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid metadata - invalid DNS name",
			context: models.Context{
				Metadata: models.ContextMetadata{
					Name: "Invalid_Context_Name",
				},
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContext(&tt.context)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContextMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    models.ContextMetadata
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid metadata",
			metadata: models.ContextMetadata{
				Name: "app-context",
			},
			expectError: false,
		},
		{
			name: "empty name",
			metadata: models.ContextMetadata{
				Name: "",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "invalid DNS name",
			metadata: models.ContextMetadata{
				Name: "Invalid_Context_Name",
			},
			expectError: true,
			errorMsg:    "name must be a valid DNS-1123 label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContextMetadata(&tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContextSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        models.ContextSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid context spec",
			spec: models.ContextSpec{
				AppRef: "my-app",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "dev",
						AppRef:         "my-app",
						EnvironmentRef: "dev-environment",
						Active:         true,
					},
				},
				GitOps: models.ContextGitOpsConfig{
					CustomerBranch: models.CustomerBranchConfig{
						Enabled: false,
					},
					Monitoring: models.MonitoringConfig{
						ApplicationSets: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty appRef",
			spec: models.ContextSpec{
				AppRef: "",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "dev",
						AppRef:         "my-app",
						EnvironmentRef: "dev-environment",
						Active:         true,
					},
				},
			},
			expectError: true,
			errorMsg:    "appRef is required",
		},
		{
			name: "no deployments",
			spec: models.ContextSpec{
				AppRef:      "my-app",
				Deployments: []models.ContextDeployment{},
			},
			expectError: true,
			errorMsg:    "at least one deployment is required",
		},
		{
			name: "no active deployments",
			spec: models.ContextSpec{
				AppRef: "my-app",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "dev",
						AppRef:         "my-app",
						EnvironmentRef: "dev-environment",
						Active:         false, // Not active
					},
					{
						Environment:    "prod",
						AppRef:         "my-app",
						EnvironmentRef: "prod-environment",
						Active:         false, // Not active
					},
				},
				GitOps: models.ContextGitOpsConfig{
					CustomerBranch: models.CustomerBranchConfig{
						Enabled: false,
					},
					Monitoring: models.MonitoringConfig{
						ApplicationSets: true,
					},
				},
			},
			expectError: true,
			errorMsg:    "at least one deployment must be active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContextSpec(&tt.spec)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContextDeployment(t *testing.T) {
	tests := []struct {
		name        string
		deployment  models.ContextDeployment
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid deployment",
			deployment: models.ContextDeployment{
				Environment:    "dev",
				AppRef:         "my-app",
				EnvironmentRef: "dev-environment",
				Active:         true,
			},
			expectError: false,
		},
		{
			name: "valid deployment - production",
			deployment: models.ContextDeployment{
				Environment:    "production",
				AppRef:         "my-app",
				EnvironmentRef: "prod-environment",
				Active:         true,
			},
			expectError: false,
		},
		{
			name: "valid deployment - case insensitive",
			deployment: models.ContextDeployment{
				Environment:    "PROD",
				AppRef:         "my-app",
				EnvironmentRef: "prod-environment",
				Active:         true,
			},
			expectError: false,
		},
		{
			name: "empty environment",
			deployment: models.ContextDeployment{
				Environment:    "",
				AppRef:         "my-app",
				EnvironmentRef: "dev-environment",
				Active:         true,
			},
			expectError: true,
			errorMsg:    "environment is required",
		},
		{
			name: "empty appRef",
			deployment: models.ContextDeployment{
				Environment:    "dev",
				AppRef:         "",
				EnvironmentRef: "dev-environment",
				Active:         true,
			},
			expectError: true,
			errorMsg:    "appRef is required",
		},
		{
			name: "empty environmentRef",
			deployment: models.ContextDeployment{
				Environment:    "dev",
				AppRef:         "my-app",
				EnvironmentRef: "",
				Active:         true,
			},
			expectError: true,
			errorMsg:    "environmentRef is required",
		},
		{
			name: "invalid environment name",
			deployment: models.ContextDeployment{
				Environment:    "invalid-env",
				AppRef:         "my-app",
				EnvironmentRef: "dev-environment",
				Active:         true,
			},
			expectError: true,
			errorMsg:    "environment 'invalid-env' is not a recognized environment name",
		},
		{
			name: "valid deployment - staging",
			deployment: models.ContextDeployment{
				Environment:    "staging",
				AppRef:         "my-app",
				EnvironmentRef: "staging-environment",
				Active:         false,
			},
			expectError: false,
		},
		{
			name: "valid deployment - testing",
			deployment: models.ContextDeployment{
				Environment:    "testing",
				AppRef:         "my-app",
				EnvironmentRef: "test-environment",
				Active:         true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContextDeployment(&tt.deployment)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContextGitOpsConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.ContextGitOpsConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid gitops config - customer branch disabled",
			config: models.ContextGitOpsConfig{
				CustomerBranch: models.CustomerBranchConfig{
					Enabled: false,
				},
				Monitoring: models.MonitoringConfig{
					ApplicationSets:       true,
					VaultSecrets:         true,
					HelmValues:           false,
					CrossEnvironmentDrift: true,
				},
			},
			expectError: false,
		},
		{
			name: "valid gitops config - customer branch enabled",
			config: models.ContextGitOpsConfig{
				CustomerBranch: models.CustomerBranchConfig{
					Enabled: true,
					Branch:  "customer/acme-corp",
				},
				Monitoring: models.MonitoringConfig{
					ApplicationSets:       false,
					VaultSecrets:         false,
					HelmValues:           false,
					CrossEnvironmentDrift: false,
				},
			},
			expectError: false,
		},
		{
			name: "invalid customer branch config",
			config: models.ContextGitOpsConfig{
				CustomerBranch: models.CustomerBranchConfig{
					Enabled: true,
					Branch:  "", // Invalid - empty branch when enabled
				},
				Monitoring: models.MonitoringConfig{
					ApplicationSets: true,
				},
			},
			expectError: true,
			errorMsg:    "branch is required when enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContextGitOpsConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCustomerBranchConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      models.CustomerBranchConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "disabled customer branch",
			config: models.CustomerBranchConfig{
				Enabled: false,
				Branch:  "", // Can be empty when disabled
			},
			expectError: false,
		},
		{
			name: "valid customer branch",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/enterprise-client",
			},
			expectError: false,
		},
		{
			name: "valid customer branch - simple name",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/acme",
			},
			expectError: false,
		},
		{
			name: "valid customer branch - with hyphens",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/big-company-name",
			},
			expectError: false,
		},
		{
			name: "valid customer branch - with numbers",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/client123",
			},
			expectError: false,
		},
		{
			name: "enabled but empty branch",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "",
			},
			expectError: true,
			errorMsg:    "branch is required when enabled",
		},
		{
			name: "invalid branch pattern - no customer prefix",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "acme-corp",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
		{
			name: "invalid branch pattern - wrong prefix",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "client/acme-corp",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
		{
			name: "invalid branch pattern - underscore",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/acme_corp",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
		{
			name: "invalid branch pattern - starts with hyphen",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/-acme",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
		{
			name: "invalid branch pattern - ends with hyphen",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/acme-",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
		{
			name: "invalid branch pattern - special characters",
			config: models.CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/acme@corp",
			},
			expectError: true,
			errorMsg:    "branch must follow customer branch pattern (customer/{customer-name})",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomerBranchConfig(&tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMonitoringConfig(t *testing.T) {
	// Monitoring config validation should always pass since all fields are optional booleans
	tests := []struct {
		name   string
		config models.MonitoringConfig
	}{
		{
			name: "all monitoring enabled",
			config: models.MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         true,
				HelmValues:           true,
				CrossEnvironmentDrift: true,
			},
		},
		{
			name: "all monitoring disabled",
			config: models.MonitoringConfig{
				ApplicationSets:       false,
				VaultSecrets:         false,
				HelmValues:           false,
				CrossEnvironmentDrift: false,
			},
		},
		{
			name: "partial monitoring",
			config: models.MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         false,
				HelmValues:           true,
				CrossEnvironmentDrift: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMonitoringConfig(&tt.config)
			assert.NoError(t, err)
		})
	}
}

func TestIsValidCustomerBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected bool
	}{
		{"valid simple", "customer/acme", true},
		{"valid with hyphens", "customer/acme-corp", true},
		{"valid with numbers", "customer/client123", true},
		{"valid complex", "customer/big-enterprise-client", true},
		{"valid single char", "customer/a", true},
		{"empty string", "", false},
		{"no customer prefix", "acme-corp", false},
		{"wrong prefix", "client/acme", false},
		{"no slash", "customer", false},
		{"empty after slash", "customer/", false},
		{"starts with hyphen", "customer/-acme", false},
		{"ends with hyphen", "customer/acme-", false},
		{"underscore", "customer/acme_corp", false},
		{"special characters", "customer/acme@corp", false},
		{"uppercase", "customer/ACME", true}, // Should be valid according to regex
		{"mixed case", "customer/AcmeCorp", true},
		{"slash in name", "customer/acme/corp", false},
		{"just customer/", "customer/", false},
		{"multiple hyphens", "customer/very-long-client-name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidCustomerBranch(tt.branch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	slice := []string{"dev", "staging", "production", "qa"}

	tests := []struct {
		name     string
		item     string
		expected bool
	}{
		{"exact match", "dev", true},
		{"uppercase match", "DEV", true},
		{"mixed case match", "Staging", true},
		{"all uppercase", "PRODUCTION", true},
		{"no match", "test", false},
		{"partial match", "prod", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIgnoreCase(slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateContextSpecComplexScenarios(t *testing.T) {
	tests := []struct {
		name        string
		spec        models.ContextSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "multi-environment deployment with customer branch",
			spec: models.ContextSpec{
				AppRef: "microservice-app",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "development",
						AppRef:         "microservice-app",
						EnvironmentRef: "dev-env",
						Active:         true,
					},
					{
						Environment:    "staging",
						AppRef:         "microservice-app",
						EnvironmentRef: "staging-env",
						Active:         true,
					},
					{
						Environment:    "production",
						AppRef:         "microservice-app",
						EnvironmentRef: "prod-env",
						Active:         false, // Prepared but not active yet
					},
				},
				GitOps: models.ContextGitOpsConfig{
					CustomerBranch: models.CustomerBranchConfig{
						Enabled: true,
						Branch:  "customer/enterprise-client-v2",
					},
					Monitoring: models.MonitoringConfig{
						ApplicationSets:       true,
						VaultSecrets:         true,
						HelmValues:           true,
						CrossEnvironmentDrift: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "deployment mismatch - different appRef",
			spec: models.ContextSpec{
				AppRef: "main-app",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "dev",
						AppRef:         "different-app", // Inconsistent with spec.AppRef
						EnvironmentRef: "dev-env",
						Active:         true,
					},
				},
				GitOps: models.ContextGitOpsConfig{
					CustomerBranch: models.CustomerBranchConfig{
						Enabled: false,
					},
					Monitoring: models.MonitoringConfig{
						ApplicationSets: true,
					},
				},
			},
			expectError: false, // This validation doesn't check for consistency between spec.AppRef and deployment.AppRef
		},
		{
			name: "single active deployment among multiple",
			spec: models.ContextSpec{
				AppRef: "web-app",
				Deployments: []models.ContextDeployment{
					{
						Environment:    "dev",
						AppRef:         "web-app",
						EnvironmentRef: "dev-env",
						Active:         false,
					},
					{
						Environment:    "staging",
						AppRef:         "web-app",
						EnvironmentRef: "staging-env",
						Active:         false,
					},
					{
						Environment:    "prod",
						AppRef:         "web-app",
						EnvironmentRef: "prod-env",
						Active:         true, // Only this one is active
					},
				},
				GitOps: models.ContextGitOpsConfig{
					CustomerBranch: models.CustomerBranchConfig{
						Enabled: false,
					},
					Monitoring: models.MonitoringConfig{
						ApplicationSets:       false,
						VaultSecrets:         true,
						HelmValues:           false,
						CrossEnvironmentDrift: true,
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContextSpec(&tt.spec)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContextWithAllEnvironmentTypes(t *testing.T) {
	validEnvironments := []string{"dev", "qa", "uat", "prod", "staging", "production", "development", "testing"}

	for _, env := range validEnvironments {
		t.Run("environment_"+env, func(t *testing.T) {
			deployment := models.ContextDeployment{
				Environment:    env,
				AppRef:         "test-app",
				EnvironmentRef: env + "-env",
				Active:         true,
			}

			err := validateContextDeployment(&deployment)
			assert.NoError(t, err)
		})
	}

	// Test case sensitivity
	t.Run("case_insensitive_environments", func(t *testing.T) {
		caseVariations := []string{"DEV", "Dev", "PRODUCTION", "Production", "STAGING", "Staging"}
		
		for _, env := range caseVariations {
			deployment := models.ContextDeployment{
				Environment:    env,
				AppRef:         "test-app",
				EnvironmentRef: "test-env",
				Active:         true,
			}

			err := validateContextDeployment(&deployment)
			assert.NoError(t, err, "Environment %s should be valid (case insensitive)", env)
		}
	})
}