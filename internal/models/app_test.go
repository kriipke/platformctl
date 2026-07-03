package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppMarshaling(t *testing.T) {
	tests := []struct {
		name        string
		app         App
		expectedErr bool
	}{
		{
			name: "valid app manifest",
			app: App{
				APIVersion: "platformctl/v1",
				Kind:       "App",
				Metadata: AppMetadata{
					Name: "test-app",
					Labels: map[string]string{
						"env": "test",
					},
					Annotations: map[string]string{
						"description": "Test application",
					},
				},
				Spec: AppSpec{
					Application: AppApplicationConfig{
						Name:       "test-app",
						Version:    "1.0.0",
						Maintainer: "test@example.com",
					},
					Helm: AppHelmConfig{
						Sources: []HelmSource{
							{
								Type:     "helm-registry",
								Registry: "registry.example.com",
								Chart:    "test-chart",
								Version:  "1.0.0",
							},
						},
						DefaultSource: 0,
					},
					ArgoCD: AppArgoCDConfig{
						ApplicationSets: []ApplicationSetConfig{
							{
								Name:      "test-appset",
								Namespace: "argocd",
								Generator: ApplicationSetGenerator{
									Type: "git",
									Git: &GitGenerator{
										RepoURL:  "https://github.com/example/repo.git",
										Revision: "main",
									},
								},
								Template: ApplicationSetTemplate{
									Metadata: ApplicationSetTemplateMetadata{
										Name: "{{name}}",
									},
									Spec: ApplicationSetTemplateSpec{
										Source: ApplicationSetTemplateSource{
											Helm: &ApplicationSetTemplateHelm{
												ValueFiles: []string{"values.yaml"},
											},
										},
									},
								},
							},
						},
					},
					Environments: []AppEnvironmentRef{
						{
							Name:           "dev",
							EnvironmentRef: "dev-env",
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
			data, err := json.Marshal(tt.app)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled App
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Verify the unmarshaled data matches original
			assert.Equal(t, tt.app.APIVersion, unmarshaled.APIVersion)
			assert.Equal(t, tt.app.Kind, unmarshaled.Kind)
			assert.Equal(t, tt.app.Metadata.Name, unmarshaled.Metadata.Name)
			assert.Equal(t, tt.app.Spec.Application.Name, unmarshaled.Spec.Application.Name)
		})
	}
}

func TestAppMetadataTimestamps(t *testing.T) {
	now := time.Now()
	metadata := AppMetadata{
		Name:      "test-app",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Test JSON marshaling with timestamps
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	var unmarshaled AppMetadata
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, metadata.Name, unmarshaled.Name)
	assert.True(t, metadata.CreatedAt.Equal(*unmarshaled.CreatedAt))
	assert.True(t, metadata.UpdatedAt.Equal(*unmarshaled.UpdatedAt))
}

func TestHelmSourceTypes(t *testing.T) {
	tests := []struct {
		name   string
		source HelmSource
		valid  bool
	}{
		{
			name: "helm-registry source",
			source: HelmSource{
				Type:     "helm-registry",
				Registry: "registry.example.com",
				Chart:    "my-chart",
				Version:  "1.0.0",
			},
			valid: true,
		},
		{
			name: "git source",
			source: HelmSource{
				Type:       "git",
				Repository: "https://github.com/example/charts.git",
				Chart:      "my-chart",
				Path:       "charts/my-chart",
				Ref:        "main",
			},
			valid: true,
		},
		{
			name: "oci source",
			source: HelmSource{
				Type:     "oci",
				Registry: "oci://registry.example.com",
				Chart:    "my-chart",
				Version:  "1.0.0",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.source)
			require.NoError(t, err)

			var unmarshaled HelmSource
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.source.Type, unmarshaled.Type)
			assert.Equal(t, tt.source.Chart, unmarshaled.Chart)
		})
	}
}

func TestApplicationSetGenerator(t *testing.T) {
	tests := []struct {
		name      string
		generator ApplicationSetGenerator
	}{
		{
			name: "git generator",
			generator: ApplicationSetGenerator{
				Type: "git",
				Git: &GitGenerator{
					RepoURL:  "https://github.com/example/config.git",
					Revision: "main",
					Directories: []GitGeneratorDirectory{
						{
							Path: "apps/*/",
						},
					},
					Files: []GitGeneratorFile{
						{
							Path: "apps/config.json",
						},
					},
				},
			},
		},
		{
			name: "list generator",
			generator: ApplicationSetGenerator{
				Type: "list",
				List: &ListGenerator{
					Elements: []map[string]interface{}{
						{
							"cluster": "dev",
							"url":     "https://dev.example.com",
						},
						{
							"cluster": "prod",
							"url":     "https://prod.example.com",
						},
					},
				},
			},
		},
		{
			name: "clusters generator",
			generator: ApplicationSetGenerator{
				Type: "clusters",
				Clusters: &ClustersGenerator{
					Selector: map[string]string{
						"env": "production",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.generator)
			require.NoError(t, err)

			var unmarshaled ApplicationSetGenerator
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.generator.Type, unmarshaled.Type)

			switch tt.generator.Type {
			case "git":
				require.NotNil(t, unmarshaled.Git)
				assert.Equal(t, tt.generator.Git.RepoURL, unmarshaled.Git.RepoURL)
				assert.Equal(t, tt.generator.Git.Revision, unmarshaled.Git.Revision)
			case "list":
				require.NotNil(t, unmarshaled.List)
				assert.Len(t, unmarshaled.List.Elements, len(tt.generator.List.Elements))
			case "clusters":
				require.NotNil(t, unmarshaled.Clusters)
				assert.Equal(t, tt.generator.Clusters.Selector, unmarshaled.Clusters.Selector)
			}
		})
	}
}

func TestAppEnvironmentRef(t *testing.T) {
	envRef := AppEnvironmentRef{
		Name:           "production",
		EnvironmentRef: "prod-env-manifest",
	}

	data, err := json.Marshal(envRef)
	require.NoError(t, err)

	var unmarshaled AppEnvironmentRef
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, envRef.Name, unmarshaled.Name)
	assert.Equal(t, envRef.EnvironmentRef, unmarshaled.EnvironmentRef)
}

