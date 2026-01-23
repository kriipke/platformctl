package readmodel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/contextops/platformctl/pkg/api"
)

// GitOpsStore provides read model access for GitOps status queries
type GitOpsStore struct {
	db *sqlx.DB
}

// NewGitOpsStore creates a new GitOps read model store
func NewGitOpsStore(db *sqlx.DB) *GitOpsStore {
	return &GitOpsStore{
		db: db,
	}
}

// ContextStatus represents the overall status of a context
type ContextStatus struct {
	CustomerID           string                 `json:"customer_id" db:"customer_id"`
	ContextName          string                 `json:"context_name" db:"context_name"`
	AppReference         string                 `json:"app_reference" db:"app_reference"`
	EnvironmentReference string                 `json:"environment_reference" db:"environment_reference"`
	PairingStatus        string                 `json:"pairing_status" db:"pairing_status"`
	SyncStatus           string                 `json:"sync_status" db:"sync_status"`
	HealthStatus         string                 `json:"health_status" db:"health_status"`
	ResourceCount        int                    `json:"resource_count" db:"resource_count"`
	LastSyncTime         *time.Time             `json:"last_sync_time" db:"last_sync_time"`
	LastDeploymentTime   *time.Time             `json:"last_deployment_time" db:"last_deployment_time"`
	CorrelationData      map[string]interface{} `json:"correlation_data"`
	ValidationErrors     []string               `json:"validation_errors" db:"validation_errors"`
	GitCommit            string                 `json:"git_commit" db:"git_commit"`
	HelmRevision         string                 `json:"helm_revision" db:"helm_revision"`
	LastUpdated          time.Time              `json:"last_updated" db:"last_updated"`
}

// AppManifestStatus represents the status of an app manifest
type AppManifestStatus struct {
	CustomerID           string                 `json:"customer_id" db:"customer_id"`
	ContextName          string                 `json:"context_name" db:"context_name"`
	AppName              string                 `json:"app_name" db:"app_name"`
	ApplicationSetName   string                 `json:"applicationset_name" db:"applicationset_name"`
	Namespace            string                 `json:"namespace" db:"namespace"`
	GeneratorType        string                 `json:"generator_type" db:"generator_type"`
	GeneratorConfig      map[string]interface{} `json:"generator_config"`
	SyncStatus           string                 `json:"sync_status" db:"sync_status"`
	HealthStatus         string                 `json:"health_status" db:"health_status"`
	ApplicationCount     int                    `json:"application_count" db:"application_count"`
	HelmSources          []api.HelmSourceStatus `json:"helm_sources"`
	GitSources           []api.GitSourceStatus  `json:"git_sources"`
	GeneratedApps        []api.ApplicationStatus `json:"generated_applications"`
	BootstrapCorrelation map[string]interface{} `json:"bootstrap_correlation"`
	LastSyncTime         *time.Time             `json:"last_sync_time" db:"last_sync_time"`
	PerformanceMetrics   api.GitOpsPerformanceMetrics `json:"performance_metrics"`
	LastUpdated          time.Time              `json:"last_updated" db:"last_updated"`
}

// EnvironmentManifestStatus represents the status of an environment manifest
type EnvironmentManifestStatus struct {
	CustomerID                string                           `json:"customer_id" db:"customer_id"`
	ContextName               string                           `json:"context_name" db:"context_name"`
	EnvironmentName           string                           `json:"environment_name" db:"environment_name"`
	VaultValidationStatus     string                           `json:"vault_validation_status" db:"vault_validation_status"`
	ClusterValidationStatus   string                           `json:"cluster_validation_status" db:"cluster_validation_status"`
	ValuesFileStatus          string                           `json:"values_file_status" db:"values_file_status"`
	VaultValidations          []api.VaultValidationResult      `json:"vault_validations"`
	ClusterValidations        []api.ClusterValidationResult    `json:"cluster_validations"`
	ValuesFileValidations     []api.ValuesFileStatus           `json:"values_file_validations"`
	PodEnvValidations         []api.PodEnvValidationResult     `json:"pod_env_validations"`
	SecretCorrelations        map[string]interface{}           `json:"secret_correlations"`
	ValidationSummary         map[string]interface{}           `json:"validation_summary"`
	LastValidated             time.Time                        `json:"last_validated" db:"last_validated"`
	PerformanceMetrics        api.GitOpsPerformanceMetrics     `json:"performance_metrics"`
	LastUpdated               time.Time                        `json:"last_updated" db:"last_updated"`
}

