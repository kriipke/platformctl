package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextMarshaling(t *testing.T) {
	tests := []struct {
		name        string
		context     Context
		expectedErr bool
	}{
		{
			name: "valid context manifest",
			context: Context{
				APIVersion: "contextops/v1",
				Kind:       "Context",
				Metadata: ContextMetadata{
					Name: "web-app-context",
					Labels: map[string]string{
						"app":  "web-app",
						"team": "backend",
					},
					Annotations: map[string]string{
						"description": "Context pairing for web application across environments",
					},
				},
				Spec: ContextSpec{
					AppRef: "web-app-manifest",
					Deployments: []ContextDeployment{
						{
							Environment:    "development",
							AppRef:         "web-app-manifest",
							EnvironmentRef: "dev-environment",
							Active:         true,
						},
						{
							Environment:    "staging",
							AppRef:         "web-app-manifest",
							EnvironmentRef: "staging-environment",
							Active:         true,
						},
						{
							Environment:    "production",
							AppRef:         "web-app-manifest",
							EnvironmentRef: "prod-environment",
							Active:         false, // Not yet deployed to production
						},
					},
					GitOps: ContextGitOpsConfig{
						CustomerBranch: CustomerBranchConfig{
							Enabled: true,
							Branch:  "customer/acme-corp",
						},
						Monitoring: MonitoringConfig{
							ApplicationSets:       true,
							VaultSecrets:         true,
							HelmValues:           true,
							CrossEnvironmentDrift: true,
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
			data, err := json.Marshal(tt.context)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled Context
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Verify the unmarshaled data matches original
			assert.Equal(t, tt.context.APIVersion, unmarshaled.APIVersion)
			assert.Equal(t, tt.context.Kind, unmarshaled.Kind)
			assert.Equal(t, tt.context.Metadata.Name, unmarshaled.Metadata.Name)
			assert.Equal(t, tt.context.Spec.AppRef, unmarshaled.Spec.AppRef)
		})
	}
}

func TestContextMetadataTimestamps(t *testing.T) {
	now := time.Now()
	metadata := ContextMetadata{
		Name:      "test-context",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Test JSON marshaling with timestamps
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	var unmarshaled ContextMetadata
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, metadata.Name, unmarshaled.Name)
	assert.True(t, metadata.CreatedAt.Equal(*unmarshaled.CreatedAt))
	assert.True(t, metadata.UpdatedAt.Equal(*unmarshaled.UpdatedAt))
}

func TestContextDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deployment ContextDeployment
	}{
		{
			name: "active deployment",
			deployment: ContextDeployment{
				Environment:    "production",
				AppRef:         "my-app",
				EnvironmentRef: "prod-env",
				Active:         true,
			},
		},
		{
			name: "inactive deployment",
			deployment: ContextDeployment{
				Environment:    "development",
				AppRef:         "my-app",
				EnvironmentRef: "dev-env",
				Active:         false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.deployment)
			require.NoError(t, err)

			var unmarshaled ContextDeployment
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.deployment.Environment, unmarshaled.Environment)
			assert.Equal(t, tt.deployment.AppRef, unmarshaled.AppRef)
			assert.Equal(t, tt.deployment.EnvironmentRef, unmarshaled.EnvironmentRef)
			assert.Equal(t, tt.deployment.Active, unmarshaled.Active)
		})
	}
}

func TestCustomerBranchConfig(t *testing.T) {
	tests := []struct {
		name   string
		config CustomerBranchConfig
	}{
		{
			name: "enabled customer branch",
			config: CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/enterprise-client",
			},
		},
		{
			name: "disabled customer branch",
			config: CustomerBranchConfig{
				Enabled: false,
				Branch:  "", // Should be empty when disabled
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			require.NoError(t, err)

			var unmarshaled CustomerBranchConfig
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.config.Enabled, unmarshaled.Enabled)
			assert.Equal(t, tt.config.Branch, unmarshaled.Branch)
		})
	}
}

