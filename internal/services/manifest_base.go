package services

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/kriipke/platformctl/internal/auth"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/storage"
	"github.com/kriipke/platformctl/pkg/api"
)

type ManifestBaseService struct {
	Name              string
	Consumer          *events.CommandConsumer
	Publisher         *events.GitOpsCommandPublisher
	AppStore          *storage.AppStore
	EnvironmentStore  *storage.EnvironmentStore
	ContextStore      *storage.ContextStore
	CustomerValidator *auth.CustomerValidator
	ServiceMetrics    *ManifestServiceMetrics
}

func NewManifestBaseService(name string, gmb *events.GitOpsMessageBus, appStore *storage.AppStore, envStore *storage.EnvironmentStore, contextStore *storage.ContextStore) *ManifestBaseService {
	consumer := events.NewCommandConsumer(gmb, fmt.Sprintf("gitops.%s.q", name))
	publisher := events.NewGitOpsCommandPublisher(gmb)

	return &ManifestBaseService{
		Name:              name,
		Consumer:          consumer,
		Publisher:         publisher,
		AppStore:          appStore,
		EnvironmentStore:  envStore,
		ContextStore:      contextStore,
		CustomerValidator: auth.NewCustomerValidator(),
		ServiceMetrics:    NewManifestServiceMetrics(name),
	}
}

func (mbs *ManifestBaseService) Start(handler ManifestCommandHandler) error {
	// Register metrics and health checks
	mbs.ServiceMetrics.RegisterMetrics()

	// Start consuming manifest commands with customer isolation
	return mbs.Consumer.StartWithCustomerIsolation(handler)
}

func (mbs *ManifestBaseService) Stop() error {
	mbs.ServiceMetrics.Shutdown()
	return mbs.Consumer.Stop()
}

func (mbs *ManifestBaseService) ValidateCustomerAccess(cmd *api.GitOpsCommandMessage) error {
	return mbs.CustomerValidator.ValidateAccess(cmd.CustomerID, cmd.ContextName)
}

func (mbs *ManifestBaseService) GetContext(customerID, contextName string) (*models.Context, error) {
	// Customer-isolated context retrieval
	return mbs.ContextStore.Get(context.Background(), contextName, customerID)
}

func (mbs *ManifestBaseService) GetApp(customerID, appName string) (*models.App, error) {
	// Customer-isolated App manifest retrieval
	return mbs.AppStore.Get(context.Background(), appName, customerID)
}

func (mbs *ManifestBaseService) GetEnvironment(customerID, environmentName string) (*models.Environment, error) {
	// Customer-isolated Environment manifest retrieval
	return mbs.EnvironmentStore.Get(context.Background(), environmentName, customerID)
}

type ManifestCommandHandler interface {
	HandleManifestCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error)
	GetSupportedManifestTypes() []string
	GetSupportedActions() []string
}

type ManifestServiceMetrics struct {
	ServiceName        string
	CommandsProcessed  prometheus.Counter
	ErrorsTotal        prometheus.Counter
	ProcessingTime     prometheus.Histogram
	CustomerRequests   *prometheus.CounterVec
	ManifestOperations *prometheus.CounterVec
}

func NewManifestServiceMetrics(serviceName string) *ManifestServiceMetrics {
	return &ManifestServiceMetrics{
		ServiceName: serviceName,
		CommandsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("manifest_%s_commands_processed_total", serviceName),
			Help: fmt.Sprintf("Total number of manifest commands processed by %s service", serviceName),
		}),
		ErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("manifest_%s_errors_total", serviceName),
			Help: fmt.Sprintf("Total number of errors in %s service", serviceName),
		}),
		ProcessingTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: fmt.Sprintf("manifest_%s_processing_seconds", serviceName),
			Help: fmt.Sprintf("Time spent processing manifest commands in %s service", serviceName),
		}),
		CustomerRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("manifest_%s_customer_requests_total", serviceName),
				Help: fmt.Sprintf("Total requests by customer for %s service", serviceName),
			},
			[]string{"customer_id", "manifest_type", "action"},
		),
		ManifestOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("manifest_%s_operations_total", serviceName),
				Help: fmt.Sprintf("Total manifest operations by type for %s service", serviceName),
			},
			[]string{"manifest_type", "operation", "status"},
		),
	}
}

func (msm *ManifestServiceMetrics) RegisterMetrics() {
	prometheus.MustRegister(msm.CommandsProcessed)
	prometheus.MustRegister(msm.ErrorsTotal)
	prometheus.MustRegister(msm.ProcessingTime)
	prometheus.MustRegister(msm.CustomerRequests)
	prometheus.MustRegister(msm.ManifestOperations)
}

func (msm *ManifestServiceMetrics) RecordCommand(customerID, manifestType, action string, duration time.Duration, success bool) {
	msm.CommandsProcessed.Inc()
	msm.ProcessingTime.Observe(duration.Seconds())
	msm.CustomerRequests.WithLabelValues(customerID, manifestType, action).Inc()

	status := "success"
	if !success {
		msm.ErrorsTotal.Inc()
		status = "error"
	}

	msm.ManifestOperations.WithLabelValues(manifestType, action, status).Inc()
}

func (msm *ManifestServiceMetrics) Shutdown() {
	// Cleanup metrics if needed
}