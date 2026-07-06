package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
)

type contextKey string

const CustomerContextKey contextKey = "customer"

const grantedPermissionContextKey contextKey = "granted_permission"

const customerContextKey contextKey = "customer"

// Customer represents the authenticated customer
type Customer struct {
	CustomerID string
	Username   string
	Roles      []string
}

// JWTMiddleware creates a JWT authentication middleware using the provided JWT manager
func JWTMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	service := &EnhancedAuthService{jwtManager: jwtManager}
	return service.JWTMiddleware()
}

// RBACMiddleware creates an RBAC middleware using the provided RBAC manager
func RBACMiddleware(rbacManager *RBACManager) func(http.Handler) http.Handler {
	service := &EnhancedAuthService{}
	return service.RBACMiddleware(rbacManager)
}

// BasicAuthMiddleware provides basic authentication middleware
func BasicAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract basic auth header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Parse basic auth
			const basicScheme = "Basic "
			if !strings.HasPrefix(authHeader, basicScheme) {
				http.Error(w, "Invalid authorization scheme", http.StatusUnauthorized)
				return
			}

			encodedCredentials := authHeader[len(basicScheme):]
			decodedCredentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
			if err != nil {
				http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
				return
			}

			credentials := strings.SplitN(string(decodedCredentials), ":", 2)
			if len(credentials) != 2 {
				http.Error(w, "Invalid credentials format", http.StatusUnauthorized)
				return
			}

			username, password := credentials[0], credentials[1]

			// Validate credentials (simplified for demo - should use proper auth)
			customer, err := validateCredentials(username, password)
			if err != nil {
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			// Add customer to context
			ctx := context.WithValue(r.Context(), CustomerContextKey, customer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validateCredentials validates username/password and returns customer info
// In a real implementation, this would validate against a database or identity provider
func validateCredentials(username, password string) (*Customer, error) {
	// Simplified validation - in reality, this would check against a database
	// and hash passwords properly
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// Extract customer ID from username (simplified format: customer_id:username)
	parts := strings.SplitN(username, ":", 2)
	customerID := username
	actualUsername := username

	if len(parts) == 2 {
		customerID = parts[0]
		actualUsername = parts[1]
	}

	return &Customer{
		CustomerID: customerID,
		Username:   actualUsername,
		Roles:      []string{"user"},
	}, nil
}

// GetCustomerFromContext extracts the customer from the request context
func GetCustomerFromContext(ctx context.Context) (*Customer, bool) {
	customer, ok := ctx.Value(CustomerContextKey).(*Customer)
	return customer, ok
}

// RequireCustomer is a helper that gets the customer or returns an error
func RequireCustomer(ctx context.Context) (*Customer, error) {
	customer, ok := GetCustomerFromContext(ctx)
	if !ok {
		return nil, ErrNoCustomerInContext
	}
	return customer, nil
}
