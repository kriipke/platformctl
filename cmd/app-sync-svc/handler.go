package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kriipke/platformctl/internal/clients/argocd"
	"github.com/kriipke/platformctl/internal/clients/git"
	"github.com/kriipke/platformctl/internal/clients/helm"
	"github.com/kriipke/platformctl/internal/clients/kubernetes"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/pkg/api"
)

type AppSyncHandler struct {
	argoCDClient     argocd.ArgoCDClient
	helmClient       helm.HelmClient
	gitClient        git.GitClient
	kubernetesClient kubernetes.KubernetesClient
}

func NewAppSyncHandler(cfg *config.Config) *AppSyncHandler {
	return &AppSyncHandler{
		argoCDClient:     argocd.NewArgoCDClient(cfg.ArgoCD),
		helmClient:       helm.NewHelmClient(cfg.Helm),
		gitClient:        git.NewGitClient(),
		kubernetesClient: kubernetes.NewMultiClusterClient(cfg),
	}
}

func (ash *AppSyncHandler) HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
	startTime := time.Now()

	switch cmd.Action {
	case "sync-app":
		return ash.handleAppSync(cmd, startTime)
	case "inspect-manifests":
		return ash.handleAppInspection(cmd, startTime)
	default:
		return ash.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
	}
}

func (ash *AppSyncHandler) GetSupportedManifestTypes() []string {
	return []string{"app"}
}

func (ash *AppSyncHandler) GetSupportedActions() []string {
	return []string{"sync-app", "inspect-manifests"}
}

func (ash *AppSyncHandler) handleAppSync(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Validate Helm sources
	helmValidations, err := ash.helmClient.ValidateHelmSources(cmd.CustomerID, cmd.AppName)
	if err != nil {
		return ash.errorResult(cmd, "helm source validation failed", err, startTime)
	}

	// Validate Git sources
	gitValidations, err := ash.gitClient.ValidateGitSources(cmd.CustomerID, cmd.AppName)
	if err != nil {
		return ash.errorResult(cmd, "git source validation failed", err, startTime)
	}

	// Get ApplicationSet statuses
	appSetStatuses, err := ash.argoCDClient.GetApplicationSetsForApp(cmd.CustomerID, cmd.AppName)
	if err != nil {
		return ash.errorResult(cmd, "applicationset status failed", err, startTime)
	}

	// Trigger ApplicationSet sync if requested
	var syncResults []api.ApplicationSetSyncResult
	if forceSync, ok := cmd.Payload["sync_applicationset"].(bool); ok && forceSync {
		for _, appSet := range appSetStatuses {
			syncResult, err := ash.argoCDClient.SyncApplicationSet(cmd.CustomerID, appSet.Name, true)
			if err != nil {
				return ash.errorResult(cmd, fmt.Sprintf("sync failed for ApplicationSet %s", appSet.Name), err, startTime)
			}
			syncResults = append(syncResults, *syncResult)
		}
	}

	// Create result
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    cmd.CorrelationID,
			CustomerID:       cmd.CustomerID,
			ContextName:      cmd.ContextName,
			Action:           cmd.Action,
			ManifestType:     "app",
			AppName:          cmd.AppName,
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "app-sync",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		AppManifestData: &api.AppManifestResult{
			AppName:            cmd.AppName,
			ApplicationSetName: "", // Will be set if we have ApplicationSets
			Namespace:          fmt.Sprintf("%s-%s", cmd.CustomerID, "apps"),
			SyncStatus:         "synced",
			HealthStatus:       "healthy",
			HelmSources:        helmValidations,
			GitSources:         gitValidations,
			Applications:       []api.ApplicationStatus{},
			LastSyncTime:       &startTime,
			Generator: api.ApplicationSetGenerator{
				Type:       "git",
				Parameters: make(map[string]interface{}),
			},
		},
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    len(helmValidations) + len(gitValidations) + len(appSetStatuses),
		},
	}

	// Set ApplicationSet name if we have any
	if len(appSetStatuses) > 0 {
		result.AppManifestData.ApplicationSetName = appSetStatuses[0].Name
		result.AppManifestData.Namespace = appSetStatuses[0].Namespace

		// Convert to ApplicationStatus
		for _, appSet := range appSetStatuses {
			for _, app := range appSet.Applications {
				result.AppManifestData.Applications = append(result.AppManifestData.Applications, api.ApplicationStatus{
					Name:         app.Name,
					Environment:  app.Environment,
					Cluster:      app.Cluster,
					Namespace:    app.Namespace,
					SyncStatus:   app.SyncStatus,
					HealthStatus: app.HealthStatus,
					LastDeployed: app.LastDeployed,
					HelmRevision: app.HelmRevision,
				})
			}
		}
	}

	// Add sync results if any
	if len(syncResults) > 0 {
		result.Payload["sync_results"] = syncResults
	}

	return result, nil
}

func (ash *AppSyncHandler) handleAppInspection(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Basic app inspection - get manifest metadata
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    cmd.CorrelationID,
			CustomerID:       cmd.CustomerID,
			ContextName:      cmd.ContextName,
			Action:           cmd.Action,
			ManifestType:     "app",
			AppName:          cmd.AppName,
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "app-sync",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    1,
		},
	}

	result.Payload["inspection_type"] = "app_manifest"
	result.Payload["manifest_type"] = "app"
	result.Payload["deep_inspection"] = cmd.Payload["deep_inspection"]
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (ash *AppSyncHandler) errorResult(cmd *api.GitOpsCommandMessage, message string, err error, startTime time.Time) (*api.GitOpsResultMessage, error) {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    cmd.CorrelationID,
			CustomerID:       cmd.CustomerID,
			ContextName:      cmd.ContextName,
			Action:           cmd.Action,
			ManifestType:     cmd.ManifestType,
			AppName:          cmd.AppName,
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName:  "app-sync",
		Status:       "error",
		ErrorMessage: fmt.Sprintf("%s: %v", message, err),
		CompletedAt:  time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    0,
		},
	}, nil
}

func generateUUID() string {
	return uuid.New().String()
}
