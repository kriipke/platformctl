package api

import (
	"time"
)

// ArgoCD Integration Types
type ApplicationSetStatus struct {
	Name         string                   `json:"name"`
	Namespace    string                   `json:"namespace"`
	AppName      string                   `json:"app_name,omitempty"`
	CustomerID   string                   `json:"customer_id,omitempty"`
	SyncStatus   string                   `json:"sync_status"`
	HealthStatus string                   `json:"health_status"`
	Message      string                   `json:"message,omitempty"`
	Generator    interface{}              `json:"generator,omitempty"`
	Conditions   []ApplicationCondition   `json:"conditions,omitempty"`
	Applications []ApplicationSetApplication `json:"applications,omitempty"`
	HelmSourceStatus interface{}         `json:"helm_source_status,omitempty"`
	GitSourceStatus  interface{}         `json:"git_source_status,omitempty"`
	LastSyncTime *time.Time               `json:"last_sync_time,omitempty"`
}

type ApplicationSetApplication struct {
	Name         string                 `json:"name"`
	Environment  string                 `json:"environment"`
	Cluster      string                 `json:"cluster"`
	Namespace    string                 `json:"namespace"`
	SyncStatus   string                 `json:"sync_status"`
	HealthStatus string                 `json:"health_status"`
	Source       ApplicationSource      `json:"source"`
	Destination  ApplicationDestination `json:"destination"`
	GitCommit    string                 `json:"git_commit,omitempty"`
	HelmRevision string                 `json:"helm_revision,omitempty"`
	LastDeployed *time.Time             `json:"last_deployed,omitempty"`
}

type ApplicationSource struct {
	RepoURL        string            `json:"repo_url"`
	Path           string            `json:"path"`
	TargetRevision string            `json:"target_revision"`
	Chart          string            `json:"chart,omitempty"`
	Helm           *HelmSource       `json:"helm,omitempty"`
	Directory      *DirectorySource  `json:"directory,omitempty"`
}

type ApplicationDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
	Name      string `json:"name,omitempty"`
}

type HelmSource struct {
	ValueFiles   []string          `json:"value_files,omitempty"`
	Parameters   []HelmParameter   `json:"parameters,omitempty"`
	ReleaseName  string            `json:"release_name,omitempty"`
	Values       string            `json:"values,omitempty"`
}

type DirectorySource struct {
	Recurse bool                   `json:"recurse,omitempty"`
	Jsonnet *DirectoryJsonnetOpts  `json:"jsonnet,omitempty"`
}

type DirectoryJsonnetOpts struct {
	ExtVars []JsonnetVar `json:"ext_vars,omitempty"`
	TLAs    []JsonnetVar `json:"tlas,omitempty"`
}

type JsonnetVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Code  bool   `json:"code,omitempty"`
}

type HelmParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ApplicationCondition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	LastTransitionTime *time.Time `json:"last_transition_time,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
}

type ApplicationSetSyncResult struct {
	Name            string     `json:"name,omitempty"`
	ApplicationName string     `json:"application_name"`
	SyncStatus      string     `json:"sync_status"`
	Status          string     `json:"status,omitempty"`
	SyncStarted     *time.Time `json:"sync_started,omitempty"`
	Message         string     `json:"message,omitempty"`
	SyncStartedAt   *time.Time `json:"sync_started_at,omitempty"`
	SyncFinishedAt  *time.Time `json:"sync_finished_at,omitempty"`
	Applications    []string      `json:"applications,omitempty"`
	Resources       []ResourceResult `json:"resources,omitempty"`
}

type ResourceResult struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// Git Integration Types
type CustomerBranchValidation struct {
	CustomerID       string                `json:"customer_id"`
	BranchName       string                `json:"branch_name"`
	Repository       string                `json:"repository"`
	BranchPattern    string                `json:"branch_pattern,omitempty"`
	PatternCompliant bool                  `json:"pattern_compliant"`
	BranchExists     bool                  `json:"branch_exists"`
	Valid            bool                  `json:"valid"`
	ValidationStatus string                `json:"validation_status,omitempty"`
	ErrorMessage     string                `json:"error_message,omitempty"`
	Errors           []string              `json:"errors,omitempty"`
	Warnings         []string              `json:"warnings,omitempty"`
	LastCommit       *GitCommit            `json:"last_commit,omitempty"`
	ValuesFiles      []HelmValuesFile      `json:"values_files,omitempty"`
	HelmValuesFiles  []HelmValuesFile      `json:"helm_values_files,omitempty"`
	EnvironmentFiles []EnvironmentValuesValidation `json:"environment_files,omitempty"`
	Environments     []EnvironmentValuesValidation `json:"environments,omitempty"`
	LastValidated    time.Time             `json:"last_validated"`
}

type GitCommit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url,omitempty"`
}

