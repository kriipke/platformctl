package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kriipke/platformctl/internal/clients/argocd"
	"github.com/kriipke/platformctl/internal/clients/git"
	"github.com/kriipke/platformctl/internal/clients/kubernetes"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/pkg/api"
)

type ContextPairingHandler struct {
	appValidator         AppValidator
	environmentValidator EnvironmentValidator
	syncMonitor          SyncMonitor
	gitClient            git.GitClient
	kubernetesClient     kubernetes.KubernetesClient
	argoCDClient         argocd.ArgoCDClient
}

type AppValidator interface {
	ValidateAppReference(customerID, appName string) (*AppValidationResult, error)
}

type EnvironmentValidator interface {
	ValidateEnvironmentReference(customerID, environmentName string) (*EnvironmentValidationResult, error)
}

type SyncMonitor interface {
	MonitorPairingSyncStatus(customerID, contextName, appName, environmentName string) (*PairingSyncResult, error)
}

type AppValidationResult struct {
	AppName          string `json:"app_name"`
	ValidationStatus string `json:"validation_status"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

type EnvironmentValidationResult struct {
	EnvironmentName  string `json:"environment_name"`
	ValidationStatus string `json:"validation_status"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

type PairingSyncResult struct {
	AppName          string    `json:"app_name"`
	EnvironmentName  string    `json:"environment_name"`
	PairingStatus    string    `json:"pairing_status"`
	SyncStatus       string    `json:"sync_status"`
	LastSyncTime     time.Time `json:"last_sync_time"`
	ResourceCount    int       `json:"resource_count"`
	ValidationErrors []string  `json:"validation_errors"`
}

func NewContextPairingHandler(cfg *config.Config) *ContextPairingHandler {
	return &ContextPairingHandler{
		appValidator:         &AppValidatorImpl{},
		environmentValidator: &EnvironmentValidatorImpl{},
		syncMonitor:          &SyncMonitorImpl{},
		gitClient:            git.NewGitClient(),
		kubernetesClient:     kubernetes.NewMultiClusterClient(cfg),
		argoCDClient:         argocd.NewArgoCDClient(cfg.ArgoCD),
	}
}

func (cph *ContextPairingHandler) HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
	startTime := time.Now()

	switch cmd.Action {
	case "correlate-context":
		return cph.handleContextCorrelation(cmd, startTime)
	case "inspect-manifests":
		return cph.handleContextInspection(cmd, startTime)
	default:
		return cph.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
	}
}

func (cph *ContextPairingHandler) GetSupportedManifestTypes() []string {
	return []string{"context"}
}

func (cph *ContextPairingHandler) GetSupportedActions() []string {
	return []string{"correlate-context", "inspect-manifests"}
}

func (cph *ContextPairingHandler) handleContextCorrelation(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Get App and Environment references from metadata
	appReference := cmd.ManifestMetadata.AppReference
	environmentReference := cmd.ManifestMetadata.EnvironmentReference

	if appReference == "" || environmentReference == "" {
		return cph.errorResult(cmd, "missing references", fmt.Errorf("app_reference or environment_reference not provided"), startTime)
	}

	// Validate App reference
	appValidation, err := cph.appValidator.ValidateAppReference(cmd.CustomerID, appReference)
	if err != nil {
		return cph.errorResult(cmd, "app validation failed", err, startTime)
	}

	// Validate Environment reference
	envValidation, err := cph.environmentValidator.ValidateEnvironmentReference(cmd.CustomerID, environmentReference)
	if err != nil {
		return cph.errorResult(cmd, "environment validation failed", err, startTime)
	}

	// Monitor pairing synchronization status
	syncResult, err := cph.syncMonitor.MonitorPairingSyncStatus(cmd.CustomerID, cmd.ContextName, appReference, environmentReference)
	if err != nil {
		return cph.errorResult(cmd, "sync monitoring failed", err, startTime)
	}

	// Determine overall pairing status
	pairingStatus := "valid"
	if appValidation.ValidationStatus != "valid" || envValidation.ValidationStatus != "valid" {
		pairingStatus = "invalid"
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
			ManifestType:     "context",
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "context-correlation",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		ContextPairingData: &api.ContextPairingResult{
			ContextName:          cmd.ContextName,
			AppReference:         appReference,
			EnvironmentReference: environmentReference,
			PairingStatus:        pairingStatus,
			SyncStatus:           syncResult.SyncStatus,
			HealthStatus:         "healthy",
			CorrelationData: map[string]interface{}{
				"app_validation":         appValidation,
				"environment_validation": envValidation,
				"sync_result":            syncResult,
			},
			ResourceCount:      syncResult.ResourceCount,
			LastDeploymentTime: &syncResult.LastSyncTime,
			ValidationErrors:   syncResult.ValidationErrors,
		},
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    3, // app validation, env validation, sync monitoring
		},
	}

	result.Payload["app_reference"] = appReference
	result.Payload["environment_reference"] = environmentReference
	result.Payload["pairing_status"] = pairingStatus
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (cph *ContextPairingHandler) handleContextInspection(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Basic context inspection - get manifest metadata
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    cmd.CorrelationID,
			CustomerID:       cmd.CustomerID,
			ContextName:      cmd.ContextName,
			Action:           cmd.Action,
			ManifestType:     "context",
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "context-correlation",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    1,
		},
	}

	result.Payload["inspection_type"] = "context_manifest"
	result.Payload["manifest_type"] = "context"
	result.Payload["deep_inspection"] = cmd.Payload["deep_inspection"]
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (cph *ContextPairingHandler) errorResult(cmd *api.GitOpsCommandMessage, message string, err error, startTime time.Time) (*api.GitOpsResultMessage, error) {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    cmd.CorrelationID,
			CustomerID:       cmd.CustomerID,
			ContextName:      cmd.ContextName,
			Action:           cmd.Action,
			ManifestType:     cmd.ManifestType,
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName:  "context-correlation",
		Status:       "error",
		ErrorMessage: fmt.Sprintf("%s: %v", message, err),
		CompletedAt:  time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    0,
		},
	}, nil
}

// Implementation of validators
type AppValidatorImpl struct{}

func (av *AppValidatorImpl) ValidateAppReference(customerID, appName string) (*AppValidationResult, error) {
	// For Phase 1C, simulate app reference validation
	return &AppValidationResult{
		AppName:          appName,
		ValidationStatus: "valid",
	}, nil
}

type EnvironmentValidatorImpl struct{}

func (ev *EnvironmentValidatorImpl) ValidateEnvironmentReference(customerID, environmentName string) (*EnvironmentValidationResult, error) {
	// For Phase 1C, simulate environment reference validation
	return &EnvironmentValidationResult{
		EnvironmentName:  environmentName,
		ValidationStatus: "valid",
	}, nil
}

type SyncMonitorImpl struct{}

func (sm *SyncMonitorImpl) MonitorPairingSyncStatus(customerID, contextName, appName, environmentName string) (*PairingSyncResult, error) {
	// For Phase 1C, simulate sync monitoring
	return &PairingSyncResult{
		AppName:          appName,
		EnvironmentName:  environmentName,
		PairingStatus:    "valid",
		SyncStatus:       "synced",
		LastSyncTime:     time.Now().Add(-30 * time.Minute),
		ResourceCount:    5,
		ValidationErrors: []string{},
	}, nil
}

func generateUUID() string {
	return uuid.New().String()
}
