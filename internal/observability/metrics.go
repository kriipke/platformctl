package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics provides a comprehensive Prometheus metrics collection for GitOps operations
type Metrics struct {
	registry        *prometheus.Registry
	serviceName     string
	
	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpActiveRequests   *prometheus.GaugeVec
	
	// Command processing metrics
	commandsProcessedTotal     *prometheus.CounterVec
	commandProcessingDuration  *prometheus.HistogramVec
	commandsActiveProcessing   *prometheus.GaugeVec
	
	// External API metrics
	externalAPICallsTotal    *prometheus.CounterVec
	externalAPICallDuration  *prometheus.HistogramVec
	externalAPICallErrors    *prometheus.CounterVec
	
	// RabbitMQ messaging metrics
	messagesPublishedTotal   *prometheus.CounterVec
	messagesConsumedTotal    *prometheus.CounterVec
	messageProcessingDuration *prometheus.HistogramVec
	messageProcessingErrors   *prometheus.CounterVec
	
	// Business metrics - Context operations
	contextsTotal           *prometheus.GaugeVec
	contextHealthStatus     *prometheus.GaugeVec
	contextSyncStatus       *prometheus.GaugeVec
	contextPairingStatus    *prometheus.GaugeVec
	
	// Business metrics - GitOps operations
	applicationSetsTotal        *prometheus.GaugeVec
	generatedApplicationsTotal  *prometheus.GaugeVec
	vaultSecretsValidated      *prometheus.CounterVec
	vaultSecretsValidationErrors *prometheus.CounterVec
	
	// Performance metrics
	databaseConnectionsActive    *prometheus.GaugeVec
	databaseQueryDuration       *prometheus.HistogramVec
	cacheHitRate               *prometheus.GaugeVec
	processingQueueLength      *prometheus.GaugeVec
	
	// Resource metrics
	kubernetesResourcesTracked  *prometheus.GaugeVec
	helmReleasesTracked        *prometheus.GaugeVec
	gitRepositoriesTracked     *prometheus.GaugeVec
	customerBranchesTracked    *prometheus.GaugeVec
}

// MetricsConfig contains configuration for metrics collection
type MetricsConfig struct {
	Enabled     bool   `env:"METRICS_ENABLED" envDefault:"true"`
	Port        string `env:"METRICS_PORT" envDefault:"9090"`
	Path        string `env:"METRICS_PATH" envDefault:"/metrics"`
	ServiceName string `env:"SERVICE_NAME" envDefault:"platformctl"`
	Namespace   string `env:"METRICS_NAMESPACE" envDefault:"platformctl"`
}

