package security

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

	"github.com/gorilla/mux"
)

// SecurityMiddleware provides comprehensive security validation for HTTP requests
type SecurityMiddleware struct {
	validator   *Validator
	rateLimiter *RateLimiter
	config      *MiddlewareConfig
}

// MiddlewareConfig holds configuration for security middleware
type MiddlewareConfig struct {
	ValidateHeaders     bool          `json:"validate_headers"`
	ValidateQueryParams bool          `json:"validate_query_params"`
	ValidateJSONBody    bool          `json:"validate_json_body"`
	MaxRequestSize      int64         `json:"max_request_size"`
	Timeout             time.Duration `json:"timeout"`
	EnableRateLimit     bool          `json:"enable_rate_limit"`
	RateLimitRPM        int           `json:"rate_limit_rpm"`
	AllowedContentTypes []string      `json:"allowed_content_types"`
	RequiredHeaders     []string      `json:"required_headers"`
}

// DefaultMiddlewareConfig returns secure default middleware configuration
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		ValidateHeaders:     true,
		ValidateQueryParams: true,
		ValidateJSONBody:    true,
		MaxRequestSize:      10 * 1024 * 1024, // 10MB
		Timeout:             30 * time.Second,
		EnableRateLimit:     true,
		RateLimitRPM:        1000, // 1000 requests per minute
		AllowedContentTypes: []string{
			"application/json",
			"application/x-www-form-urlencoded",
			"multipart/form-data",
			"text/plain",
		},
		RequiredHeaders: []string{"User-Agent"},
	}
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(validator *Validator, config *MiddlewareConfig) (*SecurityMiddleware, error) {
	if config == nil {
		config = DefaultMiddlewareConfig()
	}

	var rateLimiter *RateLimiter
	if config.EnableRateLimit {
		rateLimiter = NewRateLimiter(config.RateLimitRPM, time.Minute)
	}

	return &SecurityMiddleware{
		validator:   validator,
		rateLimiter: rateLimiter,
		config:      config,
	}, nil
}