// MultiEnvironmentAppStatus represents app status across multiple environments
type MultiEnvironmentAppStatus struct {
	CustomerID         string                       `json:"customer_id" db:"customer_id"`
	ContextName        string                       `json:"context_name" db:"context_name"`
	AppName            string                       `json:"app_name" db:"app_name"`
	Environment        string                       `json:"environment" db:"environment"`
	ClusterName        *string                      `json:"cluster_name" db:"cluster_name"`
	Namespace          string                       `json:"namespace" db:"namespace"`
	SyncStatus         string                       `json:"sync_status" db:"sync_status"`
	HealthStatus       string                       `json:"health_status" db:"health_status"`
	DeploymentStatus   string                       `json:"deployment_status" db:"deployment_status"`
	HelmRevision       *string                      `json:"helm_revision" db:"helm_revision"`
	GitCommit          *string                      `json:"git_commit" db:"git_commit"`
	ImageTags          map[string]interface{}       `json:"image_tags"`
	ResourceVersions   map[string]interface{}       `json:"resource_versions"`
	LastDeployed       *time.Time                   `json:"last_deployed" db:"last_deployed"`
	LastChecked        time.Time                    `json:"last_checked" db:"last_checked"`
	PerformanceMetrics api.GitOpsPerformanceMetrics `json:"performance_metrics"`
	ErrorMessage       *string                      `json:"error_message" db:"error_message"`
	LastUpdated        time.Time                    `json:"last_updated" db:"last_updated"`
}

// VaultValidationDetail represents detailed Vault validation status
type VaultValidationDetail struct {
	CustomerID             string                       `json:"customer_id" db:"customer_id"`
	ContextName            string                       `json:"context_name" db:"context_name"`
	EnvironmentName        string                       `json:"environment_name" db:"environment_name"`
	VaultPath              string                       `json:"vault_path" db:"vault_path"`
	SecretName             string                       `json:"secret_name" db:"secret_name"`
	ValidationStatus       string                       `json:"validation_status" db:"validation_status"`
	RequiredKeys           []string                     `json:"required_keys" db:"required_keys"`
	MissingKeys            []string                     `json:"missing_keys" db:"missing_keys"`
	ExtraKeys              []string                     `json:"extra_keys" db:"extra_keys"`
	PodCorrelations        []api.PodEnvValidationResult `json:"pod_correlations"`
	KubernetesSecretName   *string                      `json:"kubernetes_secret_name" db:"kubernetes_secret_name"`
	KubernetesNamespace    *string                      `json:"kubernetes_namespace" db:"kubernetes_namespace"`
	LastValidated          *time.Time                   `json:"last_validated" db:"last_validated"`
	ValidationError        *string                      `json:"validation_error" db:"validation_error"`
	PerformanceMetrics     api.GitOpsPerformanceMetrics `json:"performance_metrics"`
	LastUpdated            time.Time                    `json:"last_updated" db:"last_updated"`
}