// NewMetrics creates a new Prometheus metrics collector with GitOps-specific metrics
func NewMetrics(config MetricsConfig) *Metrics {
	registry := prometheus.NewRegistry()
	
	// Common labels for all metrics
	commonLabels := []string{"service", "customer_id"}
	
	metrics := &Metrics{
		registry:    registry,
		serviceName: config.ServiceName,
		
		// HTTP metrics
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests processed",
			},
			append(commonLabels, "method", "endpoint", "status_code"),
		),
		
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Name:      "http_request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds",
				Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			append(commonLabels, "method", "endpoint"),
		),
		
		httpActiveRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "http_active_requests",
				Help:      "Number of HTTP requests currently being processed",
			},
			append(commonLabels, "method", "endpoint"),
		),
		
		// Command processing metrics
		commandsProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "commands_processed_total",
				Help:      "Total number of GitOps commands processed",
			},
			append(commonLabels, "action", "manifest_type", "status"),
		),
		
		commandProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Name:      "command_processing_duration_seconds",
				Help:      "Duration of GitOps command processing in seconds",
				Buckets:   []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
			},
			append(commonLabels, "action", "manifest_type"),
		),
		
		commandsActiveProcessing: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "commands_active_processing",
				Help:      "Number of GitOps commands currently being processed",
			},
			append(commonLabels, "action", "manifest_type"),
		),
		
		// External API metrics
		externalAPICallsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "external_api_calls_total",
				Help:      "Total number of external API calls made",
			},
			append(commonLabels, "api", "endpoint", "status_code"),
		),
		
		externalAPICallDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Name:      "external_api_call_duration_seconds",
				Help:      "Duration of external API calls in seconds",
				Buckets:   []float64{0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
			append(commonLabels, "api", "endpoint"),
		),
		
		externalAPICallErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "external_api_call_errors_total",
				Help:      "Total number of external API call errors",
			},
			append(commonLabels, "api", "endpoint", "error_type"),
		),
		
		// RabbitMQ messaging metrics
		messagesPublishedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "messages_published_total",
				Help:      "Total number of messages published to RabbitMQ",
			},
			append(commonLabels, "exchange", "routing_key", "message_type"),
		),
		
		messagesConsumedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "messages_consumed_total",
				Help:      "Total number of messages consumed from RabbitMQ",
			},
			append(commonLabels, "queue", "message_type", "status"),
		),
		
		messageProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Name:      "message_processing_duration_seconds",
				Help:      "Duration of message processing in seconds",
				Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
			},
			append(commonLabels, "queue", "message_type"),
		),
		
		messageProcessingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "message_processing_errors_total",
				Help:      "Total number of message processing errors",
			},
			append(commonLabels, "queue", "message_type", "error_type"),
		),
		
		// Business metrics - Context operations
		contextsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "contexts_total",
				Help:      "Total number of contexts by customer",
			},
			[]string{"customer_id", "health_status"},
		),
		
		contextHealthStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "context_health_status",
				Help:      "Context health status distribution",
			},
			[]string{"customer_id", "context_name", "health_status"},
		),
		
		contextSyncStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "context_sync_status",
				Help:      "Context sync status distribution",
			},
			[]string{"customer_id", "context_name", "sync_status"},
		),
		
		contextPairingStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "context_pairing_status",
				Help:      "Context pairing status distribution",
			},
			[]string{"customer_id", "context_name", "pairing_status"},
		),
		
		// Business metrics - GitOps operations
		applicationSetsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "applicationsets_total",
				Help:      "Total number of ApplicationSets being tracked",
			},
			[]string{"customer_id", "context_name", "namespace", "generator_type"},
		),
		
		generatedApplicationsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "generated_applications_total",
				Help:      "Total number of applications generated from ApplicationSets",
			},
			[]string{"customer_id", "context_name", "environment", "health_status", "sync_status"},
		),
		
		vaultSecretsValidated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "vault_secrets_validated_total",
				Help:      "Total number of Vault secrets validated",
			},
			[]string{"customer_id", "context_name", "environment", "validation_status"},
		),
		
		vaultSecretsValidationErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Name:      "vault_secrets_validation_errors_total",
				Help:      "Total number of Vault secret validation errors",
			},
			[]string{"customer_id", "context_name", "environment", "error_type"},
		),
		
		// Performance metrics
		databaseConnectionsActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "database_connections_active",
				Help:      "Number of active database connections",
			},
			[]string{"service", "database_name"},
		),
		
		databaseQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Name:      "database_query_duration_seconds",
				Help:      "Duration of database queries in seconds",
				Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0},
			},
			[]string{"service", "query_type", "customer_id"},
		),
		
		cacheHitRate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "cache_hit_rate",
				Help:      "Cache hit rate percentage",
			},
			[]string{"service", "cache_type"},
		),
		
		processingQueueLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "processing_queue_length",
				Help:      "Length of processing queues",
			},
			[]string{"service", "queue_name"},
		),
		
		// Resource metrics
		kubernetesResourcesTracked: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "kubernetes_resources_tracked_total",
				Help:      "Total number of Kubernetes resources being tracked",
			},
			[]string{"customer_id", "context_name", "environment", "resource_type"},
		),
		
		helmReleasesTracked: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "helm_releases_tracked_total",
				Help:      "Total number of Helm releases being tracked",
			},
			[]string{"customer_id", "context_name", "environment", "release_status"},
		),
		
		gitRepositoriesTracked: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "git_repositories_tracked_total",
				Help:      "Total number of Git repositories being tracked",
			},
			[]string{"customer_id", "context_name", "repository_type"},
		),
		
		customerBranchesTracked: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Name:      "customer_branches_tracked_total",
				Help:      "Total number of customer branches being tracked",
			},
			[]string{"customer_id", "context_name", "branch_compliance"},
		),
	}
	
	// Register all metrics with the registry
	metrics.registerMetrics()
	
	return metrics
}