type HelmValuesFile struct {
	Path         string                    `json:"path"`
	FileName     string                    `json:"file_name,omitempty"`
	FilePath     string                    `json:"file_path,omitempty"`
	Environment  string                    `json:"environment"`
	FileSize     int64                     `json:"file_size,omitempty"`
	Valid        bool                      `json:"valid"`
	SHA          string                    `json:"sha,omitempty"`
	IsValid      bool                      `json:"is_valid"`
	Errors       []string                  `json:"errors,omitempty"`
	Content      map[string]interface{}    `json:"content,omitempty"`
	Size         int64                     `json:"size"`
	LastModified time.Time                 `json:"last_modified"`
}

type EnvironmentValuesValidation struct {
	Environment        string            `json:"environment"`
	ValuesFile         string            `json:"values_file"`
	ExpectedFileName   string            `json:"expected_file_name,omitempty"`
	ActualFileName     string            `json:"actual_file_name,omitempty"`
	FileExists         bool              `json:"file_exists"`
	FileValid          bool              `json:"file_valid"`
	Valid              bool              `json:"valid"`
	RequiredKeys       []string          `json:"required_keys"`
	MissingKeys        []string          `json:"missing_keys,omitempty"`
	InvalidValues      map[string]string `json:"invalid_values,omitempty"`
	ValidationErrors   []string          `json:"validation_errors,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
}

// Helm Integration Types
type HelmReleaseStatus struct {
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	Environment string               `json:"environment,omitempty"`
	ChartName   string               `json:"chart_name,omitempty"`
	ChartVersion string              `json:"chart_version,omitempty"`
	AppVersion  string               `json:"app_version,omitempty"`
	Revision    int                  `json:"revision,omitempty"`
	Version     int                  `json:"version"`
	Status      string               `json:"status"`
	Updated     *time.Time           `json:"updated,omitempty"`
	Chart       HelmChartInfo        `json:"chart"`
	ComputedValues map[string]interface{} `json:"computed_values,omitempty"`
	SourceValuesFile string           `json:"source_values_file,omitempty"`
	ValuesFileHash string             `json:"values_file_hash,omitempty"`
	Values      map[string]interface{} `json:"values,omitempty"`
	Manifest    string               `json:"manifest,omitempty"`
	Notes       string               `json:"notes,omitempty"`
	FirstDeployed *time.Time         `json:"first_deployed,omitempty"`
	LastDeployed  *time.Time         `json:"last_deployed,omitempty"`
	Description   string             `json:"description,omitempty"`
}

type HelmChartInfo struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	AppVersion string            `json:"app_version,omitempty"`
	Keywords   []string          `json:"keywords,omitempty"`
	Sources    []string          `json:"sources,omitempty"`
	Urls       []string          `json:"urls,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type HelmChartValidation struct {
	ChartName        string            `json:"chart_name"`
	ChartVersion     string            `json:"chart_version"`
	Repository       string            `json:"repository"`
	ValidationStatus string            `json:"validation_status,omitempty"`
	Valid            bool              `json:"valid"`
	Errors           []string          `json:"errors,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Dependencies     []HelmDependency  `json:"dependencies,omitempty"`
	Templates        []HelmTemplate    `json:"templates,omitempty"`
	TemplateCount    int               `json:"template_count,omitempty"`
	Values           HelmValuesSchema  `json:"values,omitempty"`
	LastValidated    *time.Time        `json:"last_validated,omitempty"`
}

type HelmDependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Condition  string `json:"condition,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Enabled    bool   `json:"enabled"`
	Available  bool   `json:"available"`
}

type HelmTemplate struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Valid   bool   `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Kind    string `json:"kind,omitempty"`
	APIVersion string `json:"api_version,omitempty"`
}

type HelmValuesSchema struct {
	Schema     map[string]interface{} `json:"schema,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Kubernetes Integration Types
