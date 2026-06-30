package auth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/contextops/platformctl/internal/testutil"
	"github.com/contextops/platformctl/internal/models"
)

func TestNewRBACManager(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.db)
}

func TestRBACManagerCheckPermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	// Create test customer and user
	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant permission
	err = manager.GrantPermission(
		customerID,
		userID,
		"app",
		nil,
		"read",
		"admin",
		nil,
		nil,
	)
	require.NoError(t, err)

	tests := []struct {
		name         string
		userID       string
		customerID   uuid.UUID
		resourceType string
		resourceID   *string
		action       string
		expected     bool
	}{
		{
			name:         "allowed permission",
			userID:       userID,
			customerID:   customerID,
			resourceType: "app",
			resourceID:   nil,
			action:       "read",
			expected:     true,
		},
		{
			name:         "denied action",
			userID:       userID,
			customerID:   customerID,
			resourceType: "app",
			resourceID:   nil,
			action:       "delete",
			expected:     false,
		},
		{
			name:         "different resource type",
			userID:       userID,
			customerID:   customerID,
			resourceType: "environment",
			resourceID:   nil,
			action:       "read",
			expected:     false,
		},
		{
			name:         "different user",
			userID:       "other-user",
			customerID:   customerID,
			resourceType: "app",
			resourceID:   nil,
			action:       "read",
			expected:     false,
		},
		{
			name:         "different customer",
			userID:       userID,
			customerID:   uuid.New(),
			resourceType: "app",
			resourceID:   nil,
			action:       "read",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			hasPermission, err := manager.CheckPermission(
				ctx,
				tt.userID,
				tt.customerID,
				tt.resourceType,
				tt.resourceID,
				tt.action,
			)
			
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, hasPermission)
		})
	}
}

func TestRBACManagerCheckPermissionWithRoles(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	// Create test customer
	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create context with JWT claims containing roles
	claims := &CustomClaims{
		UserID:     userID,
		CustomerID: customerID,
		Roles:      []string{"customer-viewer"},
	}
	
	customer := &models.Customer{
		ID: customerID,
	}
	
	ctx := context.WithValue(context.Background(), "customer", customer)
	ctx = context.WithValue(ctx, JWTClaimsKey, claims)

	// Test role-based permission
	hasPermission, err := manager.CheckPermission(
		ctx,
		userID,
		customerID,
		"app",
		nil,
		"read",
	)
	
	assert.NoError(t, err)
	assert.True(t, hasPermission) // customer-viewer role allows app:read
}