// registerMetrics registers all metrics with the Prometheus registry
func (m *Metrics) registerMetrics() {
	// HTTP metrics
	m.registry.MustRegister(m.httpRequestsTotal)
	m.registry.MustRegister(m.httpRequestDuration)
	m.registry.MustRegister(m.httpActiveRequests)
	
	// Command processing metrics
	m.registry.MustRegister(m.commandsProcessedTotal)
	m.registry.MustRegister(m.commandProcessingDuration)
	m.registry.MustRegister(m.commandsActiveProcessing)
	
	// External API metrics
	m.registry.MustRegister(m.externalAPICallsTotal)
	m.registry.MustRegister(m.externalAPICallDuration)
	m.registry.MustRegister(m.externalAPICallErrors)
	
	// RabbitMQ messaging metrics
	m.registry.MustRegister(m.messagesPublishedTotal)
	m.registry.MustRegister(m.messagesConsumedTotal)
	m.registry.MustRegister(m.messageProcessingDuration)
	m.registry.MustRegister(m.messageProcessingErrors)
	
	// Business metrics
	m.registry.MustRegister(m.contextsTotal)
	m.registry.MustRegister(m.contextHealthStatus)
	m.registry.MustRegister(m.contextSyncStatus)
	m.registry.MustRegister(m.contextPairingStatus)
	m.registry.MustRegister(m.applicationSetsTotal)
	m.registry.MustRegister(m.generatedApplicationsTotal)
	m.registry.MustRegister(m.vaultSecretsValidated)
	m.registry.MustRegister(m.vaultSecretsValidationErrors)
	
	// Performance metrics
	m.registry.MustRegister(m.databaseConnectionsActive)
	m.registry.MustRegister(m.databaseQueryDuration)
	m.registry.MustRegister(m.cacheHitRate)
	m.registry.MustRegister(m.processingQueueLength)
	
	// Resource metrics
	m.registry.MustRegister(m.kubernetesResourcesTracked)
	m.registry.MustRegister(m.helmReleasesTracked)
	m.registry.MustRegister(m.gitRepositoriesTracked)
	m.registry.MustRegister(m.customerBranchesTracked)
}

// HTTP Metrics Methods

// IncrementHTTPRequests increments the HTTP requests counter
func (m *Metrics) IncrementHTTPRequests(customerID, method, endpoint string, statusCode int) {
	m.httpRequestsTotal.WithLabelValues(
		m.serviceName, customerID, method, endpoint, strconv.Itoa(statusCode),
	).Inc()
}

// RecordHTTPDuration records HTTP request duration
func (m *Metrics) RecordHTTPDuration(customerID, method, endpoint string, duration time.Duration) {
	m.httpRequestDuration.WithLabelValues(
		m.serviceName, customerID, method, endpoint,
	).Observe(duration.Seconds())
}

// IncrementHTTPActiveRequests increments active HTTP requests gauge
func (m *Metrics) IncrementHTTPActiveRequests(customerID, method, endpoint string) {
	m.httpActiveRequests.WithLabelValues(
		m.serviceName, customerID, method, endpoint,
	).Inc()
}

// DecrementHTTPActiveRequests decrements active HTTP requests gauge
func (m *Metrics) DecrementHTTPActiveRequests(customerID, method, endpoint string) {
	m.httpActiveRequests.WithLabelValues(
		m.serviceName, customerID, method, endpoint,
	).Dec()
}

// Command Processing Metrics Methods

// IncrementCommandsProcessed increments the commands processed counter
func (m *Metrics) IncrementCommandsProcessed(customerID, action, manifestType, status string) {
	m.commandsProcessedTotal.WithLabelValues(
		m.serviceName, customerID, action, manifestType, status,
	).Inc()
}

// RecordCommandProcessingDuration records command processing duration
func (m *Metrics) RecordCommandProcessingDuration(customerID, action, manifestType string, duration time.Duration) {
	m.commandProcessingDuration.WithLabelValues(
		m.serviceName, customerID, action, manifestType,
	).Observe(duration.Seconds())
}