type KubernetesClusterStatus struct {
	Name          string                    `json:"name"`
	Server        string                    `json:"server"`
	Version       string                    `json:"version"`
	Status        string                    `json:"status"`
	Nodes         []KubernetesNodeStatus    `json:"nodes,omitempty"`
	Namespaces    []KubernetesNamespaceStatus `json:"namespaces,omitempty"`
	LastChecked   time.Time                 `json:"last_checked"`
	ErrorMessage  string                    `json:"error_message,omitempty"`
}

type KubernetesNodeStatus struct {
	Name      string               `json:"name"`
	Status    string               `json:"status"`
	Roles     []string             `json:"roles"`
	Version   string               `json:"version"`
	Resources KubernetesResources  `json:"resources"`
}

type KubernetesNamespaceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
}

type KubernetesResources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pods   string `json:"pods"`
}

// Vault Integration Types
type VaultSecretStatus struct {
	Path         string                    `json:"path"`
	SecretName   string                    `json:"secret_name"`
	Exists       bool                      `json:"exists"`
	Keys         []string                  `json:"keys,omitempty"`
	LastUpdated  *time.Time                `json:"last_updated,omitempty"`
	Version      int                       `json:"version,omitempty"`
	Metadata     map[string]interface{}    `json:"metadata,omitempty"`
	PodEnvChecks []PodEnvValidationResult  `json:"pod_env_checks,omitempty"`
}

type VaultSecretValidation struct {
	Path            string                  `json:"path"`
	SecretName      string                  `json:"secret_name"`
	Valid           bool                    `json:"valid"`
	RequiredKeys    []string                `json:"required_keys"`
	MissingKeys     []string                `json:"missing_keys,omitempty"`
	UnexpectedKeys  []string                `json:"unexpected_keys,omitempty"`
	Errors          []string                `json:"errors,omitempty"`
	PodValidations  []PodEnvValidationResult `json:"pod_validations,omitempty"`
	LastValidated   time.Time               `json:"last_validated"`
}

// New Relic Integration Types
type NewRelicApplicationStatus struct {
	ID             int64                  `json:"id"`
	Name           string                 `json:"name"`
	Language       string                 `json:"language"`
	HealthStatus   string                 `json:"health_status"`
	Reporting      bool                   `json:"reporting"`
	LastReportedAt *time.Time             `json:"last_reported_at,omitempty"`
	Settings       NewRelicSettings       `json:"settings,omitempty"`
	Links          NewRelicLinks          `json:"links,omitempty"`
}

type NewRelicSettings struct {
	AppApdexThreshold     float64 `json:"app_apdex_threshold"`
	EndUserApdexThreshold float64 `json:"end_user_apdex_threshold"`
	EnableRealUserMonitoring bool `json:"enable_real_user_monitoring"`
}

type NewRelicLinks struct {
	ServerIDs     []int64 `json:"server_ids,omitempty"`
	HostIDs       []int64 `json:"host_ids,omitempty"`
	InstanceIDs   []int64 `json:"instance_ids,omitempty"`
}

type NewRelicMetric struct {
	Name        string                 `json:"name"`
	Values      map[string]interface{} `json:"values"`
	Timeslice   NewRelicTimeslice      `json:"timeslice"`
}

type NewRelicTimeslice struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Additional Kubernetes Integration Types
type EnvironmentWorkloadStatus struct {
	Environment        string                 `json:"environment"`
	Namespace          string                 `json:"namespace"`
	ClusterName        string                 `json:"cluster_name,omitempty"`
	CustomerID         string                 `json:"customer_id,omitempty"`
	Applications       []EnvironmentApplication `json:"applications"`
	NetworkPolicies    []NetworkPolicyStatus `json:"network_policies"`
	PodCorrelations    []PodEnvironmentCorrelation `json:"pod_correlations"`
	SecretCorrelations []PodEnvironmentCorrelation `json:"secret_correlations,omitempty"`
	ResourceQuotas     map[string]interface{} `json:"resource_quotas"`
	LastChecked        time.Time              `json:"last_checked"`
	LastUpdated        time.Time              `json:"last_updated,omitempty"`
}