func TestMonitoringConfig(t *testing.T) {
	tests := []struct {
		name   string
		config MonitoringConfig
	}{
		{
			name: "all monitoring enabled",
			config: MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         true,
				HelmValues:           true,
				CrossEnvironmentDrift: true,
			},
		},
		{
			name: "selective monitoring",
			config: MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         false,
				HelmValues:           true,
				CrossEnvironmentDrift: false,
			},
		},
		{
			name: "no monitoring",
			config: MonitoringConfig{
				ApplicationSets:       false,
				VaultSecrets:         false,
				HelmValues:           false,
				CrossEnvironmentDrift: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			require.NoError(t, err)

			var unmarshaled MonitoringConfig
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.config.ApplicationSets, unmarshaled.ApplicationSets)
			assert.Equal(t, tt.config.VaultSecrets, unmarshaled.VaultSecrets)
			assert.Equal(t, tt.config.HelmValues, unmarshaled.HelmValues)
			assert.Equal(t, tt.config.CrossEnvironmentDrift, unmarshaled.CrossEnvironmentDrift)
		})
	}
}

func TestContextGitOpsConfig(t *testing.T) {
	gitOpsConfig := ContextGitOpsConfig{
		CustomerBranch: CustomerBranchConfig{
			Enabled: true,
			Branch:  "customer/special-deployment",
		},
		Monitoring: MonitoringConfig{
			ApplicationSets:       true,
			VaultSecrets:         true,
			HelmValues:           false,
			CrossEnvironmentDrift: true,
		},
	}

	data, err := json.Marshal(gitOpsConfig)
	require.NoError(t, err)

	var unmarshaled ContextGitOpsConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, gitOpsConfig.CustomerBranch.Enabled, unmarshaled.CustomerBranch.Enabled)
	assert.Equal(t, gitOpsConfig.CustomerBranch.Branch, unmarshaled.CustomerBranch.Branch)
	assert.Equal(t, gitOpsConfig.Monitoring.ApplicationSets, unmarshaled.Monitoring.ApplicationSets)
	assert.Equal(t, gitOpsConfig.Monitoring.VaultSecrets, unmarshaled.Monitoring.VaultSecrets)
	assert.Equal(t, gitOpsConfig.Monitoring.HelmValues, unmarshaled.Monitoring.HelmValues)
	assert.Equal(t, gitOpsConfig.Monitoring.CrossEnvironmentDrift, unmarshaled.Monitoring.CrossEnvironmentDrift)
}

func TestContextSpecComplexStructure(t *testing.T) {
	spec := ContextSpec{
		AppRef: "microservice-app",
		Deployments: []ContextDeployment{
			{
				Environment:    "development",
				AppRef:         "microservice-app",
				EnvironmentRef: "dev-env",
				Active:         true,
			},
			{
				Environment:    "testing",
				AppRef:         "microservice-app",
				EnvironmentRef: "test-env",
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
				Active:         false, // Prepared but not active
			},
		},
		GitOps: ContextGitOpsConfig{
			CustomerBranch: CustomerBranchConfig{
				Enabled: true,
				Branch:  "customer/big-enterprise",
			},
			Monitoring: MonitoringConfig{
				ApplicationSets:       true,
				VaultSecrets:         true,
				HelmValues:           true,
				CrossEnvironmentDrift: true,
			},
		},
	}

	// Test marshaling and unmarshaling
	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var unmarshaled ContextSpec
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, spec.AppRef, unmarshaled.AppRef)
	assert.Len(t, unmarshaled.Deployments, 4)

	// Verify deployment details
	for i, deployment := range spec.Deployments {
		assert.Equal(t, deployment.Environment, unmarshaled.Deployments[i].Environment)
		assert.Equal(t, deployment.AppRef, unmarshaled.Deployments[i].AppRef)
		assert.Equal(t, deployment.EnvironmentRef, unmarshaled.Deployments[i].EnvironmentRef)
		assert.Equal(t, deployment.Active, unmarshaled.Deployments[i].Active)
	}

	// Verify GitOps configuration
	assert.Equal(t, spec.GitOps.CustomerBranch.Enabled, unmarshaled.GitOps.CustomerBranch.Enabled)
	assert.Equal(t, spec.GitOps.CustomerBranch.Branch, unmarshaled.GitOps.CustomerBranch.Branch)
	assert.Equal(t, spec.GitOps.Monitoring.ApplicationSets, unmarshaled.GitOps.Monitoring.ApplicationSets)
	assert.Equal(t, spec.GitOps.Monitoring.VaultSecrets, unmarshaled.GitOps.Monitoring.VaultSecrets)
	assert.Equal(t, spec.GitOps.Monitoring.HelmValues, unmarshaled.GitOps.Monitoring.HelmValues)
	assert.Equal(t, spec.GitOps.Monitoring.CrossEnvironmentDrift, unmarshaled.GitOps.Monitoring.CrossEnvironmentDrift)
}

