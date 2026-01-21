# PHASE 1E: Basic Observability

**Duration:** 2 days  
**Prerequisites:** Phase 1D completed  
**Deliverable:** Comprehensive logging, metrics, health checks, and correlation tracking

---

## Overview

Implement structured logging, metrics collection, and health monitoring across all services. This phase provides the observability foundation needed for production operations and troubleshooting.

## Success Criteria

✅ Structured JSON logging across all services  
✅ Prometheus metrics exported from all components  
✅ Health check endpoints for all services  
✅ Correlation ID tracking through entire request flow  
✅ Request/response logging with sensitive data scrubbing  
✅ Key business metrics instrumented  
✅ Dashboard configuration for operators  

---

## Implementation Tasks

### Task 1: Structured Logging Framework

**File: `internal/observability/logger.go`**

```go
type Logger struct {
    *logrus.Logger
}

func NewLogger(service string, level string) *Logger {
    log := logrus.New()
    
    // JSON formatter for structured logs
    log.SetFormatter(&logrus.JSONFormatter{
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "level",
            logrus.FieldKeyMsg:   "message",
        },
    })
    
    // Set level
    if parsedLevel, err := logrus.ParseLevel(level); err == nil {
        log.SetLevel(parsedLevel)
    } else {
        log.SetLevel(logrus.InfoLevel)
    }
    
    // Add default fields
    log = log.WithFields(logrus.Fields{
        "service": service,
        "version": getVersion(),
    })
    
    return &Logger{log}
}

func (l *Logger) WithCorrelation(correlationID string) *logrus.Entry {
    return l.WithField("correlation_id", correlationID)
}

func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
    entry := l.Logger.WithContext(ctx)
    
    // Extract correlation ID from context if present
    if correlationID := getCorrelationID(ctx); correlationID != "" {
        entry = entry.WithField("correlation_id", correlationID)
    }
    
    return entry
}

// Sensitive data scrubber
func (l *Logger) ScrubSensitiveData(data interface{}) interface{} {
    switch v := data.(type) {
    case string:
        return scrubString(v)
    case map[string]interface{}:
        return scrubMap(v)
    case []interface{}:
        return scrubSlice(v)
    default:
        return data
    }
}

var sensitivePatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)(token|password|secret|key|auth)["']?\s*[:=]\s*["']?([^"\s,}]+)`),
    regexp.MustCompile(`Bearer\s+[A-Za-z0-9\-._~+/]+`),
    regexp.MustCompile(`(?i)authorization:\s*[^\n\r]+`),
}

func scrubString(s string) string {
    for _, pattern := range sensitivePatterns {
        s = pattern.ReplaceAllString(s, "$1: [REDACTED]")
    }
    return s
}
```

### Task 2: Prometheus Metrics

**File: `internal/observability/metrics.go`**

```go
var (
    // HTTP metrics
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "contextops_http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"service", "method", "endpoint", "status_code"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "contextops_http_request_duration_seconds",
            Help:    "Duration of HTTP requests",
            Buckets: prometheus.DefBuckets,
        },
        []string{"service", "method", "endpoint"},
    )
    
    // Command processing metrics
    commandsProcessedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "contextops_commands_processed_total",
            Help: "Total number of commands processed",
        },
        []string{"service", "action", "status"},
    )
    
    commandProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "contextops_command_processing_duration_seconds",
            Help:    "Duration of command processing",
            Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
        },
        []string{"service", "action"},
    )
    
    // External API metrics
    externalAPICallsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "contextops_external_api_calls_total",
            Help: "Total number of external API calls",
        },
        []string{"service", "api", "status_code"},
    )
    
    externalAPICallDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "contextops_external_api_call_duration_seconds",
            Help:    "Duration of external API calls",
            Buckets: []float64{.1, .5, 1, 2, 5, 10, 30},
        },
        []string{"service", "api"},
    )
    
    // RabbitMQ metrics
    rabbitmqMessagesPublished = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "contextops_rabbitmq_messages_published_total",
            Help: "Total number of messages published to RabbitMQ",
        },
        []string{"exchange", "routing_key"},
    )
    
    rabbitmqMessagesConsumed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "contextops_rabbitmq_messages_consumed_total",
            Help: "Total number of messages consumed from RabbitMQ",
        },
        []string{"service", "queue", "status"},
    )
    
    // Business metrics
    contextsTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "contextops_contexts_total",
            Help: "Total number of contexts",
        },
    )
    
    contextStatusByHealth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "contextops_context_status_by_health",
            Help: "Number of contexts by health status",
        },
        []string{"health"},
    )
)

