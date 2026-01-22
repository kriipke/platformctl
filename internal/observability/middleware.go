package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

// CorrelationMiddleware adds correlation ID to all HTTP requests
type CorrelationMiddleware struct {
	logger *Logger
	header string
}

// NewCorrelationMiddleware creates a new correlation ID middleware
func NewCorrelationMiddleware(logger *Logger, correlationHeader string) *CorrelationMiddleware {
	if correlationHeader == "" {
		correlationHeader = "X-Correlation-ID"
	}
	
	return &CorrelationMiddleware{
		logger: logger,
		header: correlationHeader,
	}
}

// GinMiddleware returns a Gin middleware function for correlation ID handling
func (cm *CorrelationMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(cm.header)
		if correlationID == "" {
			// Generate new correlation ID if not provided
			correlationID = generateCorrelationID()
		}
		
		// Add correlation ID to context
		ctx := WithCorrelationID(c.Request.Context(), correlationID)
		c.Request = c.Request.WithContext(ctx)
		
		// Add correlation ID to response header
		c.Header(cm.header, correlationID)
		
		// Store in Gin context for easy access
		c.Set("correlation_id", correlationID)
		
		// Continue with request processing
		c.Next()
	}
}

// HTTPMiddleware returns a standard HTTP middleware function for correlation ID handling
func (cm *CorrelationMiddleware) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get(cm.header)
		if correlationID == "" {
			correlationID = generateCorrelationID()
		}
		
		// Add correlation ID to context
		ctx := WithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)
		
		// Add correlation ID to response header
		w.Header().Set(cm.header, correlationID)
		
		next.ServeHTTP(w, r)
	})
}

// MetricsMiddleware captures HTTP metrics for all requests
type MetricsMiddleware struct {
	metrics *Metrics
	logger  *Logger
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(metrics *Metrics, logger *Logger) *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: metrics,
		logger:  logger,
	}
}

// GinMiddleware returns a Gin middleware function for metrics collection
func (mm *MetricsMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Extract customer ID from context (set by auth middleware)
		customerID := getCustomerIDFromContext(c)
		
		// Get endpoint pattern (without path parameters)
		endpoint := getEndpointPattern(c)
		
		// Increment active requests
		mm.metrics.IncrementHTTPActiveRequests(customerID, c.Request.Method, endpoint)
		
		// Continue with request processing
		c.Next()
		
		// Record metrics after request completion
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		
		// Record metrics
		mm.metrics.IncrementHTTPRequests(customerID, c.Request.Method, endpoint, statusCode)
		mm.metrics.RecordHTTPDuration(customerID, c.Request.Method, endpoint, duration)
		mm.metrics.DecrementHTTPActiveRequests(customerID, c.Request.Method, endpoint)
		
		// Log request completion
		correlationID := GetCorrelationID(c.Request.Context())
		mm.logger.NewContextLogger(c.Request.Context()).Info().
			Str("method", c.Request.Method).
			Str("endpoint", endpoint).
			Str("correlation_id", correlationID).
			Str("customer_id", customerID).
			Int("status_code", statusCode).
			Dur("duration", duration).
			Str("user_agent", c.Request.UserAgent()).
			Str("remote_addr", c.ClientIP()).
			Msg("HTTP request completed")
	}
}

// HTTPMiddleware returns a standard HTTP middleware function for metrics collection
func (mm *MetricsMiddleware) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Extract customer ID from context
		customerID := GetCustomerID(r.Context())
		endpoint := r.URL.Path
		
		// Increment active requests
		mm.metrics.IncrementHTTPActiveRequests(customerID, r.Method, endpoint)
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Continue with request processing
		next.ServeHTTP(wrapped, r)
		
		// Record metrics after request completion
		duration := time.Since(start)
		statusCode := wrapped.statusCode
		
		mm.metrics.IncrementHTTPRequests(customerID, r.Method, endpoint, statusCode)
		mm.metrics.RecordHTTPDuration(customerID, r.Method, endpoint, duration)
		mm.metrics.DecrementHTTPActiveRequests(customerID, r.Method, endpoint)
		
		// Log request completion
		correlationID := GetCorrelationID(r.Context())
		mm.logger.NewContextLogger(r.Context()).Info().
			Str("method", r.Method).
			Str("endpoint", endpoint).
			Str("correlation_id", correlationID).
			Str("customer_id", customerID).
			Int("status_code", statusCode).
			Dur("duration", duration).
			Str("user_agent", r.UserAgent()).
			Str("remote_addr", r.RemoteAddr).
			Msg("HTTP request completed")
	})
}

