package correlation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/contextops/platformctl/internal/readmodel"
	"github.com/contextops/platformctl/pkg/api"
)

// EnvironmentCorrelator provides advanced correlation logic for multi-environment GitOps data
type EnvironmentCorrelator struct {
	db     *sqlx.DB
	store  *readmodel.GitOpsStore
	logger zerolog.Logger
}

// NewEnvironmentCorrelator creates a new environment correlator
func NewEnvironmentCorrelator(db *sqlx.DB, store *readmodel.GitOpsStore, logger zerolog.Logger) *EnvironmentCorrelator {
	return &EnvironmentCorrelator{
		db:     db,
		store:  store,
		logger: logger.With().Str("component", "environment-correlator").Logger(),
	}
}

// EnvironmentCorrelation represents correlation data across environments
type EnvironmentCorrelation struct {
	CustomerID          string                                `json:"customer_id"`
	ContextName         string                                `json:"context_name"`
	AppName             string                                `json:"app_name"`
	Environments        []EnvironmentStatus                   `json:"environments"`
	OverallHealth       string                                `json:"overall_health"`
	SyncStatusSummary   map[string]int                        `json:"sync_status_summary"`
	HealthStatusSummary map[string]int                        `json:"health_status_summary"`
	DriftDetection      []DriftAlert                          `json:"drift_detection"`
	ValuesComparison    map[string]ValuesFileDifference       `json:"values_comparison"`
	VaultCorrelation    []VaultSecretCorrelation              `json:"vault_correlation"`
	DeploymentTimeline  []DeploymentEvent                     `json:"deployment_timeline"`
	ResourceVersions    map[string]ResourceVersionComparison  `json:"resource_versions"`
	LastCorrelated      time.Time                             `json:"last_correlated"`
}

// EnvironmentStatus represents the status of an app in a specific environment
type EnvironmentStatus struct {
	Environment      string                           `json:"environment"`
	ClusterName      string                           `json:"cluster_name"`
	Namespace        string                           `json:"namespace"`
	SyncStatus       string                           `json:"sync_status"`
	HealthStatus     string                           `json:"health_status"`
	DeploymentStatus string                           `json:"deployment_status"`
	HelmRevision     string                           `json:"helm_revision"`
	GitCommit        string                           `json:"git_commit"`
	ImageTags        map[string]string                `json:"image_tags"`
	LastDeployed     *time.Time                       `json:"last_deployed"`
	ValuesFile       string                           `json:"values_file"`
	VaultSecrets     []VaultSecretReference           `json:"vault_secrets"`
	ResourceCount    int                              `json:"resource_count"`
	Errors           []string                         `json:"errors,omitempty"`
}

// DriftAlert represents configuration drift between environments
type DriftAlert struct {
	Type             string    `json:"type"` // values, image_tag, helm_version, vault_secret
	SourceEnv        string    `json:"source_env"`
	TargetEnv        string    `json:"target_env"`
	DriftDescription string    `json:"drift_description"`
	Severity         string    `json:"severity"` // low, medium, high, critical
	DetectedAt       time.Time `json:"detected_at"`
	RecommendedAction string   `json:"recommended_action"`
}

// ValuesFileDifference represents differences in Helm values files between environments
type ValuesFileDifference struct {
	Environment     string                 `json:"environment"`
	FilePath        string                 `json:"file_path"`
	KeyDifferences  []ValuesDifference     `json:"key_differences"`
	MissingKeys     []string               `json:"missing_keys"`
	ExtraKeys       []string               `json:"extra_keys"`
	LastModified    *time.Time             `json:"last_modified"`
	ComparedAgainst string                 `json:"compared_against"` // base environment for comparison
}

// ValuesDifference represents a specific difference in values
type ValuesDifference struct {
	Key           string      `json:"key"`
	SourceValue   interface{} `json:"source_value"`
	TargetValue   interface{} `json:"target_value"`
	DifferenceType string     `json:"difference_type"` // value, type, structure
}

// VaultSecretCorrelation represents Vault secret usage across environments
type VaultSecretCorrelation struct {
	VaultPath           string                      `json:"vault_path"`
	SecretName          string                      `json:"secret_name"`
	Environments        []VaultSecretEnvironment    `json:"environments"`
	KeyConsistency      map[string]KeyConsistency   `json:"key_consistency"`
	LastValidated       time.Time                   `json:"last_validated"`
	ValidationErrors    []string                    `json:"validation_errors,omitempty"`
}

