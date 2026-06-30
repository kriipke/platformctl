package models

import (
	"time"
)

// App manifest struct
type App struct {
	APIVersion string      `json:"apiVersion" validate:"required,eq=platformctl/v1"`
	Kind       string      `json:"kind" validate:"required,eq=App"`
	Metadata   AppMetadata `json:"metadata" validate:"required"`
	Spec       AppSpec     `json:"spec" validate:"required"`
}

type AppMetadata struct {
	Name        string            `json:"name" validate:"required,dns1123label"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   *time.Time        `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type AppSpec struct {
	Application  AppApplicationConfig `json:"application" validate:"required"`
	Helm         AppHelmConfig        `json:"helm" validate:"required"`
	ArgoCD       AppArgoCDConfig      `json:"argocd" validate:"required"`
	Environments []AppEnvironmentRef  `json:"environments" validate:"required,min=1"`
}

type AppApplicationConfig struct {
	Name       string `json:"name" validate:"required,dns1123label"`
	Version    string `json:"version" validate:"required,semver"`
	Maintainer string `json:"maintainer" validate:"required,email"`
}

type AppHelmConfig struct {
	Sources       []HelmSource `json:"sources" validate:"required,min=1"`
	DefaultSource int          `json:"defaultSource" validate:"gte=0"`
}

type HelmSource struct {
	Type       string `json:"type" validate:"required,oneof=helm-registry git oci"`
	Registry   string `json:"registry,omitempty"`
	Chart      string `json:"chart,omitempty" validate:"required"`
	Version    string `json:"version,omitempty"`
	Repository string `json:"repository,omitempty"`
	Path       string `json:"path,omitempty"`
	Ref        string `json:"ref,omitempty"`
}

type AppArgoCDConfig struct {
	ApplicationSets      []ApplicationSetConfig      `json:"applicationSets" validate:"required,min=1"`
	BootstrapApplication *BootstrapApplicationConfig `json:"bootstrapApplication,omitempty"`
}

type ApplicationSetConfig struct {
	Name      string                  `json:"name" validate:"required,dns1123label"`
	Namespace string                  `json:"namespace" validate:"required,dns1123label"`
	Generator ApplicationSetGenerator `json:"generator" validate:"required"`
	Template  ApplicationSetTemplate  `json:"template" validate:"required"`
}

type ApplicationSetGenerator struct {
	Type     string              `json:"type" validate:"required,oneof=git clusters list"`
	Git      *GitGenerator       `json:"git,omitempty"`
	List     *ListGenerator      `json:"list,omitempty"`
	Clusters *ClustersGenerator  `json:"clusters,omitempty"`
}

type GitGenerator struct {
	RepoURL     string                  `json:"repoURL" validate:"required,url"`
	Revision    string                  `json:"revision" validate:"required"`
	Directories []GitGeneratorDirectory `json:"directories,omitempty"`
	Files       []GitGeneratorFile      `json:"files,omitempty"`
}

type GitGeneratorDirectory struct {
	Path    string `json:"path" validate:"required"`
	Exclude string `json:"exclude,omitempty"`
}

type GitGeneratorFile struct {
	Path string `json:"path" validate:"required"`
}

type ListGenerator struct {
	Elements []map[string]interface{} `json:"elements" validate:"required,min=1"`
}

type ClustersGenerator struct {
	Selector map[string]string `json:"selector,omitempty"`
}

type ApplicationSetTemplate struct {
	Metadata ApplicationSetTemplateMetadata `json:"metadata" validate:"required"`
	Spec     ApplicationSetTemplateSpec     `json:"spec" validate:"required"`
}

type ApplicationSetTemplateMetadata struct {
	Name   string            `json:"name" validate:"required"`
	Labels map[string]string `json:"labels,omitempty"`
}

type ApplicationSetTemplateSpec struct {
	Source ApplicationSetTemplateSource `json:"source" validate:"required"`
}

type ApplicationSetTemplateSource struct {
	Helm *ApplicationSetTemplateHelm `json:"helm,omitempty"`
}

type ApplicationSetTemplateHelm struct {
	ValueFiles []string `json:"valueFiles,omitempty"`
}

type BootstrapApplicationConfig struct {
	Name      string `json:"name" validate:"required,dns1123label"`
	Namespace string `json:"namespace" validate:"required,dns1123label"`
}

type AppEnvironmentRef struct {
	Name           string `json:"name" validate:"required"`
	EnvironmentRef string `json:"environmentRef" validate:"required"`
}