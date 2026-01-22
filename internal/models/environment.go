package models

import "time"

// Environment manifest struct
type Environment struct {
	APIVersion string              `json:"apiVersion" validate:"required,eq=contextops/v1"`
	Kind       string              `json:"kind" validate:"required,eq=Environment"`
	Metadata   EnvironmentMetadata `json:"metadata" validate:"required"`
	Spec       EnvironmentSpec     `json:"spec" validate:"required"`
}

type EnvironmentMetadata struct {
	Name        string            `json:"name" validate:"required,dns1123label"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   *time.Time        `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time        `json:"updatedAt,omitempty"`
}

type EnvironmentSpec struct {
	Environment      EnvironmentConfig          `json:"environment" validate:"required"`
	Helm             EnvironmentHelmConfig      `json:"helm" validate:"required"`
	Datasources      map[string]VaultDatasource `json:"datasources" validate:"required"`
	VaultSecrets     []VaultStaticSecret        `json:"vaultSecrets" validate:"required,min=1"`
	PodEnvValidation PodEnvValidationConfig     `json:"podEnvValidation" validate:"required"`
}

type EnvironmentConfig struct {
	Name      string        `json:"name" validate:"required"`
	Cluster   ClusterConfig `json:"cluster" validate:"required"`
	Namespace string        `json:"namespace" validate:"required,dns1123label"`
}

type ClusterConfig struct {
	KubeconfigSecretRef VaultSecretRef `json:"kubeconfigSecretRef" validate:"required"`
}

type VaultSecretRef struct {
	Vault string `json:"vault" validate:"required,vaultpath"`
	Key   string `json:"key" validate:"required"`
}

type EnvironmentHelmConfig struct {
	ValuesSource HelmValuesSource `json:"valuesSource" validate:"required"`
}

type HelmValuesSource struct {
	Type       string `json:"type" validate:"required,eq=git"`
	Repository string `json:"repository" validate:"required,url"`
	Path       string `json:"path" validate:"required"`
	Branch     string `json:"branch" validate:"required"`
}

type VaultDatasource struct {
	Vault string   `json:"vault" validate:"required,vaultpath"`
	Keys  []string `json:"keys" validate:"required,min=1"`
}

type VaultStaticSecret struct {
	Name              string   `json:"name" validate:"required,dns1123label"`
	VaultPath         string   `json:"vaultPath" validate:"required,vaultpath"`
	DestinationSecret string   `json:"destinationSecret" validate:"required,dns1123label"`
	RequiredKeys      []string `json:"requiredKeys" validate:"required,min=1"`
}

type PodEnvValidationConfig struct {
	Enabled         bool             `json:"enabled"`
	ExpectedEnvVars []ExpectedEnvVar `json:"expectedEnvVars,omitempty"`
}

type ExpectedEnvVar struct {
	Name      string `json:"name" validate:"required"`
	SecretRef string `json:"secretRef" validate:"required"`
	Key       string `json:"key" validate:"required"`
}