type EnvironmentApplication struct {
	Name            string                 `json:"name"`
	Namespace       string                 `json:"namespace"`
	Environment     string                 `json:"environment,omitempty"`
	Kind            string                 `json:"kind"` // Deployment, StatefulSet, etc.
	Replicas        int32                  `json:"replicas"`
	ReadyReplicas   int32                  `json:"ready_replicas"`
	PodCount        int32                  `json:"pod_count,omitempty"`
	ReadyPodCount   int32                  `json:"ready_pod_count,omitempty"`
	OverallStatus   string                 `json:"overall_status,omitempty"`
	Image           string                 `json:"image"`
	Status          string                 `json:"status"`
	Deployments     []DeploymentStatus     `json:"deployments,omitempty"`
	Services        []ServiceStatus        `json:"services,omitempty"`
	Ingresses       []IngressStatus        `json:"ingresses,omitempty"`
	ConfigMaps      []ConfigMapStatus      `json:"config_maps,omitempty"`
	Secrets         []SecretStatus         `json:"secrets,omitempty"`
	Resources       KubernetesResources    `json:"resources"`
	EnvVars         map[string]string      `json:"env_vars,omitempty"`
	VaultSecrets    []string               `json:"vault_secrets,omitempty"`
	LastDeployed    *time.Time             `json:"last_deployed,omitempty"`
}

type DeploymentStatus struct {
	Name            string    `json:"name"`
	Namespace       string    `json:"namespace"`
	Replicas        int32     `json:"replicas"`
	ReadyReplicas   int32     `json:"ready_replicas"`
	UpdatedReplicas int32     `json:"updated_replicas"`
	Status          string    `json:"status"`
	Image           string    `json:"image"`
	LastUpdated     *time.Time `json:"last_updated,omitempty"`
}

type ServiceStatus struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	ClusterIP string            `json:"cluster_ip"`
	Ports     []ServicePort     `json:"ports,omitempty"`
	Status    string            `json:"status"`
}

type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"target_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

type IngressStatus struct {
	Name      string              `json:"name"`
	Namespace string              `json:"namespace"`
	Class     string              `json:"class,omitempty"`
	Rules     []IngressRule       `json:"rules,omitempty"`
	Status    string              `json:"status"`
}

type IngressRule struct {
	Host  string        `json:"host,omitempty"`
	Paths []IngressPath `json:"paths,omitempty"`
}

type IngressPath struct {
	Path        string `json:"path"`
	PathType    string `json:"path_type,omitempty"`
	ServiceName string `json:"service_name"`
	ServicePort int32  `json:"service_port"`
}

type ConfigMapStatus struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Keys      []string          `json:"keys,omitempty"`
	Size      int               `json:"size"`
	Status    string            `json:"status"`
}

type SecretStatus struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	Keys      []string `json:"keys,omitempty"`
	Status    string   `json:"status"`
}

type NetworkPolicyStatus struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	PodSelector map[string]string `json:"pod_selector"`
	Ingress     []NetworkRule     `json:"ingress,omitempty"`
	Egress      []NetworkRule     `json:"egress,omitempty"`
	Status      string            `json:"status"`
}

type NetworkRule struct {
	From  []NetworkPeer `json:"from,omitempty"`
	To    []NetworkPeer `json:"to,omitempty"`
	Ports []NetworkPort `json:"ports,omitempty"`
}

type NetworkPeer struct {
	PodSelector       map[string]string `json:"pod_selector,omitempty"`
	NamespaceSelector map[string]string `json:"namespace_selector,omitempty"`
	IPBlock           *NetworkIPBlock   `json:"ip_block,omitempty"`
}

type NetworkIPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

type NetworkPort struct {
	Protocol string `json:"protocol,omitempty"`
	Port     int32  `json:"port,omitempty"`
	EndPort  int32  `json:"end_port,omitempty"`
}

type PodEnvironmentCorrelation struct {
	PodName          string                    `json:"pod_name"`
	Namespace        string                    `json:"namespace"`
	Environment      string                    `json:"environment"`
	ExpectedEnvVars  map[string]string         `json:"expected_env_vars"`
	ActualEnvVars    map[string]string         `json:"actual_env_vars"`
	MissingVars      []string                  `json:"missing_vars,omitempty"`
	ExtraVars        []string                  `json:"extra_vars,omitempty"`
	SecretRefs       []PodSecretReference      `json:"secret_refs"`
	VaultValidations []VaultSecretValidation   `json:"vault_validations"`
	Status           string                    `json:"status"` // valid, invalid, missing_secrets
}