// GetContextStatus retrieves the current status of a context
func (s *GitOpsStore) GetContextStatus(ctx context.Context, customerID, contextName string) (*ContextStatus, error) {
	var status ContextStatus
	var correlationDataJSON []byte

	query := `
		SELECT customer_id, context_name, app_reference, environment_reference,
		       pairing_status, sync_status, health_status, resource_count,
		       last_sync_time, last_deployment_time, correlation_data,
		       validation_errors, git_commit, helm_revision, last_updated
		FROM context_pairing_status 
		WHERE customer_id = $1 AND context_name = $2
	`

	row := s.db.QueryRowxContext(ctx, query, customerID, contextName)
	err := row.Scan(
		&status.CustomerID,
		&status.ContextName,
		&status.AppReference,
		&status.EnvironmentReference,
		&status.PairingStatus,
		&status.SyncStatus,
		&status.HealthStatus,
		&status.ResourceCount,
		&status.LastSyncTime,
		&status.LastDeploymentTime,
		&correlationDataJSON,
		pq.Array(&status.ValidationErrors),
		&status.GitCommit,
		&status.HelmRevision,
		&status.LastUpdated,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("context status not found for customer %s, context %s", customerID, contextName)
		}
		return nil, fmt.Errorf("failed to get context status: %w", err)
	}

	// Parse correlation data JSON
	if len(correlationDataJSON) > 0 {
		if err := json.Unmarshal(correlationDataJSON, &status.CorrelationData); err != nil {
			return nil, fmt.Errorf("failed to parse correlation data: %w", err)
		}
	}

	return &status, nil
}