// LoggingMiddleware provides structured logging for all HTTP requests
type LoggingMiddleware struct {
	logger *Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger *Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// GinMiddleware returns a Gin middleware function for request logging
func (lm *LoggingMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Extract context information
		correlationID := GetCorrelationID(c.Request.Context())
		customerID := getCustomerIDFromContext(c)
		
		// Log request start
		lm.logger.NewContextLogger(c.Request.Context()).Debug().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Str("correlation_id", correlationID).
			Str("customer_id", customerID).
			Str("user_agent", c.Request.UserAgent()).
			Str("remote_addr", c.ClientIP()).
			Msg("HTTP request started")
		
		// Continue with request processing
		c.Next()
		
		// Log request completion
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		
		logLevel := lm.logger.NewContextLogger(c.Request.Context()).Info()
		if statusCode >= 400 {
			logLevel = lm.logger.NewContextLogger(c.Request.Context()).Warn()
		}
		if statusCode >= 500 {
			logLevel = lm.logger.NewContextLogger(c.Request.Context()).Error()
		}
		
		logLevel.
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("correlation_id", correlationID).
			Str("customer_id", customerID).
			Int("status_code", statusCode).
			Dur("duration", duration).
			Int("response_size", c.Writer.Size()).
			Msg("HTTP request completed")
		
		// Log any errors that occurred during processing
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				lm.logger.NewContextLogger(c.Request.Context()).Error().
					Str("correlation_id", correlationID).
					Str("customer_id", customerID).
					Err(err.Err).
					Msg("Request processing error")
			}
		}
	}
}

// RecoveryMiddleware provides panic recovery with proper logging
type RecoveryMiddleware struct {
	logger  *Logger
	metrics *Metrics
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(logger *Logger, metrics *Metrics) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger:  logger,
		metrics: metrics,
	}
}

// GinMiddleware returns a Gin middleware function for panic recovery
func (rm *RecoveryMiddleware) GinMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		correlationID := GetCorrelationID(c.Request.Context())
		customerID := getCustomerIDFromContext(c)
		
		// Log the panic
		rm.logger.NewContextLogger(c.Request.Context()).Error().
			Str("correlation_id", correlationID).
			Str("customer_id", customerID).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Interface("panic", recovered).
			Msg("Panic recovered in HTTP handler")
		
		// Record panic metric
		if rm.metrics != nil {
			endpoint := getEndpointPattern(c)
			rm.metrics.IncrementHTTPRequests(customerID, c.Request.Method, endpoint, 500)
		}
		
		// Return error response
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "Internal server error",
			"correlation_id": correlationID,
		})
	})
}

// MessageCorrelationMiddleware adds correlation context to RabbitMQ messages
type MessageCorrelationMiddleware struct {
	logger *Logger
}

// NewMessageCorrelationMiddleware creates a new message correlation middleware
func NewMessageCorrelationMiddleware(logger *Logger) *MessageCorrelationMiddleware {
	return &MessageCorrelationMiddleware{
		logger: logger,
	}
}

// WrapPublishing adds correlation ID to outgoing messages
func (mcm *MessageCorrelationMiddleware) WrapPublishing(ctx context.Context, publishing *amqp.Publishing) {
	correlationID := GetCorrelationID(ctx)
	if correlationID == "" {
		correlationID = generateCorrelationID()
	}
	
	// Add correlation ID to message headers
	if publishing.Headers == nil {
		publishing.Headers = make(map[string]interface{})
	}
	publishing.Headers["correlation_id"] = correlationID
	
	// Set AMQP correlation ID
	publishing.CorrelationId = correlationID
	
	// Add customer ID if present in context
	if customerID := GetCustomerID(ctx); customerID != "" {
		publishing.Headers["customer_id"] = customerID
	}
	
	// Add timestamp
	publishing.Timestamp = time.Now()
}

// WrapConsuming extracts correlation ID from incoming messages
func (mcm *MessageCorrelationMiddleware) WrapConsuming(ctx context.Context, delivery amqp.Delivery) context.Context {
	// Extract correlation ID from message
	correlationID := delivery.CorrelationId
	if correlationID == "" {
		// Try to get from headers
		if delivery.Headers != nil {
			if headerCorrelationID, ok := delivery.Headers["correlation_id"].(string); ok {
				correlationID = headerCorrelationID
			}
		}
	}
	
	if correlationID == "" {
		correlationID = generateCorrelationID()
	}
	
	// Add correlation ID to context
	ctx = WithCorrelationID(ctx, correlationID)
	
	// Extract customer ID from headers if present
	if delivery.Headers != nil {
		if customerID, ok := delivery.Headers["customer_id"].(string); ok {
			ctx = WithCustomerID(ctx, customerID)
		}
	}
	
	return ctx
}

