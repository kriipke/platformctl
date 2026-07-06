package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Middleware provides HTTP middleware for audit logging
type Middleware struct {
	logger Logger
}

// NewMiddleware creates a new audit middleware
func NewMiddleware(logger Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

// LoggingMiddleware returns a middleware function for the given logger
func LoggingMiddleware(logger Logger) func(http.Handler) http.Handler {
	middleware := NewMiddleware(logger)
	return middleware.AuditMiddleware()
}

// AuditMiddleware returns an HTTP middleware that logs all requests and responses
func (m *Middleware) AuditMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate a request ID for correlation
			requestID := uuid.New()
			ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
			r = r.WithContext(ctx)

			// Create a response recorder to capture response details
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           bytes.NewBuffer(nil),
			}

			// Record request details
			startTime := time.Now()

			// Read and buffer request body for logging
			var requestBody []byte
			if r.Body != nil && shouldLogRequestBody(r) {
				requestBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(requestBody))
			}

			// Call the next handler, recovering from any panic so the request
			// is still audited (as an error) and the client gets a 500 rather
			// than a dropped connection.
			var panicValue interface{}
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicValue = rec
						recorder.WriteHeader(http.StatusInternalServerError)
					}
				}()
				next.ServeHTTP(recorder, r)
			}()

			// Log the audit event
			m.logHTTPEvent(ctx, r, recorder, startTime, requestBody, panicValue)
		})
	}
}

// CRUDMiddleware returns middleware specifically for CRUD operations
func (m *Middleware) CRUDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract resource information from URL
			vars := mux.Vars(r)
			resourceType, resourceID := extractResourceInfo(r.URL.Path, vars)

			// Determine event type based on HTTP method
			eventType := httpMethodToEventType(r.Method)

			// Create a response recorder
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           bytes.NewBuffer(nil),
			}

			// Call the next handler
			next.ServeHTTP(recorder, r)

			// Log CRUD event
			outcome := outcomeFromStatusCode(recorder.statusCode)

			if eventType != "" && resourceType != "" {
				auditCtx := ExtractAuditContext(r.Context(), r)
				event := NewAuditEvent(auditCtx, eventType, ResourceType(resourceType), r.Method, outcome)

				if resourceID != "" {
					event.WithResource(resourceID, resourceID)
				}

				// Add HTTP-specific metadata
				event.WithMetadata("http_status", recorder.statusCode)
				event.WithMetadata("response_size", recorder.body.Len())

				// Log any errors
				if recorder.statusCode >= 400 {
					errorCode := strconv.Itoa(recorder.statusCode)
					errorMessage := http.StatusText(recorder.statusCode)
					event.WithError(errorCode, errorMessage)
				}

				_ = m.logger.LogEvent(r.Context(), event)
			}
		})
	}
}

// logHTTPEvent logs a general HTTP request/response audit event
func (m *Middleware) logHTTPEvent(ctx context.Context, r *http.Request, recorder *responseRecorder, startTime time.Time, requestBody []byte, panicValue interface{}) {
	auditCtx := ExtractAuditContext(ctx, r)

	// Determine event type based on the endpoint
	eventType := determineEventTypeFromPath(r.URL.Path, r.Method)
	resourceType := determineResourceTypeFromPath(r.URL.Path)

	outcome := outcomeFromStatusCode(recorder.statusCode)
	action := formatHTTPAction(r.Method, r.URL.Path)

	event := NewAuditEvent(auditCtx, eventType, resourceType, action, outcome)

	// Add timing information
	duration := time.Since(startTime)
	event.WithMetadata("duration_ms", duration.Milliseconds())
	event.WithMetadata("http_status", recorder.statusCode)
	event.WithMetadata("request_size", len(requestBody))
	event.WithMetadata("response_size", recorder.body.Len())

	// Log request body for certain operations (if not sensitive)
	if len(requestBody) > 0 && shouldLogRequestBody(r) && !isSensitiveEndpoint(r.URL.Path) {
		var requestData map[string]interface{}
		if json.Unmarshal(requestBody, &requestData) == nil {
			event.NewValues = requestData
		}
	}

	// Log response body for certain operations
	if recorder.body.Len() > 0 && shouldLogResponseBody(r, recorder.statusCode) {
		var responseData map[string]interface{}
		if json.Unmarshal(recorder.body.Bytes(), &responseData) == nil {
			event.WithMetadata("response_data", responseData)
		}
	}

	// Mark as sensitive if dealing with auth endpoints or containing sensitive data
	if isSensitiveEndpoint(r.URL.Path) {
		event.WithSensitive(true)
	}

	// Log any errors
	if panicValue != nil {
		event.WithError(strconv.Itoa(http.StatusInternalServerError), fmt.Sprintf("panic: %v", panicValue))
	} else if recorder.statusCode >= 400 {
		errorCode := strconv.Itoa(recorder.statusCode)
		errorMessage := http.StatusText(recorder.statusCode)
		event.WithError(errorCode, errorMessage)
	}

	_ = m.logger.LogEvent(ctx, event)
}