// VaultSecretEnvironment represents Vault secret status in a specific environment
type VaultSecretEnvironment struct {
	Environment         string    `json:"environment"`
	ValidationStatus    string    `json:"validation_status"`
	KubernetesSecretName string   `json:"kubernetes_secret_name"`
	Namespace           string    `json:"namespace"`
	RequiredKeys        []string  `json:"required_keys"`
	MissingKeys         []string  `json:"missing_keys"`
	LastSyncTime        *time.Time `json:"last_sync_time"`
}

// KeyConsistency represents consistency of a key across environments
type KeyConsistency struct {
	Key                 string            `json:"key"`
	PresentInEnvs       []string          `json:"present_in_envs"`
	MissingInEnvs       []string          `json:"missing_in_envs"`
	ConsistencyStatus   string            `json:"consistency_status"` // consistent, inconsistent, missing
	RecommendedAction   string            `json:"recommended_action"`
}

// VaultSecretReference represents a reference to a Vault secret
type VaultSecretReference struct {
	Path              string    `json:"path"`
	SecretName        string    `json:"secret_name"`
	ValidationStatus  string    `json:"validation_status"`
	LastValidated     time.Time `json:"last_validated"`
}

// DeploymentEvent represents a deployment event in the timeline
type DeploymentEvent struct {
	Environment   string    `json:"environment"`
	EventType     string    `json:"event_type"` // deploy, sync, rollback, scale
	Timestamp     time.Time `json:"timestamp"`
	HelmRevision  string    `json:"helm_revision,omitempty"`
	GitCommit     string    `json:"git_commit,omitempty"`
	Status        string    `json:"status"`
	Duration      *int64    `json:"duration_ms,omitempty"`
	TriggeredBy   string    `json:"triggered_by"`
	Description   string    `json:"description"`
}

// ResourceVersionComparison represents resource version comparison across environments
type ResourceVersionComparison struct {
	ResourceType      string                    `json:"resource_type"` // Deployment, Service, ConfigMap, etc.
	Environments      map[string]string         `json:"environments"`  // env -> version
	Consistency       string                    `json:"consistency"`   // consistent, drift, unknown
	LatestVersion     string                    `json:"latest_version"`
	VersionDrift      []ResourceVersionDrift    `json:"version_drift,omitempty"`
}

// ResourceVersionDrift represents version drift for a specific resource
type ResourceVersionDrift struct {
	Environment       string    `json:"environment"`
	CurrentVersion    string    `json:"current_version"`
	ExpectedVersion   string    `json:"expected_version"`
	DriftSeverity     string    `json:"drift_severity"` // minor, major, critical
	LastUpdated       time.Time `json:"last_updated"`
}

