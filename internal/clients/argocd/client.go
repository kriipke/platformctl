package argocd

import (
	"context"
	"fmt"
	"time"

	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// ArgoCD API types
type ArgoCDApplication struct {
	APIVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Metadata   ArgoCDApplicationMetadata `json:"metadata"`
	Spec       ArgoCDApplicationSpec     `json:"spec"`
	Status     *ArgoCDApplicationStatus  `json:"status,omitempty"`
}

type ArgoCDApplicationMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ArgoCDApplicationSpec struct {
	Project     string                      `json:"project"`
	Source      ArgoCDApplicationSource     `json:"source"`
	Destination ArgoCDApplicationDestination `json:"destination"`
	SyncPolicy  *ArgoCDSyncPolicy           `json:"syncPolicy,omitempty"`
}

type ArgoCDApplicationSource struct {
	RepoURL        string                    `json:"repoURL"`
	Path           string                    `json:"path"`
	TargetRevision string                    `json:"targetRevision"`
	Helm           *ArgoCDApplicationHelm    `json:"helm,omitempty"`
}

type ArgoCDApplicationHelm struct {
	Parameters []ArgoCDHelmParameter `json:"parameters,omitempty"`
	ValueFiles []string              `json:"valueFiles,omitempty"`
}

type ArgoCDHelmParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ArgoCDApplicationDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

type ArgoCDSyncPolicy struct {
	Automated   *ArgoCDAutomatedPolicy `json:"automated,omitempty"`
	SyncOptions []string               `json:"syncOptions,omitempty"`
}

type ArgoCDAutomatedPolicy struct {
	Prune    bool `json:"prune,omitempty"`
	SelfHeal bool `json:"selfHeal,omitempty"`
}

type ArgoCDApplicationStatus struct {
	Health *ArgoCDHealthStatus `json:"health,omitempty"`
	Sync   *ArgoCDSyncStatus   `json:"sync,omitempty"`
}

type ArgoCDHealthStatus struct {
	Status string `json:"status"`
}

type ArgoCDSyncStatus struct {
	Status string `json:"status"`
}

type ArgoCDApplicationList struct {
	Items []ArgoCDApplication `json:"items"`
}

type ArgoCDClient interface {
	GetApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error)
	GetApplicationSetApplications(customerID, appSetName string) ([]api.ApplicationSetApplication, error)
	GetApplicationsForCustomer(customerID string) ([]ArgoCDApplication, error)
	GetApplicationByName(appName string) (*ArgoCDApplication, error)
	ValidateApplicationSetTemplate(customerID, appSetName string) error
	SyncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error)
}

type ArgoCDClientImpl struct {
	config       config.ArgoCDConfig
	kubeClient   dynamic.Interface
	namespace    string
}

func NewArgoCDClient(cfg config.ArgoCDConfig) *ArgoCDClientImpl {
	// Get in-cluster kubernetes config
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Failed to get in-cluster config: %v\n", err)
		return &ArgoCDClientImpl{
			config:    cfg,
			namespace: cfg.Namespace,
		}
	}

	// Create dynamic client for accessing ArgoCD Application CRDs
	kubeClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		fmt.Printf("Failed to create kubernetes client: %v\n", err)
		return &ArgoCDClientImpl{
			config:    cfg,
			namespace: cfg.Namespace,
		}
	}

	return &ArgoCDClientImpl{
		config:     cfg,
		kubeClient: kubeClient,
		namespace:  cfg.Namespace,
	}
}

// Helper to get ArgoCD Application GVR
func (ac *ArgoCDClientImpl) getApplicationGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
}

// GetApplicationsForCustomer gets all ArgoCD Applications for a customer
func (ac *ArgoCDClientImpl) GetApplicationsForCustomer(customerID string) ([]ArgoCDApplication, error) {
	if ac.kubeClient == nil {
		return nil, fmt.Errorf("kubernetes client not available")
	}

	// Query applications with customer label selector
	labelSelector := fmt.Sprintf("contextops.io/customer=%s", customerID)
	
	appGVR := ac.getApplicationGVR()
	list, err := ac.kubeClient.Resource(appGVR).Namespace(ac.namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications for customer %s: %w", customerID, err)
	}

	var applications []ArgoCDApplication
	for _, item := range list.Items {
		var app ArgoCDApplication
		
		// Extract the relevant fields from the Kubernetes object
		app.APIVersion = "argoproj.io/v1alpha1"
		app.Kind = "Application"
		
		// Parse metadata
		app.Metadata.Name = item.GetName()
		app.Metadata.Namespace = item.GetNamespace()
		app.Metadata.Labels = item.GetLabels()
		app.Metadata.Annotations = item.GetAnnotations()
		
		// Parse spec from the unstructured object
		if spec, found, err := unstructured.NestedFieldCopy(item.Object, "spec"); found && err == nil {
			if specMap, ok := spec.(map[string]interface{}); ok {
				// Parse source
				if source, found, _ := unstructured.NestedFieldCopy(specMap, "source"); found {
					if sourceMap, ok := source.(map[string]interface{}); ok {
						if repoURL, found, _ := unstructured.NestedString(sourceMap, "repoURL"); found {
							app.Spec.Source.RepoURL = repoURL
						}
						if path, found, _ := unstructured.NestedString(sourceMap, "path"); found {
							app.Spec.Source.Path = path
						}
						if targetRevision, found, _ := unstructured.NestedString(sourceMap, "targetRevision"); found {
							app.Spec.Source.TargetRevision = targetRevision
						}
						
						// Parse Helm configuration if present
						if _, found, _ := unstructured.NestedFieldCopy(sourceMap, "helm"); found {
							app.Spec.Source.Helm = &ArgoCDApplicationHelm{}
							// Note: We could parse Helm parameters and valueFiles here if needed
						}
					}
				}
				
				// Parse destination
				if destination, found, _ := unstructured.NestedFieldCopy(specMap, "destination"); found {
					if destMap, ok := destination.(map[string]interface{}); ok {
						if server, found, _ := unstructured.NestedString(destMap, "server"); found {
							app.Spec.Destination.Server = server
						}
						if namespace, found, _ := unstructured.NestedString(destMap, "namespace"); found {
							app.Spec.Destination.Namespace = namespace
						}
					}
				}
			}
		}
		
		applications = append(applications, app)
	}
	
	return applications, nil
}

