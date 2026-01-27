package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/contextops/platformctl/internal/auth"
	"github.com/contextops/platformctl/internal/clients/argocd"
	"github.com/contextops/platformctl/internal/clients/git"
	"github.com/contextops/platformctl/internal/models"
	"github.com/contextops/platformctl/internal/storage"
	"github.com/contextops/platformctl/internal/validation"
	"github.com/contextops/platformctl/pkg/api"
)

type ContextHandler struct {
	contextStore *storage.ContextStore
	argoCDClient argocd.ArgoCDClient
	gitClient    git.GitClient
}

func NewContextHandler(contextStore *storage.ContextStore, argoCDClient argocd.ArgoCDClient, gitClient git.GitClient) *ContextHandler {
	return &ContextHandler{
		contextStore: contextStore,
		argoCDClient: argoCDClient,
		gitClient:    gitClient,
	}
}

// CreateContext handles POST /contexts
func (h *ContextHandler) CreateContext(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req api.CreateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the context manifest
	if err := validation.ValidateContext(&req.Context); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "context", Message: err.Error()},
		})
		return
	}

	// Create the context
	err = h.contextStore.Create(r.Context(), &req.Context, customer.CustomerID)
	if err != nil {
		if err == storage.ErrConflict {
			writeErrorResponse(w, "Context already exists", http.StatusConflict)
			return
		}
		writeErrorResponse(w, "Failed to create context", http.StatusInternalServerError)
		return
	}

	response := api.CreateContextResponse{
		Success:     true,
		Message:     "Context created successfully",
		ContextName: req.Context.Metadata.Name,
		CreatedAt:   req.Context.Metadata.CreatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusCreated)
}

// GetContext handles GET /contexts/{name}
func (h *ContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contextName := mux.Vars(r)["name"]
	if contextName == "" {
		writeErrorResponse(w, "Context name is required", http.StatusBadRequest)
		return
	}

	context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Context not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to get context", http.StatusInternalServerError)
		return
	}

	// Fetch manifests from Git repository using ArgoCD Application data
	var manifests []api.KubernetesManifest
	if h.argoCDClient != nil && h.gitClient != nil {
		manifestFiles, err := h.fetchManifestsForContext(r.Context(), contextName, customer.CustomerID)
		if err == nil {
			// Convert git.ManifestFile to api.KubernetesManifest
			for _, mf := range manifestFiles {
				manifests = append(manifests, api.KubernetesManifest{
					Path:     mf.Path,
					Content:  mf.Content,
					Filename: mf.Filename,
				})
			}
		}
		// Note: We silently ignore errors fetching manifests to not break the main context response
	}

	response := api.GetContextResponse{
		Success:   true,
		Context:   *context,
		Manifests: manifests,
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// fetchManifestsForContext fetches Kubernetes manifests from Git repository based on ArgoCD Application configuration
func (h *ContextHandler) fetchManifestsForContext(ctx context.Context, contextName, customerID string) ([]git.ManifestFile, error) {
	// Find the ArgoCD Application for this context
	// Context names should match ArgoCD Application names (e.g., "demo-app-dev")
	app, err := h.argoCDClient.GetApplicationByName(contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to get ArgoCD Application for context %s: %w", contextName, err)
	}

	// Extract Git repository information from the ArgoCD Application
	repoURL := app.Spec.Source.RepoURL
	path := app.Spec.Source.Path
	branch := app.Spec.Source.TargetRevision

	if repoURL == "" || path == "" || branch == "" {
		return nil, fmt.Errorf("incomplete ArgoCD Application source configuration: repo=%s, path=%s, branch=%s", repoURL, path, branch)
	}

	// Fetch manifests from the Git repository
	manifests, err := h.gitClient.FetchManifests(ctx, repoURL, branch, path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifests from %s@%s:%s: %w", repoURL, branch, path, err)
	}

	return manifests, nil
}

// ListContexts handles GET /contexts
func (h *ContextHandler) ListContexts(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contexts, err := h.contextStore.List(r.Context(), customer.CustomerID)
	if err != nil {
		writeErrorResponse(w, "Failed to list contexts", http.StatusInternalServerError)
		return
	}

	// Convert pointers to values
	contextValues := make([]models.Context, len(contexts))
	for i, context := range contexts {
		contextValues[i] = *context
	}

	response := api.ListContextsResponse{
		Success:  true,
		Contexts: contextValues,
		Count:    len(contexts),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// UpdateContext handles PUT /contexts/{name}
func (h *ContextHandler) UpdateContext(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contextName := mux.Vars(r)["name"]
	if contextName == "" {
		writeErrorResponse(w, "Context name is required", http.StatusBadRequest)
		return
	}

	var req api.UpdateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the name in the URL matches the name in the request
	if req.Context.Metadata.Name != contextName {
		writeErrorResponse(w, "Context name in URL does not match request body", http.StatusBadRequest)
		return
	}

	// Validate the context manifest
	if err := validation.ValidateContext(&req.Context); err != nil {
		writeValidationErrorResponse(w, "Validation failed", []api.ValidationError{
			{Field: "context", Message: err.Error()},
		})
		return
	}

	// Update the context
	err = h.contextStore.Update(r.Context(), &req.Context, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Context not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to update context", http.StatusInternalServerError)
		return
	}

	response := api.UpdateContextResponse{
		Success:   true,
		Message:   "Context updated successfully",
		UpdatedAt: req.Context.Metadata.UpdatedAt.Format(time.RFC3339),
	}

	writeJSONResponse(w, response, http.StatusOK)
}

// DeleteContext handles DELETE /contexts/{name}
func (h *ContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.RequireCustomer(r.Context())
	if err != nil {
		writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contextName := mux.Vars(r)["name"]
	if contextName == "" {
		writeErrorResponse(w, "Context name is required", http.StatusBadRequest)
		return
	}

	err = h.contextStore.Delete(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			writeErrorResponse(w, "Context not found", http.StatusNotFound)
			return
		}
		writeErrorResponse(w, "Failed to delete context", http.StatusInternalServerError)
		return
	}

	response := api.DeleteResponse{
		Success: true,
		Message: "Context deleted successfully",
	}

	writeJSONResponse(w, response, http.StatusOK)
}