// CorrelateEnvironments performs comprehensive correlation of an app across all environments
func (ec *EnvironmentCorrelator) CorrelateEnvironments(ctx context.Context, customerID, contextName, appName string) (*EnvironmentCorrelation, error) {
	start := time.Now()
	
	ec.logger.Info().
		Str("customer_id", customerID).
		Str("context_name", contextName).
		Str("app_name", appName).
		Msg("Starting environment correlation")

	// Get multi-environment app status
	multiEnvStatus, err := ec.store.GetMultiEnvironmentAppStatus(ctx, customerID, contextName, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to get multi-environment app status: %w", err)
	}

	if len(multiEnvStatus) == 0 {
		return nil, fmt.Errorf("no environment data found for app %s in context %s", appName, contextName)
	}

	correlation := &EnvironmentCorrelation{
		CustomerID:          customerID,
		ContextName:         contextName,
		AppName:             appName,
		Environments:        make([]EnvironmentStatus, 0, len(multiEnvStatus)),
		SyncStatusSummary:   make(map[string]int),
		HealthStatusSummary: make(map[string]int),
		DriftDetection:      make([]DriftAlert, 0),
		ValuesComparison:    make(map[string]ValuesFileDifference),
		VaultCorrelation:    make([]VaultSecretCorrelation, 0),
		DeploymentTimeline:  make([]DeploymentEvent, 0),
		ResourceVersions:    make(map[string]ResourceVersionComparison),
		LastCorrelated:      time.Now(),
	}

	// Process each environment
	for _, envStatus := range multiEnvStatus {
		env := ec.buildEnvironmentStatus(envStatus)
		correlation.Environments = append(correlation.Environments, env)

		// Update summaries
		correlation.SyncStatusSummary[env.SyncStatus]++
		correlation.HealthStatusSummary[env.HealthStatus]++
	}

	// Calculate overall health
	correlation.OverallHealth = ec.calculateOverallHealth(correlation.HealthStatusSummary)

	// Detect configuration drift
	drift, err := ec.detectConfigurationDrift(ctx, customerID, contextName, appName, correlation.Environments)
	if err != nil {
		ec.logger.Warn().Err(err).Msg("Failed to detect configuration drift")
	} else {
		correlation.DriftDetection = drift
	}

	// Compare values files
	valuesComparison, err := ec.compareValuesFiles(ctx, customerID, contextName, appName, correlation.Environments)
	if err != nil {
		ec.logger.Warn().Err(err).Msg("Failed to compare values files")
	} else {
		correlation.ValuesComparison = valuesComparison
	}

	// Correlate Vault secrets
	vaultCorrelation, err := ec.correlateVaultSecrets(ctx, customerID, contextName, correlation.Environments)
	if err != nil {
		ec.logger.Warn().Err(err).Msg("Failed to correlate Vault secrets")
	} else {
		correlation.VaultCorrelation = vaultCorrelation
	}

	// Build deployment timeline
	timeline, err := ec.buildDeploymentTimeline(ctx, customerID, contextName, appName, correlation.Environments)
	if err != nil {
		ec.logger.Warn().Err(err).Msg("Failed to build deployment timeline")
	} else {
		correlation.DeploymentTimeline = timeline
	}

	// Compare resource versions
	resourceVersions, err := ec.compareResourceVersions(ctx, customerID, contextName, appName, correlation.Environments)
	if err != nil {
		ec.logger.Warn().Err(err).Msg("Failed to compare resource versions")
	} else {
		correlation.ResourceVersions = resourceVersions
	}

	duration := time.Since(start)
	ec.logger.Info().
		Str("customer_id", customerID).
		Str("context_name", contextName).
		Str("app_name", appName).
		Int("environment_count", len(correlation.Environments)).
		Int("drift_alerts", len(correlation.DriftDetection)).
		Str("overall_health", correlation.OverallHealth).
		Dur("duration", duration).
		Msg("Environment correlation completed")

	return correlation, nil
}

// buildEnvironmentStatus builds detailed environment status from multi-environment app status
func (ec *EnvironmentCorrelator) buildEnvironmentStatus(envStatus readmodel.MultiEnvironmentAppStatus) EnvironmentStatus {
	// Parse JSON fields
	var imageTags map[string]string
	if len(envStatus.ImageTags) > 0 {
		// Convert interface{} map to string map
		imageTags = make(map[string]string)
		for k, v := range envStatus.ImageTags {
			if str, ok := v.(string); ok {
				imageTags[k] = str
			}
		}
	}

	clusterName := ""
	if envStatus.ClusterName != nil {
		clusterName = *envStatus.ClusterName
	}

	helmRevision := ""
	if envStatus.HelmRevision != nil {
		helmRevision = *envStatus.HelmRevision
	}

	gitCommit := ""
	if envStatus.GitCommit != nil {
		gitCommit = *envStatus.GitCommit
	}

	return EnvironmentStatus{
		Environment:      envStatus.Environment,
		ClusterName:      clusterName,
		Namespace:        envStatus.Namespace,
		SyncStatus:       envStatus.SyncStatus,
		HealthStatus:     envStatus.HealthStatus,
		DeploymentStatus: envStatus.DeploymentStatus,
		HelmRevision:     helmRevision,
		GitCommit:        gitCommit,
		ImageTags:        imageTags,
		LastDeployed:     envStatus.LastDeployed,
		ValuesFile:       fmt.Sprintf("values-%s.yaml", envStatus.Environment),
		VaultSecrets:     make([]VaultSecretReference, 0), // Will be populated by vault correlation
		ResourceCount:    0, // Will be calculated from resource versions
		Errors:           make([]string, 0),
	}
}

// calculateOverallHealth calculates overall health based on environment health distribution
func (ec *EnvironmentCorrelator) calculateOverallHealth(healthSummary map[string]int) string {
	total := 0
	for _, count := range healthSummary {
		total += count
	}

	if total == 0 {
		return "unknown"
	}

	// If all environments are healthy
	if healthSummary["healthy"] == total {
		return "healthy"
	}

	// If any environment is unhealthy
	if healthSummary["unhealthy"] > 0 {
		return "unhealthy"
	}

	// If any environment is degraded
	if healthSummary["degraded"] > 0 {
		return "degraded"
	}

	return "unknown"
}