// responseRecorder captures response details for audit logging
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	// Write to both the actual response and our buffer
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

// Helper functions

func httpMethodToEventType(method string) EventType {
	switch method {
	case "POST":
		return EventTypeCreate
	case "GET":
		return EventTypeRead
	case "PUT", "PATCH":
		return EventTypeUpdate
	case "DELETE":
		return EventTypeDelete
	default:
		return EventTypeSystem
	}
}

func outcomeFromStatusCode(statusCode int) Outcome {
	if statusCode >= 200 && statusCode < 300 {
		return OutcomeSuccess
	} else if statusCode >= 400 && statusCode < 500 {
		return OutcomeFailure
	} else if statusCode >= 500 {
		return OutcomeError
	}
	return OutcomeError
}

func extractResourceInfo(path string, vars map[string]string) (resourceType, resourceID string) {
	if strings.HasPrefix(path, "/apps") {
		resourceType = "app"
		if name, ok := vars["name"]; ok {
			resourceID = name
		}
	} else if strings.HasPrefix(path, "/environments") {
		resourceType = "environment"
		if name, ok := vars["name"]; ok {
			resourceID = name
		}
	} else if strings.HasPrefix(path, "/contexts") {
		resourceType = "context"
		if name, ok := vars["name"]; ok {
			resourceID = name
		}
	} else if strings.HasPrefix(path, "/auth") {
		resourceType = "user"
	} else {
		resourceType = "system"
	}

	return resourceType, resourceID
}

func determineEventTypeFromPath(path, method string) EventType {
	if strings.Contains(path, "/auth") {
		return EventTypeAuth
	}

	return httpMethodToEventType(method)
}

func determineResourceTypeFromPath(path string) ResourceType {
	if strings.HasPrefix(path, "/apps") {
		return ResourceTypeApp
	} else if strings.HasPrefix(path, "/environments") {
		return ResourceTypeEnvironment
	} else if strings.HasPrefix(path, "/contexts") {
		return ResourceTypeContext
	} else if strings.HasPrefix(path, "/auth") {
		return ResourceTypeUser
	}

	return ResourceTypeSystem
}

func formatHTTPAction(method, path string) string {
	return method + " " + path
}

func shouldLogRequestBody(r *http.Request) bool {
	// Log request body for POST, PUT, PATCH
	method := r.Method
	return method == "POST" || method == "PUT" || method == "PATCH"
}

func shouldLogResponseBody(r *http.Request, statusCode int) bool {
	// Log response body for successful GET requests and error responses
	if r.Method == "GET" && statusCode >= 200 && statusCode < 300 {
		return true
	}

	// Log error responses
	if statusCode >= 400 {
		return true
	}

	return false
}

func isSensitiveEndpoint(path string) bool {
	sensitivePatterns := []string{
		"/auth",
		"/login",
		"/password",
		"/secret",
		"/token",
		"/credential",
		"/vault",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}