func init() {
    prometheus.MustRegister(
        httpRequestsTotal,
        httpRequestDuration,
        commandsProcessedTotal,
        commandProcessingDuration,
        externalAPICallsTotal,
        externalAPICallDuration,
        rabbitmqMessagesPublished,
        rabbitmqMessagesConsumed,
        contextsTotal,
        contextStatusByHealth,
    )
}

type MetricsCollector struct {
    serviceName string
}

func NewMetricsCollector(serviceName string) *MetricsCollector {
    return &MetricsCollector{serviceName: serviceName}
}

func (m *MetricsCollector) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
    httpRequestsTotal.WithLabelValues(
        m.serviceName, method, endpoint, strconv.Itoa(statusCode),
    ).Inc()
    
    httpRequestDuration.WithLabelValues(
        m.serviceName, method, endpoint,
    ).Observe(duration.Seconds())
}

func (m *MetricsCollector) RecordCommandProcessed(action, status string, duration time.Duration) {
    commandsProcessedTotal.WithLabelValues(
        m.serviceName, action, status,
    ).Inc()
    
    commandProcessingDuration.WithLabelValues(
        m.serviceName, action,
    ).Observe(duration.Seconds())
}

func (m *MetricsCollector) RecordExternalAPICall(api string, statusCode int, duration time.Duration) {
    externalAPICallsTotal.WithLabelValues(
        m.serviceName, api, strconv.Itoa(statusCode),
    ).Inc()
    
    externalAPICallDuration.WithLabelValues(
        m.serviceName, api,
    ).Observe(duration.Seconds())
}
```

### Task 3: Health Check System

**File: `internal/observability/health.go`**

```go
type HealthChecker struct {
    checks map[string]HealthCheck
    mutex  sync.RWMutex
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
}

type HealthStatus struct {
    Status    string                    `json:"status"` // healthy, degraded, unhealthy
    Timestamp time.Time                 `json:"timestamp"`
    Checks    map[string]CheckResult    `json:"checks"`
}

type CheckResult struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
    Latency string `json:"latency"`
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{
        checks: make(map[string]HealthCheck),
    }
}

func (h *HealthChecker) RegisterCheck(check HealthCheck) {
    h.mutex.Lock()
    defer h.mutex.Unlock()
    h.checks[check.Name()] = check
}

func (h *HealthChecker) CheckHealth(ctx context.Context) *HealthStatus {
    h.mutex.RLock()
    defer h.mutex.RUnlock()
    
    status := &HealthStatus{
        Timestamp: time.Now(),
        Checks:    make(map[string]CheckResult),
    }
    
    overallHealthy := true
    overallDegraded := false
    
    for name, check := range h.checks {
        start := time.Now()
        err := check.Check(ctx)
        latency := time.Since(start)
        
        result := CheckResult{
            Latency: latency.String(),
        }
        
        if err != nil {
            result.Status = "unhealthy"
            result.Message = err.Error()
            overallHealthy = false
        } else if latency > 5*time.Second {
            result.Status = "degraded"
            result.Message = "high latency"
            overallDegraded = true
        } else {
            result.Status = "healthy"
        }
        
        status.Checks[name] = result
    }
    
    if !overallHealthy {
        status.Status = "unhealthy"
    } else if overallDegraded {
        status.Status = "degraded"
    } else {
        status.Status = "healthy"
    }
    
    return status
}

// Database health check
type DatabaseHealthCheck struct {
    db *sql.DB
}

func NewDatabaseHealthCheck(db *sql.DB) *DatabaseHealthCheck {
    return &DatabaseHealthCheck{db: db}
}

func (d *DatabaseHealthCheck) Name() string {
    return "database"
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) error {
    return d.db.PingContext(ctx)
}

// RabbitMQ health check
type RabbitMQHealthCheck struct {
    conn *amqp.Connection
}

