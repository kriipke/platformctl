package models

import (
	"time"
)

// Context pairing struct
type Context struct {
	APIVersion string          `json:"apiVersion" validate:"required,eq=contextops/v1"`
	Kind       string          `json:"kind" validate:"required,eq=Context"`
	Metadata   ContextMetadata `json:"metadata" validate:"required"`
	Spec       ContextSpec     `json:"spec" validate:"required"`
}

type ContextMetadata struct {
	Name        string            `json:"name" validate:"required,dns1123label"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   *time.Time        `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type ContextSpec struct {
	AppRef      string              `json:"appRef" validate:"required"`
	Deployments []ContextDeployment `json:"deployments" validate:"required,min=1"`
	GitOps      ContextGitOpsConfig `json:"gitops" validate:"required"`
}

type ContextDeployment struct {
	Environment    string `json:"environment" validate:"required"`
	AppRef         string `json:"appRef" validate:"required"`
	EnvironmentRef string `json:"environmentRef" validate:"required"`
	Active         bool   `json:"active"`
}

type ContextGitOpsConfig struct {
	CustomerBranch CustomerBranchConfig `json:"customerBranch" validate:"required"`
	Monitoring     MonitoringConfig     `json:"monitoring" validate:"required"`
}

type CustomerBranchConfig struct {
	Enabled bool   `json:"enabled"`
	Branch  string `json:"branch" validate:"required_if=Enabled true,customer_branch"`
}

type MonitoringConfig struct {
	ApplicationSets         bool `json:"applicationSets"`
	VaultSecrets           bool `json:"vaultSecrets"`
	HelmValues             bool `json:"helmValues"`
	CrossEnvironmentDrift  bool `json:"crossEnvironmentDrift"`
}