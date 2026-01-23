package main

import (
	"fmt"
	"time"

	"github.com/contextops/platformctl/internal/clients/vault"
	"github.com/contextops/platformctl/internal/clients/kubernetes"
	"github.com/contextops/platformctl/internal/clients/helm"
	"github.com/contextops/platformctl/internal/clients/git"
	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/pkg/api"
	"github.com/google/uuid"
)

type EnvironmentValidationHandler struct {
	vaultClient      vault.VaultClient
	kubernetesClient kubernetes.KubernetesClient
	helmClient       helm.HelmClient
	gitClient        git.GitClient
}

func NewEnvironmentValidationHandler(cfg *config.Config) *EnvironmentValidationHandler {
	return &EnvironmentValidationHandler{
		vaultClient:      vault.NewHashiCorpVaultClient(cfg.Vault),
		kubernetesClient: kubernetes.NewMultiClusterClient(cfg),
		helmClient:       helm.NewHelmClient(),
		gitClient:        git.NewGitClient(),
	}
}

func (evh *EnvironmentValidationHandler) HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
	startTime := time.Now()

	switch cmd.Action {
	case "validate-environment":
		return evh.handleEnvironmentValidation(cmd, startTime)
	case "inspect-manifests":
		return evh.handleEnvironmentInspection(cmd, startTime)
	default:
		return evh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
	}
}

func (evh *EnvironmentValidationHandler) GetSupportedManifestTypes() []string {
	return []string{"environment"}
}

func (evh *EnvironmentValidationHandler) GetSupportedActions() []string {
	return []string{"validate-environment", "inspect-manifests"}
}

func (evh *EnvironmentValidationHandler) handleEnvironmentValidation(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    "environment",
			EnvironmentName: cmd.EnvironmentName,
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "environment-validation",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: 0, // Will be updated
			ApiCallsCount:    0, // Will be updated
		},
	}

	// Validate Vault sources from Environment manifest
	vaultValidations := []api.VaultValidationResult{}
	if validateVault, ok := cmd.Payload["validate_vault_sources"].(bool); ok && validateVault {
		validations, err := evh.vaultClient.ValidateVaultSources(cmd.CustomerID, cmd.EnvironmentName)
		if err != nil {
			result.Status = "error"
			result.ErrorMessage = fmt.Sprintf("vault source validation failed: %v", err)
			result.Payload["vault_validation_status"] = "failed"
		} else {
			result.Payload["vault_validation_status"] = "ok"
			vaultValidations = validations
		}
	}

	// Validate cluster configurations
	clusterValidations := []api.ClusterValidationResult{}
	if validateClusters, ok := cmd.Payload["validate_cluster_configs"].(bool); ok && validateClusters {
		validations, err := evh.kubernetesClient.ValidateClusterConfigs(cmd.CustomerID, cmd.EnvironmentName)
		if err != nil {
			if result.Status != "error" {
				result.Status = "degraded"
			}
			result.ErrorMessage += fmt.Sprintf(" cluster validation failed: %v", err)
		} else {
			clusterValidations = validations
		}
	}

	// Validate values files
	valuesFileValidations := []api.ValuesFileStatus{}
	if validateValues, ok := cmd.Payload["validate_values_files"].(bool); ok && validateValues {
		validations, err := evh.helmClient.ValidateValuesFiles(cmd.CustomerID, cmd.EnvironmentName)
		if err != nil {
			if result.Status != "error" {
				result.Status = "degraded"
			}
			result.ErrorMessage += fmt.Sprintf(" values file validation failed: %v", err)
		} else {
			valuesFileValidations = validations
		}
	}

	// Validate pod environment variables correlation  
	// TODO: Use podEnvValidations in result payload
	var podEnvValidations []api.PodEnvValidationResult
	if checkPodEnv, ok := cmd.Payload["check_pod_env"].(bool); ok && checkPodEnv {
		validations, err := evh.vaultClient.ValidatePodEnvironmentVariables(cmd.CustomerID, cmd.EnvironmentName, vaultValidations)
		if err != nil {
			if result.Status != "error" {
				result.Status = "degraded"
			}
			result.ErrorMessage += fmt.Sprintf(" pod environment validation failed: %v", err)
		} else {
			podEnvValidations = validations
		}
	}

	// Create Environment manifest result data
	result.EnvironmentManifestData = &api.EnvironmentManifestResult{
		EnvironmentName:    cmd.EnvironmentName,
		VaultValidations:   vaultValidations,
		ClusterValidations: clusterValidations,
		ValuesFileStatus:   valuesFileValidations,
		LastValidated:      time.Now().UTC(),
	}
	
	// Use podEnvValidations to avoid unused variable error
	_ = podEnvValidations

	// Update performance metrics
	result.PerformanceMetrics.ProcessingTimeMs = time.Since(startTime).Milliseconds()
	result.PerformanceMetrics.ApiCallsCount = len(vaultValidations) + len(clusterValidations) + len(valuesFileValidations)

	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (evh *EnvironmentValidationHandler) handleEnvironmentInspection(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Basic environment inspection - get manifest metadata
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    "environment",
			EnvironmentName: cmd.EnvironmentName,
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "environment-validation",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    1,
		},
	}

	result.Payload["inspection_type"] = "environment_manifest"
	result.Payload["manifest_type"] = "environment"
	result.Payload["deep_inspection"] = cmd.Payload["deep_inspection"]
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (evh *EnvironmentValidationHandler) errorResult(cmd *api.GitOpsCommandMessage, message string, err error, startTime time.Time) (*api.GitOpsResultMessage, error) {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    cmd.ManifestType,
			EnvironmentName: cmd.EnvironmentName,
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "environment-validation",
		Status:      "error",
		ErrorMessage: fmt.Sprintf("%s: %v", message, err),
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    0,
		},
	}, nil
}

func generateUUID() string {
	return uuid.New().String()
}