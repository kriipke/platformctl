package helm

import (
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/contextops/platformctl/internal/circuitbreaker"
	"github.com/contextops/platformctl/internal/clients/resilience"
	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/pkg/api"
)

type HelmClient interface {
	ValidateValuesFiles(customerID, environmentName string) ([]api.ValuesFileStatus, error)
	GetHelmReleases(customerID, environmentName, namespace string) ([]api.HelmReleaseStatus, error)
	ValidateHelmChart(customerID, chartName, chartVersion string) (*api.HelmChartValidation, error)
	ValidateHelmSources(customerID, appName string) ([]api.HelmSourceStatus, error)
}

// HelmClientImpl uses the Helm v3 SDK. Chart-repository operations read the
// configured repository index; release operations use a cluster-bound action
// configuration. External calls are routed through a circuit breaker.
type HelmClientImpl struct {
	config   config.HelmConfig
	settings *cli.EnvSettings
	cb       *circuitbreaker.CircuitBreaker
}

func NewHelmClient(cfg config.HelmConfig) *HelmClientImpl {
	return &HelmClientImpl{
		config:   cfg,
		settings: cli.New(),
		cb:       resilience.New("helm"),
	}
}

// ValidateValuesFiles intentionally returns no entries: per-environment GitOps
// values files (values-dev.yaml, ...) live in the Git repository, not in Helm,
// so they are validated by the Git client. This method exists to satisfy the
// interface without fabricating data.
func (hc *HelmClientImpl) ValidateValuesFiles(customerID, environmentName string) ([]api.ValuesFileStatus, error) {
	return []api.ValuesFileStatus{}, nil
}

func (hc *HelmClientImpl) GetHelmReleases(customerID, environmentName, namespace string) ([]api.HelmReleaseStatus, error) {
	return resilience.Run(hc.cb, func() ([]api.HelmReleaseStatus, error) {
		return hc.getHelmReleases(customerID, environmentName, namespace)
	})
}

func (hc *HelmClientImpl) ValidateHelmChart(customerID, chartName, chartVersion string) (*api.HelmChartValidation, error) {
	return resilience.Run(hc.cb, func() (*api.HelmChartValidation, error) {
		return hc.validateHelmChart(customerID, chartName, chartVersion)
	})
}

func (hc *HelmClientImpl) ValidateHelmSources(customerID, appName string) ([]api.HelmSourceStatus, error) {
	return resilience.Run(hc.cb, func() ([]api.HelmSourceStatus, error) {
		return hc.validateHelmSources(customerID, appName)
	})
}

// --- real implementations ---

func (hc *HelmClientImpl) validateHelmSources(customerID, appName string) ([]api.HelmSourceStatus, error) {
	idx, err := hc.repoIndex()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	source := api.HelmSourceStatus{
		Name:        fmt.Sprintf("%s-chart", appName),
		Type:        "repository",
		URL:         hc.config.RegistryURL,
		Status:      "unavailable",
		LastChecked: &now,
	}
	if versions, ok := idx.Entries[appName]; ok && len(versions) > 0 {
		source.Status = "available"
		source.Version = versions[0].Version
	}
	return []api.HelmSourceStatus{source}, nil
}

func (hc *HelmClientImpl) validateHelmChart(customerID, chartName, chartVersion string) (*api.HelmChartValidation, error) {
	idx, err := hc.repoIndex()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	validation := &api.HelmChartValidation{
		ChartName:     chartName,
		ChartVersion:  chartVersion,
		Dependencies:  []api.HelmDependency{},
		LastValidated: &now,
	}

	cv, err := idx.Get(chartName, chartVersion)
	if err != nil {
		validation.ValidationStatus = "invalid"
		return validation, nil
	}

	validation.ValidationStatus = "valid"
	for _, dep := range cv.Dependencies {
		validation.Dependencies = append(validation.Dependencies, api.HelmDependency{
			Name:       dep.Name,
			Version:    dep.Version,
			Repository: dep.Repository,
			Available:  true,
		})
	}
	return validation, nil
}

func (hc *HelmClientImpl) getHelmReleases(customerID, environmentName, namespace string) ([]api.HelmReleaseStatus, error) {
	cfg, err := hc.actionConfig(namespace)
	if err != nil {
		return nil, err
	}

	list := action.NewList(cfg)
	list.All = true
	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("list helm releases in %s: %w", namespace, err)
	}

	out := []api.HelmReleaseStatus{}
	for _, rel := range releases {
		out = append(out, mapRelease(rel, environmentName))
	}
	return out, nil
}

// --- Helm SDK helpers ---

func (hc *HelmClientImpl) repoIndex() (*repo.IndexFile, error) {
	if !hc.config.Enabled {
		return nil, fmt.Errorf("helm integration is disabled")
	}
	if hc.config.RegistryURL == "" {
		return nil, fmt.Errorf("helm registry URL is not configured")
	}

	entry := &repo.Entry{
		Name:     "configured",
		URL:      hc.config.RegistryURL,
		Username: hc.config.Username,
		Password: hc.config.Password,
	}

	cr, err := repo.NewChartRepository(entry, getter.All(hc.settings))
	if err != nil {
		return nil, fmt.Errorf("create chart repository: %w", err)
	}

	path, err := cr.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("download index for %s: %w", hc.config.RegistryURL, err)
	}

	idx, err := repo.LoadIndexFile(path)
	if err != nil {
		return nil, fmt.Errorf("load chart index: %w", err)
	}
	idx.SortEntries()
	return idx, nil
}

func (hc *HelmClientImpl) actionConfig(namespace string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	flags := genericclioptions.NewConfigFlags(false)
	flags.Namespace = &namespace

	if err := cfg.Init(flags, namespace, "secret", func(string, ...interface{}) {}); err != nil {
		return nil, fmt.Errorf("init helm action config: %w", err)
	}
	return cfg, nil
}

func mapRelease(rel *release.Release, environmentName string) api.HelmReleaseStatus {
	status := api.HelmReleaseStatus{
		Name:        rel.Name,
		Namespace:   rel.Namespace,
		Environment: environmentName,
		Revision:    rel.Version,
		Values:      rel.Config,
	}
	if rel.Info != nil {
		status.Status = rel.Info.Status.String()
		deployed := rel.Info.LastDeployed.Time
		status.Updated = &deployed
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		status.ChartName = rel.Chart.Metadata.Name
		status.ChartVersion = rel.Chart.Metadata.Version
		status.AppVersion = rel.Chart.Metadata.AppVersion
	}
	return status
}