// detectConfigurationDrift detects configuration drift between environments
func (ec *EnvironmentCorrelator) detectConfigurationDrift(ctx context.Context, customerID, contextName, appName string, environments []EnvironmentStatus) ([]DriftAlert, error) {
	alerts := make([]DriftAlert, 0)

	if len(environments) < 2 {
		return alerts, nil // No drift possible with less than 2 environments
	}

	// Use production as the baseline, or the last environment if production doesn't exist
	baseline := environments[len(environments)-1] // Default to last environment
	for _, env := range environments {
		if env.Environment == "production" || env.Environment == "prod" {
			baseline = env
			break
		}
	}

	for _, env := range environments {
		if env.Environment == baseline.Environment {
			continue // Skip comparing baseline to itself
		}

		// Check for Helm version drift
		if env.HelmRevision != baseline.HelmRevision && env.HelmRevision != "" && baseline.HelmRevision != "" {
			alerts = append(alerts, DriftAlert{
				Type:              "helm_version",
				SourceEnv:         baseline.Environment,
				TargetEnv:         env.Environment,
				DriftDescription:  fmt.Sprintf("Helm revision differs: %s vs %s", baseline.HelmRevision, env.HelmRevision),
				Severity:          ec.calculateHelmVersionDriftSeverity(baseline.HelmRevision, env.HelmRevision),
				DetectedAt:        time.Now(),
				RecommendedAction: "Consider syncing Helm chart versions across environments",
			})
		}

		// Check for Git commit drift
		if env.GitCommit != baseline.GitCommit && env.GitCommit != "" && baseline.GitCommit != "" {
			alerts = append(alerts, DriftAlert{
				Type:              "git_commit",
				SourceEnv:         baseline.Environment,
				TargetEnv:         env.Environment,
				DriftDescription:  fmt.Sprintf("Git commit differs: %s vs %s", baseline.GitCommit, env.GitCommit),
				Severity:          "medium",
				DetectedAt:        time.Now(),
				RecommendedAction: "Review commit differences and consider environment promotion",
			})
		}

		// Check for container image tag drift
		for image, baselineTag := range baseline.ImageTags {
			if envTag, exists := env.ImageTags[image]; exists && envTag != baselineTag {
				alerts = append(alerts, DriftAlert{
					Type:              "image_tag",
					SourceEnv:         baseline.Environment,
					TargetEnv:         env.Environment,
					DriftDescription:  fmt.Sprintf("Image %s tag differs: %s vs %s", image, baselineTag, envTag),
					Severity:          ec.calculateImageTagDriftSeverity(baselineTag, envTag),
					DetectedAt:        time.Now(),
					RecommendedAction: fmt.Sprintf("Verify if image tag drift for %s is intentional", image),
				})
			}
		}

		// Check for health status drift (if baseline is healthy but others aren't)
		if baseline.HealthStatus == "healthy" && env.HealthStatus != "healthy" {
			alerts = append(alerts, DriftAlert{
				Type:              "health_status",
				SourceEnv:         baseline.Environment,
				TargetEnv:         env.Environment,
				DriftDescription:  fmt.Sprintf("Health status degraded: %s is %s while %s is %s", env.Environment, env.HealthStatus, baseline.Environment, baseline.HealthStatus),
				Severity:          "high",
				DetectedAt:        time.Now(),
				RecommendedAction: "Investigate health issues in " + env.Environment,
			})
		}
	}

	return alerts, nil
}

// compareValuesFiles compares Helm values files between environments
func (ec *EnvironmentCorrelator) compareValuesFiles(ctx context.Context, customerID, contextName, appName string, environments []EnvironmentStatus) (map[string]ValuesFileDifference, error) {
	comparison := make(map[string]ValuesFileDifference)

	// This would typically involve fetching and parsing actual values files from Git
	// For now, we'll create placeholder logic that would be implemented with actual Git integration

	for _, env := range environments {
		// Simulate values file comparison
		comparison[env.Environment] = ValuesFileDifference{
			Environment:     env.Environment,
			FilePath:        env.ValuesFile,
			KeyDifferences:  make([]ValuesDifference, 0), // Would be populated from actual comparison
			MissingKeys:     make([]string, 0),
			ExtraKeys:       make([]string, 0),
			LastModified:    nil, // Would get from Git metadata
			ComparedAgainst: "production", // Default comparison baseline
		}
	}

	return comparison, nil
}