// ListContextStatuses retrieves all context statuses for a customer
func (s *GitOpsStore) ListContextStatuses(ctx context.Context, customerID string) ([]ContextStatus, error) {
	var statuses []ContextStatus

	query := `
		SELECT customer_id, context_name, app_reference, environment_reference,
		       pairing_status, sync_status, health_status, resource_count,
		       last_sync_time, last_deployment_time, correlation_data,
		       validation_errors, git_commit, helm_revision, last_updated
		FROM context_pairing_status 
		WHERE customer_id = $1
		ORDER BY context_name ASC
	`

	rows, err := s.db.QueryxContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query context statuses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status ContextStatus
		var correlationDataJSON []byte

		err := rows.Scan(
			&status.CustomerID, &status.ContextName, &status.AppReference, &status.EnvironmentReference,
			&status.PairingStatus, &status.SyncStatus, &status.HealthStatus, &status.ResourceCount,
			&status.LastSyncTime, &status.LastDeploymentTime, &correlationDataJSON,
			pq.Array(&status.ValidationErrors), &status.GitCommit, &status.HelmRevision, &status.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan context status: %w", err)
		}

		// Parse correlation data JSON
		if len(correlationDataJSON) > 0 {
			if err := json.Unmarshal(correlationDataJSON, &status.CorrelationData); err != nil {
				return nil, fmt.Errorf("failed to parse correlation data: %w", err)
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, rows.Err()
}

// GetAppManifestStatus retrieves the status of an app manifest
func (s *GitOpsStore) GetAppManifestStatus(ctx context.Context, customerID, contextName, appName string) (*AppManifestStatus, error) {
	var status AppManifestStatus
	var generatorConfigJSON, helmSourcesJSON, gitSourcesJSON, generatedAppsJSON, bootstrapCorrelationJSON, performanceMetricsJSON []byte

	query := `
		SELECT customer_id, context_name, app_name, applicationset_name, namespace,
		       generator_type, generator_config, sync_status, health_status, application_count,
		       helm_sources, git_sources, generated_applications, bootstrap_correlation,
		       last_sync_time, performance_metrics, last_updated
		FROM app_manifest_correlation 
		WHERE customer_id = $1 AND context_name = $2 AND app_name = $3
	`

	err := s.db.QueryRowxContext(ctx, query, customerID, contextName, appName).Scan(
		&status.CustomerID, &status.ContextName, &status.AppName, &status.ApplicationSetName, &status.Namespace,
		&status.GeneratorType, &generatorConfigJSON, &status.SyncStatus, &status.HealthStatus, &status.ApplicationCount,
		&helmSourcesJSON, &gitSourcesJSON, &generatedAppsJSON, &bootstrapCorrelationJSON,
		&status.LastSyncTime, &performanceMetricsJSON, &status.LastUpdated,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("app manifest status not found for customer %s, context %s, app %s", customerID, contextName, appName)
		}
		return nil, fmt.Errorf("failed to get app manifest status: %w", err)
	}

	// Parse JSON fields
	if len(generatorConfigJSON) > 0 {
		if err := json.Unmarshal(generatorConfigJSON, &status.GeneratorConfig); err != nil {
			return nil, fmt.Errorf("failed to parse generator config: %w", err)
		}
	}

	if len(helmSourcesJSON) > 0 {
		if err := json.Unmarshal(helmSourcesJSON, &status.HelmSources); err != nil {
			return nil, fmt.Errorf("failed to parse helm sources: %w", err)
		}
	}

	if len(gitSourcesJSON) > 0 {
		if err := json.Unmarshal(gitSourcesJSON, &status.GitSources); err != nil {
			return nil, fmt.Errorf("failed to parse git sources: %w", err)
		}
	}

	if len(generatedAppsJSON) > 0 {
		if err := json.Unmarshal(generatedAppsJSON, &status.GeneratedApps); err != nil {
			return nil, fmt.Errorf("failed to parse generated applications: %w", err)
		}
	}

	if len(bootstrapCorrelationJSON) > 0 {
		if err := json.Unmarshal(bootstrapCorrelationJSON, &status.BootstrapCorrelation); err != nil {
			return nil, fmt.Errorf("failed to parse bootstrap correlation: %w", err)
		}
	}

	if len(performanceMetricsJSON) > 0 {
		if err := json.Unmarshal(performanceMetricsJSON, &status.PerformanceMetrics); err != nil {
			return nil, fmt.Errorf("failed to parse performance metrics: %w", err)
		}
	}

	return &status, nil
}

// GetEnvironmentManifestStatus retrieves the status of an environment manifest
func (s *GitOpsStore) GetEnvironmentManifestStatus(ctx context.Context, customerID, contextName, environmentName string) (*EnvironmentManifestStatus, error) {
	var status EnvironmentManifestStatus
	var vaultValidationsJSON, clusterValidationsJSON, valuesFileValidationsJSON, podEnvValidationsJSON, secretCorrelationsJSON, validationSummaryJSON, performanceMetricsJSON []byte

	query := `
		SELECT customer_id, context_name, environment_name,
		       vault_validation_status, cluster_validation_status, values_file_status,
		       vault_validations, cluster_validations, values_file_validations,
		       pod_env_validations, secret_correlations, validation_summary,
		       last_validated, performance_metrics, last_updated
		FROM environment_manifest_validation 
		WHERE customer_id = $1 AND context_name = $2 AND environment_name = $3
	`

	err := s.db.QueryRowxContext(ctx, query, customerID, contextName, environmentName).Scan(
		&status.CustomerID, &status.ContextName, &status.EnvironmentName,
		&status.VaultValidationStatus, &status.ClusterValidationStatus, &status.ValuesFileStatus,
		&vaultValidationsJSON, &clusterValidationsJSON, &valuesFileValidationsJSON,
		&podEnvValidationsJSON, &secretCorrelationsJSON, &validationSummaryJSON,
		&status.LastValidated, &performanceMetricsJSON, &status.LastUpdated,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("environment manifest status not found for customer %s, context %s, environment %s", customerID, contextName, environmentName)
		}
		return nil, fmt.Errorf("failed to get environment manifest status: %w", err)
	}

	// Parse JSON fields
	if len(vaultValidationsJSON) > 0 {
		if err := json.Unmarshal(vaultValidationsJSON, &status.VaultValidations); err != nil {
			return nil, fmt.Errorf("failed to parse vault validations: %w", err)
		}
	}

	if len(clusterValidationsJSON) > 0 {
		if err := json.Unmarshal(clusterValidationsJSON, &status.ClusterValidations); err != nil {
			return nil, fmt.Errorf("failed to parse cluster validations: %w", err)
		}
	}

	if len(valuesFileValidationsJSON) > 0 {
		if err := json.Unmarshal(valuesFileValidationsJSON, &status.ValuesFileValidations); err != nil {
			return nil, fmt.Errorf("failed to parse values file validations: %w", err)
		}
	}

	if len(podEnvValidationsJSON) > 0 {
		if err := json.Unmarshal(podEnvValidationsJSON, &status.PodEnvValidations); err != nil {
			return nil, fmt.Errorf("failed to parse pod env validations: %w", err)
		}
	}

	if len(secretCorrelationsJSON) > 0 {
		if err := json.Unmarshal(secretCorrelationsJSON, &status.SecretCorrelations); err != nil {
			return nil, fmt.Errorf("failed to parse secret correlations: %w", err)
		}
	}

	if len(validationSummaryJSON) > 0 {
		if err := json.Unmarshal(validationSummaryJSON, &status.ValidationSummary); err != nil {
			return nil, fmt.Errorf("failed to parse validation summary: %w", err)
		}
	}

	if len(performanceMetricsJSON) > 0 {
		if err := json.Unmarshal(performanceMetricsJSON, &status.PerformanceMetrics); err != nil {
			return nil, fmt.Errorf("failed to parse performance metrics: %w", err)
		}
	}

	return &status, nil
}

