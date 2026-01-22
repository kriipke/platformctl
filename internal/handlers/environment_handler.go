package handlers

import (
	"encoding/json"
		"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/contextops/platformctl/internal/auth"
	"github.com/contextops/platformctl/internal/models"
	"github.com/contextops/platformctl/internal/storage"
	"github.com/contextops/platformctl/internal/validation"
	"github.com/contextops/platformctl/pkg/api"
)

type EnvironmentHandler struct {
	environmentStore *storage.EnvironmentStore
}

func NewEnvironmentHandler(environmentStore *storage.EnvironmentStore) *EnvironmentHandler {
	return &EnvironmentHandler{
		environmentStore: environmentStore,
	}
}

// CreateEnvironment handles POST /environments
func (h *EnvironmentHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req api.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the environment manifest
	if err := validation.ValidateEnvironment(&req.Environment); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "environment", Message: err.Error()},
		})
		return
	}

	// Create the environment
	err = h.environmentStore.Create(r.Context(), &req.Environment, customer.CustomerID)
	if err != nil {
		if err == storage.ErrConflict {
			writeErrorResponse(w, "Environment already exists", http.StatusConflict)
			return
		}
		writeErrorResponse(w, "Failed to create environment", http.StatusInternalServerError)
		return
	}

	response := api.CreateEnvironmentResponse{
		Success:         true,
		Message:         "Environment created successfully",
		EnvironmentName: req.Environment.Metadata.Name,
		CreatedAt:       req.Environment.Metadata.CreatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusCreated)
}

// GetEnvironment handles GET /environments/{name}
func (h *EnvironmentHandler) GetEnvironment(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	environmentName := mux.Vars(r)["name"]
	if environmentName == "" {
		writeErrorResponse(w, "Environment name is required", http.StatusBadRequest)
		return
	}

	environment, err := h.environmentStore.Get(r.Context(), environmentName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Environment not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to get environment", http.StatusInternalServerError)
		return
	}

	response := api.GetEnvironmentResponse{
		Success:     true,
		Environment: *environment,
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// ListEnvironments handles GET /environments
func (h *EnvironmentHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	environments, err := h.environmentStore.List(r.Context(), customer.CustomerID)
	if err != nil {
		writeErrorResponse(w, "Failed to list environments", http.StatusInternalServerError)
		return
	}

	// Convert pointers to values
	envValues := make([]models.Environment, len(environments))
	for i, env := range environments {
		envValues[i] = *env
	}

	response := api.ListEnvironmentsResponse{
		Success:      true,
		Environments: envValues,
		Count:        len(environments),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// UpdateEnvironment handles PUT /environments/{name}
func (h *EnvironmentHandler) UpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	environmentName := mux.Vars(r)["name"]
	if environmentName == "" {
		writeErrorResponse(w, "Environment name is required", http.StatusBadRequest)
		return
	}

	var req api.UpdateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the name in the URL matches the name in the request
	if req.Environment.Metadata.Name != environmentName {
		writeErrorResponse(w, "Environment name in URL does not match request body", http.StatusBadRequest)
		return
	}

	// Validate the environment manifest
	if err := validation.ValidateEnvironment(&req.Environment); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "environment", Message: err.Error()},
		})
		return
	}

	// Update the environment
	err = h.environmentStore.Update(r.Context(), &req.Environment, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Environment not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to update environment", http.StatusInternalServerError)
		return
	}

	response := api.UpdateEnvironmentResponse{
		Success:   true,
		Message:   "Environment updated successfully",
		UpdatedAt: req.Environment.Metadata.UpdatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// DeleteEnvironment handles DELETE /environments/{name}
func (h *EnvironmentHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	environmentName := mux.Vars(r)["name"]
	if environmentName == "" {
		writeErrorResponse(w, "Environment name is required", http.StatusBadRequest)
		return
	}

	err = h.environmentStore.Delete(r.Context(), environmentName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Environment not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to delete environment", http.StatusInternalServerError)
		return
	}

	response := api.DeleteResponse{
		Success: true,
		Message: "Environment deleted successfully",
	}

	writeJSONResponse(w, response, http.StatusOK)
}

