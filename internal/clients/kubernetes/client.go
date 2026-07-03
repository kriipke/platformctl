package kubernetes

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/pkg/api"
)

type KubernetesClient interface {
	ValidateClusterConfigs(customerID, environmentName string) ([]api.ClusterValidationResult, error)
	GetWorkloadStatus(customerID, environmentName, namespace string) (*api.EnvironmentWorkloadStatus, error)
	GetMultiEnvironmentComparison(customerID string, environments []string) (*api.MultiEnvironmentComparison, error)
}

type MultiClusterClient struct {
	clients map[string]*kubernetes.Clientset
	configs map[string]*rest.Config
	config  *config.Config
}

func NewMultiClusterClient(cfg *config.Config) *MultiClusterClient {
	return &MultiClusterClient{
		clients: make(map[string]*kubernetes.Clientset),
		configs: make(map[string]*rest.Config),
		config:  cfg,
	}
}

func (mc *MultiClusterClient) ValidateClusterConfigs(customerID, environmentName string) ([]api.ClusterValidationResult, error) {
	var validations []api.ClusterValidationResult

	// For Phase 1C, simulate cluster validation
	// In a real implementation, this would:
	// 1. Get Environment manifest for the customer
	// 2. Extract cluster configurations
	// 3. Try to connect to each cluster
	// 4. Validate RBAC permissions

	// Simulate cluster validation for common environments
	environments := []string{"dev", "staging", "prod"}
	if environmentName != "" {
		environments = []string{environmentName}
	}

	for _, env := range environments {
		validation := api.ClusterValidationResult{
			ClusterName:      fmt.Sprintf("%s-%s-cluster", customerID, env),
			Server:           fmt.Sprintf("https://k8s-%s.example.com", env),
			Namespace:        fmt.Sprintf("%s-%s", customerID, env),
			ConnectionStatus: "connected",
			LastChecked:      time.Now().UTC(),
		}

		// Try to create a client for this environment
		client, err := mc.getClusterClient(env)
		if err != nil {
			validation.ConnectionStatus = "error"
		} else if client != nil {
			// Test the connection
			_, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
			if err != nil {
				validation.ConnectionStatus = "disconnected"
			}
		} else {
			validation.ConnectionStatus = "disconnected"
		}

		validations = append(validations, validation)
	}

	return validations, nil
}

func (mc *MultiClusterClient) GetWorkloadStatus(customerID, environmentName, namespace string) (*api.EnvironmentWorkloadStatus, error) {
	status := &api.EnvironmentWorkloadStatus{
		Environment:        environmentName,
		ClusterName:        fmt.Sprintf("%s-%s-cluster", customerID, environmentName),
		Namespace:          namespace,
		CustomerID:         customerID,
		Applications:       []api.EnvironmentApplication{},
		ResourceQuotas:     nil,
		NetworkPolicies:    []api.NetworkPolicyStatus{},
		SecretCorrelations: []api.PodEnvironmentCorrelation{},
		LastUpdated:        time.Now().UTC(),
	}

	// Try to get actual workload data if we have a client
	client, err := mc.getClusterClient(environmentName)
	if err != nil || client == nil {
		// Return simulated data for Phase 1C
		return mc.simulateWorkloadStatus(customerID, environmentName, namespace), nil
	}

	ctx := context.Background()

	// Get deployments
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, deployment := range deployments.Items {
			app := api.EnvironmentApplication{
				Name:          deployment.Name,
				Environment:   environmentName,
				PodCount:      int32(*deployment.Spec.Replicas),
				ReadyPodCount: int32(deployment.Status.ReadyReplicas),
				OverallStatus: "healthy",
				Deployments:   []api.DeploymentStatus{},
				Services:      []api.ServiceStatus{},
				Ingresses:     []api.IngressStatus{},
				ConfigMaps:    []api.ConfigMapStatus{},
				Secrets:       []api.SecretStatus{},
			}

			if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
				app.OverallStatus = "degraded"
			}

			status.Applications = append(status.Applications, app)
		}
	}

	return status, nil
}

func (mc *MultiClusterClient) GetMultiEnvironmentComparison(customerID string, environments []string) (*api.MultiEnvironmentComparison, error) {
	envMap := make(map[string]api.EnvironmentWorkloadStatus)
	for _, env := range environments {
		envMap[env] = api.EnvironmentWorkloadStatus{
			Environment: env,
			Namespace:   fmt.Sprintf("app-%s", env),
		}
	}

	comparison := &api.MultiEnvironmentComparison{
		CustomerID:            customerID,
		ContextName:           "", // Would be passed in
		Environments:          envMap,
		EnvironmentStatuses:   []api.EnvironmentWorkloadStatus{},
		CrossEnvironmentDrift: []api.EnvironmentDrift{},
		ComparisonSummary: api.EnvironmentComparisonSummary{
			TotalEnvironments:   len(environments),
			HealthyEnvironments: len(environments),
			DriftDetected:       false,
			HighSeverityIssues:  0,
			DriftByType:         make(map[string]int),
			LastComparisonTime:  time.Now().UTC(),
			RecommendedActions:  []string{},
		},
		LastCompared: func() *time.Time { t := time.Now().UTC(); return &t }(),
	}

	// Get status for each environment
	for _, env := range environments {
		status, err := mc.GetWorkloadStatus(customerID, env, fmt.Sprintf("%s-%s", customerID, env))
		if err != nil {
			continue
		}
		comparison.EnvironmentStatuses = append(comparison.EnvironmentStatuses, *status)
		// Update summary metrics based on status
		if status.Applications != nil {
			// Count healthy vs unhealthy applications
			for _, app := range status.Applications {
				if app.OverallStatus != "healthy" {
					comparison.ComparisonSummary.HighSeverityIssues++
					if comparison.ComparisonSummary.HealthyEnvironments > 0 {
						comparison.ComparisonSummary.HealthyEnvironments--
					}
					comparison.ComparisonSummary.DriftDetected = true
					break
				}
			}
		}
	}

	return comparison, nil
}

func (mc *MultiClusterClient) getClusterClient(environment string) (*kubernetes.Clientset, error) {
	if client, exists := mc.clients[environment]; exists {
		return client, nil
	}

	// For Phase 1C, try to create a client using in-cluster config or kubeconfig
	var kubeConfig *rest.Config
	var err error

	// Try in-cluster config first
	kubeConfig, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config for %s: %w", environment, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client for %s: %w", environment, err)
	}

	mc.clients[environment] = clientset
	mc.configs[environment] = kubeConfig

	return clientset, nil
}

func (mc *MultiClusterClient) simulateWorkloadStatus(customerID, environmentName, namespace string) *api.EnvironmentWorkloadStatus {
	return &api.EnvironmentWorkloadStatus{
		Environment: environmentName,
		ClusterName: fmt.Sprintf("%s-%s-cluster", customerID, environmentName),
		Namespace:   namespace,
		CustomerID:  customerID,
		Applications: []api.EnvironmentApplication{
			{
				Name:          fmt.Sprintf("app-%s", environmentName),
				Environment:   environmentName,
				PodCount:      3,
				ReadyPodCount: 3,
				OverallStatus: "healthy",
			},
		},
		LastUpdated: time.Now().UTC(),
	}
}