type PodSecretReference struct {
	SecretName string            `json:"secret_name"`
	Keys       map[string]string `json:"keys"` // env_var_name -> secret_key
	Status     string            `json:"status"`
}

type MultiEnvironmentComparison struct {
	AppName                string                              `json:"app_name"`
	CustomerID             string                              `json:"customer_id,omitempty"`
	ContextName            string                              `json:"context_name,omitempty"`
	Environments           map[string]EnvironmentWorkloadStatus `json:"environments"`
	EnvironmentStatuses    []EnvironmentWorkloadStatus         `json:"environment_statuses,omitempty"`
	Differences            []EnvironmentDifference             `json:"differences"`
	CrossEnvironmentDrift  []EnvironmentDrift                  `json:"cross_environment_drift,omitempty"`
	Summary                ComparisonSummary                   `json:"summary"`
	ComparisonSummary      EnvironmentComparisonSummary        `json:"comparison_summary,omitempty"`
	GeneratedAt            time.Time                          `json:"generated_at"`
	LastCompared           *time.Time                          `json:"last_compared,omitempty"`
}

type EnvironmentDifference struct {
	Type        string      `json:"type"` // config, image, replicas, env_vars, secrets
	Resource    string      `json:"resource"`
	Field       string      `json:"field"`
	Values      map[string]interface{} `json:"values"` // environment -> value
	Severity    string      `json:"severity"` // info, warning, critical
	Recommended string      `json:"recommended,omitempty"`
}

type ComparisonSummary struct {
	TotalDifferences int               `json:"total_differences"`
	BySeverity       map[string]int    `json:"by_severity"`
	ByType           map[string]int    `json:"by_type"`
	Environments     []string          `json:"environments"`
}

type EnvironmentDrift struct {
	SourceEnvironment string      `json:"source_environment"`
	TargetEnvironment string      `json:"target_environment"`
	DriftType         string      `json:"drift_type"` // config, version, scale, etc.
	Resource          string      `json:"resource"`
	Field             string      `json:"field"`
	SourceValue       interface{} `json:"source_value"`
	TargetValue       interface{} `json:"target_value"`
	Severity          string      `json:"severity"`
	Impact            string      `json:"impact,omitempty"`
}

type EnvironmentComparisonSummary struct {
	TotalEnvironments   int                    `json:"total_environments"`
	HealthyEnvironments int                    `json:"healthy_environments"`
	DriftDetected       bool                   `json:"drift_detected"`
	HighSeverityIssues  int                    `json:"high_severity_issues"`
	DriftByType         map[string]int         `json:"drift_by_type"`
	LastComparisonTime  time.Time              `json:"last_comparison_time"`
	RecommendedActions  []string               `json:"recommended_actions,omitempty"`
}

// Pod Environment Validation Types - Extended Results
type EnvVarValidationResult struct {
	Name           string `json:"name"`
	Expected       string `json:"expected"`
	Actual         string `json:"actual"`
	Source         string `json:"source"` // configmap, secret, direct
	Valid          bool   `json:"valid"`
	Error          string `json:"error,omitempty"`
}

type SecretValidationResult struct {
	SecretName     string `json:"secret_name"`
	VaultPath      string `json:"vault_path,omitempty"`
	Keys           []string `json:"keys"`
	Valid          bool   `json:"valid"`
	Error          string `json:"error,omitempty"`
}

// Values Environment Correlation Types
type ValuesEnvironmentCorrelation struct {
	Environment       string                 `json:"environment"`
	ValuesFile        string                 `json:"values_file"`
	FilePath          string                 `json:"file_path,omitempty"`
	CustomerBranch    string                 `json:"customer_branch,omitempty"`
	CorrelationStatus string                 `json:"correlation_status"`
	DeployedValues    map[string]interface{} `json:"deployed_values,omitempty"`
	Differences       []ValuesDifference     `json:"differences,omitempty"`
	LastCorrelated    time.Time              `json:"last_correlated"`
	CorrelationScore  float64                `json:"correlation_score,omitempty"`
}

type ValuesDifference struct {
	Key            string      `json:"key"`
	ExpectedValue  interface{} `json:"expected_value"`
	ActualValue    interface{} `json:"actual_value"`
	DifferenceType string      `json:"difference_type"` // added, removed, modified
	Severity       string      `json:"severity"`        // low, medium, high
}