package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/contextops/platformctl/internal/models"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidAudience  = errors.New("invalid audience")
	ErrInvalidIssuer    = errors.New("invalid issuer")
	ErrMissingClaims    = errors.New("missing required claims")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// TokenType represents different types of JWT tokens
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeMFA     TokenType = "mfa"
)

// CustomClaims extends jwt.RegisteredClaims with application-specific claims
type CustomClaims struct {
	UserID         string                 `json:"user_id"`
	CustomerID     uuid.UUID              `json:"customer_id"`
	SessionID      string                 `json:"session_id"`
	TokenType      TokenType              `json:"token_type"`
	Permissions    []string               `json:"permissions,omitempty"`
	Roles          []string               `json:"roles,omitempty"`
	MFAVerified    bool                   `json:"mfa_verified"`
	IsPrivileged   bool                   `json:"is_privileged"`
	Scopes         []string               `json:"scopes,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

// JWTConfig holds the configuration for JWT token management
type JWTConfig struct {
	PrivateKey              *rsa.PrivateKey
	PublicKey               *rsa.PublicKey
	Issuer                  string
	AccessTokenExpiry       time.Duration
	RefreshTokenExpiry      time.Duration
	MFATokenExpiry          time.Duration
	AllowedAudiences        []string
	RequireMFAForPrivileged bool
}

// JWTManager manages JWT token creation and validation
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager creates a new JWT manager with the given configuration
func NewJWTManager(config *JWTConfig) (*JWTManager, error) {
	if config.PrivateKey == nil {
		// Generate a new RSA key pair if not provided
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %w", err)
		}
		config.PrivateKey = privateKey
		config.PublicKey = &privateKey.PublicKey
	}

	// Set default values
	if config.Issuer == "" {
		config.Issuer = "platformctl"
	}
	if config.AccessTokenExpiry == 0 {
		config.AccessTokenExpiry = 15 * time.Minute
	}
	if config.RefreshTokenExpiry == 0 {
		config.RefreshTokenExpiry = 24 * time.Hour
	}
	if config.MFATokenExpiry == 0 {
		config.MFATokenExpiry = 5 * time.Minute
	}
	if len(config.AllowedAudiences) == 0 {
		config.AllowedAudiences = []string{"platformctl-api", "platformctl-cli"}
	}

	return &JWTManager{config: config}, nil
}

// GenerateTokenPair generates both access and refresh tokens for a user
func (j *JWTManager) GenerateTokenPair(user *models.Customer, sessionID string, permissions []string, roles []string) (*TokenPair, error) {
	now := time.Now()
	
	// Create access token
	accessClaims := &CustomClaims{
		UserID:      user.Username, // Using username as userID
		CustomerID:  user.ID,
		SessionID:   sessionID,
		TokenType:   TokenTypeAccess,
		Permissions: permissions,
		Roles:       roles,
		MFAVerified: false, // Will be updated when MFA is verified
		IsPrivileged: containsPrivilegedRole(roles),
		Scopes:      []string{"read", "write"},
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    j.config.Issuer,
			Subject:   user.Username,
			Audience:  j.config.AllowedAudiences,
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.AccessTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken, err := j.createToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}

	// Create refresh token with longer expiry and minimal claims
	refreshClaims := &CustomClaims{
		UserID:     user.Username,
		CustomerID: user.ID,
		SessionID:  sessionID,
		TokenType:  TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    j.config.Issuer,
			Subject:   user.Username,
			Audience:  j.config.AllowedAudiences,
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.RefreshTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	refreshToken, err := j.createToken(refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(j.config.AccessTokenExpiry.Seconds()),
	}, nil
}

// GenerateMFAToken generates a short-lived MFA verification token
func (j *JWTManager) GenerateMFAToken(userID string, customerID uuid.UUID, sessionID string) (string, error) {
	now := time.Now()
	
	claims := &CustomClaims{
		UserID:     userID,
		CustomerID: customerID,
		SessionID:  sessionID,
		TokenType:  TokenTypeMFA,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    j.config.Issuer,
			Subject:   userID,
			Audience:  j.config.AllowedAudiences,
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.MFATokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	return j.createToken(claims)
}

// ValidateToken validates and parses a JWT token
func (j *JWTManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.config.PublicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Validate custom claims
	if err := j.validateCustomClaims(claims); err != nil {
		return nil, err
	}

	// Ensure the metadata map is always initialized so callers can safely
	// read from and write to it without a nil-map check. The "omitempty" tag
	// means an empty map is dropped during serialization, so a freshly parsed
	// token always arrives here with a nil map.
	if claims.Metadata == nil {
		claims.Metadata = make(map[string]interface{})
	}

	return claims, nil
}

// RefreshTokens generates new access and refresh tokens using a valid refresh token
func (j *JWTManager) RefreshTokens(refreshTokenString string, permissions []string, roles []string) (*TokenPair, error) {
	claims, err := j.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	if claims.TokenType != TokenTypeRefresh {
		return nil, errors.New("not a refresh token")
	}

	// Create a mock customer for token generation
	customer := &models.Customer{
		ID:       claims.CustomerID,
		Username: claims.UserID,
	}

	return j.GenerateTokenPair(customer, claims.SessionID, permissions, roles)
}

// UpdateMFAStatus creates a new access token with updated MFA status
func (j *JWTManager) UpdateMFAStatus(accessTokenString string, mfaVerified bool) (string, error) {
	claims, err := j.ValidateToken(accessTokenString)
	if err != nil {
		return "", fmt.Errorf("invalid access token: %w", err)
	}

	if claims.TokenType != TokenTypeAccess {
		return "", errors.New("not an access token")
	}

	// Update MFA status
	claims.MFAVerified = mfaVerified
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	
	// Generate new token ID
	claims.ID = uuid.New().String()

	return j.createToken(claims)
}

// ExtractClaims extracts claims from a token without full validation (useful for debugging)
func (j *JWTManager) ExtractClaims(tokenString string) (*CustomClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &CustomClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// createToken creates and signs a JWT token with the given claims
func (j *JWTManager) createToken(claims *CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	
	// Add key ID to header for key rotation support
	token.Header["kid"] = "1" // Key identifier
	
	tokenString, err := token.SignedString(j.config.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// validateCustomClaims validates application-specific claims
func (j *JWTManager) validateCustomClaims(claims *CustomClaims) error {
	// Validate required fields
	if claims.UserID == "" {
		return fmt.Errorf("%w: user_id is required", ErrMissingClaims)
	}
	
	if claims.CustomerID == uuid.Nil {
		return fmt.Errorf("%w: customer_id is required", ErrMissingClaims)
	}

	if claims.SessionID == "" {
		return fmt.Errorf("%w: session_id is required", ErrMissingClaims)
	}

	// Validate token type
	validTokenTypes := []TokenType{TokenTypeAccess, TokenTypeRefresh, TokenTypeMFA}
	validType := false
	for _, validTokenType := range validTokenTypes {
		if claims.TokenType == validTokenType {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("%w: invalid token type", ErrInvalidToken)
	}

	// Validate issuer
	if claims.Issuer != j.config.Issuer {
		return ErrInvalidIssuer
	}

	// Validate audience
	if len(claims.Audience) == 0 {
		return ErrInvalidAudience
	}
	
	validAudience := false
	for _, tokenAud := range claims.Audience {
		for _, allowedAud := range j.config.AllowedAudiences {
			if tokenAud == allowedAud {
				validAudience = true
				break
			}
		}
		if validAudience {
			break
		}
	}
	if !validAudience {
		return ErrInvalidAudience
	}

	// Validate MFA requirement for privileged operations
	if j.config.RequireMFAForPrivileged && claims.IsPrivileged && !claims.MFAVerified {
		return errors.New("MFA required for privileged operations")
	}

	return nil
}

// TokenPair represents an access/refresh token pair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// containsPrivilegedRole checks if the roles contain any privileged roles
func containsPrivilegedRole(roles []string) bool {
	privilegedRoles := []string{"admin", "operator", "deployer"}
	
	for _, role := range roles {
		for _, privilegedRole := range privilegedRoles {
			if role == privilegedRole {
				return true
			}
		}
	}
	
	return false
}

// DefaultJWTConfig returns a default JWT configuration for development
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		Issuer:                  "platformctl",
		AccessTokenExpiry:       15 * time.Minute,
		RefreshTokenExpiry:      24 * time.Hour,
		MFATokenExpiry:          5 * time.Minute,
		AllowedAudiences:        []string{"platformctl-api", "platformctl-cli"},
		RequireMFAForPrivileged: true,
	}
}