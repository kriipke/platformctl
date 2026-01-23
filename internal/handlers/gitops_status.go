package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/contextops/platformctl/internal/models"
	"github.com/contextops/platformctl/internal/readmodel"
)

// GitOpsStatusHandler provides GitOps status API endpoints
type GitOpsStatusHandler struct {
	store  *readmodel.GitOpsStore
	logger zerolog.Logger
}

// NewGitOpsStatusHandler creates a new GitOps status handler
func NewGitOpsStatusHandler(store *readmodel.GitOpsStore, logger zerolog.Logger) *GitOpsStatusHandler {
	return &GitOpsStatusHandler{
		store:  store,
		logger: logger.With().Str("component", "gitops-status-handler").Logger(),
	}
}

// GetContextStatus handles GET /gitops/contexts/{contextName}/status
func (h *GitOpsStatusHandler) GetContextStatus(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")

	if contextName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name is required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Msg("Getting context status")

	status, err := h.store.GetContextStatus(c.Request.Context(), customerData.Name, contextName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Context status not found"})
			return
		}
		h.logger.Error().Err(err).Str("context_name", contextName).Msg("Failed to get context status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListContextStatuses handles GET /gitops/contexts/status
func (h *GitOpsStatusHandler) ListContextStatuses(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)

	// Optional health status filter
	healthStatusParam := c.Query("health_status")
	var healthStatuses []string
	if healthStatusParam != "" {
		healthStatuses = strings.Split(healthStatusParam, ",")
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Strs("health_statuses", healthStatuses).
		Msg("Listing context statuses")

	var statuses []readmodel.ContextStatus
	var err error

	if len(healthStatuses) > 0 {
		statuses, err = h.store.GetContextsByHealthStatus(c.Request.Context(), customerData.Name, healthStatuses)
	} else {
		statuses, err = h.store.ListContextStatuses(c.Request.Context(), customerData.ID.String())
	}

	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list context statuses")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contexts": statuses,
		"count":    len(statuses),
	})
}

// GetAppManifestStatus handles GET /gitops/contexts/{contextName}/apps/{appName}/status
func (h *GitOpsStatusHandler) GetAppManifestStatus(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")
	appName := c.Param("appName")

	if contextName == "" || appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name and app name are required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Str("app_name", appName).
		Msg("Getting app manifest status")

	status, err := h.store.GetAppManifestStatus(c.Request.Context(), customerData.Name, contextName, appName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "App manifest status not found"})
			return
		}
		h.logger.Error().Err(err).
			Str("context_name", contextName).
			Str("app_name", appName).
			Msg("Failed to get app manifest status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetEnvironmentManifestStatus handles GET /gitops/contexts/{contextName}/environments/{environmentName}/status
func (h *GitOpsStatusHandler) GetEnvironmentManifestStatus(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")
	environmentName := c.Param("environmentName")

	if contextName == "" || environmentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name and environment name are required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Str("environment_name", environmentName).
		Msg("Getting environment manifest status")

	status, err := h.store.GetEnvironmentManifestStatus(c.Request.Context(), customerData.Name, contextName, environmentName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Environment manifest status not found"})
			return
		}
		h.logger.Error().Err(err).
			Str("context_name", contextName).
			Str("environment_name", environmentName).
			Msg("Failed to get environment manifest status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetMultiEnvironmentAppStatus handles GET /gitops/contexts/{contextName}/apps/{appName}/environments/status
func (h *GitOpsStatusHandler) GetMultiEnvironmentAppStatus(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")
	appName := c.Param("appName")

	if contextName == "" || appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name and app name are required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Str("app_name", appName).
		Msg("Getting multi-environment app status")

	statuses, err := h.store.GetMultiEnvironmentAppStatus(c.Request.Context(), customerData.Name, contextName, appName)
	if err != nil {
		h.logger.Error().Err(err).
			Str("context_name", contextName).
			Str("app_name", appName).
			Msg("Failed to get multi-environment app status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environments": statuses,
		"count":        len(statuses),
	})
}

// GetVaultValidationDetails handles GET /gitops/contexts/{contextName}/environments/{environmentName}/vault/status
func (h *GitOpsStatusHandler) GetVaultValidationDetails(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")
	environmentName := c.Param("environmentName")

	if contextName == "" || environmentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name and environment name are required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Str("environment_name", environmentName).
		Msg("Getting vault validation details")

	validations, err := h.store.GetVaultValidationDetails(c.Request.Context(), customerData.Name, contextName, environmentName)
	if err != nil {
		h.logger.Error().Err(err).
			Str("context_name", contextName).
			Str("environment_name", environmentName).
			Msg("Failed to get vault validation details")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"validations": validations,
		"count":       len(validations),
	})
}

// GetContextHealth handles GET /gitops/contexts/{contextName}/health  
func (h *GitOpsStatusHandler) GetContextHealth(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)
	contextName := c.Param("contextName")

	if contextName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Context name is required"})
		return
	}

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Str("context_name", contextName).
		Msg("Getting context health summary")

	// Get overall context status
	contextStatus, err := h.store.GetContextStatus(c.Request.Context(), customerData.ID.String(), contextName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Context not found"})
			return
		}
		h.logger.Error().Err(err).Str("context_name", contextName).Msg("Failed to get context status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Get app manifest status
	appStatus, err := h.store.GetAppManifestStatus(c.Request.Context(), customerData.Name, contextName, contextStatus.AppReference)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		h.logger.Error().Err(err).Msg("Failed to get app manifest status")
		// Don't return error, just log it
	}

	// Get environment manifest status
	envStatus, err := h.store.GetEnvironmentManifestStatus(c.Request.Context(), customerData.Name, contextName, contextStatus.EnvironmentReference)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		h.logger.Error().Err(err).Msg("Failed to get environment manifest status")
		// Don't return error, just log it
	}

	// Build health summary
	healthSummary := gin.H{
		"context_name":    contextName,
		"pairing_status":  contextStatus.PairingStatus,
		"sync_status":     contextStatus.SyncStatus,
		"health_status":   contextStatus.HealthStatus,
		"resource_count":  contextStatus.ResourceCount,
		"last_updated":    contextStatus.LastUpdated,
	}

	if appStatus != nil {
		healthSummary["app_manifest"] = gin.H{
			"sync_status":   appStatus.SyncStatus,
			"health_status": appStatus.HealthStatus,
			"app_count":     appStatus.ApplicationCount,
		}
	}

	if envStatus != nil {
		healthSummary["environment_manifest"] = gin.H{
			"vault_status":   envStatus.VaultValidationStatus,
			"cluster_status": envStatus.ClusterValidationStatus,
			"values_status":  envStatus.ValuesFileStatus,
		}
	}

	c.JSON(http.StatusOK, healthSummary)
}