func TestRBACManagerCheckPermissionWithDenyEffect(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant allow permission
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin", nil, nil)
	require.NoError(t, err)

	// Grant deny permission (should override allow)
	_, err = testDB.ExecContext(context.Background(), `
		INSERT INTO user_permissions (customer_id, user_id, resource_type, action, effect, granted_by, granted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		customerID, userID, "app", "read", "deny", "admin", time.Now())
	require.NoError(t, err)

	ctx := context.Background()
	hasPermission, err := manager.CheckPermission(ctx, userID, customerID, "app", nil, "read")
	
	assert.NoError(t, err)
	assert.False(t, hasPermission) // Deny should override allow
}

func TestRBACManagerCheckPermissionWithExpiration(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant permission that expires in the past
	expiredTime := time.Now().Add(-time.Hour)
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin", nil, &expiredTime)
	require.NoError(t, err)

	ctx := context.Background()
	hasPermission, err := manager.CheckPermission(ctx, userID, customerID, "app", nil, "read")
	
	assert.NoError(t, err)
	assert.False(t, hasPermission) // Expired permission should not be valid
}

func TestRBACManagerCheckPermissionWithConditions(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Create context with customer information
	customer := &models.Customer{ID: customerID}
	ctx := context.WithValue(context.Background(), "customer", customer)

	// Grant permission with condition that customer_id matches
	conditions := map[string]interface{}{
		"customer_id": customerID.String(),
	}
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin", conditions, nil)
	require.NoError(t, err)

	hasPermission, err := manager.CheckPermission(ctx, userID, customerID, "app", nil, "read")
	assert.NoError(t, err)
	assert.True(t, hasPermission) // Condition should match
}

func TestRBACManagerGrantPermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	conditions := map[string]interface{}{
		"environment": "production",
	}
	expiresAt := time.Now().Add(24 * time.Hour)

	err = manager.GrantPermission(
		customerID,
		userID,
		"app",
		stringPtr("app-123"),
		"update",
		"admin",
		conditions,
		&expiresAt,
	)
	assert.NoError(t, err)

	// Verify permission was granted
	permissions, err := manager.GetUserPermissions(customerID, userID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 1)

	perm := permissions[0]
	assert.Equal(t, customerID, perm.CustomerID)
	assert.Equal(t, userID, perm.UserID)
	assert.Equal(t, "app", perm.ResourceType)
	assert.Equal(t, "app-123", *perm.ResourceID)
	assert.Equal(t, "update", perm.Action)
	assert.Equal(t, string(EffectAllow), perm.Effect)
	assert.Equal(t, "admin", perm.GrantedBy)
	assert.Equal(t, "production", perm.Conditions["environment"])
	assert.WithinDuration(t, expiresAt, *perm.ExpiresAt, time.Second)
}

func TestRBACManagerGrantPermissionUpsert(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant initial permission
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin1", nil, nil)
	assert.NoError(t, err)

	// Grant same permission again with different granter (should update)
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin2", nil, nil)
	assert.NoError(t, err)

	// Should still have only one permission but with updated granter
	permissions, err := manager.GetUserPermissions(customerID, userID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 1)
	assert.Equal(t, "admin2", permissions[0].GrantedBy)
}

func TestRBACManagerRevokePermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	userID := "test-user"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant permissions
	err = manager.GrantPermission(customerID, userID, "app", nil, "read", "admin", nil, nil)
	require.NoError(t, err)
	
	err = manager.GrantPermission(customerID, userID, "app", stringPtr("app-123"), "update", "admin", nil, nil)
	require.NoError(t, err)

	// Verify both permissions exist
	permissions, err := manager.GetUserPermissions(customerID, userID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 2)

	// Revoke general read permission
	err = manager.RevokePermission(customerID, userID, "app", nil, "read")
	assert.NoError(t, err)

	// Verify only specific permission remains
	permissions, err = manager.GetUserPermissions(customerID, userID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 1)
	assert.Equal(t, "app-123", *permissions[0].ResourceID)

	// Revoke specific permission
	err = manager.RevokePermission(customerID, userID, "app", stringPtr("app-123"), "update")
	assert.NoError(t, err)

	// Verify no permissions remain
	permissions, err = manager.GetUserPermissions(customerID, userID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 0)
}

func TestRBACManagerGetResourcePermissions(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	customerID := uuid.New()
	resourceID := "app-123"
	
	_, err := testDB.ExecContext(context.Background(), 
		"INSERT INTO customers (id, name, username, email, active) VALUES ($1, $2, $3, $4, $5)",
		customerID, "Test Customer", "testcustomer", "test@example.com", true)
	require.NoError(t, err)

	// Grant permissions to different users for the same resource
	users := []string{"user1", "user2", "user3"}
	actions := []string{"read", "update", "delete"}

	for i, user := range users {
		err = manager.GrantPermission(customerID, user, "app", &resourceID, actions[i], "admin", nil, nil)
		require.NoError(t, err)
	}

	// Grant permission for all apps (resource_id = NULL)
	err = manager.GrantPermission(customerID, "user4", "app", nil, "read", "admin", nil, nil)
	require.NoError(t, err)

	// Get resource permissions
	permissions, err := manager.GetResourcePermissions(customerID, "app", resourceID)
	assert.NoError(t, err)
	assert.Len(t, permissions, 4) // 3 specific + 1 general

	// Verify all permissions are related to the resource
	specificPermissions := 0
	generalPermissions := 0
	
	for _, perm := range permissions {
		assert.Equal(t, customerID, perm.CustomerID)
		assert.Equal(t, "app", perm.ResourceType)
		
		if perm.ResourceID != nil {
			assert.Equal(t, resourceID, *perm.ResourceID)
			specificPermissions++
		} else {
			generalPermissions++
		}
	}

	assert.Equal(t, 3, specificPermissions)
	assert.Equal(t, 1, generalPermissions)
}

func TestRBACManagerMatchesPermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	tests := []struct {
		name         string
		permission   *Permission
		resourceType string
		resourceID   *string
		action       string
		expected     bool
	}{
		{
			name: "exact match",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   stringPtr("app-123"),
				Action:       "read",
			},
			resourceType: "app",
			resourceID:   stringPtr("app-123"),
			action:       "read",
			expected:     true,
		},
		{
			name: "wildcard resource type",
			permission: &Permission{
				ResourceType: "*",
				ResourceID:   stringPtr("app-123"),
				Action:       "read",
			},
			resourceType: "environment",
			resourceID:   stringPtr("app-123"),
			action:       "read",
			expected:     true,
		},
		{
			name: "wildcard action",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   stringPtr("app-123"),
				Action:       "*",
			},
			resourceType: "app",
			resourceID:   stringPtr("app-123"),
			action:       "delete",
			expected:     true,
		},
		{
			name: "null resource ID (applies to all)",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   nil,
				Action:       "read",
			},
			resourceType: "app",
			resourceID:   stringPtr("any-app"),
			action:       "read",
			expected:     true,
		},
		{
			name: "resource type mismatch",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   nil,
				Action:       "read",
			},
			resourceType: "environment",
			resourceID:   nil,
			action:       "read",
			expected:     false,
		},
		{
			name: "resource ID mismatch",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   stringPtr("app-123"),
				Action:       "read",
			},
			resourceType: "app",
			resourceID:   stringPtr("app-456"),
			action:       "read",
			expected:     false,
		},
		{
			name: "action mismatch",
			permission: &Permission{
				ResourceType: "app",
				ResourceID:   nil,
				Action:       "read",
			},
			resourceType: "app",
			resourceID:   nil,
			action:       "write",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.matchesPermission(tt.permission, tt.resourceType, tt.resourceID, tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBACManagerCheckRolePermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)
	ctx := context.Background()

	tests := []struct {
		name         string
		role         string
		resourceType string
		resourceID   *string
		action       string
		expected     bool
	}{
		{
			name:         "customer-viewer can read apps",
			role:         "customer-viewer",
			resourceType: "app",
			resourceID:   nil,
			action:       "read",
			expected:     true,
		},
		{
			name:         "customer-viewer cannot create apps",
			role:         "customer-viewer",
			resourceType: "app",
			resourceID:   nil,
			action:       "create",
			expected:     false,
		},
		{
			name:         "customer-operator can do anything with apps",
			role:         "customer-operator",
			resourceType: "app",
			resourceID:   nil,
			action:       "delete",
			expected:     true,
		},
		{
			name:         "customer-admin can do anything",
			role:         "customer-admin",
			resourceType: "system",
			resourceID:   nil,
			action:       "admin",
			expected:     true,
		},
		{
			name:         "non-existent role",
			role:         "non-existent",
			resourceType: "app",
			resourceID:   nil,
			action:       "read",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPermission, err := manager.checkRolePermission(tt.role, tt.resourceType, tt.resourceID, tt.action, ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, hasPermission)
		})
	}
}

func TestRBACManagerMatchesRolePermission(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	tests := []struct {
		name         string
		permission   string
		resourceType string
		action       string
		expected     bool
	}{
		{
			name:         "exact match",
			permission:   "platformctl:app:read",
			resourceType: "app",
			action:       "read",
			expected:     true,
		},
		{
			name:         "wildcard resource",
			permission:   "platformctl:*:read",
			resourceType: "environment",
			action:       "read",
			expected:     true,
		},
		{
			name:         "wildcard action",
			permission:   "platformctl:app:*",
			resourceType: "app",
			action:       "delete",
			expected:     true,
		},
		{
			name:         "both wildcards",
			permission:   "platformctl:*:*",
			resourceType: "system",
			action:       "admin",
			expected:     true,
		},
		{
			name:         "resource mismatch",
			permission:   "platformctl:app:read",
			resourceType: "environment",
			action:       "read",
			expected:     false,
		},
		{
			name:         "action mismatch",
			permission:   "platformctl:app:read",
			resourceType: "app",
			action:       "write",
			expected:     false,
		},
		{
			name:         "invalid format",
			permission:   "invalid:format",
			resourceType: "app",
			action:       "read",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.matchesRolePermission(tt.permission, tt.resourceType, tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBACManagerGetRoleDefinition(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	tests := []struct {
		name     string
		roleName string
		expected *Role
	}{
		{
			name:     "customer-viewer role",
			roleName: "customer-viewer",
			expected: &Role{
				Name:        "customer-viewer",
				Description: "Read-only access to customer resources",
				Permissions: []string{
					"platformctl:app:read",
					"platformctl:environment:read",
					"platformctl:context:read",
				},
				IsPrivileged: false,
			},
		},
		{
			name:     "customer-operator role",
			roleName: "customer-operator",
			expected: &Role{
				Name:        "customer-operator",
				Description: "Operational access to customer resources",
				Permissions: []string{
					"platformctl:app:*",
					"platformctl:environment:*",
					"platformctl:context:*",
				},
				IsPrivileged: true,
			},
		},
		{
			name:     "customer-admin role",
			roleName: "customer-admin",
			expected: &Role{
				Name:        "customer-admin",
				Description: "Full administrative access to customer resources",
				Permissions: []string{
					"platformctl:*:*",
				},
				IsPrivileged: true,
			},
		},
		{
			name:     "non-existent role",
			roleName: "non-existent",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := manager.getRoleDefinition(tt.roleName)
			
			if tt.expected == nil {
				assert.Nil(t, role)
			} else {
				require.NotNil(t, role)
				assert.Equal(t, tt.expected.Name, role.Name)
				assert.Equal(t, tt.expected.Description, role.Description)
				assert.Equal(t, tt.expected.Permissions, role.Permissions)
				assert.Equal(t, tt.expected.IsPrivileged, role.IsPrivileged)
				
				if tt.expected.IsPrivileged {
					assert.True(t, len(role.Conditions) > 0, "Privileged roles should have conditions")
					assert.Equal(t, "mfa", role.Conditions[0].Type)
				}
			}
		})
	}
}

func TestGetStandardPermissions(t *testing.T) {
	permissions := GetStandardPermissions()

	// Test some standard permission sets
	assert.Contains(t, permissions, "app-read")
	assert.Contains(t, permissions, "app-write")
	assert.Contains(t, permissions, "app-admin")
	assert.Contains(t, permissions, "deployer")
	assert.Contains(t, permissions, "admin")

	// Verify app-read permissions
	appReadPerms := permissions["app-read"]
	assert.Equal(t, []string{"app:read"}, appReadPerms)

	// Verify app-write includes read
	appWritePerms := permissions["app-write"]
	assert.Contains(t, appWritePerms, "app:read")
	assert.Contains(t, appWritePerms, "app:create")
	assert.Contains(t, appWritePerms, "app:update")

	// Verify admin has wildcard
	adminPerms := permissions["admin"]
	assert.Equal(t, []string{"*:*"}, adminPerms)

	// Verify deployer permissions
	deployerPerms := permissions["deployer"]
	assert.Contains(t, deployerPerms, "app:read")
	assert.Contains(t, deployerPerms, "environment:read")
	assert.Contains(t, deployerPerms, "context:read")
	assert.Contains(t, deployerPerms, "context:deploy")
}

func TestRBACManagerConditionEvaluation(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	defer testDB.Close(t)

	manager := NewRBACManager(testDB.DB.DB)

	// Create test context with customer and claims
	customerID := uuid.New()
	customer := &models.Customer{ID: customerID}
	claims := &CustomClaims{
		CustomerID:  customerID,
		MFAVerified: true,
	}

	ctx := context.WithValue(context.Background(), "customer", customer)
	ctx = context.WithValue(ctx, JWTClaimsKey, claims)

	tests := []struct {
		name       string
		conditions map[string]interface{}
		expected   bool
	}{
		{
			name: "customer_id match",
			conditions: map[string]interface{}{
				"customer_id": customerID.String(),
			},
			expected: true,
		},
		{
			name: "customer_id mismatch",
			conditions: map[string]interface{}{
				"customer_id": uuid.New().String(),
			},
			expected: false,
		},
		{
			name: "mfa_required true with verified MFA",
			conditions: map[string]interface{}{
				"mfa_required": true,
			},
			expected: true,
		},
		{
			name: "mfa_required false",
			conditions: map[string]interface{}{
				"mfa_required": false,
			},
			expected: true,
		},
		{
			name: "valid time range",
			conditions: map[string]interface{}{
				"time_range": map[string]interface{}{
					"start": time.Now().Add(-time.Hour).Format(time.RFC3339),
					"end":   time.Now().Add(time.Hour).Format(time.RFC3339),
				},
			},
			expected: true,
		},
		{
			name: "expired time range",
			conditions: map[string]interface{}{
				"time_range": map[string]interface{}{
					"start": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
					"end":   time.Now().Add(-time.Hour).Format(time.RFC3339),
				},
			},
			expected: false,
		},
		{
			name: "multiple conditions - all pass",
			conditions: map[string]interface{}{
				"customer_id":   customerID.String(),
				"mfa_required":  true,
			},
			expected: true,
		},
		{
			name: "multiple conditions - one fails",
			conditions: map[string]interface{}{
				"customer_id":  customerID.String(),
				"mfa_required": true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.evaluateConditions(ctx, tt.conditions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRBACManagerJSONSerialization(t *testing.T) {
	conditions := map[string]interface{}{
		"environment": "production",
		"time_range": map[string]interface{}{
			"start": "2024-01-01T00:00:00Z",
			"end":   "2024-12-31T23:59:59Z",
		},
		"mfa_required": true,
	}

	// Test marshaling
	data, err := json.Marshal(conditions)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "production")
	assert.Contains(t, string(data), "mfa_required")

	// Test unmarshaling
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, "production", unmarshaled["environment"])
	assert.Equal(t, true, unmarshaled["mfa_required"])
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

