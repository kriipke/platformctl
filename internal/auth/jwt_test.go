package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kriipke/platformctl/internal/models"
)

func TestNewJWTManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *JWTConfig
		wantErr bool
		check   func(*testing.T, *JWTManager)
	}{
		{
			name: "with provided keys",
			config: func() *JWTConfig {
				privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				return &JWTConfig{
					PrivateKey:              privateKey,
					PublicKey:               &privateKey.PublicKey,
					Issuer:                  "test-issuer",
					AccessTokenExpiry:       10 * time.Minute,
					RefreshTokenExpiry:      24 * time.Hour,
					MFATokenExpiry:          5 * time.Minute,
					AllowedAudiences:        []string{"test-api"},
					RequireMFAForPrivileged: true,
				}
			}(),
			wantErr: false,
			check: func(t *testing.T, manager *JWTManager) {
				assert.Equal(t, "test-issuer", manager.config.Issuer)
				assert.Equal(t, 10*time.Minute, manager.config.AccessTokenExpiry)
				assert.Equal(t, []string{"test-api"}, manager.config.AllowedAudiences)
				assert.True(t, manager.config.RequireMFAForPrivileged)
			},
		},
		{
			name: "without keys (auto-generate)",
			config: &JWTConfig{
				Issuer: "auto-gen-test",
			},
			wantErr: false,
			check: func(t *testing.T, manager *JWTManager) {
				assert.NotNil(t, manager.config.PrivateKey)
				assert.NotNil(t, manager.config.PublicKey)
				assert.Equal(t, "auto-gen-test", manager.config.Issuer)
				// Check defaults
				assert.Equal(t, 15*time.Minute, manager.config.AccessTokenExpiry)
				assert.Equal(t, 24*time.Hour, manager.config.RefreshTokenExpiry)
				assert.Equal(t, 5*time.Minute, manager.config.MFATokenExpiry)
				assert.Equal(t, []string{"platformctl-api", "platformctl-cli"}, manager.config.AllowedAudiences)
			},
		},
		{
			name:   "default config",
			config: &JWTConfig{},
			wantErr: false,
			check: func(t *testing.T, manager *JWTManager) {
				assert.Equal(t, "platformctl", manager.config.Issuer)
				assert.NotNil(t, manager.config.PrivateKey)
				assert.NotNil(t, manager.config.PublicKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewJWTManager(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				if tt.check != nil {
					tt.check(t, manager)
				}
			}
		})
	}
}

func TestJWTManagerGenerateTokenPair(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{
		Issuer:            "test",
		AccessTokenExpiry: 15 * time.Minute,
		RefreshTokenExpiry: 24 * time.Hour,
	})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	sessionID := "test-session"
	permissions := []string{"app:read", "app:create"}
	roles := []string{"customer-viewer", "deployer"}

	tokenPair, err := manager.GenerateTokenPair(customer, sessionID, permissions, roles)
	assert.NoError(t, err)
	assert.NotNil(t, tokenPair)

	// Validate token pair structure
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.Equal(t, int64(900), tokenPair.ExpiresIn) // 15 minutes

	// Validate access token claims
	accessClaims, err := manager.ValidateToken(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, customer.Username, accessClaims.UserID)
	assert.Equal(t, customer.ID, accessClaims.CustomerID)
	assert.Equal(t, sessionID, accessClaims.SessionID)
	assert.Equal(t, TokenTypeAccess, accessClaims.TokenType)
	assert.Equal(t, permissions, accessClaims.Permissions)
	assert.Equal(t, roles, accessClaims.Roles)
	assert.False(t, accessClaims.MFAVerified)
	assert.True(t, accessClaims.IsPrivileged) // deployer is privileged
	assert.Equal(t, []string{"read", "write"}, accessClaims.Scopes)

	// Validate refresh token claims
	refreshClaims, err := manager.ValidateToken(tokenPair.RefreshToken)
	assert.NoError(t, err)
	assert.Equal(t, customer.Username, refreshClaims.UserID)
	assert.Equal(t, customer.ID, refreshClaims.CustomerID)
	assert.Equal(t, sessionID, refreshClaims.SessionID)
	assert.Equal(t, TokenTypeRefresh, refreshClaims.TokenType)
	assert.Empty(t, refreshClaims.Permissions) // Refresh tokens have no permissions
	assert.Empty(t, refreshClaims.Roles)
}

func TestJWTManagerGenerateMFAToken(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{
		MFATokenExpiry: 5 * time.Minute,
	})
	require.NoError(t, err)

	userID := "test-user"
	customerID := uuid.New()
	sessionID := "test-session"

	mfaToken, err := manager.GenerateMFAToken(userID, customerID, sessionID)
	assert.NoError(t, err)
	assert.NotEmpty(t, mfaToken)

	// Validate MFA token
	claims, err := manager.ValidateToken(mfaToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, customerID, claims.CustomerID)
	assert.Equal(t, sessionID, claims.SessionID)
	assert.Equal(t, TokenTypeMFA, claims.TokenType)

	// Check expiration (should be 5 minutes)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), claims.ExpiresAt.Time, time.Second)
}

