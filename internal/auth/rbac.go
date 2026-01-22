package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Permission represents a single permission
type Permission struct {
	ID           int64                  `json:"id" db:"id"`
	CustomerID   uuid.UUID              `json:"customer_id" db:"customer_id"`
	UserID       string                 `json:"user_id" db:"user_id"`
	ResourceType string                 `json:"resource_type" db:"resource_type"`
	ResourceID   *string                `json:"resource_id,omitempty" db:"resource_id"`
	Action       string                 `json:"action" db:"action"`
	Effect       string                 `json:"effect" db:"effect"`
	Conditions   map[string]interface{} `json:"conditions,omitempty" db:"conditions"`
	GrantedBy    string                 `json:"granted_by" db:"granted_by"`
	GrantedAt    time.Time              `json:"granted_at" db:"granted_at"`
	ExpiresAt    *time.Time             `json:"expires_at,omitempty" db:"expires_at"`
	Reason       *string                `json:"reason,omitempty" db:"reason"`
	IsInherited  bool                   `json:"is_inherited" db:"is_inherited"`
}

// Role represents a role with associated permissions
type Role struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Permissions  []string     `json:"permissions"`
	IsPrivileged bool         `json:"is_privileged"`
	Conditions   []Condition  `json:"conditions,omitempty"`
}

// Condition represents an access condition
type Condition struct {
	Type     string      `json:"type"`
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// ActionType represents different types of actions
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionRead   ActionType = "read"
	ActionUpdate ActionType = "update"
	ActionDelete ActionType = "delete"
	ActionDeploy ActionType = "deploy"
	ActionAdmin  ActionType = "admin"
	ActionAll    ActionType = "*"
)

// ResourceType represents different types of resources
type ResourceTypeEnum string

const (
	ResourceApp         ResourceTypeEnum = "app"
	ResourceEnvironment ResourceTypeEnum = "environment"
	ResourceContext     ResourceTypeEnum = "context"
	ResourceUser        ResourceTypeEnum = "user"
	ResourceSystem      ResourceTypeEnum = "system"
	ResourceAll         ResourceTypeEnum = "*"
)