// GetMultiEnvironmentAppStatus retrieves app status across all environments
func (s *GitOpsStore) GetMultiEnvironmentAppStatus(ctx context.Context, customerID, contextName, appName string) ([]MultiEnvironmentAppStatus, error) {
	var statuses []MultiEnvironmentAppStatus

	query := `
		SELECT customer_id, context_name, app_name, environment, cluster_name, namespace,
		       sync_status, health_status, deployment_status, helm_revision, git_commit,
		       image_tags, resource_versions, last_deployed, last_checked,
		       performance_metrics, error_message, last_updated
		FROM multi_environment_app_status 
		WHERE customer_id = $1 AND context_name = $2 AND app_name = $3
		ORDER BY environment ASC
	`

	rows, err := s.db.QueryxContext(ctx, query, customerID, contextName, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to query multi-environment app status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status MultiEnvironmentAppStatus
		var imageTagsJSON, resourceVersionsJSON, performanceMetricsJSON []byte

		err := rows.Scan(
			&status.CustomerID, &status.ContextName, &status.AppName, &status.Environment, &status.ClusterName, &status.Namespace,
			&status.SyncStatus, &status.HealthStatus, &status.DeploymentStatus, &status.HelmRevision, &status.GitCommit,
			&imageTagsJSON, &resourceVersionsJSON, &status.LastDeployed, &status.LastChecked,
			&performanceMetricsJSON, &status.ErrorMessage, &status.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan multi-environment app status: %w", err)
		}

		// Parse JSON fields
		if len(imageTagsJSON) > 0 {
			if err := json.Unmarshal(imageTagsJSON, &status.ImageTags); err != nil {
				return nil, fmt.Errorf("failed to parse image tags: %w", err)
			}
		}

		if len(resourceVersionsJSON) > 0 {
			if err := json.Unmarshal(resourceVersionsJSON, &status.ResourceVersions); err != nil {
				return nil, fmt.Errorf("failed to parse resource versions: %w", err)
			}
		}

		if len(performanceMetricsJSON) > 0 {
			if err := json.Unmarshal(performanceMetricsJSON, &status.PerformanceMetrics); err != nil {
				return nil, fmt.Errorf("failed to parse performance metrics: %w", err)
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, rows.Err()
}

// GetVaultValidationDetails retrieves detailed Vault validation status
func (s *GitOpsStore) GetVaultValidationDetails(ctx context.Context, customerID, contextName, environmentName string) ([]VaultValidationDetail, error) {
	var validations []VaultValidationDetail

	query := `
		SELECT customer_id, context_name, environment_name, vault_path, secret_name,
		       validation_status, required_keys, missing_keys, extra_keys,
		       pod_correlations, kubernetes_secret_name, kubernetes_namespace,
		       last_validated, validation_error, performance_metrics, last_updated
		FROM gitops_vault_validation_status 
		WHERE customer_id = $1 AND context_name = $2 AND environment_name = $3
		ORDER BY vault_path, secret_name ASC
	`

	rows, err := s.db.QueryxContext(ctx, query, customerID, contextName, environmentName)
	if err != nil {
		return nil, fmt.Errorf("failed to query vault validation details: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var validation VaultValidationDetail
		var podCorrelationsJSON, performanceMetricsJSON []byte

		err := rows.Scan(
			&validation.CustomerID, &validation.ContextName, &validation.EnvironmentName, &validation.VaultPath, &validation.SecretName,
			&validation.ValidationStatus, pq.Array(&validation.RequiredKeys), pq.Array(&validation.MissingKeys), pq.Array(&validation.ExtraKeys),
			&podCorrelationsJSON, &validation.KubernetesSecretName, &validation.KubernetesNamespace,
			&validation.LastValidated, &validation.ValidationError, &performanceMetricsJSON, &validation.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vault validation detail: %w", err)
		}

		// Parse JSON fields
		if len(podCorrelationsJSON) > 0 {
			if err := json.Unmarshal(podCorrelationsJSON, &validation.PodCorrelations); err != nil {
				return nil, fmt.Errorf("failed to parse pod correlations: %w", err)
			}
		}

		if len(performanceMetricsJSON) > 0 {
			if err := json.Unmarshal(performanceMetricsJSON, &validation.PerformanceMetrics); err != nil {
				return nil, fmt.Errorf("failed to parse performance metrics: %w", err)
			}
		}

		validations = append(validations, validation)
	}

	return validations, rows.Err()
}

// GetContextsByHealthStatus retrieves contexts filtered by health status
func (s *GitOpsStore) GetContextsByHealthStatus(ctx context.Context, customerID string, healthStatuses []string) ([]ContextStatus, error) {
	if len(healthStatuses) == 0 {
		return s.ListContextStatuses(ctx, customerID)
	}

	var statuses []ContextStatus

	query := `
		SELECT customer_id, context_name, app_reference, environment_reference,
		       pairing_status, sync_status, health_status, resource_count,
		       last_sync_time, last_deployment_time, correlation_data,
		       validation_errors, git_commit, helm_revision, last_updated
		FROM context_pairing_status 
		WHERE customer_id = $1 AND health_status = ANY($2)
		ORDER BY context_name ASC
	`

	rows, err := s.db.QueryxContext(ctx, query, customerID, pq.Array(healthStatuses))
	if err != nil {
		return nil, fmt.Errorf("failed to query contexts by health status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status ContextStatus
		var correlationDataJSON []byte

		err := rows.Scan(
			&status.CustomerID, &status.ContextName, &status.AppReference, &status.EnvironmentReference,
			&status.PairingStatus, &status.SyncStatus, &status.HealthStatus, &status.ResourceCount,
			&status.LastSyncTime, &status.LastDeploymentTime, &correlationDataJSON,
			pq.Array(&status.ValidationErrors), &status.GitCommit, &status.HelmRevision, &status.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan context status: %w", err)
		}

		// Parse correlation data JSON
		if len(correlationDataJSON) > 0 {
			if err := json.Unmarshal(correlationDataJSON, &status.CorrelationData); err != nil {
				return nil, fmt.Errorf("failed to parse correlation data: %w", err)
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, rows.Err()
}