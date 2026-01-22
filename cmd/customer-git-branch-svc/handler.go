package main

import (
	"fmt"
	"time"

	"github.com/contextops/platformctl/internal/clients/git"
	"github.com/contextops/platformctl/internal/clients/helm"
	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/pkg/api"
	"github.com/google/uuid"
)

type CustomerGitBranchHandler struct {
	gitClient        git.GitClient
	helmClient       helm.HelmClient
	valuesCorrelator ValuesCorrelator
}

type ValuesCorrelator interface {
	CorrelateValuesWithEnvironments(customerID string, valuesFiles []api.HelmValuesFile, environments []string) ([]api.ValuesEnvironmentCorrelation, error)
}

func NewCustomerGitBranchHandler(cfg *config.Config) *CustomerGitBranchHandler {
	return &CustomerGitBranchHandler{
		gitClient:        git.NewGitClient(),
		helmClient:       helm.NewHelmClient(),
		valuesCorrelator: &ValuesCorrelatorImpl{},
	}
}

func (cgbh *CustomerGitBranchHandler) HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error) {
	startTime := time.Now()

	switch cmd.Action {
	case "sync-customer-branch":
		return cgbh.handleCustomerBranchSync(cmd, startTime)
	case "inspect-manifests":
		return cgbh.handleCustomerBranchInspection(cmd, startTime)
	default:
		return cgbh.errorResult(cmd, "unsupported action", fmt.Errorf("action %s not supported", cmd.Action), startTime)
	}
}

func (cgbh *CustomerGitBranchHandler) GetSupportedManifestTypes() []string {
	return []string{"git", "app", "environment"}
}

func (cgbh *CustomerGitBranchHandler) GetSupportedActions() []string {
	return []string{"sync-customer-branch", "inspect-manifests"}
}

func (cgbh *CustomerGitBranchHandler) handleCustomerBranchSync(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Get customer branch from metadata
	customerBranch := cmd.ManifestMetadata.CustomerBranch
	if customerBranch == "" {
		customerBranch = fmt.Sprintf("customer/%s", cmd.CustomerID)
	}

	// Simulate repository URL - in real implementation this would come from context
	repositoryURL := fmt.Sprintf("https://github.com/%s/configs", cmd.CustomerID)

	// Validate customer branch
	branchValidation, err := cgbh.gitClient.ValidateCustomerBranch(cmd.CustomerID, customerBranch, repositoryURL)
	if err != nil {
		return cgbh.errorResult(cmd, "customer branch validation failed", err, startTime)
	}

	// Get Helm values files from the branch
	valuesFiles := branchValidation.HelmValuesFiles

	// Correlate values with environments
	environments := []string{"dev", "staging", "prod"}
	if syncAllEnvs, ok := cmd.Payload["sync_all_environments"].(bool); ok && syncAllEnvs {
		valuesCorrelations, err := cgbh.valuesCorrelator.CorrelateValuesWithEnvironments(cmd.CustomerID, valuesFiles, environments)
		if err != nil {
			return cgbh.errorResult(cmd, "values correlation failed", err, startTime)
		}
		
		// Store correlations in payload
		cmd.Payload["values_correlations"] = valuesCorrelations
	}

	// Create result
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    "git",
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "customer-git-branch",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    2, // Branch validation + values correlation
		},
	}

	// Add branch validation data to payload
	result.Payload["customer_branch"] = customerBranch
	result.Payload["repository_url"] = repositoryURL
	result.Payload["branch_validation"] = branchValidation
	result.Payload["helm_values_files"] = valuesFiles
	result.Payload["environment_files"] = branchValidation.EnvironmentFiles
	result.Payload["validation_status"] = branchValidation.ValidationStatus
	result.Payload["sync_all_environments"] = cmd.Payload["sync_all_environments"]
	result.Payload["validate_after_sync"] = cmd.Payload["validate_after_sync"]
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	// Set status based on branch validation
	if branchValidation.ValidationStatus != "valid" {
		result.Status = "degraded"
		result.ErrorMessage = branchValidation.ErrorMessage
	}

	return result, nil
}

