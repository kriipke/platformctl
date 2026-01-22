package validation

import (
	"testing"

	"platformctl/internal/models"
)

func TestValidateContextCustomerBranch(t *testing.T) {
	validator := NewValidator()

	contextModel := &models.Context{
		APIVersion: "contextops/v1",
		Kind:       "Context",
		Metadata: models.ContextMetadata{
			Name: "example",
		},
		Spec: models.ContextSpec{
			AppRef: "app",
			Deployments: []models.ContextDeployment{
				{
					Environment:    "dev",
					AppRef:         "app",
					EnvironmentRef: "env",
					Active:         true,
				},
			},
			GitOps: models.ContextGitOpsConfig{
				CustomerBranch: models.CustomerBranchConfig{
					Enabled: true,
					Branch:  "customer/acme",
				},
				Monitoring: models.MonitoringConfig{},
			},
		},
	}

	if err := ValidateContext(validator, contextModel); err != nil {
		t.Fatalf("expected context to be valid, got error: %v", err)
	}

	contextModel.Spec.GitOps.CustomerBranch.Branch = "invalid"
	if err := ValidateContext(validator, contextModel); err == nil {
		t.Fatal("expected validation error for invalid customer branch")
	}
}

func TestValidateAppSemver(t *testing.T) {
	validator := NewValidator()

	app := &models.App{
		APIVersion: "contextops/v1",
		Kind:       "App",
		Metadata: models.AppMetadata{
			Name: "webapp",
		},
		Spec: models.AppSpec{
			Application: models.AppApplicationConfig{
				Name:       "webapp",
				Version:    "1.2.3",
				Maintainer: "team@example.com",
			},
			Helm: models.AppHelmConfig{
				Sources:       []models.HelmSource{{Type: "git", Chart: "webapp"}},
				DefaultSource: 0,
			},
			ArgoCD: models.AppArgoCDConfig{
				ApplicationSets: []models.ApplicationSetConfig{
					{
						Name:      "webapp",
						Namespace: "argocd",
						Generator: models.ApplicationSetGenerator{Type: "git", Git: &models.GitGenerator{RepoURL: "https://example.com", Revision: "main"}},
						Template:  models.ApplicationSetTemplate{Metadata: models.ApplicationSetTemplateMetadata{Name: "webapp"}, Spec: models.ApplicationSetTemplateSpec{Source: models.ApplicationSetTemplateSource{Helm: &models.ApplicationSetTemplateHelm{}}}},
					},
				},
			},
			Environments: []models.AppEnvironmentRef{{Name: "dev", EnvironmentRef: "dev"}},
		},
	}

	if err := ValidateApp(validator, app); err != nil {
		t.Fatalf("expected app to be valid, got error: %v", err)
	}

	app.Spec.Application.Version = "not-semver"
	if err := ValidateApp(validator, app); err == nil {
		t.Fatal("expected validation error for invalid semver")
	}
}