// SecurityMiddleware adds security headers and logging
type SecurityMiddleware struct {
	logger *Logger
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(logger *Logger) *SecurityMiddleware {
	return &SecurityMiddleware{
		logger: logger,
	}
}

// GinMiddleware returns a Gin middleware function for security enhancements
func (sm *SecurityMiddleware) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		
		// Remove server information
		c.Header("Server", "")
		
		// Log security-relevant events
		correlationID := GetCorrelationID(c.Request.Context())
		customerID := getCustomerIDFromContext(c)
		
		// Check for suspicious headers or patterns
		userAgent := c.Request.UserAgent()
		if containsSuspiciousPatterns(userAgent) {
			sm.logger.Audit(c.Request.Context()).
				WithUser(customerID, "").
				WithIPAddress(c.ClientIP()).
				WithAction("suspicious_request").
				WithResult(false, "Suspicious user agent detected").
				Send("Suspicious request detected")
		}
		
		// Continue with request processing
		c.Next()
	}
}

// Helper functions

// generateCorrelationID generates a new correlation ID
func generateCorrelationID() string {
	return uuid.New().String()
}

// getCustomerIDFromContext extracts customer ID from Gin context
func getCustomerIDFromContext(c *gin.Context) string {
	if customer, exists := c.Get("customer"); exists {
		if customerData, ok := customer.(*struct {
			ID string `json:"id"`
		}); ok {
			return customerData.ID
		}
	}
	
	// Try to get from request context
	if customerID := GetCustomerID(c.Request.Context()); customerID != "" {
		return customerID
	}
	
	return "anonymous"
}

// getEndpointPattern extracts the endpoint pattern from Gin context
func getEndpointPattern(c *gin.Context) string {
	// Try to get the matched route pattern
	if route := c.FullPath(); route != "" {
		return route
	}
	
	// Fallback to request path
	return c.Request.URL.Path
}

// containsSuspiciousPatterns checks for suspicious patterns in user agent
func containsSuspiciousPatterns(userAgent string) bool {
	suspiciousPatterns := []string{
		"sqlmap", "nikto", "nmap", "masscan",
		"dirbuster", "gobuster", "ffuf",
		"<script>", "javascript:", "eval(",
	}
	
	userAgentLower := strings.ToLower(userAgent)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(userAgentLower, pattern) {
			return true
		}
	}
	
	return false
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ObservabilityMiddlewareStack combines all observability middleware
type ObservabilityMiddlewareStack struct {
	correlation *CorrelationMiddleware
	metrics     *MetricsMiddleware
	logging     *LoggingMiddleware
	recovery    *RecoveryMiddleware
	security    *SecurityMiddleware
}

// NewObservabilityMiddlewareStack creates a complete observability middleware stack
func NewObservabilityMiddlewareStack(logger *Logger, metrics *Metrics, correlationHeader string) *ObservabilityMiddlewareStack {
	return &ObservabilityMiddlewareStack{
		correlation: NewCorrelationMiddleware(logger, correlationHeader),
		metrics:     NewMetricsMiddleware(metrics, logger),
		logging:     NewLoggingMiddleware(logger),
		recovery:    NewRecoveryMiddleware(logger, metrics),
		security:    NewSecurityMiddleware(logger),
	}
}

// ApplyToGin applies all middleware to a Gin engine in the correct order
func (stack *ObservabilityMiddlewareStack) ApplyToGin(engine *gin.Engine) {
	// Apply middleware in order: Recovery → Security → Correlation → Logging → Metrics
	engine.Use(stack.recovery.GinMiddleware())
	engine.Use(stack.security.GinMiddleware())
	engine.Use(stack.correlation.GinMiddleware())
	engine.Use(stack.logging.GinMiddleware())
	engine.Use(stack.metrics.GinMiddleware())
}

// GetCorrelationMiddleware returns the correlation middleware
func (stack *ObservabilityMiddlewareStack) GetCorrelationMiddleware() *CorrelationMiddleware {
	return stack.correlation
}

// GetMetricsMiddleware returns the metrics middleware
func (stack *ObservabilityMiddlewareStack) GetMetricsMiddleware() *MetricsMiddleware {
	return stack.metrics
}

// GetMessageCorrelationMiddleware returns a message correlation middleware instance
func (stack *ObservabilityMiddlewareStack) GetMessageCorrelationMiddleware() *MessageCorrelationMiddleware {
	return NewMessageCorrelationMiddleware(stack.logging.logger)
}