func (cgbh *CustomerGitBranchHandler) handleCustomerBranchInspection(cmd *api.GitOpsCommandMessage, startTime time.Time) (*api.GitOpsResultMessage, error) {
	// Basic customer git branch inspection
	result := &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    "git",
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName: "customer-git-branch",
		Status:      "healthy",
		CompletedAt: time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    1,
		},
	}

	result.Payload["inspection_type"] = "customer_git_branch"
	result.Payload["manifest_type"] = "git"
	result.Payload["deep_inspection"] = cmd.Payload["deep_inspection"]
	result.Payload["customer_branch"] = cmd.ManifestMetadata.CustomerBranch
	result.Payload["latency_ms"] = time.Since(startTime).Milliseconds()

	return result, nil
}

func (cgbh *CustomerGitBranchHandler) errorResult(cmd *api.GitOpsCommandMessage, message string, err error, startTime time.Time) (*api.GitOpsResultMessage, error) {
	return &api.GitOpsResultMessage{
		GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
			SchemaVersion:   1,
			MessageID:       generateUUID(),
			CorrelationID:   cmd.CorrelationID,
			CustomerID:      cmd.CustomerID,
			ContextName:     cmd.ContextName,
			Action:          cmd.Action,
			ManifestType:    cmd.ManifestType,
			RequestedBy:     cmd.RequestedBy,
			RequestedAt:     cmd.RequestedAt,
			Priority:        cmd.Priority,
			Payload:         make(map[string]interface{}),
			ManifestMetadata: cmd.ManifestMetadata,
		},
		ServiceName:  "customer-git-branch",
		Status:       "error",
		ErrorMessage: fmt.Sprintf("%s: %v", message, err),
		CompletedAt:  time.Now().UTC(),
		PerformanceMetrics: api.GitOpsPerformanceMetrics{
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			ApiCallsCount:    0,
		},
	}, nil
}

// Implementation of values correlator
type ValuesCorrelatorImpl struct{}

func (vc *ValuesCorrelatorImpl) CorrelateValuesWithEnvironments(customerID string, valuesFiles []api.HelmValuesFile, environments []string) ([]api.ValuesEnvironmentCorrelation, error) {
	var correlations []api.ValuesEnvironmentCorrelation

	// For Phase 1C, simulate values environment correlation
	for _, env := range environments {
		correlation := api.ValuesEnvironmentCorrelation{
			Environment:       env,
			ValuesFile:        fmt.Sprintf("values-%s.yaml", env),
			CorrelationStatus: "matched",
			Differences:       []api.ValuesDifference{},
			LastCorrelated:    time.Now().UTC(),
		}

		// Find corresponding values file
		for _, valuesFile := range valuesFiles {
			if valuesFile.Environment == env {
				correlation.DeployedValues = map[string]interface{}{
					"image.repository": "myapp",
					"image.tag":        fmt.Sprintf("%s-latest", env),
					"replicas":         getReplicasForEnv(env),
				}
				break
			}
		}

		// Simulate some drift for staging environment
		if env == "staging" {
			correlation.CorrelationStatus = "drift"
			correlation.Differences = []api.ValuesDifference{
				{
					Key:            "image.tag",
					ExpectedValue:  "staging-v1.2.0",
					ActualValue:    "staging-v1.1.0",
					DifferenceType: "changed",
				},
			}
		}

		correlations = append(correlations, correlation)
	}

	return correlations, nil
}

func getReplicasForEnv(env string) int {
	switch env {
	case "dev":
		return 1
	case "staging":
		return 2
	case "prod":
		return 3
	default:
		return 1
	}
}

func generateUUID() string {
	return uuid.New().String()
}