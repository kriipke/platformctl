package api

import (
	"time"

	"github.com/google/uuid"
)

type GitOpsMessageEnvelope struct {
	SchemaVersion    int                    `json:"schema_version"`
	MessageID        string                 `json:"message_id"`
	CorrelationID    string                 `json:"correlation_id"`
	CustomerID       string                 `json:"customer_id"`
	ContextName      string                 `json:"context_name"`
	Action           string                 `json:"action"`
	RequestedBy      string                 `json:"requested_by"`
	RequestedAt      time.Time              `json:"requested_at"`
	ManifestType     string                 `json:"manifest_type"` // app, environment, context
	AppName          string                 `json:"app_name,omitempty"`
	EnvironmentName  string                 `json:"environment_name,omitempty"`
	Priority         int                    `json:"priority"` // 1-10, higher is more urgent
	Payload          map[string]interface{} `json:"payload"`
	ManifestMetadata ManifestMetadata       `json:"manifest_metadata"`
}

type ManifestMetadata struct {
	// App manifest metadata
	ApplicationSetName string           `json:"applicationset_name,omitempty"`
	HelmSources        []HelmSourceInfo `json:"helm_sources,omitempty"`
	GitSources         []GitSourceInfo  `json:"git_sources,omitempty"`

	// Environment manifest metadata
	VaultSources   []VaultSourceInfo `json:"vault_sources,omitempty"`
	ClusterConfigs []ClusterInfo     `json:"cluster_configs,omitempty"`
	ValuesFiles    []string          `json:"values_files,omitempty"`

	// Context pairing metadata
	AppReference         string `json:"app_reference,omitempty"`
	EnvironmentReference string `json:"environment_reference,omitempty"`
	CustomerBranch       string `json:"customer_branch,omitempty"`
}

type HelmSourceInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // registry, git, oci
	URL     string `json:"url"`
	Version string `json:"version,omitempty"`
}

type GitSourceInfo struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}

type VaultSourceInfo struct {
	Path       string `json:"path"`
	SecretName string `json:"secret_name"`
}

type ClusterInfo struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

type GitOpsCommandMessage struct {
	GitOpsMessageEnvelope
	CommandType   string            `json:"command_type"`   // sync, validate, inspect, correlate
	TargetService string            `json:"target_service"` // app-sync-service, environment-validator, context-correlator
	Timeout       time.Duration     `json:"timeout"`
	RetryPolicy   GitOpsRetryPolicy `json:"retry_policy"`
}

type GitOpsResultMessage struct {
	GitOpsMessageEnvelope
	ServiceName             string                     `json:"service_name"`
	Status                  string                     `json:"status"` // healthy, degraded, unhealthy, error
	CompletedAt             time.Time                  `json:"completed_at"`
	ErrorMessage            string                     `json:"error_message,omitempty"`
	ResultPayload           interface{}                `json:"result_payload"`
	AppManifestData         *AppManifestResult         `json:"app_manifest_data,omitempty"`
	EnvironmentManifestData *EnvironmentManifestResult `json:"environment_manifest_data,omitempty"`
	ContextPairingData      *ContextPairingResult      `json:"context_pairing_data,omitempty"`
	PerformanceMetrics      GitOpsPerformanceMetrics   `json:"performance_metrics"`
}

type GitOpsRetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	RetryDelay      time.Duration `json:"retry_delay"`
	BackoffStrategy string        `json:"backoff_strategy"` // linear, exponential
}

type AppManifestResult struct {
	AppName            string                  `json:"app_name"`
	ApplicationSetName string                  `json:"applicationset_name"`
	Namespace          string                  `json:"namespace"`
	SyncStatus         string                  `json:"sync_status"`
	HealthStatus       string                  `json:"health_status"`
	HelmSources        []HelmSourceStatus      `json:"helm_sources"`
	GitSources         []GitSourceStatus       `json:"git_sources"`
	Applications       []ApplicationStatus     `json:"applications"`
	LastSyncTime       *time.Time              `json:"last_sync_time,omitempty"`
	Generator          ApplicationSetGenerator `json:"generator"`
}

type HelmSourceStatus struct {
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	URL         string     `json:"url"`
	Version     string     `json:"version"`
	Status      string     `json:"status"` // available, unavailable, error
	LastChecked *time.Time `json:"last_checked,omitempty"`
}

