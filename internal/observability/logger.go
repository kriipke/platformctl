package observability

import (
	"context"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CorrelationIDKey is the context key for correlation IDs
type CorrelationIDKey struct{}

// CustomerIDKey is the context key for customer IDs
type CustomerIDKey struct{}

// ServiceKey is the context key for service names
type ServiceKey struct{}

// Logger provides structured logging with GitOps-specific features
type Logger struct {
	logger         zerolog.Logger
	serviceName    string
	sensitiveRegex *regexp.Regexp
}

// LoggerConfig contains configuration for the logger
type LoggerConfig struct {
	Level         string `env:"LOG_LEVEL" envDefault:"info"`
	Format        string `env:"LOG_FORMAT" envDefault:"json"`
	ServiceName   string `env:"SERVICE_NAME" envDefault:"platformctl"`
	EnableConsole bool   `env:"LOG_CONSOLE" envDefault:"false"`
}

// NewLogger creates a new structured logger with GitOps-specific enhancements
func NewLogger(config LoggerConfig) *Logger {
	// Set global log level
	level := parseLogLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	// Configure output format
	var logger zerolog.Logger
	if config.Format == "console" || config.EnableConsole {
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		})
	} else {
		logger = zerolog.New(os.Stdout)
	}

	// Add standard fields
	logger = logger.With().
		Timestamp().
		Str("service", config.ServiceName).
		Str("version", getServiceVersion()).
		Logger()

	// Create sensitive data regex for scrubbing
	sensitiveRegex := compileSensitiveRegex()

	return &Logger{
		logger:         logger,
		serviceName:    config.ServiceName,
		sensitiveRegex: sensitiveRegex,
	}
}

// NewContextLogger creates a logger with context-specific fields
func (l *Logger) NewContextLogger(ctx context.Context) *zerolog.Logger {
	logger := l.logger

	// Add correlation ID if present
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		logger = logger.With().Str("correlation_id", correlationID).Logger()
	}

	// Add customer ID if present (with customer isolation)
	if customerID := GetCustomerID(ctx); customerID != "" {
		logger = logger.With().Str("customer_id", customerID).Logger()
	}

	// Add service context if present
	if service := GetServiceContext(ctx); service != "" {
		logger = logger.With().Str("component", service).Logger()
	}

	return &logger
}

// GitOpsEvent logs GitOps-specific events with structured data
func (l *Logger) GitOpsEvent(ctx context.Context, event string) *GitOpsEventLogger {
	logger := l.NewContextLogger(ctx)
	return &GitOpsEventLogger{
		logger: logger.Info(),
		event:  event,
	}
}

// GitOpsEventLogger provides fluent interface for GitOps event logging
type GitOpsEventLogger struct {
	logger *zerolog.Event
	event  string
}

// WithContext adds context information
func (gel *GitOpsEventLogger) WithContext(contextName string) *GitOpsEventLogger {
	gel.logger = gel.logger.Str("context_name", contextName)
	return gel
}

// WithManifest adds manifest information
func (gel *GitOpsEventLogger) WithManifest(manifestType, manifestName string) *GitOpsEventLogger {
	gel.logger = gel.logger.
		Str("manifest_type", manifestType).
		Str("manifest_name", manifestName)
	return gel
}

// WithAction adds action information
func (gel *GitOpsEventLogger) WithAction(action string) *GitOpsEventLogger {
	gel.logger = gel.logger.Str("action", action)
	return gel
}

// WithEnvironment adds environment information
func (gel *GitOpsEventLogger) WithEnvironment(environment string) *GitOpsEventLogger {
	gel.logger = gel.logger.Str("environment", environment)
	return gel
}

// WithApp adds application information
func (gel *GitOpsEventLogger) WithApp(appName string) *GitOpsEventLogger {
	gel.logger = gel.logger.Str("app_name", appName)
	return gel
}

// WithDuration adds performance timing
func (gel *GitOpsEventLogger) WithDuration(duration time.Duration) *GitOpsEventLogger {
	gel.logger = gel.logger.
		Dur("duration", duration).
		Float64("duration_ms", float64(duration.Nanoseconds())/1e6)
	return gel
}

// WithError adds error information
func (gel *GitOpsEventLogger) WithError(err error) *GitOpsEventLogger {
	if err != nil {
		gel.logger = gel.logger.Err(err)
	}
	return gel
}

// WithStatus adds status information
func (gel *GitOpsEventLogger) WithStatus(status string) *GitOpsEventLogger {
	gel.logger = gel.logger.Str("status", status)
	return gel
}

