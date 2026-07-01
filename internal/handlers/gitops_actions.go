package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kriipke/platformctl/internal/auth"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/storage"
)

type GitOpsActionHandler struct {
	appStore         *storage.AppStore
	environmentStore *storage.EnvironmentStore
	contextStore     *storage.ContextStore
	publisher        *events.GitOpsCommandPublisher
}

func NewGitOpsActionHandler(appStore *storage.AppStore, envStore *storage.EnvironmentStore, contextStore *storage.ContextStore, pub *events.GitOpsCommandPublisher) *GitOpsActionHandler {
	return &GitOpsActionHandler{
		appStore:         appStore,
		environmentStore: envStore,
		contextStore:     contextStore,
		publisher:        pub,
	}
}

type GitOpsActionResponse struct {
	Success              bool     `json:"success"`
	CorrelationID        string   `json:"correlation_id"`
	Message              string   `json:"message"`
	Action               string   `json:"action"`
	ManifestType         string   `json:"manifest_type"`
	CustomerID           string   `json:"customer_id"`
	AppNames             []string `json:"app_names,omitempty"`
	EnvironmentNames     []string `json:"environment_names,omitempty"`
	ContextPairings      []string `json:"context_pairings,omitempty"`
	ApplicationSets      []string `json:"applicationsets,omitempty"`
	VaultSources         []string `json:"vault_sources,omitempty"`
	ClusterConfigs       []string `json:"cluster_configs,omitempty"`
}