func NewRabbitMQHealthCheck(conn *amqp.Connection) *RabbitMQHealthCheck {
    return &RabbitMQHealthCheck{conn: conn}
}

func (r *RabbitMQHealthCheck) Name() string {
    return "rabbitmq"
}

func (r *RabbitMQHealthCheck) Check(ctx context.Context) error {
    if r.conn.IsClosed() {
        return errors.New("connection is closed")
    }
    return nil
}
```

### Task 4: Correlation ID Middleware

**File: `internal/observability/correlation.go`**

```go
const CorrelationIDHeader = "X-Correlation-ID"

type correlationIDKey struct{}

func CorrelationIDMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            correlationID := r.Header.Get(CorrelationIDHeader)
            if correlationID == "" {
                correlationID = generateUUID()
            }
            
            // Add to response header
            w.Header().Set(CorrelationIDHeader, correlationID)
            
            // Add to context
            ctx := context.WithValue(r.Context(), correlationIDKey{}, correlationID)
            
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GetCorrelationID(ctx context.Context) string {
    if id, ok := ctx.Value(correlationIDKey{}).(string); ok {
        return id
    }
    return ""
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
    return context.WithValue(ctx, correlationIDKey{}, correlationID)
}
```

### Task 5: Instrumented Request Logging Middleware

**File: `internal/observability/logging_middleware.go`**

```go
func LoggingMiddleware(logger *Logger, metrics *MetricsCollector) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Wrap response writer to capture status code
            wrapped := &responseWriter{
                ResponseWriter: w,
                statusCode:     200,
            }
            
            // Get correlation ID from context
            correlationID := GetCorrelationID(r.Context())
            
            // Log request
            logger.WithFields(logrus.Fields{
                "correlation_id": correlationID,
                "method":        r.Method,
                "path":          r.URL.Path,
                "user_agent":    r.Header.Get("User-Agent"),
                "remote_addr":   r.RemoteAddr,
            }).Info("HTTP request started")
            
            // Process request
            next.ServeHTTP(wrapped, r)
            
            duration := time.Since(start)
            
            // Record metrics
            endpoint := getEndpointPattern(r)
            metrics.RecordHTTPRequest(r.Method, endpoint, wrapped.statusCode, duration)
            
            // Log response
            logLevel := logrus.InfoLevel
            if wrapped.statusCode >= 400 {
                logLevel = logrus.WarnLevel
            }
            if wrapped.statusCode >= 500 {
                logLevel = logrus.ErrorLevel
            }
            
            logger.WithFields(logrus.Fields{
                "correlation_id": correlationID,
                "method":        r.Method,
                "path":          r.URL.Path,
                "status_code":   wrapped.statusCode,
                "duration_ms":   duration.Milliseconds(),
            }).Log(logLevel, "HTTP request completed")
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
    rw.statusCode = statusCode
    rw.ResponseWriter.WriteHeader(statusCode)
}

func getEndpointPattern(r *http.Request) string {
    // Extract route pattern from mux
    if route := mux.CurrentRoute(r); route != nil {
        if template, err := route.GetPathTemplate(); err == nil {
            return template
        }
    }
    return r.URL.Path
}
```

### Task 6: Update Services with Observability

**Example: Update Gateway Main**

```go
func main() {
    cfg := loadConfig()
    
    // Initialize observability
    logger := observability.NewLogger("gateway", cfg.LogLevel)
    metrics := observability.NewMetricsCollector("gateway")
    healthChecker := observability.NewHealthChecker()
    
    // Database connection with health check
    db := setupDatabase(cfg.DatabaseURL)
    defer db.Close()
    healthChecker.RegisterCheck(observability.NewDatabaseHealthCheck(db))
    
    // RabbitMQ connection with health check
    messageBus, err := events.NewMessageBus(cfg.RabbitMQURL)
    if err != nil {
        logger.Fatal("Failed to connect to RabbitMQ:", err)
    }
    defer messageBus.Close()
    healthChecker.RegisterCheck(observability.NewRabbitMQHealthCheck(messageBus.Connection()))
    
    // Dependencies
    contextStore := storage.NewContextStore(db)
    readStore := storage.NewReadModelStore(db)
    publisher := events.NewCommandPublisher(messageBus, metrics)
    
    // Handlers
    contextHandler := handlers.NewContextHandler(contextStore)
    actionHandler := handlers.NewActionHandler(contextStore, publisher)
    statusHandler := handlers.NewStatusHandler(readStore)
    
    // Router with observability middleware
    router := setupRouter(contextHandler, actionHandler, statusHandler, logger, metrics, healthChecker)
    
    // Start server
    server := &http.Server{
        Addr:    cfg.Port,
        Handler: router,
    }
    
    logger.WithField("port", cfg.Port).Info("Gateway server starting")
    log.Fatal(server.ListenAndServe())
}

