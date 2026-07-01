package argocd

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/kriipke/platformctl/internal/circuitbreaker"
	"github.com/kriipke/platformctl/internal/clients/resilience"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/pkg/api"
)

const argoCDTimeout = 30 * time.Second

type ArgoCDClient interface {
	GetApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error)
	GetApplicationSetApplications(customerID, appSetName string) ([]api.ApplicationSetApplication, error)
	ValidateApplicationSetTemplate(customerID, appSetName string) error
	SyncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error)
}

// ArgoCDClientImpl talks to an ArgoCD server via its gRPC API client. External
// calls are routed through a circuit breaker.
type ArgoCDClientImpl struct {
	config config.ArgoCDConfig
	cb     *circuitbreaker.CircuitBreaker
}

func NewArgoCDClient(cfg config.ArgoCDConfig) *ArgoCDClientImpl {
	return &ArgoCDClientImpl{
		config: cfg,
		cb:     resilience.New("argocd"),
	}
}

func (ac *ArgoCDClientImpl) GetApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error) {
	return resilience.Run(ac.cb, func() ([]api.ApplicationSetStatus, error) {
		return ac.getApplicationSetsForApp(customerID, appName)
	})
}

func (ac *ArgoCDClientImpl) GetApplicationSetApplications(customerID, appSetName string) ([]api.ApplicationSetApplication, error) {
	return resilience.Run(ac.cb, func() ([]api.ApplicationSetApplication, error) {
		return ac.getApplicationSetApplications(customerID, appSetName)
	})
}

func (ac *ArgoCDClientImpl) ValidateApplicationSetTemplate(customerID, appSetName string) error {
	_, err := resilience.Run(ac.cb, func() (struct{}, error) {
		return struct{}{}, ac.validateApplicationSetTemplate(customerID, appSetName)
	})
	return err
}

func (ac *ArgoCDClientImpl) SyncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error) {
	return resilience.Run(ac.cb, func() (*api.ApplicationSetSyncResult, error) {
		return ac.syncApplicationSet(customerID, appSetName, forceSync)
	})
}

// --- real implementations ---

func (ac *ArgoCDClientImpl) getApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error) {
	client, err := ac.newClient()
	if err != nil {
		return nil, err
	}
	closer, asClient, err := client.NewApplicationSetClient()
	if err != nil {
		return nil, fmt.Errorf("create applicationset client: %w", err)
	}
	defer closer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), argoCDTimeout)
	defer cancel()

	list, err := asClient.List(ctx, &applicationsetpkg.ApplicationSetListQuery{
		AppsetNamespace: ac.config.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("list applicationsets: %w", err)
	}

	out := []api.ApplicationSetStatus{}
	for i := range list.Items {
		appset := &list.Items[i]
		if appName != "" && !strings.Contains(appset.Name, appName) {
			continue
		}
		out = append(out, mapApplicationSet(appset, customerID, appName))
	}
	return out, nil
}

func (ac *ArgoCDClientImpl) getApplicationSetApplications(customerID, appSetName string) ([]api.ApplicationSetApplication, error) {
	client, err := ac.newClient()
	if err != nil {
		return nil, err
	}
	closer, asClient, err := client.NewApplicationSetClient()
	if err != nil {
		return nil, fmt.Errorf("create applicationset client: %w", err)
	}
	defer closer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), argoCDTimeout)
	defer cancel()

	appset, err := asClient.Get(ctx, &applicationsetpkg.ApplicationSetGetQuery{
		Name:            appSetName,
		AppsetNamespace: ac.config.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("get applicationset %s: %w", appSetName, err)
	}
	return mapAppSetApplications(appset.Status.ApplicationStatus), nil
}

func (ac *ArgoCDClientImpl) validateApplicationSetTemplate(customerID, appSetName string) error {
	client, err := ac.newClient()
	if err != nil {
		return err
	}
	closer, asClient, err := client.NewApplicationSetClient()
	if err != nil {
		return fmt.Errorf("create applicationset client: %w", err)
	}
	defer closer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), argoCDTimeout)
	defer cancel()

	appset, err := asClient.Get(ctx, &applicationsetpkg.ApplicationSetGetQuery{
		Name:            appSetName,
		AppsetNamespace: ac.config.Namespace,
	})
	if err != nil {
		return fmt.Errorf("get applicationset %s: %w", appSetName, err)
	}
	if len(appset.Spec.Generators) == 0 {
		return fmt.Errorf("applicationset %s has no generators", appSetName)
	}
	return nil
}