// SecurityMiddleware returns the main security middleware
func (sm *SecurityMiddleware) SecurityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply timeout to the request context
			ctx, cancel := context.WithTimeout(r.Context(), sm.config.Timeout)
			defer cancel()
			r = r.WithContext(ctx)

			// Rate limiting
			if sm.rateLimiter != nil {
				clientIP := getClientIPFromRequest(r)
				if !sm.rateLimiter.Allow(clientIP) {
					sm.writeError(w, "Rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}

			// Request size validation
			if r.ContentLength > sm.config.MaxRequestSize {
				sm.writeError(w, "Request too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Header validation
			if sm.config.ValidateHeaders {
				if err := sm.validateHeaders(r); err != nil {
					sm.writeError(w, fmt.Sprintf("Invalid headers: %v", err), http.StatusBadRequest)
					return
				}
			}

			// Query parameter validation
			if sm.config.ValidateQueryParams {
				if err := sm.validateQueryParams(r); err != nil {
					sm.writeError(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
					return
				}
			}

			// Body validation for applicable methods
			if sm.shouldValidateBody(r) {
				if err := sm.validateBody(w, r); err != nil {
					sm.writeError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
					return
				}
			}

			// URL path validation
			if err := sm.validateURLPath(r); err != nil {
				sm.writeError(w, fmt.Sprintf("Invalid URL path: %v", err), http.StatusBadRequest)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// InputValidationMiddleware provides specific input validation for URL parameters
func (sm *SecurityMiddleware) InputValidationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate URL path variables
			vars := mux.Vars(r)
			for key, value := range vars {
				if err := sm.validatePathVariable(key, value); err != nil {
					sm.writeError(w, fmt.Sprintf("Invalid path parameter %s: %v", key, err), http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFMiddleware provides CSRF protection (simplified implementation)
func (sm *SecurityMiddleware) CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF check for safe methods
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			// Simple CSRF check using custom header
			csrfHeader := r.Header.Get("X-CSRF-Token")
			if csrfHeader == "" {
				sm.writeError(w, "Missing CSRF token", http.StatusForbidden)
				return
			}

			// In a real implementation, you would validate the CSRF token
			// For now, we just check that it exists
			if err := sm.validator.ValidateString(csrfHeader, "CSRF token"); err != nil {
				sm.writeError(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContentSecurityMiddleware adds security headers
func (sm *SecurityMiddleware) ContentSecurityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// Remove potentially sensitive headers
			w.Header().Del("Server")
			w.Header().Del("X-Powered-By")

			next.ServeHTTP(w, r)
		})
	}
}

// Validation methods

// validateHeaders validates HTTP headers for security threats
func (sm *SecurityMiddleware) validateHeaders(r *http.Request) error {
	// Check required headers
	for _, required := range sm.config.RequiredHeaders {
		if r.Header.Get(required) == "" {
			return fmt.Errorf("missing required header: %s", required)
		}
	}

	// Validate individual headers
	for name, values := range r.Header {
		for _, value := range values {
			if err := sm.validator.ValidateString(value, fmt.Sprintf("header %s", name)); err != nil {
				return fmt.Errorf("invalid header %s: %w", name, err)
			}

			// Specific header validations
			switch strings.ToLower(name) {
			case "host":
				if err := sm.validateHostHeader(value); err != nil {
					return fmt.Errorf("invalid Host header: %w", err)
				}
			case "content-type":
				if err := sm.validateContentType(value); err != nil {
					return fmt.Errorf("invalid Content-Type: %w", err)
				}
			case "content-length":
				if length, err := strconv.ParseInt(value, 10, 64); err == nil {
					if length > sm.config.MaxRequestSize {
						return fmt.Errorf("content length exceeds maximum allowed size")
					}
				}
			}
		}
	}

	return nil
}

// validateQueryParams validates URL query parameters
func (sm *SecurityMiddleware) validateQueryParams(r *http.Request) error {
	for key, values := range r.URL.Query() {
		// Validate parameter name
		if err := sm.validator.ValidateString(key, fmt.Sprintf("query parameter name %s", key)); err != nil {
			return fmt.Errorf("invalid parameter name %s: %w", key, err)
		}

		// Validate parameter values
		for _, value := range values {
			if err := sm.validator.ValidateString(value, fmt.Sprintf("query parameter %s", key)); err != nil {
				return fmt.Errorf("invalid parameter %s: %w", key, err)
			}
		}
	}

	return nil
}

// validateBody validates request body content
func (sm *SecurityMiddleware) validateBody(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore body for next handler
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Validate body size
	if err := sm.validator.ValidateJSONSize(body, "request body"); err != nil {
		return err
	}

	// Content type specific validation
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return sm.validateJSONBody(body)
	}

	return nil
}

// validateJSONBody validates JSON request body
func (sm *SecurityMiddleware) validateJSONBody(body []byte) error {
	// Check if it's valid JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate JSON content recursively
	return sm.validateJSONContent(jsonData, "")
}

// validateJSONContent recursively validates JSON content
func (sm *SecurityMiddleware) validateJSONContent(data interface{}, path string) error {
	switch v := data.(type) {
	case string:
		if err := sm.validator.ValidateString(v, fmt.Sprintf("JSON field %s", path)); err != nil {
			return err
		}
	case map[string]interface{}:
		for key, value := range v {
			// Validate key
			if err := sm.validator.ValidateString(key, fmt.Sprintf("JSON key %s", key)); err != nil {
				return err
			}

			// Recursively validate value
			fieldPath := key
			if path != "" {
				fieldPath = path + "." + key
			}
			if err := sm.validateJSONContent(value, fieldPath); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, item := range v {
			fieldPath := fmt.Sprintf("%s[%d]", path, i)
			if err := sm.validateJSONContent(item, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateURLPath validates the URL path for security threats
func (sm *SecurityMiddleware) validateURLPath(r *http.Request) error {
	path := r.URL.Path

	// Basic path validation
	if err := sm.validator.ValidateString(path, "URL path"); err != nil {
		return err
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal attempt detected")
	}

	// Check for encoded path traversal
	if strings.Contains(path, "%2e%2e") || strings.Contains(path, "%2E%2E") {
		return fmt.Errorf("encoded path traversal attempt detected")
	}

	return nil
}

// validatePathVariable validates specific path variables based on their context
func (sm *SecurityMiddleware) validatePathVariable(name, value string) error {
	switch name {
	case "name":
		// Validate as DNS name for resource names
		return sm.validator.ValidateDNSName(value, name)
	case "id":
		// Validate as UUID if it looks like one, otherwise as general string
		if len(value) == 36 && strings.Count(value, "-") == 4 {
			return sm.validator.ValidateUUID(value, name)
		}
		return sm.validator.ValidateString(value, name)
	case "version":
		// Validate as semantic version
		return sm.validator.ValidateSemVer(value, name)
	case "environment":
		// Validate environment names
		validEnvironments := []string{"dev", "qa", "uat", "prod", "staging", "production", "development", "testing"}
		for _, env := range validEnvironments {
			if strings.EqualFold(value, env) {
				return sm.validator.ValidateDNSName(value, name)
			}
		}
		return fmt.Errorf("invalid environment name")
	default:
		// Default string validation
		return sm.validator.ValidateString(value, name)
	}
}

// validateHostHeader validates the Host header
func (sm *SecurityMiddleware) validateHostHeader(host string) error {
	// Remove port if present
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Validate as hostname
	return sm.validator.ValidateString(host, "Host header")
}

// validateContentType validates Content-Type header
func (sm *SecurityMiddleware) validateContentType(contentType string) error {
	// Extract media type (ignore parameters like charset)
	mediaType := strings.Split(contentType, ";")[0]
	mediaType = strings.TrimSpace(mediaType)

	// Check if content type is allowed
	for _, allowed := range sm.config.AllowedContentTypes {
		if mediaType == allowed {
			return nil
		}
	}

	return fmt.Errorf("content type %s is not allowed", mediaType)
}

// shouldValidateBody determines if the request body should be validated
func (sm *SecurityMiddleware) shouldValidateBody(r *http.Request) bool {
	if !sm.config.ValidateJSONBody {
		return false
	}

	// Only validate body for methods that typically have a body
	switch r.Method {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}

// Utility methods

// writeError writes a standardized error response
func (sm *SecurityMiddleware) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := map[string]interface{}{
		"error":     message,
		"code":      statusCode,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(errorResponse)
}

// getClientIPFromRequest extracts client IP for rate limiting
func getClientIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// Take the first IP in case of multiple IPs
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex]
	}

	return ip
}

// CreateSecurityMiddleware creates a simple security validation middleware
func CreateSecurityMiddleware(validator *Validator) func(http.Handler) http.Handler {
	middleware, _ := NewSecurityMiddleware(validator, DefaultMiddlewareConfig())
	return middleware.SecurityMiddleware()
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rateLimiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIPFromRequest(r)

			if !rateLimiter.Allow(clientIP) {
				writeErrorResponse(w, fmt.Sprintf("Rate limit exceeded for client %s", clientIP), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper function for rate limit middleware

func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := map[string]interface{}{
		"error":     message,
		"code":      statusCode,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(errorResponse)
}