// IncrementCommandsActiveProcessing increments active command processing gauge
func (m *Metrics) IncrementCommandsActiveProcessing(customerID, action, manifestType string) {
	m.commandsActiveProcessing.WithLabelValues(
		m.serviceName, customerID, action, manifestType,
	).Inc()
}

// DecrementCommandsActiveProcessing decrements active command processing gauge
func (m *Metrics) DecrementCommandsActiveProcessing(customerID, action, manifestType string) {
	m.commandsActiveProcessing.WithLabelValues(
		m.serviceName, customerID, action, manifestType,
	).Dec()
}

// External API Metrics Methods

// IncrementExternalAPICalls increments external API calls counter
func (m *Metrics) IncrementExternalAPICalls(customerID, api, endpoint string, statusCode int) {
	m.externalAPICallsTotal.WithLabelValues(
		m.serviceName, customerID, api, endpoint, strconv.Itoa(statusCode),
	).Inc()
}

// RecordExternalAPICallDuration records external API call duration
func (m *Metrics) RecordExternalAPICallDuration(customerID, api, endpoint string, duration time.Duration) {
	m.externalAPICallDuration.WithLabelValues(
		m.serviceName, customerID, api, endpoint,
	).Observe(duration.Seconds())
}

// IncrementExternalAPICallErrors increments external API call errors
func (m *Metrics) IncrementExternalAPICallErrors(customerID, api, endpoint, errorType string) {
	m.externalAPICallErrors.WithLabelValues(
		m.serviceName, customerID, api, endpoint, errorType,
	).Inc()
}

// RabbitMQ Messaging Metrics Methods

// IncrementMessagesPublished increments messages published counter
func (m *Metrics) IncrementMessagesPublished(customerID, exchange, routingKey, messageType string) {
	m.messagesPublishedTotal.WithLabelValues(
		m.serviceName, customerID, exchange, routingKey, messageType,
	).Inc()
}

// IncrementMessagesConsumed increments messages consumed counter
func (m *Metrics) IncrementMessagesConsumed(customerID, queue, messageType, status string) {
	m.messagesConsumedTotal.WithLabelValues(
		m.serviceName, customerID, queue, messageType, status,
	).Inc()
}

// RecordMessageProcessingDuration records message processing duration
func (m *Metrics) RecordMessageProcessingDuration(customerID, queue, messageType string, duration time.Duration) {
	m.messageProcessingDuration.WithLabelValues(
		m.serviceName, customerID, queue, messageType,
	).Observe(duration.Seconds())
}

// IncrementMessageProcessingErrors increments message processing errors
func (m *Metrics) IncrementMessageProcessingErrors(customerID, queue, messageType, errorType string) {
	m.messageProcessingErrors.WithLabelValues(
		m.serviceName, customerID, queue, messageType, errorType,
	).Inc()
}

// Business Metrics Methods

// SetContextsTotal sets the total number of contexts
func (m *Metrics) SetContextsTotal(customerID, healthStatus string, count int) {
	m.contextsTotal.WithLabelValues(customerID, healthStatus).Set(float64(count))
}

// SetContextHealthStatus sets context health status
func (m *Metrics) SetContextHealthStatus(customerID, contextName, healthStatus string, value float64) {
	m.contextHealthStatus.WithLabelValues(customerID, contextName, healthStatus).Set(value)
}

// SetContextSyncStatus sets context sync status
func (m *Metrics) SetContextSyncStatus(customerID, contextName, syncStatus string, value float64) {
	m.contextSyncStatus.WithLabelValues(customerID, contextName, syncStatus).Set(value)
}

// SetContextPairingStatus sets context pairing status
func (m *Metrics) SetContextPairingStatus(customerID, contextName, pairingStatus string, value float64) {
	m.contextPairingStatus.WithLabelValues(customerID, contextName, pairingStatus).Set(value)
}

// SetApplicationSetsTotal sets the total number of ApplicationSets
func (m *Metrics) SetApplicationSetsTotal(customerID, contextName, namespace, generatorType string, count int) {
	m.applicationSetsTotal.WithLabelValues(customerID, contextName, namespace, generatorType).Set(float64(count))
}