// Effect represents the effect of a permission
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// RBACManager manages role-based access control
type RBACManager struct {
	db *sql.DB
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager(db *sql.DB) *RBACManager {
	return &RBACManager{db: db}
}

// CheckPermission checks if a user has permission to perform an action on a resource
func (r *RBACManager) CheckPermission(ctx context.Context, userID string, customerID uuid.UUID, resourceType string, resourceID *string, action string) (bool, error) {
	permissions, err := r.GetUserPermissions(customerID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Check explicit permissions first
	for _, perm := range permissions {
		if r.matchesPermission(perm, resourceType, resourceID, action) {
			// Check if permission is expired
			if perm.ExpiresAt != nil && perm.ExpiresAt.Before(time.Now()) {
				continue
			}

			// Check conditions if present
			if len(perm.Conditions) > 0 && !r.evaluateConditions(ctx, perm.Conditions) {
				continue
			}

			// Deny takes precedence over allow
			if perm.Effect == string(EffectDeny) {
				return false, nil
			}

			if perm.Effect == string(EffectAllow) {
				return true, nil
			}
		}
	}

	// Check role-based permissions
	claims, _ := ctx.Value(JWTClaimsKey).(*CustomClaims)
	if claims != nil {
		for _, role := range claims.Roles {
			hasPermission, err := r.checkRolePermission(role, resourceType, resourceID, action, ctx)
			if err != nil {
				return false, err
			}
			if hasPermission {
				return true, nil
			}
		}
	}

	return false, nil
}

// GrantPermission grants a permission to a user
func (r *RBACManager) GrantPermission(customerID uuid.UUID, userID, resourceType string, resourceID *string, action, grantedBy string, conditions map[string]interface{}, expiresAt *time.Time) error {
	conditionsJSON, _ := json.Marshal(conditions)

	query := `
		INSERT INTO user_permissions (customer_id, user_id, resource_type, resource_id, action, effect, conditions, granted_by, granted_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (customer_id, user_id, resource_type, resource_id, action) 
		DO UPDATE SET 
			effect = EXCLUDED.effect,
			conditions = EXCLUDED.conditions,
			granted_by = EXCLUDED.granted_by,
			granted_at = EXCLUDED.granted_at,
			expires_at = EXCLUDED.expires_at`

	_, err := r.db.Exec(query,
		customerID,
		userID,
		resourceType,
		resourceID,
		action,
		string(EffectAllow),
		conditionsJSON,
		grantedBy,
		time.Now(),
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}

	return nil
}

// RevokePermission revokes a permission from a user
func (r *RBACManager) RevokePermission(customerID uuid.UUID, userID, resourceType string, resourceID *string, action string) error {
	query := `DELETE FROM user_permissions WHERE customer_id = $1 AND user_id = $2 AND resource_type = $3 AND action = $4`
	args := []interface{}{customerID, userID, resourceType, action}

	if resourceID != nil {
		query += " AND resource_id = $5"
		args = append(args, *resourceID)
	} else {
		query += " AND resource_id IS NULL"
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}

	return nil
}

// GetUserPermissions retrieves all permissions for a user
func (r *RBACManager) GetUserPermissions(customerID uuid.UUID, userID string) ([]*Permission, error) {
	query := `
		SELECT id, customer_id, user_id, resource_type, resource_id, action, effect, 
			   conditions, granted_by, granted_at, expires_at, reason, is_inherited
		FROM user_permissions 
		WHERE customer_id = $1 AND user_id = $2`

	rows, err := r.db.Query(query, customerID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*Permission
	for rows.Next() {
		perm := &Permission{}
		var conditionsJSON []byte

		err := rows.Scan(
			&perm.ID,
			&perm.CustomerID,
			&perm.UserID,
			&perm.ResourceType,
			&perm.ResourceID,
			&perm.Action,
			&perm.Effect,
			&conditionsJSON,
			&perm.GrantedBy,
			&perm.GrantedAt,
			&perm.ExpiresAt,
			&perm.Reason,
			&perm.IsInherited,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}

		if len(conditionsJSON) > 0 {
			if err := json.Unmarshal(conditionsJSON, &perm.Conditions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
			}
		}

		permissions = append(permissions, perm)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}

	return permissions, nil
}

// GetResourcePermissions retrieves all permissions for a specific resource
func (r *RBACManager) GetResourcePermissions(customerID uuid.UUID, resourceType, resourceID string) ([]*Permission, error) {
	query := `
		SELECT id, customer_id, user_id, resource_type, resource_id, action, effect, 
			   conditions, granted_by, granted_at, expires_at, reason, is_inherited
		FROM user_permissions 
		WHERE customer_id = $1 AND resource_type = $2 AND (resource_id = $3 OR resource_id IS NULL)`

	rows, err := r.db.Query(query, customerID, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*Permission
	for rows.Next() {
		perm := &Permission{}
		var conditionsJSON []byte

		err := rows.Scan(
			&perm.ID,
			&perm.CustomerID,
			&perm.UserID,
			&perm.ResourceType,
			&perm.ResourceID,
			&perm.Action,
			&perm.Effect,
			&conditionsJSON,
			&perm.GrantedBy,
			&perm.GrantedAt,
			&perm.ExpiresAt,
			&perm.Reason,
			&perm.IsInherited,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}

		if len(conditionsJSON) > 0 {
			if err := json.Unmarshal(conditionsJSON, &perm.Conditions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
			}
		}

		permissions = append(permissions, perm)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}

	return permissions, nil
}

// Helper methods

// matchesPermission checks if a permission matches the requested resource and action
func (r *RBACManager) matchesPermission(perm *Permission, resourceType string, resourceID *string, action string) bool {
	// Check resource type
	if perm.ResourceType != resourceType && perm.ResourceType != "*" {
		return false
	}

	// Check resource ID
	if perm.ResourceID != nil {
		if resourceID == nil || *perm.ResourceID != *resourceID {
			return false
		}
	}

	// Check action
	if perm.Action != action && perm.Action != "*" {
		return false
	}

	return true
}

// evaluateConditions evaluates permission conditions
func (r *RBACManager) evaluateConditions(ctx context.Context, conditions map[string]interface{}) bool {
	// For now, implement basic condition evaluation
	// In a full implementation, this would support complex condition logic

	// Get current context information
	customer, claims, err := RequireAuth(ctx)
	if err != nil {
		return false
	}

	// Example condition checks
	for key, value := range conditions {
		switch key {
		case "time_range":
			if !r.checkTimeRange(value) {
				return false
			}
		case "environment":
			if !r.checkEnvironmentCondition(value, ctx) {
				return false
			}
		case "customer_id":
			if customer.ID.String() != value {
				return false
			}
		case "mfa_required":
			if required, ok := value.(bool); ok && required && claims != nil && !claims.MFAVerified {
				return false
			}
		}
	}

	return true
}

// checkTimeRange checks if current time is within the specified range
func (r *RBACManager) checkTimeRange(condition interface{}) bool {
	timeRange, ok := condition.(map[string]interface{})
	if !ok {
		return false
	}

	now := time.Now()

	if start, ok := timeRange["start"].(string); ok {
		if startTime, err := time.Parse(time.RFC3339, start); err == nil && now.Before(startTime) {
			return false
		}
	}

	if end, ok := timeRange["end"].(string); ok {
		if endTime, err := time.Parse(time.RFC3339, end); err == nil && now.After(endTime) {
			return false
		}
	}

	return true
}

// checkEnvironmentCondition checks environment-specific conditions
func (r *RBACManager) checkEnvironmentCondition(condition interface{}, ctx context.Context) bool {
	// Extract environment information from context or request
	// This is a simplified implementation
	return true
}

// checkRolePermission checks if a role has the required permission
func (r *RBACManager) checkRolePermission(role, resourceType string, resourceID *string, action string, ctx context.Context) (bool, error) {
	roleDefinition := r.getRoleDefinition(role)
	if roleDefinition == nil {
		return false, nil
	}

	// Check if role has the required permission
	for _, permission := range roleDefinition.Permissions {
		if r.matchesRolePermission(permission, resourceType, action) {
			// Evaluate role conditions
			if len(roleDefinition.Conditions) > 0 && !r.evaluateRoleConditions(roleDefinition.Conditions, ctx) {
				continue
			}
			return true, nil
		}
	}

	return false, nil
}

// matchesRolePermission checks if a role permission matches the requested action
func (r *RBACManager) matchesRolePermission(permission, resourceType, action string) bool {
	parts := strings.Split(permission, ":")
	if len(parts) != 3 {
		return false
	}

	permResourceType, permAction := parts[1], parts[2]

	// Check resource type match
	if permResourceType != resourceType && permResourceType != "*" {
		return false
	}

	// Check action match
	if permAction != action && permAction != "*" {
		return false
	}

	return true
}

// evaluateRoleConditions evaluates role-specific conditions
func (r *RBACManager) evaluateRoleConditions(conditions []Condition, ctx context.Context) bool {
	for _, condition := range conditions {
		if !r.evaluateCondition(condition, ctx) {
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition
func (r *RBACManager) evaluateCondition(condition Condition, ctx context.Context) bool {
	// Implement condition evaluation logic based on type
	switch condition.Type {
	case "time":
		return r.evaluateTimeCondition(condition)
	case "environment":
		return r.evaluateEnvironmentCondition(condition, ctx)
	case "mfa":
		return r.evaluateMFACondition(condition, ctx)
	}
	return true
}

// evaluateTimeCondition evaluates time-based conditions
func (r *RBACManager) evaluateTimeCondition(condition Condition) bool {
	// Implement time condition logic
	return true
}

// evaluateEnvironmentCondition evaluates environment-based conditions
func (r *RBACManager) evaluateEnvironmentCondition(condition Condition, ctx context.Context) bool {
	// Implement environment condition logic
	return true
}

// evaluateMFACondition evaluates MFA-based conditions
func (r *RBACManager) evaluateMFACondition(condition Condition, ctx context.Context) bool {
	claims, err := RequireClaims(ctx)
	if err != nil {
		return false
	}

	if required, ok := condition.Value.(bool); ok {
		return !required || claims.MFAVerified
	}

	return true
}

// getRoleDefinition returns the definition for a role
func (r *RBACManager) getRoleDefinition(roleName string) *Role {
	// Return predefined roles
	// In a full implementation, these could be stored in database
	roles := map[string]*Role{
		"customer-viewer": {
			Name:        "customer-viewer",
			Description: "Read-only access to customer resources",
			Permissions: []string{
				"platformctl:app:read",
				"platformctl:environment:read",
				"platformctl:context:read",
			},
			IsPrivileged: false,
		},
		"customer-operator": {
			Name:        "customer-operator",
			Description: "Operational access to customer resources",
			Permissions: []string{
				"platformctl:app:*",
				"platformctl:environment:*",
				"platformctl:context:*",
			},
			IsPrivileged: true,
			Conditions: []Condition{
				{
					Type:     "mfa",
					Field:    "required",
					Operator: "eq",
					Value:    true,
				},
			},
		},
		"customer-admin": {
			Name:        "customer-admin",
			Description: "Full administrative access to customer resources",
			Permissions: []string{
				"platformctl:*:*",
			},
			IsPrivileged: true,
			Conditions: []Condition{
				{
					Type:     "mfa",
					Field:    "required",
					Operator: "eq",
					Value:    true,
				},
			},
		},
	}

	return roles[roleName]
}

// Standard permission sets

// GetStandardPermissions returns commonly used permission sets
func GetStandardPermissions() map[string][]string {
	return map[string][]string{
		"app-read": {
			"app:read",
		},
		"app-write": {
			"app:read",
			"app:create",
			"app:update",
		},
		"app-admin": {
			"app:*",
		},
		"environment-read": {
			"environment:read",
		},
		"environment-write": {
			"environment:read",
			"environment:create",
			"environment:update",
		},
		"environment-admin": {
			"environment:*",
		},
		"context-read": {
			"context:read",
		},
		"context-write": {
			"context:read",
			"context:create",
			"context:update",
		},
		"context-admin": {
			"context:*",
		},
		"deployer": {
			"app:read",
			"environment:read",
			"context:read",
			"context:deploy",
		},
		"admin": {
			"*:*",
		},
	}
}