// WithExternalService adds external service call information
func (gel *GitOpsEventLogger) WithExternalService(service, endpoint string, statusCode int) *GitOpsEventLogger {
	gel.logger = gel.logger.
		Str("external_service", service).
		Str("external_endpoint", endpoint).
		Int("external_status_code", statusCode)
	return gel
}

// WithMetrics adds performance metrics
func (gel *GitOpsEventLogger) WithMetrics(apiCalls int, cacheHitRate float64) *GitOpsEventLogger {
	gel.logger = gel.logger.
		Int("api_calls", apiCalls).
		Float64("cache_hit_rate", cacheHitRate)
	return gel
}

// WithSecretRef adds secret reference (safely, without exposing values)
func (gel *GitOpsEventLogger) WithSecretRef(vaultPath, secretName string) *GitOpsEventLogger {
	gel.logger = gel.logger.
		Str("vault_path", vaultPath).
		Str("secret_name", secretName)
	return gel
}

// WithResourceCount adds resource count information
func (gel *GitOpsEventLogger) WithResourceCount(count int) *GitOpsEventLogger {
	gel.logger = gel.logger.Int("resource_count", count)
	return gel
}

// Send logs the event
func (gel *GitOpsEventLogger) Send(message string) {
	gel.logger.Str("event", gel.event).Msg(message)
}

// Audit logs security and compliance events
func (l *Logger) Audit(ctx context.Context) *AuditEventLogger {
	logger := l.NewContextLogger(ctx)
	return &AuditEventLogger{
		logger: logger.Info().Str("log_type", "audit"),
	}
}

// AuditEventLogger provides fluent interface for audit logging
type AuditEventLogger struct {
	logger *zerolog.Event
}

// WithUser adds user information to audit log
func (ael *AuditEventLogger) WithUser(userID, userName string) *AuditEventLogger {
	ael.logger = ael.logger.
		Str("user_id", userID).
		Str("user_name", userName)
	return ael
}

// WithResource adds resource information to audit log
func (ael *AuditEventLogger) WithResource(resourceType, resourceName string) *AuditEventLogger {
	ael.logger = ael.logger.
		Str("resource_type", resourceType).
		Str("resource_name", resourceName)
	return ael
}

// WithAction adds action information to audit log
func (ael *AuditEventLogger) WithAction(action string) *AuditEventLogger {
	ael.logger = ael.logger.Str("audit_action", action)
	return ael
}

// WithResult adds result information to audit log
func (ael *AuditEventLogger) WithResult(success bool, reason string) *AuditEventLogger {
	ael.logger = ael.logger.
		Bool("success", success).
		Str("reason", reason)
	return ael
}

// WithIPAddress adds IP address to audit log
func (ael *AuditEventLogger) WithIPAddress(ip string) *AuditEventLogger {
	ael.logger = ael.logger.Str("ip_address", ip)
	return ael
}

// Send logs the audit event
func (ael *AuditEventLogger) Send(message string) {
	ael.logger.Msg(message)
}

// Performance logs performance-related events
func (l *Logger) Performance(ctx context.Context) *PerformanceEventLogger {
	logger := l.NewContextLogger(ctx)
	return &PerformanceEventLogger{
		logger: logger.Info().Str("log_type", "performance"),
	}
}

// PerformanceEventLogger provides fluent interface for performance logging
type PerformanceEventLogger struct {
	logger *zerolog.Event
}

// WithOperation adds operation information
func (pel *PerformanceEventLogger) WithOperation(operation string) *PerformanceEventLogger {
	pel.logger = pel.logger.Str("operation", operation)
	return pel
}

// WithLatency adds latency information
func (pel *PerformanceEventLogger) WithLatency(latency time.Duration) *PerformanceEventLogger {
	pel.logger = pel.logger.
		Dur("latency", latency).
		Float64("latency_ms", float64(latency.Nanoseconds())/1e6)
	return pel
}

// WithThroughput adds throughput information
func (pel *PerformanceEventLogger) WithThroughput(itemsProcessed int, duration time.Duration) *PerformanceEventLogger {
	throughput := float64(itemsProcessed) / duration.Seconds()
	pel.logger = pel.logger.
		Int("items_processed", itemsProcessed).
		Float64("throughput_per_second", throughput)
	return pel
}

// WithCacheStats adds cache statistics
func (pel *PerformanceEventLogger) WithCacheStats(hits, misses int) *PerformanceEventLogger {
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	pel.logger = pel.logger.
		Int("cache_hits", hits).
		Int("cache_misses", misses).
		Float64("cache_hit_rate", hitRate)
	return pel
}

// WithQueueStats adds queue statistics
func (pel *PerformanceEventLogger) WithQueueStats(queueLength, processedCount int) *PerformanceEventLogger {
	pel.logger = pel.logger.
		Int("queue_length", queueLength).
		Int("processed_count", processedCount)
	return pel
}

