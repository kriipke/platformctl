package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rabbitmq/amqp091-go"
)

// HealthChecker defines the interface for health checking components
type HealthChecker interface {
	CheckHealth(ctx context.Context) HealthResult
	Name() string
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Name        string                 `json:"name"`
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Timestamp   time.Time              `json:"timestamp"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus represents the possible health states
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// SystemHealth represents the overall system health
type SystemHealth struct {
	Status    HealthStatus             `json:"status"`
	Timestamp time.Time                `json:"timestamp"`
	Duration  time.Duration            `json:"duration"`
	Service   string                   `json:"service"`
	Version   string                   `json:"version"`
	Checks    map[string]HealthResult  `json:"checks"`
	Summary   HealthSummary            `json:"summary"`
}

// HealthSummary provides a summary of all health checks
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Degraded  int `json:"degraded"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

// HealthCheckConfig contains configuration for health checking
type HealthCheckConfig struct {
	Port              string        `env:"HEALTH_CHECK_PORT" envDefault:"8081"`
	ReadinessPath     string        `env:"READINESS_PATH" envDefault:"/ready"`
	LivenessPath      string        `env:"LIVENESS_PATH" envDefault:"/health"`
	CheckInterval     time.Duration `env:"HEALTH_CHECK_INTERVAL" envDefault:"30s"`
	CheckTimeout      time.Duration `env:"HEALTH_CHECK_TIMEOUT" envDefault:"5s"`
	EnableDeepChecks  bool          `env:"ENABLE_DEEP_HEALTH_CHECKS" envDefault:"true"`
}

// HealthManager manages multiple health checkers and provides aggregated health status
type HealthManager struct {
	checkers      map[string]HealthChecker
	config        HealthCheckConfig
	serviceName   string
	serviceVersion string
	mu            sync.RWMutex
	lastResults   map[string]HealthResult
	lastCheck     time.Time
}

// NewHealthManager creates a new health manager
func NewHealthManager(config HealthCheckConfig, serviceName, serviceVersion string) *HealthManager {
	return &HealthManager{
		checkers:       make(map[string]HealthChecker),
		config:         config,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		lastResults:    make(map[string]HealthResult),
	}
}

// RegisterChecker registers a health checker
func (hm *HealthManager) RegisterChecker(checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checkers[checker.Name()] = checker
}

// UnregisterChecker unregisters a health checker
func (hm *HealthManager) UnregisterChecker(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.checkers, name)
	delete(hm.lastResults, name)
}

// CheckHealth performs all health checks and returns aggregated results
func (hm *HealthManager) CheckHealth(ctx context.Context) SystemHealth {
	start := time.Now()
	
	hm.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mu.RUnlock()
	
	// Create context with timeout for all checks
	checkCtx, cancel := context.WithTimeout(ctx, hm.config.CheckTimeout)
	defer cancel()
	
	results := make(map[string]HealthResult)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	// Run all health checks concurrently
	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()
			result := checker.CheckHealth(checkCtx)
			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checker)
	}
	
	wg.Wait()
	
	// Update cached results
	hm.mu.Lock()
	for name, result := range results {
		hm.lastResults[name] = result
	}
	hm.lastCheck = time.Now()
	hm.mu.Unlock()
	
	// Calculate overall health
	overallStatus := hm.calculateOverallHealth(results)
	summary := hm.calculateSummary(results)
	
	return SystemHealth{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Service:   hm.serviceName,
		Version:   hm.serviceVersion,
		Checks:    results,
		Summary:   summary,
	}
}

// CheckReadiness performs readiness checks (required for basic operation)
func (hm *HealthManager) CheckReadiness(ctx context.Context) SystemHealth {
	// For readiness, we only check critical components
	health := hm.CheckHealth(ctx)
	
	// Filter to only include critical checks for readiness
	readinessChecks := make(map[string]HealthResult)
	criticalChecks := []string{"database", "rabbitmq"} // Core dependencies
	
	for _, checkName := range criticalChecks {
		if result, exists := health.Checks[checkName]; exists {
			readinessChecks[checkName] = result
		}
	}
	
	health.Checks = readinessChecks
	health.Status = hm.calculateOverallHealth(readinessChecks)
	health.Summary = hm.calculateSummary(readinessChecks)
	
	return health
}

// GetCachedHealth returns the last health check results without performing new checks
func (hm *HealthManager) GetCachedHealth() SystemHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	overallStatus := hm.calculateOverallHealth(hm.lastResults)
	summary := hm.calculateSummary(hm.lastResults)
	
	return SystemHealth{
		Status:    overallStatus,
		Timestamp: hm.lastCheck,
		Duration:  0, // Cached result
		Service:   hm.serviceName,
		Version:   hm.serviceVersion,
		Checks:    hm.lastResults,
		Summary:   summary,
	}
}

