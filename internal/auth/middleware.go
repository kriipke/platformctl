package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

type CustomerContext struct {
	CustomerID   string   `json:"customer_id"`
	CustomerName string   `json:"customer_name"`
	UserID       string   `json:"user_id"`
	Permissions  []string `json:"permissions"`
}

type contextKey string

const customerKey contextKey = "customer"

func CustomerAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			customerID := r.Header.Get("X-Customer-ID")
			if customerID == "" {
				customerID = "system"
			}

			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				userID = "system-user"
			}

			customerCtx := &CustomerContext{
				CustomerID:   customerID,
				CustomerName: customerID,
				UserID:       userID,
				Permissions:  []string{"context:read", "context:write"},
			}

			ctx := context.WithValue(r.Context(), customerKey, customerCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetCustomerFromContext(ctx context.Context) (*CustomerContext, error) {
	customer, ok := ctx.Value(customerKey).(*CustomerContext)
	if !ok {
		return nil, errors.New("no customer context found")
	}
	return customer, nil
}

func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			customer, err := GetCustomerFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
				return
			}

			hasPermission := false
			for _, perm := range customer.Permissions {
				if perm == permission || perm == "*" {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				http.Error(w, fmt.Sprintf("Forbidden: missing permission %s", permission), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func CustomerIsolationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			customer, err := GetCustomerFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized: no customer context", http.StatusUnauthorized)
				return
			}

			r.Header.Set("X-Database-Customer-ID", customer.CustomerID)
			next.ServeHTTP(w, r)
		})
	}
}
