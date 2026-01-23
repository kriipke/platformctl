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

type AppHandler struct {
	appStore *storage.AppStore
}

func NewAppHandler(appStore *storage.AppStore) *AppHandler {
	return &AppHandler{
		appStore: appStore,
	}
}

// CreateApp handles POST /apps
func (h *AppHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req api.CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the app manifest
	if err := validation.ValidateApp(&req.App); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "app", Message: err.Error()},
		})
		return
	}

	// Create the app
	err = h.appStore.Create(r.Context(), &req.App, customer.CustomerID)
	if err != nil {
		if err == storage.ErrConflict {
			writeErrorResponse(w, "App already exists", http.StatusConflict)
			return
		}
		writeErrorResponse(w, "Failed to create app", http.StatusInternalServerError)
		return
	}

	response := api.CreateAppResponse{
		Success:   true,
		Message:   "App created successfully",
		AppName:   req.App.Metadata.Name,
		CreatedAt: req.App.Metadata.CreatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusCreated)
}

// GetApp handles GET /apps/{name}
func (h *AppHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	appName := mux.Vars(r)["name"]
	if appName == "" {
		writeErrorResponse(w, "App name is required", http.StatusBadRequest)
		return
	}

	app, err := h.appStore.Get(r.Context(), appName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "App not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to get app", http.StatusInternalServerError)
		return
	}

	response := api.GetAppResponse{
		Success: true,
		App:     *app,
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// ListApps handles GET /apps
func (h *AppHandler) ListApps(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	apps, err := h.appStore.List(r.Context(), customer.CustomerID)
	if err != nil {
		writeErrorResponse(w, "Failed to list apps", http.StatusInternalServerError)
		return
	}

	// Convert pointers to values
	appValues := make([]models.App, len(apps))
	for i, app := range apps {
		appValues[i] = *app
	}

	response := api.ListAppsResponse{
		Success: true,
		Apps:    appValues,
		Count:   len(apps),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// UpdateApp handles PUT /apps/{name}
func (h *AppHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	appName := mux.Vars(r)["name"]
	if appName == "" {
		writeErrorResponse(w, "App name is required", http.StatusBadRequest)
		return
	}

	var req api.UpdateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the name in the URL matches the name in the request
	if req.App.Metadata.Name != appName {
		writeErrorResponse(w, "App name in URL does not match request body", http.StatusBadRequest)
		return
	}

	// Validate the app manifest
	if err := validation.ValidateApp(&req.App); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "app", Message: err.Error()},
		})
		return
	}

	// Update the app
	err = h.appStore.Update(r.Context(), &req.App, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "App not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to update app", http.StatusInternalServerError)
		return
	}

	response := api.UpdateAppResponse{
		Success:   true,
		Message:   "App updated successfully",
		UpdatedAt: req.App.Metadata.UpdatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// DeleteApp handles DELETE /apps/{name}
func (h *AppHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	appName := mux.Vars(r)["name"]
	if appName == "" {
		writeErrorResponse(w, "App name is required", http.StatusBadRequest)
		return
	}

	err = h.appStore.Delete(r.Context(), appName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "App not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to delete app", http.StatusInternalServerError)
		return
	}

	response := api.DeleteResponse{
		Success: true,
		Message: "App deleted successfully",
	}

	writeJSONResponse(w, response, http.StatusOK)
}

