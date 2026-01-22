package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/contextops/platformctl/internal/models"
	"golang.org/x/crypto/argon2"
)

// Enhanced context keys
type EnhancedContextKey string

const (
	EnhancedCustomerKey  EnhancedContextKey = "enhanced_customer"
	UserIDKey           EnhancedContextKey = "user_id"
	SessionIDKey        EnhancedContextKey = "session_id"
	JWTClaimsKey        EnhancedContextKey = "jwt_claims"
	RequestIDKey        EnhancedContextKey = "request_id"
)

var (
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrInvalidAuthHeader  = errors.New("invalid authorization header")
	ErrMissingToken       = errors.New("missing authentication token")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
)

// EnhancedCustomer represents a customer with additional authentication context
type EnhancedCustomer struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Salt         string    `json:"-" db:"salt"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	IsActive     bool      `json:"is_active" db:"is_active"`
}

// SessionInfo represents session information
type SessionInfo struct {
	ID           string    `json:"id" db:"id"`
	CustomerID   uuid.UUID `json:"customer_id" db:"customer_id"`
	UserID       string    `json:"user_id" db:"user_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	LastActivity time.Time `json:"last_activity" db:"last_activity"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	IPAddress    *string   `json:"ip_address" db:"ip_address"`
	UserAgent    *string   `json:"user_agent" db:"user_agent"`
	LoginMethod  string    `json:"login_method" db:"login_method"`
	MFAVerified  bool      `json:"mfa_verified" db:"mfa_verified"`
}

// EnhancedAuthService provides comprehensive authentication services
type EnhancedAuthService struct {
	db         *sql.DB
	jwtManager *JWTManager
}

// NewEnhancedAuthService creates a new enhanced authentication service
func NewEnhancedAuthService(db *sql.DB, jwtManager *JWTManager) *EnhancedAuthService {
	return &EnhancedAuthService{
		db:         db,
		jwtManager: jwtManager,
	}
}

