package api

import "platformctl/internal/models"

// AppManifest wraps the App manifest for API responses.
type AppManifest struct {
	App models.App `json:"app"`
}

// EnvironmentManifest wraps the Environment manifest for API responses.
type EnvironmentManifest struct {
	Environment models.Environment `json:"environment"`
}

// ContextManifest wraps the Context manifest for API responses.
type ContextManifest struct {
	Context models.Context `json:"context"`
}
