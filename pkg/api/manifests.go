package api

import (
	"time"

	"github.com/contextops/platformctl/internal/models"
)

// API request/response types for App, Environment, and Context manifests

// App API types
type CreateAppRequest struct {
	App models.App `json:"app" validate:"required"`
}

type CreateAppResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	AppName   string `json:"appName"`
	CreatedAt string `json:"createdAt"`
}

type GetAppResponse struct {
	Success bool       `json:"success"`
	App     models.App `json:"app"`
}

type ListAppsResponse struct {
	Success bool          `json:"success"`
	Apps    []models.App  `json:"apps"`
	Count   int           `json:"count"`
}

type UpdateAppRequest struct {
	App models.App `json:"app" validate:"required"`
}

type UpdateAppResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

// Environment API types
type CreateEnvironmentRequest struct {
	Environment models.Environment `json:"environment" validate:"required"`
}

type CreateEnvironmentResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	EnvironmentName string `json:"environmentName"`
	CreatedAt       string `json:"createdAt"`
}

type GetEnvironmentResponse struct {
	Success     bool               `json:"success"`
	Environment models.Environment `json:"environment"`
}

type ListEnvironmentsResponse struct {
	Success      bool                 `json:"success"`
	Environments []models.Environment `json:"environments"`
	Count        int                  `json:"count"`
}

type UpdateEnvironmentRequest struct {
	Environment models.Environment `json:"environment" validate:"required"`
}

type UpdateEnvironmentResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

// Context API types
type CreateContextRequest struct {
	Context models.Context `json:"context" validate:"required"`
}

type CreateContextResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ContextName string `json:"contextName"`
	CreatedAt   string `json:"createdAt"`
}

type GetContextResponse struct {
	Success bool           `json:"success"`
	Context models.Context `json:"context"`
}

type ListContextsResponse struct {
	Success  bool             `json:"success"`
	Contexts []models.Context `json:"contexts"`
	Count    int              `json:"count"`
}

type UpdateContextRequest struct {
	Context models.Context `json:"context" validate:"required"`
}

type UpdateContextResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

// Common response types
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
}

// Health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Services  struct {
		Database bool `json:"database"`
		Storage  bool `json:"storage"`
	} `json:"services"`
}

// Validation error details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type ValidationErrorResponse struct {
	Success bool              `json:"success"`
	Error   string            `json:"error"`
	Details []ValidationError `json:"details"`
}