package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/pkg/api"
)

// GitOpsAggregator processes result messages from GitOps integration services
// and maintains the read model for efficient dashboard queries
type GitOpsAggregator struct {
	db      *sqlx.DB
	logger  zerolog.Logger
	metrics *observability.Metrics
}

// NewGitOpsAggregator creates a new GitOps aggregator service
func NewGitOpsAggregator(db *sqlx.DB, logger zerolog.Logger, metrics *observability.Metrics) *GitOpsAggregator {
	return &GitOpsAggregator{
		db:      db,
		logger:  logger.With().Str("component", "gitops-aggregator").Logger(),
		metrics: metrics,
	}
}

// ProcessResultMessage processes a GitOps result message and updates the read model
func (a *GitOpsAggregator) ProcessResultMessage(ctx context.Context, result *api.GitOpsResultMessage) error {
	start := time.Now()

	a.logger.Info().
		Str("correlation_id", result.CorrelationID).
		Str("service_name", result.ServiceName).
		Str("customer_id", result.CustomerID).
		Str("context_name", result.ContextName).
		Str("manifest_type", result.ManifestType).
		Str("status", result.Status).
		Msg("Processing GitOps result message")

	// Start database transaction
	tx, err := a.db.BeginTxx(ctx, nil)
	if err != nil {
		a.metrics.IncrementCounter("aggregator_transaction_errors", map[string]string{"error": "begin"})
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process based on manifest type and service
	switch result.ManifestType {
	case "app":
		if err := a.processAppManifestResult(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_processing_errors", map[string]string{"type": "app"})
			return fmt.Errorf("failed to process app manifest result: %w", err)
		}
	case "environment":
		if err := a.processEnvironmentManifestResult(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_processing_errors", map[string]string{"type": "environment"})
			return fmt.Errorf("failed to process environment manifest result: %w", err)
		}
	case "context":
		if err := a.processContextPairingResult(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_processing_errors", map[string]string{"type": "context"})
			return fmt.Errorf("failed to process context pairing result: %w", err)
		}
	case "git":
		if err := a.processCustomerBranchResult(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_processing_errors", map[string]string{"type": "git"})
			return fmt.Errorf("failed to process customer branch result: %w", err)
		}
	case "kubernetes":
		if err := a.processMultiEnvironmentResult(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_processing_errors", map[string]string{"type": "kubernetes"})
			return fmt.Errorf("failed to process multi-environment result: %w", err)
		}
	default:
		a.logger.Warn().Str("manifest_type", result.ManifestType).Msg("Unknown manifest type")
		return fmt.Errorf("unknown manifest type: %s", result.ManifestType)
	}

	// Update command run status
	if err := a.updateCommandRunStatus(ctx, tx, result); err != nil {
		a.metrics.IncrementCounter("aggregator_command_update_errors", nil)
		return fmt.Errorf("failed to update command run status: %w", err)
	}

	// Update context pairing status for pairing-relevant manifest types only.
	// git (customer-branch) and kubernetes (multi-environment) results are
	// materialized into their own tables and do not represent an app+environment
	// pairing, so they are skipped here.
	switch result.ManifestType {
	case "app", "environment", "context":
		if err := a.updateContextPairingStatus(ctx, tx, result); err != nil {
			a.metrics.IncrementCounter("aggregator_context_update_errors", nil)
			return fmt.Errorf("failed to update context pairing status: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		a.metrics.IncrementCounter("aggregator_transaction_errors", map[string]string{"error": "commit"})
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Record metrics
	duration := time.Since(start)
	a.metrics.RecordHistogram("aggregator_processing_duration", duration.Seconds(), map[string]string{
		"service": result.ServiceName,
		"type":    result.ManifestType,
		"status":  result.Status,
	})

	a.logger.Info().
		Str("correlation_id", result.CorrelationID).
		Str("service_name", result.ServiceName).
		Dur("duration", duration).
		Msg("Successfully processed GitOps result message")

	return nil
}

// processAppManifestResult processes results from app sync service
func (a *GitOpsAggregator) processAppManifestResult(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	if result.AppManifestData == nil {
		return fmt.Errorf("app manifest data is nil")
	}

	appData := result.AppManifestData

	// Serialize complex data to JSONB
	helmSources, _ := json.Marshal(appData.HelmSources)
	gitSources, _ := json.Marshal(appData.GitSources)
	applications, _ := json.Marshal(appData.Applications)
	generatorConfig, _ := json.Marshal(appData.Generator.Parameters)
	performanceMetrics, _ := json.Marshal(result.PerformanceMetrics)
	bootstrapCorrelation := json.RawMessage("{}")

	// Upsert app manifest correlation
	query := `
		INSERT INTO app_manifest_correlation (
			customer_id, context_name, app_name, applicationset_name, namespace,
			generator_type, generator_config, sync_status, health_status, application_count,
			helm_sources, git_sources, generated_applications, bootstrap_correlation,
			last_sync_time, performance_metrics, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW()
		)
		ON CONFLICT (customer_id, context_name, app_name, applicationset_name, namespace)
		DO UPDATE SET
			generator_type = EXCLUDED.generator_type,
			generator_config = EXCLUDED.generator_config,
			sync_status = EXCLUDED.sync_status,
			health_status = EXCLUDED.health_status,
			application_count = EXCLUDED.application_count,
			helm_sources = EXCLUDED.helm_sources,
			git_sources = EXCLUDED.git_sources,
			generated_applications = EXCLUDED.generated_applications,
			last_sync_time = EXCLUDED.last_sync_time,
			performance_metrics = EXCLUDED.performance_metrics,
			last_updated = NOW()
	`

	_, err := tx.ExecContext(ctx, query,
		result.CustomerID,
		result.ContextName,
		appData.AppName,
		appData.ApplicationSetName,
		appData.Namespace,
		appData.Generator.Type,
		generatorConfig,
		appData.SyncStatus,
		appData.HealthStatus,
		len(appData.Applications),
		helmSources,
		gitSources,
		applications,
		bootstrapCorrelation,
		appData.LastSyncTime,
		performanceMetrics,
	)

	return err
}

// processEnvironmentManifestResult processes results from environment validation service
func (a *GitOpsAggregator) processEnvironmentManifestResult(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	if result.EnvironmentManifestData == nil {
		return fmt.Errorf("environment manifest data is nil")
	}

	envData := result.EnvironmentManifestData

	// Serialize complex data to JSONB
	vaultValidations, _ := json.Marshal(envData.VaultValidations)
	clusterValidations, _ := json.Marshal(envData.ClusterValidations)
	valuesFileValidations, _ := json.Marshal(envData.ValuesFileStatus)
	performanceMetrics, _ := json.Marshal(result.PerformanceMetrics)

	// Calculate overall status
	vaultStatus := a.calculateVaultValidationStatus(envData.VaultValidations)
	clusterStatus := a.calculateClusterValidationStatus(envData.ClusterValidations)
	valuesStatus := a.calculateValuesFileStatus(envData.ValuesFileStatus)

	// Upsert environment manifest validation
	query := `
		INSERT INTO environment_manifest_validation (
			customer_id, context_name, environment_name,
			vault_validation_status, cluster_validation_status, values_file_status,
			vault_validations, cluster_validations, values_file_validations,
			last_validated, performance_metrics, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
		)
		ON CONFLICT (customer_id, context_name, environment_name)
		DO UPDATE SET
			vault_validation_status = EXCLUDED.vault_validation_status,
			cluster_validation_status = EXCLUDED.cluster_validation_status,
			values_file_status = EXCLUDED.values_file_status,
			vault_validations = EXCLUDED.vault_validations,
			cluster_validations = EXCLUDED.cluster_validations,
			values_file_validations = EXCLUDED.values_file_validations,
			last_validated = EXCLUDED.last_validated,
			performance_metrics = EXCLUDED.performance_metrics,
			last_updated = NOW()
	`

	_, err := tx.ExecContext(ctx, query,
		result.CustomerID,
		result.ContextName,
		result.EnvironmentName,
		vaultStatus,
		clusterStatus,
		valuesStatus,
		vaultValidations,
		clusterValidations,
		valuesFileValidations,
		envData.LastValidated,
		performanceMetrics,
	)

	if err != nil {
		return err
	}

	// Process individual vault validations for detailed tracking
	return a.processVaultValidations(ctx, tx, result.CustomerID, result.ContextName, result.EnvironmentName, envData.VaultValidations)
}

// processContextPairingResult processes results from context correlation service
func (a *GitOpsAggregator) processContextPairingResult(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	if result.ContextPairingData == nil {
		return fmt.Errorf("context pairing data is nil")
	}

	pairingData := result.ContextPairingData

	// Serialize complex data to JSONB
	correlationData, _ := json.Marshal(pairingData.CorrelationData)
	performanceMetrics, _ := json.Marshal(result.PerformanceMetrics)

	// Create context pairing operation record
	operationQuery := `
		INSERT INTO context_pairing_operations (
			customer_id, context_name, operation_type, operation_status,
			correlation_results, error_details, started_at, completed_at,
			performance_metrics, correlation_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	operationStatus := "completed"
	if result.Status == "error" {
		operationStatus = "failed"
	}

	_, err := tx.ExecContext(ctx, operationQuery,
		result.CustomerID,
		result.ContextName,
		"correlate",
		operationStatus,
		correlationData,
		result.ErrorMessage,
		result.RequestedAt,
		result.CompletedAt,
		performanceMetrics,
		result.CorrelationID,
	)

	return err
}

// processCustomerBranchResult materializes a customer-git-branch result into the
// customer_git_branch_correlation read-model table. The customer-git-branch
// service carries its detail in the result payload (not a typed field), so the
// core fields are extracted from there.
func (a *GitOpsAggregator) processCustomerBranchResult(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	repoURL, branch, compliance := customerBranchFields(result)
	performanceMetrics, _ := json.Marshal(result.PerformanceMetrics)

	query := `
		INSERT INTO customer_git_branch_correlation (
			customer_id, context_name, repository_url, customer_branch,
			branch_compliance, last_validated, validation_error, performance_metrics, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW()
		)
		ON CONFLICT (customer_id, context_name, repository_url, customer_branch)
		DO UPDATE SET
			branch_compliance = EXCLUDED.branch_compliance,
			last_validated = EXCLUDED.last_validated,
			validation_error = EXCLUDED.validation_error,
			performance_metrics = EXCLUDED.performance_metrics,
			last_updated = NOW()
	`

	_, err := tx.ExecContext(ctx, query,
		result.CustomerID,
		result.ContextName,
		repoURL,
		branch,
		compliance,
		result.CompletedAt,
		result.ErrorMessage,
		performanceMetrics,
	)

	return err
}

// processMultiEnvironmentResult records a multi-environment Kubernetes
// correlation as an operation row, stashing the correlation payload in the
// multi_env_correlation column. Per-app materialization into
// multi_environment_app_status is left as a follow-up (it requires structured
// result data the service does not yet emit).
func (a *GitOpsAggregator) processMultiEnvironmentResult(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	multiEnvData, _ := json.Marshal(result.Payload)
	performanceMetrics, _ := json.Marshal(result.PerformanceMetrics)

	operationStatus := "completed"
	if result.Status == "error" {
		operationStatus = "failed"
	}

	query := `
		INSERT INTO context_pairing_operations (
			customer_id, context_name, operation_type, operation_status,
			multi_env_correlation, error_details, started_at, completed_at,
			performance_metrics, correlation_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := tx.ExecContext(ctx, query,
		result.CustomerID,
		result.ContextName,
		"correlate",
		operationStatus,
		multiEnvData,
		result.ErrorMessage,
		result.RequestedAt,
		result.CompletedAt,
		performanceMetrics,
		result.CorrelationID,
	)

	return err
}

// customerBranchFields extracts the repository URL, customer branch, and
// compliance status for a customer-git-branch result. The detail lives in the
// result payload; the branch falls back to the manifest metadata.
func customerBranchFields(result *api.GitOpsResultMessage) (repoURL, branch, compliance string) {
	repoURL = stringFromPayload(result.Payload, "repository_url")

	branch = stringFromPayload(result.Payload, "customer_branch")
	if branch == "" {
		branch = result.ManifestMetadata.CustomerBranch
	}

	validationStatus := stringFromPayload(result.Payload, "validation_status")
	switch {
	case result.Status == "error":
		compliance = "non_compliant"
	case validationStatus == "valid" || result.Status == "healthy":
		compliance = "compliant"
	case validationStatus == "":
		compliance = "unknown"
	default:
		compliance = "non_compliant"
	}

	return repoURL, branch, compliance
}

// stringFromPayload safely reads a string value from a result payload map.
func stringFromPayload(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

// updateCommandRunStatus updates the command run status with the result
func (a *GitOpsAggregator) updateCommandRunStatus(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	status := "completed"
	if result.Status == "error" {
		status = "failed"
	}

	resultPayload, _ := json.Marshal(result.ResultPayload)

	query := `
		UPDATE command_runs 
		SET status = $1, completed_at = $2, error_message = $3, result_payload = $4, updated_at = NOW()
		WHERE correlation_id = $5
	`

	_, err := tx.ExecContext(ctx, query,
		status,
		result.CompletedAt,
		result.ErrorMessage,
		resultPayload,
		result.CorrelationID,
	)

	return err
}

// updateContextPairingStatus updates the overall context pairing status
func (a *GitOpsAggregator) updateContextPairingStatus(ctx context.Context, tx *sqlx.Tx, result *api.GitOpsResultMessage) error {
	// Get app and environment references for this context
	var appRef, envRef string
	err := tx.GetContext(ctx, &appRef,
		"SELECT app_reference FROM contexts WHERE name = $1 AND customer_id = $2",
		result.ContextName, result.CustomerID)
	if err != nil {
		return err
	}

	err = tx.GetContext(ctx, &envRef,
		"SELECT environment_reference FROM contexts WHERE name = $1 AND customer_id = $2",
		result.ContextName, result.CustomerID)
	if err != nil {
		return err
	}

	// Determine pairing status based on result
	pairingStatus := "valid"
	syncStatus := "synced"
	healthStatus := "healthy"

	if result.Status == "error" {
		pairingStatus = "invalid"
		syncStatus = "failed"
		healthStatus = "unhealthy"
	} else if result.Status == "degraded" {
		pairingStatus = "valid"
		syncStatus = "synced"
		healthStatus = "degraded"
	}

	// Extract additional data based on manifest type
	var lastSyncTime, lastDeploymentTime *time.Time
	var gitCommit, helmRevision string
	var resourceCount int

	if result.AppManifestData != nil {
		lastSyncTime = result.AppManifestData.LastSyncTime
		for _, app := range result.AppManifestData.Applications {
			if app.LastDeployed != nil {
				lastDeploymentTime = app.LastDeployed
			}
			if app.HelmRevision != "" {
				helmRevision = app.HelmRevision
			}
		}
		resourceCount = len(result.AppManifestData.Applications)
	}

	if result.ContextPairingData != nil {
		lastDeploymentTime = result.ContextPairingData.LastDeploymentTime
		resourceCount = result.ContextPairingData.ResourceCount
		syncStatus = result.ContextPairingData.SyncStatus
		healthStatus = result.ContextPairingData.HealthStatus
		pairingStatus = result.ContextPairingData.PairingStatus
	}

	correlationData, _ := json.Marshal(map[string]interface{}{
		"service":        result.ServiceName,
		"last_processed": time.Now(),
		"manifest_type":  result.ManifestType,
	})

	// Upsert context pairing status
	query := `
		INSERT INTO context_pairing_status (
			customer_id, context_name, app_reference, environment_reference,
			pairing_status, sync_status, health_status, resource_count,
			last_sync_time, last_deployment_time, correlation_data,
			git_commit, helm_revision, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW()
		)
		ON CONFLICT (customer_id, context_name, app_reference, environment_reference)
		DO UPDATE SET
			pairing_status = EXCLUDED.pairing_status,
			sync_status = EXCLUDED.sync_status,
			health_status = EXCLUDED.health_status,
			resource_count = EXCLUDED.resource_count,
			last_sync_time = COALESCE(EXCLUDED.last_sync_time, context_pairing_status.last_sync_time),
			last_deployment_time = COALESCE(EXCLUDED.last_deployment_time, context_pairing_status.last_deployment_time),
			correlation_data = EXCLUDED.correlation_data,
			git_commit = COALESCE(EXCLUDED.git_commit, context_pairing_status.git_commit),
			helm_revision = COALESCE(EXCLUDED.helm_revision, context_pairing_status.helm_revision),
			last_updated = NOW()
	`

	_, err = tx.ExecContext(ctx, query,
		result.CustomerID,
		result.ContextName,
		appRef,
		envRef,
		pairingStatus,
		syncStatus,
		healthStatus,
		resourceCount,
		lastSyncTime,
		lastDeploymentTime,
		correlationData,
		gitCommit,
		helmRevision,
	)

	return err
}

// processVaultValidations processes individual vault validations for detailed tracking
func (a *GitOpsAggregator) processVaultValidations(ctx context.Context, tx *sqlx.Tx, customerID, contextName, environmentName string, validations []api.VaultValidationResult) error {
	for _, validation := range validations {
		podCorrelations, _ := json.Marshal(validation.PodEnvValidations)
		performanceMetrics := json.RawMessage("{}")

		query := `
			INSERT INTO gitops_vault_validation_status (
				customer_id, context_name, environment_name, vault_path, secret_name,
				validation_status, required_keys, missing_keys, extra_keys,
				pod_correlations, last_validated, validation_error, performance_metrics
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
			)
			ON CONFLICT (customer_id, context_name, environment_name, vault_path, secret_name)
			DO UPDATE SET
				validation_status = EXCLUDED.validation_status,
				required_keys = EXCLUDED.required_keys,
				missing_keys = EXCLUDED.missing_keys,
				extra_keys = EXCLUDED.extra_keys,
				pod_correlations = EXCLUDED.pod_correlations,
				last_validated = EXCLUDED.last_validated,
				validation_error = EXCLUDED.validation_error,
				performance_metrics = EXCLUDED.performance_metrics,
				last_updated = NOW()
		`

		var requiredKeys, missingKeys, extraKeys []string
		// Convert from interface arrays if needed - this would need proper conversion based on actual data structure

		_, err := tx.ExecContext(ctx, query,
			customerID,
			contextName,
			environmentName,
			validation.VaultPath,
			validation.SecretName,
			validation.ValidationStatus,
			requiredKeys,
			missingKeys,
			extraKeys,
			podCorrelations,
			validation.LastValidated,
			"", // validation_error - would extract from validation if available
			performanceMetrics,
		)

		if err != nil {
			return err
		}
	}
	return nil
}

// Helper methods for calculating overall statuses

func (a *GitOpsAggregator) calculateVaultValidationStatus(validations []api.VaultValidationResult) string {
	if len(validations) == 0 {
		return "unknown"
	}

	validCount := 0
	for _, v := range validations {
		if v.ValidationStatus == "valid" {
			validCount++
		}
	}

	if validCount == len(validations) {
		return "valid"
	} else if validCount > 0 {
		return "partial"
	} else {
		return "invalid"
	}
}

func (a *GitOpsAggregator) calculateClusterValidationStatus(validations []api.ClusterValidationResult) string {
	if len(validations) == 0 {
		return "unknown"
	}

	connectedCount := 0
	for _, v := range validations {
		if v.ConnectionStatus == "connected" {
			connectedCount++
		}
	}

	if connectedCount == len(validations) {
		return "connected"
	} else if connectedCount > 0 {
		return "partial"
	} else {
		return "disconnected"
	}
}

func (a *GitOpsAggregator) calculateValuesFileStatus(files []api.ValuesFileStatus) string {
	if len(files) == 0 {
		return "unknown"
	}

	availableCount := 0
	for _, f := range files {
		if f.Status == "available" {
			availableCount++
		}
	}

	if availableCount == len(files) {
		return "available"
	} else if availableCount > 0 {
		return "partial"
	} else {
		return "missing"
	}
}