// JWTMiddleware returns a middleware that validates JWT tokens
func (a *EnhancedAuthService) JWTMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID for tracing
			requestID := uuid.New()
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				a.writeAuthError(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Check if it's a Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				a.writeAuthError(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate the token
			claims, err := a.jwtManager.ValidateToken(tokenString)
			if err != nil {
				if errors.Is(err, ErrTokenExpired) {
					a.writeAuthError(w, "Token expired", http.StatusUnauthorized)
					return
				}
				a.writeAuthError(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Verify session is still active
			session, err := a.GetSession(claims.SessionID)
			if err != nil {
				a.writeAuthError(w, "Session not found", http.StatusUnauthorized)
				return
			}

			if !session.IsActive || session.ExpiresAt.Before(time.Now()) {
				a.writeAuthError(w, "Session expired", http.StatusUnauthorized)
				return
			}

			// Update last activity
			if err := a.UpdateSessionActivity(claims.SessionID); err != nil {
				// Log error but don't fail the request
				fmt.Printf("Warning: Failed to update session activity: %v\n", err)
			}

			// Get customer from database
			customer, err := a.GetCustomerByID(claims.CustomerID)
			if err != nil {
				a.writeAuthError(w, "Customer not found", http.StatusUnauthorized)
				return
			}

			// Add authentication context
			ctx = context.WithValue(ctx, EnhancedCustomerKey, customer)
			ctx = context.WithValue(ctx, JWTClaimsKey, claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BasicAuthFallbackMiddleware provides basic auth fallback for development/testing
func (a *EnhancedAuthService) BasicAuthFallbackMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID for tracing
			requestID := uuid.New()
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)

			// Extract basic auth credentials
			username, password, ok := r.BasicAuth()
			if !ok {
				a.writeAuthError(w, "Missing basic authentication", http.StatusUnauthorized)
				return
			}

			// Validate credentials
			customer, err := a.ValidateCustomer(username, password)
			if err != nil {
				a.writeAuthError(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			// Convert to models.Customer for context
			modelCustomer := &models.Customer{
				ID:        customer.ID,
				Username:  customer.Username,
				CreatedAt: customer.CreatedAt,
				UpdatedAt: customer.UpdatedAt,
				Active:    customer.IsActive,
			}

			// Add customer to context (without session/JWT claims)
			ctx = context.WithValue(ctx, EnhancedCustomerKey, modelCustomer)
			ctx = context.WithValue(ctx, UserIDKey, customer.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth extracts authentication info from context
func RequireAuth(ctx context.Context) (*models.Customer, *CustomClaims, error) {
	customer, ok := ctx.Value(EnhancedCustomerKey).(*models.Customer)
	if !ok {
		return nil, nil, ErrUnauthorized
	}

	// JWT claims are optional (may not exist for basic auth)
	claims, _ := ctx.Value(JWTClaimsKey).(*CustomClaims)

	return customer, claims, nil
}

// RequireEnhancedCustomer extracts the customer from context
func RequireEnhancedCustomer(ctx context.Context) (*models.Customer, error) {
	customer, _, err := RequireAuth(ctx)
	return customer, err
}

// RequireClaims extracts JWT claims from context
func RequireClaims(ctx context.Context) (*CustomClaims, error) {
	claims, ok := ctx.Value(JWTClaimsKey).(*CustomClaims)
	if !ok {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// RequirePermission checks if the user has the specified permission
func RequirePermission(ctx context.Context, permission string) error {
	claims, err := RequireClaims(ctx)
	if err != nil {
		return err
	}

	for _, perm := range claims.Permissions {
		if perm == permission || perm == "*" {
			return nil
		}
	}

	return fmt.Errorf("insufficient permissions: %s required", permission)
}

// RequireMFA checks if MFA is verified for the current session
func RequireMFA(ctx context.Context) error {
	claims, err := RequireClaims(ctx)
	if err != nil {
		return err
	}

	if !claims.MFAVerified {
		return errors.New("MFA verification required")
	}

	return nil
}

// Login authenticates a user and returns JWT tokens with session
func (a *EnhancedAuthService) Login(r *http.Request, username, password string, permissions []string, roles []string) (*TokenPair, string, error) {
	// Validate credentials
	customer, err := a.ValidateCustomer(username, password)
	if err != nil {
		return nil, "", err
	}

	// Create a session
	sessionID := uuid.New().String()
	clientIP := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")
	
	// Store session in database
	err = a.CreateSession(sessionID, customer.ID, username, userAgent, clientIP)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Convert to models.Customer for token generation
	modelCustomer := &models.Customer{
		ID:       customer.ID,
		Username: customer.Username,
	}

	// Generate JWT tokens
	tokens, err := a.jwtManager.GenerateTokenPair(modelCustomer, sessionID, permissions, roles)
	if err != nil {
		// Clean up session if token generation fails
		a.InvalidateSession(sessionID)
		return nil, "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	return tokens, sessionID, nil
}

// RefreshToken refreshes tokens and extends session
func (a *EnhancedAuthService) RefreshToken(refreshToken string, permissions []string, roles []string) (*TokenPair, error) {
	// Validate refresh token and get new tokens
	tokens, err := a.jwtManager.RefreshTokens(refreshToken, permissions, roles)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}

	// Extract claims to get session ID
	claims, err := a.jwtManager.ExtractClaims(tokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	// Update session activity
	if err := a.UpdateSessionActivity(claims.SessionID); err != nil {
		// Log warning but don't fail the request
		fmt.Printf("Warning: Failed to update session activity: %v\n", err)
	}

	return tokens, nil
}

// Logout invalidates session and tokens
func (a *EnhancedAuthService) Logout(sessionID string) error {
	return a.InvalidateSession(sessionID)
}

// Database operations

// CreateSession creates a new user session
func (a *EnhancedAuthService) CreateSession(sessionID string, customerID uuid.UUID, userID, userAgent, ipAddress string) error {
	query := `
		INSERT INTO sessions (id, customer_id, user_id, created_at, expires_at, last_activity, 
							  is_active, ip_address, user_agent, login_method, mfa_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour) // 24 hour session

	_, err := a.db.Exec(query,
		sessionID,
		customerID,
		userID,
		now,
		expiresAt,
		now,
		true,
		ipAddress,
		userAgent,
		"password",
		false,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves session information
func (a *EnhancedAuthService) GetSession(sessionID string) (*SessionInfo, error) {
	query := `
		SELECT id, customer_id, user_id, created_at, expires_at, last_activity,
			   is_active, ip_address, user_agent, login_method, mfa_verified
		FROM sessions WHERE id = $1`

	session := &SessionInfo{}
	err := a.db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.CustomerID,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActivity,
		&session.IsActive,
		&session.IPAddress,
		&session.UserAgent,
		&session.LoginMethod,
		&session.MFAVerified,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (a *EnhancedAuthService) UpdateSessionActivity(sessionID string) error {
	query := `UPDATE sessions SET last_activity = $1 WHERE id = $2 AND is_active = true`
	
	_, err := a.db.Exec(query, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}

	return nil
}

// InvalidateSession marks a session as inactive
func (a *EnhancedAuthService) InvalidateSession(sessionID string) error {
	query := `UPDATE sessions SET is_active = false WHERE id = $1`
	
	_, err := a.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to invalidate session: %w", err)
	}

	return nil
}

// Customer management

// GetCustomerByID retrieves a customer by ID
func (a *EnhancedAuthService) GetCustomerByID(customerID uuid.UUID) (*models.Customer, error) {
	query := `SELECT id, username, created_at, updated_at, is_active 
			  FROM customers WHERE id = $1 AND is_active = true`

	customer := &models.Customer{}
	err := a.db.QueryRow(query, customerID).Scan(
		&customer.ID,
		&customer.Username,
		&customer.CreatedAt,
		&customer.UpdatedAt,
		&customer.Active,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return customer, nil
}

// GetCustomer retrieves a customer by username
func (a *EnhancedAuthService) GetCustomer(username string) (*EnhancedCustomer, error) {
	query := `SELECT id, username, password_hash, salt, created_at, updated_at, is_active 
			  FROM customers WHERE username = $1 AND is_active = true`

	customer := &EnhancedCustomer{}
	err := a.db.QueryRow(query, username).Scan(
		&customer.ID,
		&customer.Username,
		&customer.PasswordHash,
		&customer.Salt,
		&customer.CreatedAt,
		&customer.UpdatedAt,
		&customer.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return customer, nil
}

// ValidateCustomer validates customer credentials
func (a *EnhancedAuthService) ValidateCustomer(username, password string) (*EnhancedCustomer, error) {
	customer, err := a.GetCustomer(username)
	if err != nil {
		return nil, err
	}

	valid, err := a.VerifyPassword(password, customer.PasswordHash, customer.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}

	if !valid {
		return nil, ErrInvalidCredentials
	}

	return customer, nil
}

// Password management

// HashPassword generates a secure hash of the password using Argon2
func (a *EnhancedAuthService) HashPassword(password string) (string, string, error) {
	// Generate a random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}

	// Generate hash using Argon2
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Encode salt and hash to base64 for storage
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return encodedHash, encodedSalt, nil
}

// VerifyPassword verifies a password against a stored hash and salt
func (a *EnhancedAuthService) VerifyPassword(password, storedHash, storedSalt string) (bool, error) {
	// Decode the stored salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(storedSalt)
	if err != nil {
		return false, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(storedHash)
	if err != nil {
		return false, err
	}

	// Generate hash for the provided password
	newHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Compare hashes using constant time comparison
	return subtle.ConstantTimeCompare(hash, newHash) == 1, nil
}

// Utility functions

// writeAuthError writes a standardized authentication error response
func (a *EnhancedAuthService) writeAuthError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"error": "%s", "code": %d}`, message, statusCode)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// Take the first IP in case of multiple IPs
		ips := strings.Split(xForwardedFor, ",")
		clientIP := strings.TrimSpace(ips[0])
		if net.ParseIP(clientIP) != nil {
			return clientIP
		}
	}

	// Check X-Real-IP header
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" && net.ParseIP(xRealIP) != nil {
		return xRealIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}