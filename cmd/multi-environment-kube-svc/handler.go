package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kriipke/platformctl/internal/clients/kubernetes"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/pkg/api"
)

type MultiEnvironmentKubernetesHandler struct {
	kubernetesClients map[string]kubernetes.KubernetesClient
	environmentMapper EnvironmentMapper
}

type EnvironmentMapper interface {
	GetEnvironmentsForCustomer(customerID string) ([]string, error)
	MapEnvironmentToCluster(environment string) (string, error)
}

func NewMultiEnvironmentKubernetesHandler(cfg *config.Config) *MultiEnvironmentKubernetesHandler {
	return &MultiEnvironmentKubernetesHandler{
		kubernetesClients: map[string]kubernetes.KubernetesClient{
			"default": kubernetes.NewMultiClusterClient(cfg),
		},
		environmentMapper: &EnvironmentMapperImpl{},
	}
}

func (mekh *MultiEnvironmentKubernetesHandler) HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
	startTime := time.Now()

	switch cmd.Action {
	case "correlate-context":
		return mekh.handleMultiEnvironmentCorrelation(cmd, startTime)
	case "inspect-manifests":
		return mekh.handleMultiEnvironmentInspection(cmd, startTime)
	default:
		return mekh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
	}
}

func (mekh *MultiEnvironmentKubernetesHandler) GetSupportedManifestTypes() []string {
	return []string{"context", "environment", "app"}
}

func (mekh *MultiEnvironmentKubernetesHandler) GetSupportedActions() []string {
	return []string{"correlate-context", "inspect-manifests"}
}

func (mekh *MultiEnvironmentKubernetesHandler) handleMultiEnvironmentCorrelation(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Get environments for the customer
	environments, err := mekh.environmentMapper.GetEnvironmentsForCustomer(cmd.CustomerID)
	if err != nil {
		return mekh.errorResult(cmd, "failed to get environments", err, startTime)
	}

	// Get workload status for each environment
	environmentStatuses := make(map[string]*api.EnvironmentWorkloadStatus)
	for _, env := range environments {
		client := mekh.kubernetesClients["default"]
		namespace := fmt.Sprintf("%s-%s", cmd.CustomerID, env)

		status, err := client.GetWorkloadStatus(cmd.CustomerID, env, namespace)
		if err != nil {
			// Log error but continue with other environments
			continue
		}
		environmentStatuses[env] = status
	}

	// Get multi-environment comparison
	comparison, err := mekh.kubernetesClients["default"].GetMultiEnvironmentComparison(cmd.CustomerID, environments)
	if err != nil {
		return mekh.errorResult(cmd, "failed to get multi-environment comparison", err, startTime)
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
			ManifestType:     cmd.ManifestType,
			RequestedBy:      cmd.RequestedBy,
			RequestedAt:      cmd.RequestedAt,
			Priority:         cmd.Priority,
			Payload:          make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "multi-environment-kubernetes",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    len(environments) + 1, // One call per environment + comparison
		},
	}

	// Add multi-environment data to payload
	result.Payload["environments"] = environments
	result.Payload["environment_statuses"] = environmentStatuses
	result.Payload["comparison"] = comparison
	result.Payload["total_environments"] = len(environments)
	result.Payload["healthy_environments"] = mekh.countHealthyEnvironments(environmentStatuses)
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (mekh *MultiEnvironmentKubernetesHandler) handleMultiEnvironmentInspection(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Basic multi-environment inspection
	result := &api.GitOpsResultMessage{
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
		ServiceName: "multi-environment-kubernetes",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    1,
		},
	}

	result.Payload["inspection_type"] = "multi_environment_kubernetes"
	result.Payload["manifest_type"] = cmd.ManifestType
	result.Payload["deep_inspection"] = cmd.Payload["deep_inspection"]
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (mekh *MultiEnvironmentKubernetesHandler) countHealthyEnvironments(environmentStatuses map[string]*api.EnvironmentWorkloadStatus) int {
	count := 0
	for _, status := range environmentStatuses {
		healthyApps := 0
		for _, app := range status.Applications {
			if app.OverallStatus == "healthy" {
				healthyApps++
			}
		}
		if healthyApps > 0 && healthyApps == len(status.Applications) {
			count++
		}
	}
	return count
}

func (mekh *MultiEnvironmentKubernetesHandler) errorResult(cmd *api.GitOpsCommandMessage, message string, err error, startTime time.Time) (*api.GitOpsResultMessage, error) {
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
		ServiceName:  "multi-environment-kubernetes",
		Status:       "error",
		ErrorMessage: fmt.Sprintf("%s: %v", message, err),
		CompletedAt:  time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    0,
		},
	}, nil
}

// Implementation of environment mapper
type EnvironmentMapperImpl struct{}

func (em *EnvironmentMapperImpl) GetEnvironmentsForCustomer(customerID string) ([]string, error) {
	// For Phase 1C, simulate common environments
	return []string{"dev", "staging", "prod"}, nil
}

func (em *EnvironmentMapperImpl) MapEnvironmentToCluster(environment string) (string, error) {
	// For Phase 1C, simulate cluster mapping
	clusterMap := map[string]string{
		"dev":     "dev-cluster",
		"staging": "staging-cluster",
		"prod":    "prod-cluster",
	}

	if cluster, exists := clusterMap[environment]; exists {
		return cluster, nil
	}

	return "", fmt.Errorf("no cluster mapping found for environment: %s", environment)
}

func generateUUID() string {
	return uuid.New().String()
}