// Send logs the performance event
func (pel *PerformanceEventLogger) Send(message string) {
	pel.logger.Msg(message)
}

// ScrubSensitiveData removes sensitive information from log messages and structured data
func (l *Logger) ScrubSensitiveData(data string) string {
	if l.sensitiveRegex == nil {
		return data
	}
	return l.sensitiveRegex.ReplaceAllString(data, "[REDACTED]")
}

// ScrubMap scrubs sensitive data from a map
func (l *Logger) ScrubMap(data map[string]interface{}) map[string]interface{} {
	scrubbed := make(map[string]interface{})
	for k, v := range data {
		key := strings.ToLower(k)
		if isSensitiveKey(key) {
			scrubbed[k] = "[REDACTED]"
		} else if str, ok := v.(string); ok {
			scrubbed[k] = l.ScrubSensitiveData(str)
		} else {
			scrubbed[k] = v
		}
	}
	return scrubbed
}

// Context helper functions

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey{}, correlationID)
}

// GetCorrelationID retrieves correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(CorrelationIDKey{}).(string); ok {
		return correlationID
	}
	return ""
}

// WithCustomerID adds customer ID to context
func WithCustomerID(ctx context.Context, customerID string) context.Context {
	return context.WithValue(ctx, CustomerIDKey{}, customerID)
}

// GetCustomerID retrieves customer ID from context
func GetCustomerID(ctx context.Context) string {
	if customerID, ok := ctx.Value(CustomerIDKey{}).(string); ok {
		return customerID
	}
	return ""
}

// WithServiceContext adds service context to context
func WithServiceContext(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, ServiceKey{}, service)
}

// GetServiceContext retrieves service context from context
func GetServiceContext(ctx context.Context) string {
	if service, ok := ctx.Value(ServiceKey{}).(string); ok {
		return service
	}
	return ""
}

// Helper functions

func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

func getServiceVersion() string {
	// In production, this would read from build info or environment variables
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "development"
}

func compileSensitiveRegex() *regexp.Regexp {
	// Regex patterns for common sensitive data formats
	patterns := []string{
		`(?i)(password|passwd|pwd|secret|token|key|auth)["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`,
		`(?i)(api[_-]?key|api[_-]?secret)["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`,
		`(?i)(bearer|jwt)["\s]*[:=]?\s*["']?([a-zA-Z0-9\-._~+/]+=*)["']?`,
		`(?i)authorization["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`,
		`(?i)(vault[_-]?token)["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`,
		`(?i)(database[_-]?url|db[_-]?url)["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`,
	}

	combined := strings.Join(patterns, "|")
	regex, err := regexp.Compile(combined)
	if err != nil {
		// Fallback to basic password detection
		regex, _ = regexp.Compile(`(?i)(password|secret|token|key)["\s]*[:=]\s*["']?([^"'\s,}]+)["']?`)
	}
	return regex
}

func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"password", "passwd", "pwd", "secret", "token", "key", "auth",
		"api_key", "api_secret", "apikey", "apisecret",
		"vault_token", "vault_secret", "vaulttoken",
		"database_password", "db_password", "dbpassword",
		"private_key", "privatekey", "cert_key", "certkey",
		"authorization", "bearer", "jwt", "session",
	}

	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(key, sensitiveKey) {
			return true
		}
	}
	return false
}

// Global logger instance
var globalLogger *Logger

// InitGlobalLogger initializes the global logger instance
func InitGlobalLogger(config LoggerConfig) {
	globalLogger = NewLogger(config)
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		// Initialize with default config if not already initialized
		InitGlobalLogger(LoggerConfig{
			Level:         "info",
			Format:        "json",
			ServiceName:   "platformctl",
			EnableConsole: false,
		})
	}
	return globalLogger
}

// Global convenience functions

// Info logs an info level message with context
func Info(ctx context.Context) *zerolog.Event {
	return GetGlobalLogger().NewContextLogger(ctx).Info()
}

// Error logs an error level message with context
func Error(ctx context.Context) *zerolog.Event {
	return GetGlobalLogger().NewContextLogger(ctx).Error()
}

// Warn logs a warning level message with context
func Warn(ctx context.Context) *zerolog.Event {
	return GetGlobalLogger().NewContextLogger(ctx).Warn()
}

// Debug logs a debug level message with context
func Debug(ctx context.Context) *zerolog.Event {
	return GetGlobalLogger().NewContextLogger(ctx).Debug()
}

// GitOps logs a GitOps event with context
func GitOps(ctx context.Context, event string) *GitOpsEventLogger {
	return GetGlobalLogger().GitOpsEvent(ctx, event)
}