// SetGeneratedApplicationsTotal sets the total number of generated applications
func (m *Metrics) SetGeneratedApplicationsTotal(customerID, contextName, environment, healthStatus, syncStatus string, count int) {
	m.generatedApplicationsTotal.WithLabelValues(customerID, contextName, environment, healthStatus, syncStatus).Set(float64(count))
}

// IncrementVaultSecretsValidated increments validated Vault secrets counter
func (m *Metrics) IncrementVaultSecretsValidated(customerID, contextName, environment, validationStatus string) {
	m.vaultSecretsValidated.WithLabelValues(customerID, contextName, environment, validationStatus).Inc()
}

// IncrementVaultSecretsValidationErrors increments Vault secret validation errors
func (m *Metrics) IncrementVaultSecretsValidationErrors(customerID, contextName, environment, errorType string) {
	m.vaultSecretsValidationErrors.WithLabelValues(customerID, contextName, environment, errorType).Inc()
}

// Performance Metrics Methods

// SetDatabaseConnectionsActive sets active database connections gauge
func (m *Metrics) SetDatabaseConnectionsActive(databaseName string, count int) {
	m.databaseConnectionsActive.WithLabelValues(m.serviceName, databaseName).Set(float64(count))
}

// RecordDatabaseQueryDuration records database query duration
func (m *Metrics) RecordDatabaseQueryDuration(customerID, queryType string, duration time.Duration) {
	m.databaseQueryDuration.WithLabelValues(m.serviceName, queryType, customerID).Observe(duration.Seconds())
}

// SetCacheHitRate sets cache hit rate gauge
func (m *Metrics) SetCacheHitRate(cacheType string, hitRate float64) {
	m.cacheHitRate.WithLabelValues(m.serviceName, cacheType).Set(hitRate)
}

// SetProcessingQueueLength sets processing queue length gauge
func (m *Metrics) SetProcessingQueueLength(queueName string, length int) {
	m.processingQueueLength.WithLabelValues(m.serviceName, queueName).Set(float64(length))
}

// Resource Metrics Methods

// SetKubernetesResourcesTracked sets Kubernetes resources tracked gauge
func (m *Metrics) SetKubernetesResourcesTracked(customerID, contextName, environment, resourceType string, count int) {
	m.kubernetesResourcesTracked.WithLabelValues(customerID, contextName, environment, resourceType).Set(float64(count))
}

// SetHelmReleasesTracked sets Helm releases tracked gauge
func (m *Metrics) SetHelmReleasesTracked(customerID, contextName, environment, releaseStatus string, count int) {
	m.helmReleasesTracked.WithLabelValues(customerID, contextName, environment, releaseStatus).Set(float64(count))
}

// SetGitRepositoriesTracked sets Git repositories tracked gauge
func (m *Metrics) SetGitRepositoriesTracked(customerID, contextName, repositoryType string, count int) {
	m.gitRepositoriesTracked.WithLabelValues(customerID, contextName, repositoryType).Set(float64(count))
}

// SetCustomerBranchesTracked sets customer branches tracked gauge
func (m *Metrics) SetCustomerBranchesTracked(customerID, contextName, branchCompliance string, count int) {
	m.customerBranchesTracked.WithLabelValues(customerID, contextName, branchCompliance).Set(float64(count))
}