// GetApplicationByName gets a specific ArgoCD Application by name
func (ac *ArgoCDClientImpl) GetApplicationByName(appName string) (*ArgoCDApplication, error) {
	if ac.kubeClient == nil {
		return nil, fmt.Errorf("kubernetes client not available")
	}

	appGVR := ac.getApplicationGVR()
	item, err := ac.kubeClient.Resource(appGVR).Namespace(ac.namespace).Get(
		context.Background(),
		appName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get application %s: %w", appName, err)
	}

	var app ArgoCDApplication
	
	// Extract the relevant fields from the Kubernetes object (same logic as above)
	app.APIVersion = "argoproj.io/v1alpha1"
	app.Kind = "Application"
	
	// Parse metadata
	app.Metadata.Name = item.GetName()
	app.Metadata.Namespace = item.GetNamespace()
	app.Metadata.Labels = item.GetLabels()
	app.Metadata.Annotations = item.GetAnnotations()
	
	// Parse spec from the unstructured object
	if spec, found, err := unstructured.NestedFieldCopy(item.Object, "spec"); found && err == nil {
		if specMap, ok := spec.(map[string]interface{}); ok {
			// Parse source
			if source, found, _ := unstructured.NestedFieldCopy(specMap, "source"); found {
				if sourceMap, ok := source.(map[string]interface{}); ok {
					if repoURL, found, _ := unstructured.NestedString(sourceMap, "repoURL"); found {
						app.Spec.Source.RepoURL = repoURL
					}
					if path, found, _ := unstructured.NestedString(sourceMap, "path"); found {
						app.Spec.Source.Path = path
					}
					if targetRevision, found, _ := unstructured.NestedString(sourceMap, "targetRevision"); found {
						app.Spec.Source.TargetRevision = targetRevision
					}
					
					// Parse Helm configuration if present
					if _, found, _ := unstructured.NestedFieldCopy(sourceMap, "helm"); found {
						app.Spec.Source.Helm = &ArgoCDApplicationHelm{}
					}
				}
			}
			
			// Parse destination
			if destination, found, _ := unstructured.NestedFieldCopy(specMap, "destination"); found {
				if destMap, ok := destination.(map[string]interface{}); ok {
					if server, found, _ := unstructured.NestedString(destMap, "server"); found {
						app.Spec.Destination.Server = server
					}
					if namespace, found, _ := unstructured.NestedString(destMap, "namespace"); found {
						app.Spec.Destination.Namespace = namespace
					}
				}
			}
		}
	}
	
	return &app, nil
}

