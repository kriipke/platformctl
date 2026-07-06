package argocd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

// ArgoCDClientImpl talks to an ArgoCD server via its REST API. The gRPC / gRPC-web
// apiclient does not interoperate with every ArgoCD server configuration (see the
// stemx v2.13 server, which rejects gRPC-web), whereas the REST API is stable, so
// this client uses REST over HTTPS with a bearer token. External calls are routed
// through a circuit breaker.
type ArgoCDClientImpl struct {
	config config.ArgoCDConfig
	cb     *circuitbreaker.CircuitBreaker
	http   *http.Client
}

func NewArgoCDClient(cfg config.ArgoCDConfig) *ArgoCDClientImpl {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- ArgoCD serves a self-signed cert; opt-in via ARGOCD_INSECURE
	}
	return &ArgoCDClientImpl{
		config: cfg,
		cb:     resilience.New("argocd"),
		http:   &http.Client{Timeout: argoCDTimeout, Transport: transport},
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

// --- real implementations (REST) ---

func (ac *ArgoCDClientImpl) getApplicationSetsForApp(customerID, appName string) ([]api.ApplicationSetStatus, error) {
	q := url.Values{}
	if ac.config.Namespace != "" {
		q.Set("appsetNamespace", ac.config.Namespace)
	}

	var list v1alpha1.ApplicationSetList
	if err := ac.get("/api/v1/applicationsets", q, &list); err != nil {
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
	appset, err := ac.getApplicationSet(appSetName)
	if err != nil {
		return nil, err
	}
	return mapAppSetApplications(appset.Status.ApplicationStatus), nil
}

func (ac *ArgoCDClientImpl) validateApplicationSetTemplate(customerID, appSetName string) error {
	appset, err := ac.getApplicationSet(appSetName)
	if err != nil {
		return err
	}
	if len(appset.Spec.Generators) == 0 {
		return fmt.Errorf("applicationset %s has no generators", appSetName)
	}
	return nil
}

func (ac *ArgoCDClientImpl) syncApplicationSet(customerID, appSetName string, forceSync bool) (*api.ApplicationSetSyncResult, error) {
	appset, err := ac.getApplicationSet(appSetName)
	if err != nil {
		return nil, err
	}

	// ApplicationSets are not synced directly; sync each generated application.
	started := time.Now()
	prune := forceSync
	synced := []string{}

	for _, appStatus := range appset.Status.ApplicationStatus {
		name := appStatus.Application
		if name == "" {
			continue
		}
		body := map[string]interface{}{"name": name, "prune": prune}
		if err := ac.post("/api/v1/applications/"+url.PathEscape(name)+"/sync", body, nil); err != nil {
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

func (ac *ArgoCDClientImpl) getApplicationSet(name string) (*v1alpha1.ApplicationSet, error) {
	q := url.Values{}
	if ac.config.Namespace != "" {
		q.Set("appsetNamespace", ac.config.Namespace)
	}
	var appset v1alpha1.ApplicationSet
	if err := ac.get("/api/v1/applicationsets/"+url.PathEscape(name), q, &appset); err != nil {
		return nil, fmt.Errorf("get applicationset %s: %w", name, err)
	}
	return &appset, nil
}

// --- REST transport ---

func (ac *ArgoCDClientImpl) get(path string, query url.Values, out interface{}) error {
	base, err := ac.baseURL()
	if err != nil {
		return err
	}
	u := base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return ac.do(req, out)
}

func (ac *ArgoCDClientImpl) post(path string, body, out interface{}) error {
	base, err := ac.baseURL()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return ac.do(req, out)
}

func (ac *ArgoCDClientImpl) do(req *http.Request, out interface{}) error {
	if ac.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+ac.config.AuthToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := ac.http.Do(req)
	if err != nil {
		return fmt.Errorf("argocd request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("argocd %s %s: status %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode argocd response: %w", err)
		}
	}
	return nil
}

// baseURL returns the ArgoCD REST base URL (scheme://host[:port]) from config.
func (ac *ArgoCDClientImpl) baseURL() (string, error) {
	if !ac.config.Enabled {
		return "", fmt.Errorf("argocd integration is disabled")
	}
	if ac.config.ServerURL == "" {
		return "", fmt.Errorf("argocd server URL is not configured")
	}
	return restBaseURL(ac.config.ServerURL), nil
}

// restBaseURL normalizes a configured server address into an HTTPS base URL.
// A bare host (no scheme) is assumed to be HTTPS, since argocd-server serves TLS.
func restBaseURL(serverURL string) string {
	s := strings.TrimSpace(serverURL)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

// --- mapping helpers ---

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