// correlateVaultSecrets correlates Vault secret usage across environments
func (ec *EnvironmentCorrelator) correlateVaultSecrets(ctx context.Context, customerID, contextName string, environments []EnvironmentStatus) ([]VaultSecretCorrelation, error) {
	correlations := make([]VaultSecretCorrelation, 0)

	// Get vault validations for each environment
	for _, env := range environments {
		vaultDetails, err := ec.store.GetVaultValidationDetails(ctx, customerID, contextName, env.Environment)
		if err != nil {
			ec.logger.Warn().Err(err).Str("environment", env.Environment).Msg("Failed to get vault validation details")
			continue
		}

		// Group by vault path and secret name
		secretMap := make(map[string]*VaultSecretCorrelation)
		
		for _, detail := range vaultDetails {
			key := fmt.Sprintf("%s/%s", detail.VaultPath, detail.SecretName)
			
			if correlation, exists := secretMap[key]; exists {
				// Add to existing correlation
				correlation.Environments = append(correlation.Environments, VaultSecretEnvironment{
					Environment:          env.Environment,
					ValidationStatus:     detail.ValidationStatus,
					KubernetesSecretName: ec.safeStringDeref(detail.KubernetesSecretName),
					Namespace:           ec.safeStringDeref(detail.KubernetesNamespace),
					RequiredKeys:         detail.RequiredKeys,
					MissingKeys:          detail.MissingKeys,
					LastSyncTime:         detail.LastValidated,
				})
			} else {
				// Create new correlation
				secretMap[key] = &VaultSecretCorrelation{
					VaultPath:  detail.VaultPath,
					SecretName: detail.SecretName,
					Environments: []VaultSecretEnvironment{
						{
							Environment:          env.Environment,
							ValidationStatus:     detail.ValidationStatus,
							KubernetesSecretName: ec.safeStringDeref(detail.KubernetesSecretName),
							Namespace:           ec.safeStringDeref(detail.KubernetesNamespace),
							RequiredKeys:         detail.RequiredKeys,
							MissingKeys:          detail.MissingKeys,
							LastSyncTime:         detail.LastValidated,
						},
					},
					KeyConsistency: make(map[string]KeyConsistency),
					LastValidated:  time.Now(),
				}
			}
		}

		// Add correlations to result
		for _, correlation := range secretMap {
			correlations = append(correlations, *correlation)
		}
	}

	// Calculate key consistency for each correlation
	for i := range correlations {
		ec.calculateKeyConsistency(&correlations[i])
	}

	return correlations, nil
}

// buildDeploymentTimeline builds a timeline of deployment events across environments
func (ec *EnvironmentCorrelator) buildDeploymentTimeline(ctx context.Context, customerID, contextName, appName string, environments []EnvironmentStatus) ([]DeploymentEvent, error) {
	timeline := make([]DeploymentEvent, 0)

	for _, env := range environments {
		if env.LastDeployed != nil {
			timeline = append(timeline, DeploymentEvent{
				Environment:  env.Environment,
				EventType:    "deploy",
				Timestamp:    *env.LastDeployed,
				HelmRevision: env.HelmRevision,
				GitCommit:    env.GitCommit,
				Status:       env.DeploymentStatus,
				TriggeredBy:  "gitops", // Would be more specific in real implementation
				Description:  fmt.Sprintf("Deployment to %s environment", env.Environment),
			})
		}
	}

	// Sort timeline by timestamp (most recent first)
	for i := 0; i < len(timeline)-1; i++ {
		for j := i + 1; j < len(timeline); j++ {
			if timeline[i].Timestamp.Before(timeline[j].Timestamp) {
				timeline[i], timeline[j] = timeline[j], timeline[i]
			}
		}
	}

	return timeline, nil
}