// GetSystemHealthOverview handles GET /gitops/health/overview
func (h *GitOpsStatusHandler) GetSystemHealthOverview(c *gin.Context) {
	customer, exists := c.Get("customer")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "models.Customer not found in request context"})
		return
	}

	customerData := customer.(*models.Customer)

	h.logger.Info().
		Str("customer_id", customerData.Name).
		Msg("Getting system health overview")

	// Get all context statuses
	statuses, err := h.store.ListContextStatuses(c.Request.Context(), customerData.Name)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list context statuses")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Calculate health summary
	healthCounts := map[string]int{
		"healthy":   0,
		"degraded":  0,
		"unhealthy": 0,
		"unknown":   0,
	}

	syncCounts := map[string]int{
		"synced":     0,
		"out_of_sync": 0,
		"syncing":    0,
		"failed":     0,
		"unknown":    0,
	}

	pairingCounts := map[string]int{
		"valid":               0,
		"invalid":             0,
		"missing_app":         0,
		"missing_environment": 0,
		"unknown":             0,
	}

	totalResources := 0
	activeContexts := 0

	for _, status := range statuses {
		if _, exists := healthCounts[status.HealthStatus]; exists {
			healthCounts[status.HealthStatus]++
		} else {
			healthCounts["unknown"]++
		}

		if _, exists := syncCounts[status.SyncStatus]; exists {
			syncCounts[status.SyncStatus]++
		} else {
			syncCounts["unknown"]++
		}

		if _, exists := pairingCounts[status.PairingStatus]; exists {
			pairingCounts[status.PairingStatus]++
		} else {
			pairingCounts["unknown"]++
		}

		totalResources += status.ResourceCount
		activeContexts++
	}

	overview := gin.H{
		"summary": gin.H{
			"total_contexts":   len(statuses),
			"active_contexts":  activeContexts,
			"total_resources":  totalResources,
		},
		"health_distribution":  healthCounts,
		"sync_distribution":    syncCounts,
		"pairing_distribution": pairingCounts,
		"timestamp":            statuses,  // Include timestamp from most recent status
	}

	c.JSON(http.StatusOK, overview)
}