func TestJWTManagerValidateToken(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{
		Issuer: "test-issuer",
		AllowedAudiences: []string{"test-api"},
	})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	tokenPair, err := manager.GenerateTokenPair(customer, "session", []string{"read"}, []string{"viewer"})
	require.NoError(t, err)

	// Test valid token
	claims, err := manager.ValidateToken(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, customer.Username, claims.UserID)

	// Test invalid token
	_, err = manager.ValidateToken("invalid.token.here")
	assert.Error(t, err)

	// Test expired token
	expiredManager, err := NewJWTManager(&JWTConfig{
		AccessTokenExpiry: -time.Hour, // Already expired
	})
	require.NoError(t, err)

	expiredTokenPair, err := expiredManager.GenerateTokenPair(customer, "session", nil, nil)
	require.NoError(t, err)

	_, err = expiredManager.ValidateToken(expiredTokenPair.AccessToken)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestJWTManagerRefreshTokens(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	// Generate initial token pair
	originalTokenPair, err := manager.GenerateTokenPair(customer, "session", []string{"read"}, []string{"viewer"})
	require.NoError(t, err)

	// Refresh tokens
	newPermissions := []string{"read", "write"}
	newRoles := []string{"editor"}
	
	refreshedTokenPair, err := manager.RefreshTokens(originalTokenPair.RefreshToken, newPermissions, newRoles)
	assert.NoError(t, err)
	assert.NotNil(t, refreshedTokenPair)

	// Verify new tokens are different
	assert.NotEqual(t, originalTokenPair.AccessToken, refreshedTokenPair.AccessToken)
	assert.NotEqual(t, originalTokenPair.RefreshToken, refreshedTokenPair.RefreshToken)

	// Verify new permissions and roles
	newClaims, err := manager.ValidateToken(refreshedTokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, newPermissions, newClaims.Permissions)
	assert.Equal(t, newRoles, newClaims.Roles)

	// Test refresh with invalid token
	_, err = manager.RefreshTokens("invalid.refresh.token", nil, nil)
	assert.Error(t, err)

	// Test refresh with access token (should fail)
	_, err = manager.RefreshTokens(originalTokenPair.AccessToken, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a refresh token")
}

func TestJWTManagerUpdateMFAStatus(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	tokenPair, err := manager.GenerateTokenPair(customer, "session", nil, nil)
	require.NoError(t, err)

	// Initially MFA should be false
	claims, err := manager.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	assert.False(t, claims.MFAVerified)

	// Update MFA status to true
	updatedToken, err := manager.UpdateMFAStatus(tokenPair.AccessToken, true)
	assert.NoError(t, err)
	assert.NotEqual(t, tokenPair.AccessToken, updatedToken)

	// Verify MFA status is updated
	updatedClaims, err := manager.ValidateToken(updatedToken)
	assert.NoError(t, err)
	assert.True(t, updatedClaims.MFAVerified)
	assert.NotEqual(t, claims.ID, updatedClaims.ID) // Token ID should be different

	// Test with refresh token (should fail)
	_, err = manager.UpdateMFAStatus(tokenPair.RefreshToken, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an access token")

	// Test with invalid token
	_, err = manager.UpdateMFAStatus("invalid.token", true)
	assert.Error(t, err)
}

func TestJWTManagerExtractClaims(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	tokenPair, err := manager.GenerateTokenPair(customer, "session", []string{"read"}, []string{"viewer"})
	require.NoError(t, err)

	// Extract claims without validation
	claims, err := manager.ExtractClaims(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, customer.Username, claims.UserID)
	assert.Equal(t, customer.ID, claims.CustomerID)
	assert.Equal(t, []string{"read"}, claims.Permissions)

	// Test with invalid token format
	_, err = manager.ExtractClaims("not.a.jwt")
	assert.Error(t, err)
}

func TestJWTManagerValidateCustomClaims(t *testing.T) {
	tests := []struct {
		name    string
		config  *JWTConfig
		claims  *CustomClaims
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid claims",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				UserID:      "user123",
				CustomerID:  uuid.New(),
				SessionID:   "session123",
				TokenType:   TokenTypeAccess,
				MFAVerified: false,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				CustomerID: uuid.New(),
				SessionID:  "session123",
				TokenType:  TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "user_id is required",
		},
		{
			name: "missing customer ID",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				UserID:     "user123",
				CustomerID: uuid.Nil,
				SessionID:  "session123",
				TokenType:  TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "customer_id is required",
		},
		{
			name: "missing session ID",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				UserID:     "user123",
				CustomerID: uuid.New(),
				TokenType:  TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "session_id is required",
		},
		{
			name: "invalid token type",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				UserID:     "user123",
				CustomerID: uuid.New(),
				SessionID:  "session123",
				TokenType:  TokenType("invalid"),
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "invalid token type",
		},
		{
			name: "wrong issuer",
			config: &JWTConfig{
				Issuer:           "correct-issuer",
				AllowedAudiences: []string{"test-api"},
			},
			claims: &CustomClaims{
				UserID:     "user123",
				CustomerID: uuid.New(),
				SessionID:  "session123",
				TokenType:  TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "wrong-issuer",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "invalid issuer",
		},
		{
			name: "wrong audience",
			config: &JWTConfig{
				Issuer:           "test",
				AllowedAudiences: []string{"correct-api"},
			},
			claims: &CustomClaims{
				UserID:     "user123",
				CustomerID: uuid.New(),
				SessionID:  "session123",
				TokenType:  TokenTypeAccess,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"wrong-api"},
				},
			},
			wantErr: true,
			errMsg:  "invalid audience",
		},
		{
			name: "MFA required but not verified",
			config: &JWTConfig{
				Issuer:                  "test",
				AllowedAudiences:        []string{"test-api"},
				RequireMFAForPrivileged: true,
			},
			claims: &CustomClaims{
				UserID:       "user123",
				CustomerID:   uuid.New(),
				SessionID:    "session123",
				TokenType:    TokenTypeAccess,
				IsPrivileged: true,
				MFAVerified:  false,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:   "test",
					Audience: []string{"test-api"},
				},
			},
			wantErr: true,
			errMsg:  "MFA required for privileged operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewJWTManager(tt.config)
			require.NoError(t, err)

			err = manager.validateCustomClaims(tt.claims)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainsPrivilegedRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		expected bool
	}{
		{
			name:     "contains admin role",
			roles:    []string{"user", "admin", "viewer"},
			expected: true,
		},
		{
			name:     "contains operator role",
			roles:    []string{"operator", "viewer"},
			expected: true,
		},
		{
			name:     "contains deployer role",
			roles:    []string{"deployer"},
			expected: true,
		},
		{
			name:     "no privileged roles",
			roles:    []string{"viewer", "user"},
			expected: false,
		},
		{
			name:     "empty roles",
			roles:    []string{},
			expected: false,
		},
		{
			name:     "nil roles",
			roles:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPrivilegedRole(tt.roles)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultJWTConfig(t *testing.T) {
	config := DefaultJWTConfig()

	assert.Equal(t, "platformctl", config.Issuer)
	assert.Equal(t, 15*time.Minute, config.AccessTokenExpiry)
	assert.Equal(t, 24*time.Hour, config.RefreshTokenExpiry)
	assert.Equal(t, 5*time.Minute, config.MFATokenExpiry)
	assert.Equal(t, []string{"platformctl-api", "platformctl-cli"}, config.AllowedAudiences)
	assert.True(t, config.RequireMFAForPrivileged)
	assert.Nil(t, config.PrivateKey) // Should be nil, to be auto-generated
	assert.Nil(t, config.PublicKey)
}

func TestJWTManagerTokenWithMetadata(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	tokenPair, err := manager.GenerateTokenPair(customer, "session", nil, nil)
	require.NoError(t, err)

	claims, err := manager.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)

	// Verify metadata is initialized
	assert.NotNil(t, claims.Metadata)

	// Test adding custom metadata during token creation
	claims.Metadata["custom_field"] = "custom_value"
	
	// This would require extending the API to support custom metadata
	// For now, we verify the structure is correct
	assert.Equal(t, "custom_value", claims.Metadata["custom_field"])
}

func TestJWTManagerSigningMethodValidation(t *testing.T) {
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	// Create a token with wrong signing method (HS256 instead of RS256)
	claims := &CustomClaims{
		UserID:     "user123",
		CustomerID: uuid.New(),
		SessionID:  "session123",
		TokenType:  TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "platformctl",
			Audience:  []string{"platformctl-api", "platformctl-cli"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token with HMAC (should be rejected)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("secret"))
	require.NoError(t, err)

	// Validation should fail due to wrong signing method
	_, err = manager.ValidateToken(tokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected signing method")
}

func TestJWTManagerKeyRotation(t *testing.T) {
	// This test verifies that the key ID is included in the header
	manager, err := NewJWTManager(&JWTConfig{})
	require.NoError(t, err)

	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "testuser",
	}

	tokenPair, err := manager.GenerateTokenPair(customer, "session", nil, nil)
	require.NoError(t, err)

	// Parse token to check header
	token, _, err := new(jwt.Parser).ParseUnverified(tokenPair.AccessToken, &CustomClaims{})
	require.NoError(t, err)

	// Verify key ID is present in header
	kidInterface, ok := token.Header["kid"]
	assert.True(t, ok, "Key ID should be present in token header")
	assert.Equal(t, "1", kidInterface, "Key ID should be '1'")
}

// Benchmark tests
func BenchmarkJWTManagerGenerateTokenPair(b *testing.B) {
	manager, _ := NewJWTManager(&JWTConfig{})
	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "benchuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GenerateTokenPair(customer, "session", []string{"read"}, []string{"user"})
	}
}

func BenchmarkJWTManagerValidateToken(b *testing.B) {
	manager, _ := NewJWTManager(&JWTConfig{})
	customer := &models.Customer{
		ID:       uuid.New(),
		Username: "benchuser",
	}

	tokenPair, _ := manager.GenerateTokenPair(customer, "session", []string{"read"}, []string{"user"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ValidateToken(tokenPair.AccessToken)
	}
}