func (ac *ArgoCDClientImpl) GetApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error) {
	// For Phase 1C, simulate ApplicationSet data
	// In a real implementation, this would:
	// 1. Connect to ArgoCD using the configured credentials
	// 2. Query ApplicationSets that match the app name or labels
	// 3. Get their current status and applications

	var applicationSets []api.ApplicationSetStatus

	// Simulate ApplicationSet for the app
	appSet := api.ApplicationSetStatus{
		Name:         fmt.Sprintf("%s-appset", appName),
		Namespace:    ac.config.Namespace,
		AppName:      appName,
		CustomerID:   customerID,
		SyncStatus:   "synced",
		HealthStatus: "healthy",
		Applications: []api.ApplicationSetApplication{},
		Generator: api.ApplicationSetGenerator{
			Type: "git",
			Parameters: map[string]interface{}{
				"repoURL": fmt.Sprintf("https://github.com/%s/%s", customerID, appName),
				"path":    "helm",
			},
		},
		LastSyncTime: timePtr(time.Now().Add(-30 * time.Minute)),
		HelmSourceStatus: []api.HelmSourceStatus{
			{
				Name:        "chart-repo",
				Type:        "registry",
				URL:         "https://charts.example.com",
				Version:     "1.0.0",
				Status:      "available",
				LastChecked: timePtr(time.Now().Add(-5 * time.Minute)),
			},
		},
		GitSourceStatus: []api.GitSourceStatus{
			{
				URL:         fmt.Sprintf("https://github.com/%s/%s", customerID, appName),
				Path:        "helm",
				Revision:    "main",
				Status:      "available",
				LastCommit:  "abc123def456",
				LastChecked: timePtr(time.Now().Add(-10 * time.Minute)),
			},
		},
	}

	// Simulate applications generated by this ApplicationSet
	environments := []string{"dev", "staging", "prod"}
	for _, env := range environments {
		app := api.ApplicationSetApplication{
			Name:         fmt.Sprintf("%s-%s", appName, env),
			Environment:  env,
			Cluster:      fmt.Sprintf("https://k8s-%s.example.com", env),
			Namespace:    fmt.Sprintf("%s-%s", customerID, env),
			SyncStatus:   "synced",
			HealthStatus: "healthy",
			GitCommit:    "abc123def456",
			HelmRevision: "1.0.0",
			LastDeployed: timePtr(time.Now().Add(-1 * time.Hour)),
		}
		appSet.Applications = append(appSet.Applications, app)
	}

	applicationSets = append(applicationSets, appSet)
	return applicationSets, nil
}

func (ac *ArgoCDClientImpl) GetApplicationSetApplications(customerID, appSetName string) ([]api.ApplicationSetApplication, error) {
	// For Phase 1C, simulate getting applications for a specific ApplicationSet
	applications := []api.ApplicationSetApplication{}

	// Simulate applications
	environments := []string{"dev", "staging", "prod"}
	for _, env := range environments {
		app := api.ApplicationSetApplication{
			Name:         fmt.Sprintf("%s-%s", appSetName, env),
			Environment:  env,
			Cluster:      fmt.Sprintf("https://k8s-%s.example.com", env),
			Namespace:    fmt.Sprintf("%s-%s", customerID, env),
			SyncStatus:   "synced",
			HealthStatus: "healthy",
			GitCommit:    "abc123def456",
			HelmRevision: "1.0.0",
			LastDeployed: timePtr(time.Now().Add(-2 * time.Hour)),
		}
		applications = append(applications, app)
	}

	return applications, nil
}

func (ac *ArgoCDClientImpl) ValidateApplicationSetTemplate(customerID, appSetName string) error {
	// For Phase 1C, simulate template validation
	// In a real implementation, this would:
	// 1. Get the ApplicationSet template
	// 2. Validate the template syntax
	// 3. Check if referenced repositories and paths exist
	// 4. Validate the generator configuration

	if !ac.config.Enabled {
		return fmt.Errorf("ArgoCD is not enabled")
	}

	// Simulate validation - always pass for now
	return nil
}

func (ac *ArgoCDClientImpl) SyncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error) {
	// For Phase 1C, simulate ApplicationSet sync
	// In a real implementation, this would:
	// 1. Connect to ArgoCD
	// 2. Trigger sync for the ApplicationSet
	// 3. Wait for sync to complete or return async result
	// 4. Return sync status and any generated applications

	syncResult := &api.ApplicationSetSyncResult{
		Name:        appSetName,
		SyncStarted: func() *time.Time { t := time.Now().UTC(); return &t }(),
		Status:      "syncing",
		Applications: []string{
			fmt.Sprintf("%s-dev", appSetName),
			fmt.Sprintf("%s-staging", appSetName),
			fmt.Sprintf("%s-prod", appSetName),
		},
	}

	// Simulate sync completion after a short delay
	// In real implementation, this might be async
	time.Sleep(100 * time.Millisecond)
	syncResult.Status = "synced"

	return syncResult, nil
}

// Helm source validation methods
func (ac *ArgoCDClientImpl) ValidateHelmSources(customerID, appName string) ([]api.HelmSourceStatus, error) {
	var sources []api.HelmSourceStatus

	// Simulate Helm source validation
	source := api.HelmSourceStatus{
		Name:        "helm-chart",
		Type:        "registry",
		URL:         "https://charts.example.com",
		Version:     "1.0.0",
		Status:      "available",
		LastChecked: timePtr(time.Now().Add(-5 * time.Minute)),
	}

	sources = append(sources, source)
	return sources, nil
}

// Git source validation methods
func (ac *ArgoCDClientImpl) ValidateGitSources(customerID, appName string) ([]api.GitSourceStatus, error) {
	var sources []api.GitSourceStatus

	// Simulate Git source validation
	source := api.GitSourceStatus{
		URL:         fmt.Sprintf("https://github.com/%s/%s", customerID, appName),
		Path:        "helm",
		Revision:    "main",
		Status:      "available",
		LastCommit:  "abc123def456",
		LastChecked: timePtr(time.Now().Add(-10 * time.Minute)),
	}

	sources = append(sources, source)
	return sources, nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}