type GitSourceStatus struct {
	URL         string     `json:"url"`
	Path        string     `json:"path"`
	Revision    string     `json:"revision"`
	Status      string     `json:"status"` // available, unavailable, error
	LastCommit  string     `json:"last_commit,omitempty"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
}

type ApplicationStatus struct {
	Name         string     `json:"name"`
	Environment  string     `json:"environment"`
	Cluster      string     `json:"cluster"`
	Namespace    string     `json:"namespace"`
	SyncStatus   string     `json:"sync_status"`
	HealthStatus string     `json:"health_status"`
	LastDeployed *time.Time `json:"last_deployed,omitempty"`
	HelmRevision string     `json:"helm_revision,omitempty"`
}

type ApplicationSetGenerator struct {
	Type       string                 `json:"type"` // git, clusters, list
	Parameters map[string]interface{} `json:"parameters"`
}

type EnvironmentManifestResult struct {
	EnvironmentName    string                    `json:"environment_name"`
	VaultValidations   []VaultValidationResult   `json:"vault_validations"`
	ClusterValidations []ClusterValidationResult `json:"cluster_validations"`
	ValuesFileStatus   []ValuesFileStatus        `json:"values_file_status"`
	LastValidated      time.Time                 `json:"last_validated"`
}

type VaultValidationResult struct {
	VaultPath         string                   `json:"vault_path"`
	SecretName        string                   `json:"secret_name"`
	ValidationStatus  string                   `json:"validation_status"` // valid, invalid, missing, error
	MissingKeys       []string                 `json:"missing_keys,omitempty"`
	ExtraKeys         []string                 `json:"extra_keys,omitempty"`
	PodEnvValidations []PodEnvValidationResult `json:"pod_env_validations"`
	LastValidated     time.Time                `json:"last_validated"`
}

type ClusterValidationResult struct {
	ClusterName      string    `json:"cluster_name"`
	Server           string    `json:"server"`
	Namespace        string    `json:"namespace"`
	ConnectionStatus string    `json:"connection_status"` // connected, disconnected, error
	LastChecked      time.Time `json:"last_checked"`
}

type ValuesFileStatus struct {
	FilePath     string     `json:"file_path"`
	Status       string     `json:"status"` // available, missing, error
	LastModified *time.Time `json:"last_modified,omitempty"`
	Size         int64      `json:"size,omitempty"`
}

type PodEnvValidationResult struct {
	PodName          string `json:"pod_name"`
	Namespace        string `json:"namespace"`
	EnvVarName       string `json:"env_var_name"`
	SecretRef        string `json:"secret_ref,omitempty"`
	SecretKey        string `json:"secret_key,omitempty"`
	ExpectedValue    string `json:"expected_value,omitempty"`
	ActualValue      string `json:"actual_value,omitempty"`
	ValidationStatus string `json:"validation_status"` // match, mismatch, missing
	ErrorMessage     string `json:"error_message,omitempty"`
}

type ContextPairingResult struct {
	ContextName          string                 `json:"context_name"`
	AppReference         string                 `json:"app_reference"`
	EnvironmentReference string                 `json:"environment_reference"`
	PairingStatus        string                 `json:"pairing_status"` // valid, invalid, missing_app, missing_environment
	SyncStatus           string                 `json:"sync_status"`
	HealthStatus         string                 `json:"health_status"`
	CorrelationData      map[string]interface{} `json:"correlation_data"`
	ResourceCount        int                    `json:"resource_count"`
	LastDeploymentTime   *time.Time             `json:"last_deployment_time,omitempty"`
	ValidationErrors     []string               `json:"validation_errors,omitempty"`
}

type GitOpsPerformanceMetrics struct {
	ProcessingTimeMs int64   `json:"processing_time_ms"`
	ApiCallsCount    int     `json:"api_calls_count"`
	CacheHitRate     float64 `json:"cache_hit_rate,omitempty"`
}

// GitOps Message creation helpers
func generateUUID() string {
	return uuid.New().String()
}

func NewGitOpsCommandMessage(customerID, contextName, action, manifestType, user string) *GitOpsCommandMessage {
	return &GitOpsCommandMessage{
		GitOpsMessageEnvelope: GitOpsMessageEnvelope{
			SchemaVersion:    1,
			MessageID:        generateUUID(),
			CorrelationID:    generateUUID(),
			CustomerID:       customerID,
			ContextName:      contextName,
			Action:           action,
			ManifestType:     manifestType,
			RequestedBy:      user,
			RequestedAt:      time.Now().UTC(),
			Priority:         5, // Default priority
			Payload:          make(map[string]interface{}),
			ManifestMetadata: ManifestMetadata{},
		},
		CommandType: "sync",          // Default command type
		Timeout:     5 * time.Minute, // Default timeout
		RetryPolicy: GitOpsRetryPolicy{
			MaxRetries:      3,
			RetryDelay:      30 * time.Second,
			BackoffStrategy: "exponential",
		},
	}
}

func NewAppManifestCommandMessage(customerID, contextName, appName, user string) *GitOpsCommandMessage {
	cmd := NewGitOpsCommandMessage(customerID, contextName, "sync-app", "app", user)
	cmd.TargetService = "app-sync-service"
	cmd.AppName = appName
	cmd.Priority = 8 // High priority for App manifest synchronization
	return cmd
}

func NewEnvironmentManifestCommandMessage(customerID, contextName, environmentName, user string) *GitOpsCommandMessage {
	cmd := NewGitOpsCommandMessage(customerID, contextName, "validate-environment", "environment", user)
	cmd.TargetService = "environment-validator"
	cmd.EnvironmentName = environmentName
	cmd.Priority = 7 // High priority for Environment manifest validation
	return cmd
}

func NewContextPairingCommandMessage(customerID, contextName, user string) *GitOpsCommandMessage {
	cmd := NewGitOpsCommandMessage(customerID, contextName, "correlate-context", "context", user)
	cmd.TargetService = "context-correlator"
	cmd.Priority = 6 // Medium-high priority for Context pairing correlation
	return cmd
}