func setupRouter(
    contextHandler *handlers.ContextHandler,
    actionHandler *handlers.ActionHandler, 
    statusHandler *handlers.StatusHandler,
    logger *observability.Logger,
    metrics *observability.MetricsCollector,
    healthChecker *observability.HealthChecker,
) *mux.Router {
    r := mux.NewRouter()
    
    // Global middleware
    r.Use(observability.CorrelationIDMiddleware())
    r.Use(observability.LoggingMiddleware(logger, metrics))
    r.Use(auth.BasicAuthMiddleware())
    
    // Health and metrics endpoints
    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        health := healthChecker.CheckHealth(r.Context())
        
        statusCode := http.StatusOK
        if health.Status == "unhealthy" {
            statusCode = http.StatusServiceUnavailable
        } else if health.Status == "degraded" {
            statusCode = http.StatusOK // 200 but with degraded status
        }
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(statusCode)
        json.NewEncoder(w).Encode(health)
    }).Methods("GET")
    
    r.Handle("/metrics", promhttp.Handler()).Methods("GET")
    
    // Application routes
    // ... existing routes
    
    return r
}
```

### Task 7: Dashboard Configuration

**File: `deploy/dashboards/contextops-overview.json`**

```json
{
  "dashboard": {
    "title": "ContextOps Overview",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(contextops_http_requests_total[5m])) by (service)"
          }
        ]
      },
      {
        "title": "Request Latency",
        "type": "graph", 
        "targets": [
          {
            "expr": "histogram_quantile(0.95, sum(rate(contextops_http_request_duration_seconds_bucket[5m])) by (le, service))"
          }
        ]
      },
      {
        "title": "Command Processing Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(contextops_commands_processed_total[5m])) by (service, action)"
          }
        ]
      },
      {
        "title": "Context Health Status",
        "type": "pie",
        "targets": [
          {
            "expr": "contextops_context_status_by_health"
          }
        ]
      },
      {
        "title": "External API Call Latency",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, sum(rate(contextops_external_api_call_duration_seconds_bucket[5m])) by (le, api))"
          }
        ]
      }
    ]
  }
}
```

---

## Dependencies

Add to `go.mod`:
```
require (
    github.com/sirupsen/logrus v1.9.3
    github.com/prometheus/client_golang v1.17.0
)
```

---

## Validation Checklist

Before marking Phase 1E complete:

**Logging:**
- [ ] All services produce structured JSON logs
- [ ] Correlation IDs tracked through entire request flow
- [ ] Sensitive data properly scrubbed from logs
- [ ] Log levels configurable per service
- [ ] Request/response logging captures key details

**Metrics:**
- [ ] Prometheus metrics exported from all services
- [ ] HTTP request metrics include status codes and latency
- [ ] Command processing metrics track success/failure
- [ ] External API call metrics available
- [ ] Business metrics (context counts, health status) working

**Health Checks:**
- [ ] Health endpoints return proper status codes
- [ ] Database connectivity checks working
- [ ] RabbitMQ connectivity checks working
- [ ] Health status properly aggregated across dependencies
- [ ] Kubernetes readiness/liveness probe compatible

**Integration:**
- [ ] Metrics correlate with actual system behavior
- [ ] Dashboard displays meaningful operational data
- [ ] Log aggregation captures errors and warnings
- [ ] Health checks detect real service issues

---

## Next Steps

Upon completion, Phase 1E provides:
- Complete observability foundation for production operations
- Structured logging for troubleshooting and audit
- Metrics for performance monitoring and alerting
- Health checks for container orchestration

**Handoff to Phase 1F:** Deployment configurations can now include proper health checks, metrics scraping, and log collection setup.