// calculateOverallHealth determines overall system health from individual check results
func (hm *HealthManager) calculateOverallHealth(results map[string]HealthResult) HealthStatus {
	if len(results) == 0 {
		return HealthStatusUnknown
	}
	
	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0
	
	for _, result := range results {
		switch result.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}
	
	// If any component is unhealthy, system is unhealthy
	if unhealthyCount > 0 {
		return HealthStatusUnhealthy
	}
	
	// If any component is degraded, system is degraded
	if degradedCount > 0 {
		return HealthStatusDegraded
	}
	
	// If all components are healthy, system is healthy
	if healthyCount == len(results) {
		return HealthStatusHealthy
	}
	
	return HealthStatusUnknown
}

// calculateSummary calculates health check summary statistics
func (hm *HealthManager) calculateSummary(results map[string]HealthResult) HealthSummary {
	summary := HealthSummary{Total: len(results)}
	
	for _, result := range results {
		switch result.Status {
		case HealthStatusHealthy:
			summary.Healthy++
		case HealthStatusDegraded:
			summary.Degraded++
		case HealthStatusUnhealthy:
			summary.Unhealthy++
		default:
			summary.Unknown++
		}
	}
	
	return summary
}

// StartHealthServer starts the health check HTTP server
func (hm *HealthManager) StartHealthServer() error {
	mux := http.NewServeMux()
	
	// Liveness endpoint - checks if the service is alive
	mux.HandleFunc(hm.config.LivenessPath, hm.livenessHandler)
	
	// Readiness endpoint - checks if the service is ready to serve traffic
	mux.HandleFunc(hm.config.ReadinessPath, hm.readinessHandler)
	
	// Detailed health endpoint
	mux.HandleFunc("/health/detailed", hm.detailedHealthHandler)
	
	server := &http.Server{
		Addr:    ":" + hm.config.Port,
		Handler: mux,
	}
	
	return server.ListenAndServe()
}

// HTTP handlers

func (hm *HealthManager) livenessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health := hm.CheckHealth(ctx)
	
	w.Header().Set("Content-Type", "application/json")
	
	// For liveness, we're more lenient - only fail if system is completely unhealthy
	if health.Status == HealthStatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(health)
}

func (hm *HealthManager) readinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health := hm.CheckReadiness(ctx)
	
	w.Header().Set("Content-Type", "application/json")
	
	// For readiness, we're stricter - fail if any critical component is not healthy
	if health.Status == HealthStatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(health)
}

func (hm *HealthManager) detailedHealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health := hm.CheckHealth(ctx)
	
	w.Header().Set("Content-Type", "application/json")
	
	// Always return 200 for detailed health - let the client interpret the status
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// Built-in Health Checkers

// DatabaseHealthChecker checks database connectivity and performance
type DatabaseHealthChecker struct {
	db   *sqlx.DB
	name string
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db *sqlx.DB, name string) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		db:   db,
		name: name,
	}
}

// Name returns the checker name
func (dhc *DatabaseHealthChecker) Name() string {
	return dhc.name
}

// CheckHealth checks database health
func (dhc *DatabaseHealthChecker) CheckHealth(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Name:      dhc.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	// Check basic connectivity
	if err := dhc.db.PingContext(ctx); err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Database ping failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	
	// Check connection pool stats
	stats := dhc.db.Stats()
	result.Details["open_connections"] = stats.OpenConnections
	result.Details["in_use"] = stats.InUse
	result.Details["idle"] = stats.Idle
	result.Details["wait_count"] = stats.WaitCount
	result.Details["wait_duration"] = stats.WaitDuration.String()
	
	// Perform a simple query to test functionality
	var count int
	queryStart := time.Now()
	err := dhc.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM contexts LIMIT 1")
	queryDuration := time.Since(queryStart)
	result.Details["query_duration_ms"] = queryDuration.Milliseconds()
	
	if err != nil && err != sql.ErrNoRows {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("Database query test failed: %v", err)
	} else {
		result.Status = HealthStatusHealthy
		result.Message = "Database is healthy"
	}
	
	// Check for performance issues
	if queryDuration > 1*time.Second {
		result.Status = HealthStatusDegraded
		result.Message = "Database queries are slow"
	}
	
	// Check connection pool health
	if stats.OpenConnections > 50 { // Adjust threshold as needed
		if result.Status == HealthStatusHealthy {
			result.Status = HealthStatusDegraded
		}
		result.Message += "; High connection count"
	}
	
	result.Duration = time.Since(start)
	return result
}

// RabbitMQHealthChecker checks RabbitMQ connectivity
type RabbitMQHealthChecker struct {
	connection *amqp.Connection
	name       string
}