func (ac *ArgoCDClientImpl) syncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error) {
	client, err := ac.newClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), argoCDTimeout)
	defer cancel()

	// Discover the applications generated by the ApplicationSet.
	asCloser, asClient, err := client.NewApplicationSetClient()
	if err != nil {
		return nil, fmt.Errorf("create applicationset client: %w", err)
	}
	defer asCloser.Close()

	appset, err := asClient.Get(ctx, &applicationsetpkg.ApplicationSetGetQuery{
		Name:            appSetName,
		AppsetNamespace: ac.config.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("get applicationset %s: %w", appSetName, err)
	}

	// ApplicationSets are not synced directly; sync each generated application.
	appCloser, appClient, err := client.NewApplicationClient()
	if err != nil {
		return nil, fmt.Errorf("create application client: %w", err)
	}
	defer appCloser.Close()

	started := time.Now()
	prune := forceSync
	synced := []string{}

	for _, appStatus := range appset.Status.ApplicationStatus {
		name := appStatus.Application
		if name == "" {
			continue
		}
		if _, err := appClient.Sync(ctx, &applicationpkg.ApplicationSyncRequest{
			Name:  &name,
			Prune: &prune,
		}); err != nil {
			return nil, fmt.Errorf("sync application %s: %w", name, err)
		}
		synced = append(synced, name)
	}

	finished := time.Now()
	return &api.ApplicationSetSyncResult{
		Name:            appSetName,
		ApplicationName: appSetName,
		SyncStatus:      "synced",
		Status:          "succeeded",
		SyncStarted:     &started,
		SyncStartedAt:   &started,
		SyncFinishedAt:  &finished,
		Applications:    synced,
	}, nil
}

// --- client + mapping helpers ---

func (ac *ArgoCDClientImpl) newClient() (apiclient.Client, error) {
	if !ac.config.Enabled {
		return nil, fmt.Errorf("argocd integration is disabled")
	}
	if ac.config.ServerURL == "" {
		return nil, fmt.Errorf("argocd server URL is not configured")
	}
	return apiclient.NewClient(&apiclient.ClientOptions{
		ServerAddr: serverAddr(ac.config.ServerURL),
		AuthToken:  ac.config.AuthToken,
		Insecure:   ac.config.Insecure,
		GRPCWeb:    true,
	})
}

func mapApplicationSet(appset *v1alpha1.ApplicationSet, customerID, appName string) api.ApplicationSetStatus {
	status := api.ApplicationSetStatus{
		Name:         appset.Name,
		Namespace:    appset.Namespace,
		AppName:      appName,
		CustomerID:   customerID,
		SyncStatus:   appSetSyncStatus(appset),
		HealthStatus: appSetHealth(appset),
		Applications: mapAppSetApplications(appset.Status.ApplicationStatus),
		Conditions:   []api.ApplicationCondition{},
	}
	if len(appset.Spec.Generators) > 0 {
		status.Generator = generatorType(appset.Spec.Generators[0])
	}
	return status
}

func mapAppSetApplications(statuses []v1alpha1.ApplicationSetApplicationStatus) []api.ApplicationSetApplication {
	out := []api.ApplicationSetApplication{}
	for _, s := range statuses {
		out = append(out, api.ApplicationSetApplication{
			Name:         s.Application,
			SyncStatus:   s.Status,
			HealthStatus: s.Status,
		})
	}
	return out
}

// appSetHealth reports "degraded" when the ApplicationSet has an error
// condition, otherwise "healthy".
func appSetHealth(appset *v1alpha1.ApplicationSet) string {
	for _, c := range appset.Status.Conditions {
		if c.Type == v1alpha1.ApplicationSetConditionErrorOccurred &&
			c.Status == v1alpha1.ApplicationSetConditionStatusTrue {
			return "degraded"
		}
	}
	return "healthy"
}

// appSetSyncStatus aggregates the per-application statuses into a single value.
func appSetSyncStatus(appset *v1alpha1.ApplicationSet) string {
	statuses := appset.Status.ApplicationStatus
	if len(statuses) == 0 {
		return "unknown"
	}
	for _, s := range statuses {
		if s.Status != "Healthy" && s.Status != "Synced" {
			return "out_of_sync"
		}
	}
	return "synced"
}

func generatorType(g v1alpha1.ApplicationSetGenerator) string {
	switch {
	case g.Git != nil:
		return "git"
	case g.Clusters != nil:
		return "clusters"
	case g.List != nil:
		return "list"
	case g.Matrix != nil:
		return "matrix"
	case g.Merge != nil:
		return "merge"
	case g.SCMProvider != nil:
		return "scmProvider"
	case g.PullRequest != nil:
		return "pullRequest"
	default:
		return "unknown"
	}
}

// serverAddr strips the scheme and trailing slash so the ArgoCD client receives
// a host:port address.
func serverAddr(serverURL string) string {
	s := strings.TrimPrefix(serverURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.TrimRight(s, "/")
}