// GetHandler returns the Prometheus HTTP handler for metrics endpoint
func (m *Metrics) GetHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// GetRegistry returns the Prometheus registry
func (m *Metrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

// Helper Methods

// SanitizeCustomerID sanitizes customer ID for use in metrics labels
func (m *Metrics) SanitizeCustomerID(customerID string) string {
	// Replace any characters that aren't alphanumeric, underscore, or dash
	sanitized := strings.ReplaceAll(customerID, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	return sanitized
}

// SanitizeLabelValue sanitizes any label value for Prometheus compatibility
func (m *Metrics) SanitizeLabelValue(value string) string {
	// Prometheus labels should not contain newlines or tabs
	sanitized := strings.ReplaceAll(value, "\n", "_")
	sanitized = strings.ReplaceAll(sanitized, "\t", "_")
	sanitized = strings.ReplaceAll(sanitized, "\r", "_")
	
	// Truncate very long labels
	if len(sanitized) > 100 {
		sanitized = sanitized[:97] + "..."
	}
	
	return sanitized
}

// GitOpsMetricsRecorder provides a convenient interface for recording GitOps-specific metrics
type GitOpsMetricsRecorder struct {
	metrics    *Metrics
	customerID string
	contextName string
	startTime  time.Time
}

// NewGitOpsMetricsRecorder creates a new GitOps metrics recorder
func (m *Metrics) NewGitOpsMetricsRecorder(customerID, contextName string) *GitOpsMetricsRecorder {
	return &GitOpsMetricsRecorder{
		metrics:    m,
		customerID: m.SanitizeCustomerID(customerID),
		contextName: contextName,
		startTime:  time.Now(),
	}
}

// RecordCommandProcessing records command processing completion
func (gmr *GitOpsMetricsRecorder) RecordCommandProcessing(action, manifestType, status string) {
	duration := time.Since(gmr.startTime)
	gmr.metrics.IncrementCommandsProcessed(gmr.customerID, action, manifestType, status)
	gmr.metrics.RecordCommandProcessingDuration(gmr.customerID, action, manifestType, duration)
}

// RecordExternalAPICall records external API call completion
func (gmr *GitOpsMetricsRecorder) RecordExternalAPICall(api, endpoint string, statusCode int, duration time.Duration) {
	gmr.metrics.IncrementExternalAPICalls(gmr.customerID, api, endpoint, statusCode)
	gmr.metrics.RecordExternalAPICallDuration(gmr.customerID, api, endpoint, duration)
}

// RecordMessageProcessing records message processing completion
func (gmr *GitOpsMetricsRecorder) RecordMessageProcessing(queue, messageType, status string, duration time.Duration) {
	gmr.metrics.IncrementMessagesConsumed(gmr.customerID, queue, messageType, status)
	gmr.metrics.RecordMessageProcessingDuration(gmr.customerID, queue, messageType, duration)
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(metrics *Metrics, config MetricsConfig) error {
	if !config.Enabled {
		return nil
	}
	
	mux := http.NewServeMux()
	// Guard against an empty path: services construct MetricsConfig as a struct
	// literal (so the `envDefault:"/metrics"` tag never applies) and don't set
	// Path, which would make ServeMux.Handle panic with "http: invalid pattern".
	path := config.Path
	if path == "" {
		path = "/metrics"
	}
	mux.Handle(path, metrics.GetHandler())
	
	// Add health check endpoint for the metrics server itself
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})
	
	server := &http.Server{
		Addr:    ":" + config.Port,
		Handler: mux,
	}
	
	return server.ListenAndServe()
}

// Generic metrics methods for backward compatibility
func (m *Metrics) IncrementCounter(counterName string, labels prometheus.Labels) {
	// Try to find a matching counter by name and increment it
	switch counterName {
	case "messages_consumed_total":
		if customer, ok := labels["customer_id"]; ok {
			if queue, ok := labels["queue"]; ok {
				if messageType, ok := labels["message_type"]; ok {
					if status, ok := labels["status"]; ok {
						m.IncrementMessagesConsumed(customer, queue, messageType, status)
					}
				}
			}
		}
	case "messages_published_total":
		if customer, ok := labels["customer_id"]; ok {
			if exchange, ok := labels["exchange"]; ok {
				if routingKey, ok := labels["routing_key"]; ok {
					if messageType, ok := labels["message_type"]; ok {
						m.IncrementMessagesPublished(customer, exchange, routingKey, messageType)
					}
				}
			}
		}
	case "message_processing_errors_total":
		if customer, ok := labels["customer_id"]; ok {
			if queue, ok := labels["queue"]; ok {
				if messageType, ok := labels["message_type"]; ok {
					if errorType, ok := labels["error_type"]; ok {
						m.IncrementMessageProcessingErrors(customer, queue, messageType, errorType)
					}
				}
			}
		}
	}
}

func (m *Metrics) RecordHistogram(histogramName string, value float64, labels prometheus.Labels) {
	switch histogramName {
	case "message_processing_duration_seconds":
		if customer, ok := labels["customer_id"]; ok {
			if service, ok := labels["service"]; ok {
				if action, ok := labels["action"]; ok {
					m.RecordMessageProcessingDuration(customer, service, action, time.Duration(value*float64(time.Second)))
				}
			}
		}
	}
}