func TestContextManifestRelationships(t *testing.T) {
	// Test that demonstrates relationships between manifests
	context := Context{
		APIVersion: "contextops/v1",
		Kind:       "Context",
		Metadata: ContextMetadata{
			Name: "ecommerce-context",
			Labels: map[string]string{
				"project": "ecommerce",
				"version": "v2.1",
			},
		},
		Spec: ContextSpec{
			AppRef: "ecommerce-api-app", // References an App manifest
			Deployments: []ContextDeployment{
				{
					Environment:    "development",
					AppRef:         "ecommerce-api-app",      // Same App manifest
					EnvironmentRef: "dev-ecommerce-env",     // References Environment manifest
					Active:         true,
				},
				{
					Environment:    "production",
					AppRef:         "ecommerce-api-app",      // Same App manifest
					EnvironmentRef: "prod-ecommerce-env",    // Different Environment manifest
					Active:         true,
				},
			},
			GitOps: ContextGitOpsConfig{
				CustomerBranch: CustomerBranchConfig{
					Enabled: false, // No customer-specific branch
				},
				Monitoring: MonitoringConfig{
					ApplicationSets:       true,
					VaultSecrets:         true,
					HelmValues:           true,
					CrossEnvironmentDrift: true, // Monitor drift across environments
				},
			},
		},
	}

	data, err := json.Marshal(context)
	require.NoError(t, err)

	var unmarshaled Context
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify manifest relationships are preserved
	assert.Equal(t, "ecommerce-api-app", unmarshaled.Spec.AppRef)
	
	// Check that all deployments reference the same app
	for _, deployment := range unmarshaled.Spec.Deployments {
		assert.Equal(t, "ecommerce-api-app", deployment.AppRef)
		assert.NotEmpty(t, deployment.EnvironmentRef)
	}

	// Verify deployment environment references are different
	devDeployment := unmarshaled.Spec.Deployments[0]
	prodDeployment := unmarshaled.Spec.Deployments[1]
	assert.NotEqual(t, devDeployment.EnvironmentRef, prodDeployment.EnvironmentRef)
	assert.Equal(t, "dev-ecommerce-env", devDeployment.EnvironmentRef)
	assert.Equal(t, "prod-ecommerce-env", prodDeployment.EnvironmentRef)
}

func TestContextDeploymentStates(t *testing.T) {
	// Test various deployment states and scenarios
	deployments := []ContextDeployment{
		{
			Environment:    "development",
			AppRef:         "my-app",
			EnvironmentRef: "dev-env",
			Active:         true, // Active development deployment
		},
		{
			Environment:    "feature-branch",
			AppRef:         "my-app",
			EnvironmentRef: "feature-env",
			Active:         true, // Active feature branch deployment
		},
		{
			Environment:    "staging",
			AppRef:         "my-app",
			EnvironmentRef: "staging-env",
			Active:         false, // Staged but not active
		},
		{
			Environment:    "production",
			AppRef:         "my-app",
			EnvironmentRef: "prod-env",
			Active:         true, // Active production deployment
		},
	}

	for i, deployment := range deployments {
		t.Run(deployment.Environment, func(t *testing.T) {
			data, err := json.Marshal(deployment)
			require.NoError(t, err)

			var unmarshaled ContextDeployment
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, deployments[i].Environment, unmarshaled.Environment)
			assert.Equal(t, deployments[i].Active, unmarshaled.Active)
		})
	}
}