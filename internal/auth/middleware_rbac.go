package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// RBACMiddleware provides Role-Based Access Control middleware
func (a *EnhancedAuthService) RBACMiddleware(rbacManager *RBACManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract authentication context
			customer, claims, err := RequireAuth(r.Context())
			if err != nil {
				a.writeAuthError(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Determine required permission based on request
			permission := a.determineRequiredPermission(r)
			if permission.Action == "" {
				// No specific permission required, proceed
				next.ServeHTTP(w, r)
				return
			}

			// Check permission
			var userID string
			if claims != nil {
				userID = claims.UserID
			} else {
				userID = customer.Username
			}

			hasPermission, err := rbacManager.CheckPermission(
				r.Context(),
				userID,
				customer.ID,
				permission.ResourceType,
				permission.ResourceID,
				permission.Action,
			)

			if err != nil {
				a.writeAuthError(w, "Error checking permissions", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				a.writeAuthError(w, fmt.Sprintf("Insufficient permissions: %s:%s required", permission.ResourceType, permission.Action), http.StatusForbidden)
				return
			}

			// Add permission context for downstream handlers
			ctx := context.WithValue(r.Context(), "granted_permission", permission)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermissionMiddleware creates middleware that requires specific permission
func RequirePermissionMiddleware(rbacManager *RBACManager, resourceType, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract authentication context
			customer, claims, err := RequireAuth(r.Context())
			if err != nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Extract resource ID from URL parameters if available
			vars := mux.Vars(r)
			var resourceID *string
			if name, ok := vars["name"]; ok && name != "" {
				resourceID = &name
			}

			// Check permission
			var userID string
			if claims != nil {
				userID = claims.UserID
			} else {
				userID = customer.Username
			}

			hasPermission, err := rbacManager.CheckPermission(
				r.Context(),
				userID,
				customer.ID,
				resourceType,
				resourceID,
				action,
			)

			if err != nil {
				http.Error(w, "Error checking permissions", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				http.Error(w, fmt.Sprintf("Insufficient permissions: %s:%s required", resourceType, action), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MFARequiredMiddleware ensures MFA is verified for privileged operations
func MFARequiredMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := RequireClaims(r.Context())
			if err != nil {
				http.Error(w, "JWT authentication required", http.StatusUnauthorized)
				return
			}

			// Check if this is a privileged operation requiring MFA
			if claims.IsPrivileged && !claims.MFAVerified {
				http.Error(w, "MFA verification required for privileged operations", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnlyMiddleware restricts access to admin users only
func AdminOnlyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := RequireClaims(r.Context())
			if err != nil {
				http.Error(w, "JWT authentication required", http.StatusUnauthorized)
				return
			}

			// Check if user has admin role
			isAdmin := false
			for _, role := range claims.Roles {
				if role == "customer-admin" || role == "admin" {
					isAdmin = true
					break
				}
			}

			if !isAdmin {
				http.Error(w, "Admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiredPermission represents a permission requirement
type RequiredPermission struct {
	ResourceType string
	ResourceID   *string
	Action       string
}

// determineRequiredPermission determines the required permission based on the HTTP request
func (a *EnhancedAuthService) determineRequiredPermission(r *http.Request) RequiredPermission {
	method := r.Method
	path := r.URL.Path
	vars := mux.Vars(r)

	// Extract resource name from URL if available
	var resourceID *string
	if name, ok := vars["name"]; ok && name != "" {
		resourceID = &name
	}

	// Map HTTP methods to actions
	actionMap := map[string]string{
		"GET":    "read",
		"POST":   "create",
		"PUT":    "update",
		"PATCH":  "update",
		"DELETE": "delete",
	}

	action := actionMap[method]
	if action == "" {
		action = "read" // Default to read for unknown methods
	}

	// Determine resource type from path
	resourceType := a.extractResourceType(path)

	// Handle special cases
	if method == "POST" && resourceType != "" && resourceID != nil {
		// POST to specific resource might be a special action (e.g., deploy)
		if path == fmt.Sprintf("/%ss/%s/deploy", resourceType, *resourceID) {
			action = "deploy"
		}
	}

	return RequiredPermission{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
	}
}

// extractResourceType extracts the resource type from the request path
func (a *EnhancedAuthService) extractResourceType(path string) string {
	// Remove leading slash and split by slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	parts := splitPath(path)
	if len(parts) == 0 {
		return ""
	}

	// Map URL prefixes to resource types
	resourceMap := map[string]string{
		"apps":         "app",
		"environments": "environment",
		"contexts":     "context",
		"users":        "user",
		"admin":        "system",
		"health":       "system",
	}

	if resourceType, ok := resourceMap[parts[0]]; ok {
		return resourceType
	}

	return "system" // Default to system for unknown paths
}

// splitPath splits a path by forward slash
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	var parts []string
	start := 0

	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}

	// Add the last part
	if start < len(path) {
		parts = append(parts, path[start:])
	}

	return parts
}

// ConditionalPermissionMiddleware applies permissions based on conditions
func ConditionalPermissionMiddleware(rbacManager *RBACManager, condition func(*http.Request) bool, resourceType, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if condition applies
			if !condition(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Apply permission check
			permissionMiddleware := RequirePermissionMiddleware(rbacManager, resourceType, action)
			permissionMiddleware(next).ServeHTTP(w, r)
		})
	}
}

// EnvironmentSpecificMiddleware applies different permissions based on environment
func EnvironmentSpecificMiddleware(rbacManager *RBACManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract environment from context or URL
			environment := extractEnvironmentFromRequest(r)
			
			// Different permission requirements for different environments
			var requiredAction string
			switch environment {
			case "production", "prod":
				// Production requires higher privileges
				requiredAction = "deploy-prod"
			case "staging", "uat":
				requiredAction = "deploy-staging"
			default:
				requiredAction = "deploy"
			}

			// Apply environment-specific permission check
			vars := mux.Vars(r)
			var resourceID *string
			if name, ok := vars["name"]; ok {
				resourceID = &name
			}

			customer, claims, err := RequireAuth(r.Context())
			if err != nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			var userID string
			if claims != nil {
				userID = claims.UserID
			} else {
				userID = customer.Username
			}

			hasPermission, err := rbacManager.CheckPermission(
				r.Context(),
				userID,
				customer.ID,
				"context", // Deployment permissions are typically context-based
				resourceID,
				requiredAction,
			)

			if err != nil {
				http.Error(w, "Error checking permissions", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				http.Error(w, fmt.Sprintf("Insufficient permissions for %s environment", environment), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractEnvironmentFromRequest extracts environment information from request
func extractEnvironmentFromRequest(r *http.Request) string {
	// Try to get environment from URL parameters
	vars := mux.Vars(r)
	if env, ok := vars["environment"]; ok {
		return env
	}

	// Try to get environment from query parameters
	if env := r.URL.Query().Get("environment"); env != "" {
		return env
	}

	// Try to get environment from headers
	if env := r.Header.Get("X-Environment"); env != "" {
		return env
	}

	return "development" // Default environment
}

// AuditMiddleware combines RBAC with audit logging
func (a *EnhancedAuthService) AuditMiddleware(rbacManager *RBACManager, auditLogger *AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply RBAC first
			rbacMiddleware := a.RBACMiddleware(rbacManager)
			
			// Then apply audit logging
			auditMiddleware := (*auditLogger).Middleware()
			
			// Chain the middlewares
			rbacMiddleware(auditMiddleware(next)).ServeHTTP(w, r)
		})
	}
}

// AuditLogger interface for audit middleware integration
type AuditLogger interface {
	Middleware() func(http.Handler) http.Handler
}