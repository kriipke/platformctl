package api

import (
	"time"
)

// ArgoCD Integration Types
type ApplicationSetStatus struct {
	Name         string                   `json:"name"`
	Namespace    string                   `json:"namespace"`
	SyncStatus   string                   `json:"sync_status"`
	HealthStatus string                   `json:"health_status"`
	Message      string                   `json:"message,omitempty"`
	Conditions   []ApplicationCondition   `json:"conditions,omitempty"`
	Applications []ApplicationSetApplication `json:"applications,omitempty"`
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
	ApplicationName string     `json:"application_name"`
	SyncStatus      string     `json:"sync_status"`
	Message         string     `json:"message,omitempty"`
	SyncStartedAt   *time.Time `json:"sync_started_at,omitempty"`
	SyncFinishedAt  *time.Time `json:"sync_finished_at,omitempty"`
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
	CustomerID    string                `json:"customer_id"`
	BranchName    string                `json:"branch_name"`
	Repository    string                `json:"repository"`
	Valid         bool                  `json:"valid"`
	Errors        []string              `json:"errors,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	LastCommit    *GitCommit            `json:"last_commit,omitempty"`
	ValuesFiles   []HelmValuesFile      `json:"values_files,omitempty"`
	Environments  []EnvironmentValuesValidation `json:"environments,omitempty"`
	LastValidated time.Time             `json:"last_validated"`
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
	Environment  string                    `json:"environment"`
	Valid        bool                      `json:"valid"`
	Errors       []ValidationError         `json:"errors,omitempty"`
	Content      map[string]interface{}    `json:"content,omitempty"`
	Size         int64                     `json:"size"`
	LastModified time.Time                 `json:"last_modified"`
}

type EnvironmentValuesValidation struct {
	Environment   string            `json:"environment"`
	ValuesFile    string            `json:"values_file"`
	Valid         bool              `json:"valid"`
	RequiredKeys  []string          `json:"required_keys"`
	MissingKeys   []string          `json:"missing_keys,omitempty"`
	InvalidValues map[string]string `json:"invalid_values,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
}

// Helm Integration Types
type HelmReleaseStatus struct {
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	Version     int                  `json:"version"`
	Status      string               `json:"status"`
	Chart       HelmChartInfo        `json:"chart"`
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
	ChartName    string            `json:"chart_name"`
	ChartVersion string            `json:"chart_version"`
	Repository   string            `json:"repository"`
	Valid        bool              `json:"valid"`
	Errors       []ValidationError `json:"errors,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Dependencies []HelmDependency  `json:"dependencies,omitempty"`
	Templates    []HelmTemplate    `json:"templates,omitempty"`
	Values       HelmValuesSchema  `json:"values,omitempty"`
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
	Errors          []ValidationError       `json:"errors,omitempty"`
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
	Environment   string                 `json:"environment"`
	Namespace     string                 `json:"namespace"`
	Applications  []EnvironmentApplication `json:"applications"`
	NetworkPolicies []NetworkPolicyStatus `json:"network_policies"`
	PodCorrelations []PodEnvironmentCorrelation `json:"pod_correlations"`
	ResourceQuotas  map[string]interface{} `json:"resource_quotas"`
	LastChecked     time.Time              `json:"last_checked"`
}

type EnvironmentApplication struct {
	Name            string                 `json:"name"`
	Namespace       string                 `json:"namespace"`
	Kind            string                 `json:"kind"` // Deployment, StatefulSet, etc.
	Replicas        int32                  `json:"replicas"`
	ReadyReplicas   int32                  `json:"ready_replicas"`
	Image           string                 `json:"image"`
	Status          string                 `json:"status"`
	Resources       KubernetesResources    `json:"resources"`
	EnvVars         map[string]string      `json:"env_vars,omitempty"`
	VaultSecrets    []string               `json:"vault_secrets,omitempty"`
	LastDeployed    *time.Time             `json:"last_deployed,omitempty"`
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
	AppName      string                              `json:"app_name"`
	Environments map[string]EnvironmentWorkloadStatus `json:"environments"`
	Differences  []EnvironmentDifference             `json:"differences"`
	Summary      ComparisonSummary                   `json:"summary"`
	GeneratedAt  time.Time                          `json:"generated_at"`
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