func TestBootstrapApplicationConfig(t *testing.T) {
	bootstrap := &BootstrapApplicationConfig{
		Name:      "bootstrap-app",
		Namespace: "argocd",
	}

	argoCDConfig := AppArgoCDConfig{
		ApplicationSets: []ApplicationSetConfig{
			{
				Name:      "test-appset",
				Namespace: "argocd",
				Generator: ApplicationSetGenerator{
					Type: "git",
					Git: &GitGenerator{
						RepoURL:  "https://github.com/example/repo.git",
						Revision: "main",
					},
				},
				Template: ApplicationSetTemplate{
					Metadata: ApplicationSetTemplateMetadata{
						Name: "{{name}}",
					},
					Spec: ApplicationSetTemplateSpec{
						Source: ApplicationSetTemplateSource{},
					},
				},
			},
		},
		BootstrapApplication: bootstrap,
	}

	data, err := json.Marshal(argoCDConfig)
	require.NoError(t, err)

	var unmarshaled AppArgoCDConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	require.NotNil(t, unmarshaled.BootstrapApplication)
	assert.Equal(t, bootstrap.Name, unmarshaled.BootstrapApplication.Name)
	assert.Equal(t, bootstrap.Namespace, unmarshaled.BootstrapApplication.Namespace)
}

func TestAppSpecValidStructure(t *testing.T) {
	spec := AppSpec{
		Application: AppApplicationConfig{
			Name:       "test-app",
			Version:    "1.2.3",
			Maintainer: "maintainer@example.com",
		},
		Helm: AppHelmConfig{
			Sources: []HelmSource{
				{
					Type:     "helm-registry",
					Registry: "registry.example.com",
					Chart:    "test-chart",
					Version:  "1.0.0",
				},
				{
					Type:       "git",
					Repository: "https://github.com/example/charts.git",
					Chart:      "another-chart",
					Path:       "charts/another-chart",
					Ref:        "v1.0.0",
				},
			},
			DefaultSource: 1, // Use second source as default
		},
		ArgoCD: AppArgoCDConfig{
			ApplicationSets: []ApplicationSetConfig{
				{
					Name:      "test-appset",
					Namespace: "argocd",
					Generator: ApplicationSetGenerator{
						Type: "list",
						List: &ListGenerator{
							Elements: []map[string]interface{}{
								{"env": "dev", "cluster": "dev-cluster"},
								{"env": "prod", "cluster": "prod-cluster"},
							},
						},
					},
					Template: ApplicationSetTemplate{
						Metadata: ApplicationSetTemplateMetadata{
							Name: "{{env}}-{{cluster}}",
							Labels: map[string]string{
								"environment": "{{env}}",
							},
						},
						Spec: ApplicationSetTemplateSpec{
							Source: ApplicationSetTemplateSource{
								Helm: &ApplicationSetTemplateHelm{
									ValueFiles: []string{
										"values-{{env}}.yaml",
										"values-{{cluster}}.yaml",
									},
								},
							},
						},
					},
				},
			},
		},
		Environments: []AppEnvironmentRef{
			{Name: "dev", EnvironmentRef: "dev-environment"},
			{Name: "prod", EnvironmentRef: "prod-environment"},
		},
	}

	// Test marshaling and unmarshaling
	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var unmarshaled AppSpec
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, spec.Application.Name, unmarshaled.Application.Name)
	assert.Len(t, unmarshaled.Helm.Sources, 2)
	assert.Equal(t, spec.Helm.DefaultSource, unmarshaled.Helm.DefaultSource)
	assert.Len(t, unmarshaled.ArgoCD.ApplicationSets, 1)
	assert.Len(t, unmarshaled.Environments, 2)

	// Verify specific nested structures
	assert.Equal(t, "list", unmarshaled.ArgoCD.ApplicationSets[0].Generator.Type)
	assert.Len(t, unmarshaled.ArgoCD.ApplicationSets[0].Generator.List.Elements, 2)
	assert.Len(t, unmarshaled.ArgoCD.ApplicationSets[0].Template.Spec.Source.Helm.ValueFiles, 2)
}