// compareResourceVersions compares Kubernetes resource versions across environments
func (ec *EnvironmentCorrelator) compareResourceVersions(ctx context.Context, customerID, contextName, appName string, environments []EnvironmentStatus) (map[string]ResourceVersionComparison, error) {
	comparisons := make(map[string]ResourceVersionComparison)

	// This would typically involve querying Kubernetes clusters for actual resource versions
	// For now, we'll simulate based on the stored resource versions data

	resourceTypes := []string{"Deployment", "Service", "ConfigMap", "Secret"}

	for _, resourceType := range resourceTypes {
		comparison := ResourceVersionComparison{
			ResourceType:  resourceType,
			Environments:  make(map[string]string),
			VersionDrift:  make([]ResourceVersionDrift, 0),
		}

		// Simulate resource version data
		for _, env := range environments {
			// Extract resource version from stored resource versions JSON
			if resourceVersions, ok := env.ResourceCount > 0, true; ok {
				// In real implementation, parse actual resource versions from env.ResourceVersions
				version := fmt.Sprintf("v%d", time.Now().Unix()%100) // Simulate version
				comparison.Environments[env.Environment] = version
			}
		}

		// Determine consistency and latest version
		comparison.Consistency, comparison.LatestVersion = ec.calculateResourceVersionConsistency(comparison.Environments)
		
		// Detect drift
		for envName, version := range comparison.Environments {
			if version != comparison.LatestVersion {
				comparison.VersionDrift = append(comparison.VersionDrift, ResourceVersionDrift{
					Environment:     envName,
					CurrentVersion:  version,
					ExpectedVersion: comparison.LatestVersion,
					DriftSeverity:   ec.calculateVersionDriftSeverity(version, comparison.LatestVersion),
					LastUpdated:     time.Now(),
				})
			}
		}

		comparisons[resourceType] = comparison
	}

	return comparisons, nil
}

// Helper methods for severity calculation

func (ec *EnvironmentCorrelator) calculateHelmVersionDriftSeverity(baseline, target string) string {
	// Simple heuristic - would be more sophisticated in practice
	if baseline == "" || target == "" {
		return "low"
	}
	return "medium" // Default for Helm version differences
}

func (ec *EnvironmentCorrelator) calculateImageTagDriftSeverity(baseline, target string) string {
	// Simple heuristic for image tag drift
	if baseline == "latest" || target == "latest" {
		return "high" // "latest" tags are risky
	}
	return "medium"
}

func (ec *EnvironmentCorrelator) calculateVersionDriftSeverity(current, expected string) string {
	// Placeholder logic for version drift severity
	return "medium"
}

func (ec *EnvironmentCorrelator) calculateKeyConsistency(correlation *VaultSecretCorrelation) {
	// Build key consistency map across all environments
	allKeys := make(map[string][]string) // key -> list of environments where it's present
	
	for _, env := range correlation.Environments {
		for _, key := range env.RequiredKeys {
			if _, exists := allKeys[key]; !exists {
				allKeys[key] = make([]string, 0)
			}
			allKeys[key] = append(allKeys[key], env.Environment)
		}
	}

	// Determine consistency for each key
	totalEnvs := len(correlation.Environments)
	for key, presentEnvs := range allKeys {
		missingEnvs := make([]string, 0)
		for _, env := range correlation.Environments {
			found := false
			for _, presentEnv := range presentEnvs {
				if env.Environment == presentEnv {
					found = true
					break
				}
			}
			if !found {
				missingEnvs = append(missingEnvs, env.Environment)
			}
		}

		status := "consistent"
		action := "No action required"
		
		if len(missingEnvs) > 0 {
			if len(presentEnvs) == 1 {
				status = "missing"
				action = "Consider adding key to all environments or removing from " + presentEnvs[0]
			} else {
				status = "inconsistent"
				action = "Add missing key to: " + fmt.Sprintf("%v", missingEnvs)
			}
		}

		correlation.KeyConsistency[key] = KeyConsistency{
			Key:               key,
			PresentInEnvs:     presentEnvs,
			MissingInEnvs:     missingEnvs,
			ConsistencyStatus: status,
			RecommendedAction: action,
		}
	}
}

func (ec *EnvironmentCorrelator) calculateResourceVersionConsistency(envVersions map[string]string) (string, string) {
	if len(envVersions) == 0 {
		return "unknown", ""
	}

	if len(envVersions) == 1 {
		for _, version := range envVersions {
			return "consistent", version
		}
	}

	// Find the most common version (assumes it's the latest/correct one)
	versionCounts := make(map[string]int)
	for _, version := range envVersions {
		versionCounts[version]++
	}

	maxCount := 0
	latestVersion := ""
	for version, count := range versionCounts {
		if count > maxCount {
			maxCount = count
			latestVersion = version
		}
	}

	consistency := "consistent"
	if len(versionCounts) > 1 {
		consistency = "drift"
	}

	return consistency, latestVersion
}

func (ec *EnvironmentCorrelator) safeStringDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}