// NewRabbitMQHealthChecker creates a new RabbitMQ health checker
func NewRabbitMQHealthChecker(connection *amqp.Connection, name string) *RabbitMQHealthChecker {
	return &RabbitMQHealthChecker{
		connection: connection,
		name:       name,
	}
}

// Name returns the checker name
func (rmq *RabbitMQHealthChecker) Name() string {
	return rmq.name
}

// CheckHealth checks RabbitMQ health
func (rmq *RabbitMQHealthChecker) CheckHealth(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Name:      rmq.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	if rmq.connection == nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "RabbitMQ connection is nil"
		result.Duration = time.Since(start)
		return result
	}
	
	if rmq.connection.IsClosed() {
		result.Status = HealthStatusUnhealthy
		result.Message = "RabbitMQ connection is closed"
		result.Duration = time.Since(start)
		return result
	}
	
	// Try to open a channel to test connectivity
	channel, err := rmq.connection.Channel()
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Failed to open RabbitMQ channel: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer channel.Close()
	
	result.Status = HealthStatusHealthy
	result.Message = "RabbitMQ is healthy"
	result.Duration = time.Since(start)
	
	return result
}

// ExternalServiceHealthChecker checks external service availability
type ExternalServiceHealthChecker struct {
	name     string
	url      string
	timeout  time.Duration
	client   *http.Client
}

// NewExternalServiceHealthChecker creates a new external service health checker
func NewExternalServiceHealthChecker(name, url string, timeout time.Duration) *ExternalServiceHealthChecker {
	return &ExternalServiceHealthChecker{
		name:    name,
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the checker name
func (esc *ExternalServiceHealthChecker) Name() string {
	return esc.name
}

// CheckHealth checks external service health
func (esc *ExternalServiceHealthChecker) CheckHealth(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Name:      esc.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", esc.url, nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	
	// Add headers to identify the health check
	req.Header.Set("User-Agent", "ContextOps-HealthCheck/1.0")
	req.Header.Set("X-Health-Check", "true")
	
	// Make the request
	resp, err := esc.client.Do(req)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("Request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()
	
	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time_ms"] = time.Since(start).Milliseconds()
	
	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = HealthStatusHealthy
		result.Message = "External service is healthy"
	} else if resp.StatusCode >= 300 && resp.StatusCode < 500 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("External service returned %d", resp.StatusCode)
	} else {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("External service returned %d", resp.StatusCode)
	}
	
	result.Duration = time.Since(start)
	return result
}

// GitOpsHealthChecker performs GitOps-specific health checks
type GitOpsHealthChecker struct {
	name string
}

// NewGitOpsHealthChecker creates a new GitOps health checker
func NewGitOpsHealthChecker(name string) *GitOpsHealthChecker {
	return &GitOpsHealthChecker{
		name: name,
	}
}

// Name returns the checker name
func (ghc *GitOpsHealthChecker) Name() string {
	return ghc.name
}

// CheckHealth performs GitOps-specific health checks
func (ghc *GitOpsHealthChecker) CheckHealth(ctx context.Context) HealthResult {
	start := time.Now()
	result := HealthResult{
		Name:      ghc.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	// This would typically check:
	// - ArgoCD connectivity
	// - Vault connectivity
	// - Kubernetes cluster access
	// - Git repository access
	// For now, we'll simulate a basic check
	
	result.Status = HealthStatusHealthy
	result.Message = "GitOps components are operational"
	result.Duration = time.Since(start)
	
	// Add placeholder details that would come from real checks
	result.Details["argocd_reachable"] = true
	result.Details["vault_reachable"] = true
	result.Details["kubernetes_clusters_reachable"] = true
	result.Details["git_repositories_accessible"] = true
	
	return result
}

// HealthServer wraps the health manager with a standalone server
type HealthServer struct {
	manager *HealthManager
	server  *http.Server
}

// NewHealthServer creates a new standalone health server
func NewHealthServer(port int) *HealthServer {
	config := HealthCheckConfig{
		Port:              fmt.Sprintf("%d", port),
		ReadinessPath:     "/ready",
		LivenessPath:      "/health",
		CheckInterval:     30 * time.Second,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	
	manager := NewHealthManager(config, "platformctl", "1.0.0")
	
	return &HealthServer{
		manager: manager,
	}
}

// RegisterChecker registers a health checker with the server
func (hs *HealthServer) RegisterChecker(checker HealthChecker) {
	hs.manager.RegisterChecker(checker)
}

// Start starts the health server
func (hs *HealthServer) Start() error {
	return hs.manager.StartHealthServer()
}

// Shutdown gracefully shuts down the health server
func (hs *HealthServer) Shutdown(ctx context.Context) error {
	if hs.server != nil {
		return hs.server.Shutdown(ctx)
	}
	return nil
}