// App manifest synchronization endpoint
func (h *GitOpsActionHandler) HandleSyncApps(w http.ResponseWriter, r *http.Request) {
	contextName := mux.Vars(r)["name"]
	customer, ok := auth.GetCustomerFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
		return
	}

	// Verify Context exists with customer isolation
	context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Context not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
		return
	}

	// Get App manifest referenced by Context
	app, err := h.appStore.Get(r.Context(), context.Spec.AppRef, customer.CustomerID)
	if err != nil {
		http.Error(w, "Failed to get App manifest", http.StatusInternalServerError)
		return
	}

	var correlationIDs []string
	var applicationSetNames []string

	// Publish App sync command
	cmd, err := h.publisher.PublishAppSync(customer.CustomerID, contextName, app.Metadata.Name, customer.Username)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish App sync command for %s", app.Metadata.Name), http.StatusInternalServerError)
		return
	}
	correlationIDs = append(correlationIDs, cmd.CorrelationID)

	// Collect ApplicationSet names from App manifest
	for _, appSet := range app.Spec.ArgoCD.ApplicationSets {
		applicationSetNames = append(applicationSetNames, appSet.Name)
	}

	response := GitOpsActionResponse{
		Success:         true,
		CorrelationID:   strings.Join(correlationIDs, ","),
		Message:         fmt.Sprintf("App sync command published successfully for %s with %d ApplicationSets", app.Metadata.Name, len(applicationSetNames)),
		Action:          "sync-apps",
		ManifestType:    "app",
		CustomerID:      customer.CustomerID,
		AppNames:        []string{app.Metadata.Name},
		ApplicationSets: applicationSetNames,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Environment manifest validation endpoint
func (h *GitOpsActionHandler) HandleValidateEnvironments(w http.ResponseWriter, r *http.Request) {
	contextName := mux.Vars(r)["name"]
	customer, ok := auth.GetCustomerFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
		return
	}

	// Verify Context exists with customer isolation
	context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Context not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
		return
	}

	// Get Environment manifest referenced by Context
	environment, err := h.environmentStore.Get(r.Context(), context.Spec.Deployments[0].EnvironmentRef, customer.CustomerID)
	if err != nil {
		http.Error(w, "Failed to get Environment manifest", http.StatusInternalServerError)
		return
	}

	var correlationIDs []string
	var vaultSources []string
	var clusterConfigs []string

	// Publish Environment validation command
	cmd, err := h.publisher.PublishEnvironmentValidation(customer.CustomerID, contextName, environment.Metadata.Name, customer.Username)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish Environment validation command for %s", environment.Metadata.Name), http.StatusInternalServerError)
		return
	}
	correlationIDs = append(correlationIDs, cmd.CorrelationID)

	// Collect Vault sources from Environment manifest
	for sourceName, vaultSource := range environment.Spec.Datasources {
		vaultSources = append(vaultSources, fmt.Sprintf("%s:%s", sourceName, vaultSource.Vault))
	}

	// Collect cluster configs from Environment manifest
	clusterConfigs = append(clusterConfigs, environment.Spec.Environment.Cluster.KubeconfigSecretRef.Vault)

	response := GitOpsActionResponse{
		Success:          true,
		CorrelationID:    strings.Join(correlationIDs, ","),
		Message:          fmt.Sprintf("Environment validation command published successfully for %s with %d vault sources and %d clusters", environment.Metadata.Name, len(vaultSources), len(clusterConfigs)),
		Action:           "validate-environments",
		ManifestType:     "environment",
		CustomerID:       customer.CustomerID,
		EnvironmentNames: []string{environment.Metadata.Name},
		VaultSources:     vaultSources,
		ClusterConfigs:   clusterConfigs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Context pairing correlation endpoint
func (h *GitOpsActionHandler) HandleCorrelateContexts(w http.ResponseWriter, r *http.Request) {
	contextName := mux.Vars(r)["name"]
	customer, ok := auth.GetCustomerFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
		return
	}

	// Verify Context exists with customer isolation
	context, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Context not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
		return
	}

	// Publish Context pairing correlation command
	cmd, err := h.publisher.PublishContextCorrelation(customer.CustomerID, contextName, customer.Username, context.Spec.AppRef, context.Spec.Deployments[0].EnvironmentRef)
	if err != nil {
		http.Error(w, "Failed to publish Context correlation command", http.StatusInternalServerError)
		return
	}

	contextPairingDescription := fmt.Sprintf("%s+%s", context.Spec.AppRef, context.Spec.Deployments[0].EnvironmentRef)

	response := GitOpsActionResponse{
		Success:         true,
		CorrelationID:   cmd.CorrelationID,
		Message:         fmt.Sprintf("Context correlation command published successfully for pairing: %s", contextPairingDescription),
		Action:          "correlate-contexts",
		ManifestType:    "context",
		CustomerID:      customer.CustomerID,
		ContextPairings: []string{contextPairingDescription},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Multi-environment correlation endpoint - routes work to the multi-environment
// Kubernetes service (cmd.kubernetes.*) for cross-environment workload status.
func (h *GitOpsActionHandler) HandleCorrelateMultiEnvironment(w http.ResponseWriter, r *http.Request) {
	contextName := mux.Vars(r)["name"]
	customer, ok := auth.GetCustomerFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
		return
	}

	// Verify Context exists with customer isolation
	_, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Context not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
		return
	}

	// Publish multi-environment correlation command
	cmd, err := h.publisher.PublishMultiEnvironmentCorrelation(customer.CustomerID, contextName, customer.Username)
	if err != nil {
		http.Error(w, "Failed to publish multi-environment correlation command", http.StatusInternalServerError)
		return
	}

	response := GitOpsActionResponse{
		Success:       true,
		CorrelationID: cmd.CorrelationID,
		Message:       "Multi-environment correlation command published successfully",
		Action:        "correlate-multi-environment",
		ManifestType:  "kubernetes",
		CustomerID:    customer.CustomerID,
	}

	w.Header().Set("Content-Type", "application/json")
	// Response headers are already committed; nothing actionable remains if the
	// client disconnects mid-encode, so the error is explicitly discarded.
	_ = json.NewEncoder(w).Encode(response)
}

// Manifest inspection endpoint for detailed analysis
func (h *GitOpsActionHandler) HandleInspectManifests(w http.ResponseWriter, r *http.Request) {
	contextName := mux.Vars(r)["name"]
	customer, ok := auth.GetCustomerFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
		return
	}

	manifestType := r.URL.Query().Get("type") // app, environment, context, all

	if manifestType == "" {
		manifestType = "all"
	}

	// Verify Context exists with customer isolation
	_, err := h.contextStore.Get(r.Context(), contextName, customer.CustomerID)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Context not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to verify Context", http.StatusInternalServerError)
		return
	}

	// Publish manifest inspection command
	cmd, err := h.publisher.PublishManifestInspection(customer.CustomerID, contextName, customer.Username, manifestType)
	if err != nil {
		http.Error(w, "Failed to publish manifest inspection command", http.StatusInternalServerError)
		return
	}

	response := GitOpsActionResponse{
		Success:      true,
		CorrelationID: cmd.CorrelationID,
		Message:      fmt.Sprintf("Manifest inspection command published successfully (type: %s)", manifestType),
		Action:       "inspect-manifests",
		ManifestType: manifestType,
		CustomerID:   customer.CustomerID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}