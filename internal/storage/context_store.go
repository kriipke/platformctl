package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/contextops/platformctl/internal/models"
)

type ContextStore struct {
	db *DB
}

func NewContextStore(db *DB) *ContextStore {
	return &ContextStore{db: db}
}

// Create creates a new Context pairing
func (s *ContextStore) Create(ctx context.Context, contextObj *models.Context, customerID string) error {
	specJSON, err := json.Marshal(contextObj.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal context spec: %w", err)
	}

	now := time.Now()
	contextObj.Metadata.CreatedAt = &now
	contextObj.Metadata.UpdatedAt = &now

	// Extract app and environment references from first deployment
	if len(contextObj.Spec.Deployments) == 0 {
		return fmt.Errorf("context must have at least one deployment")
	}

	firstDeployment := contextObj.Spec.Deployments[0]

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO contexts (name, customer_id, app_reference, environment_reference, spec, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		contextObj.Metadata.Name, customerID, firstDeployment.AppRef, firstDeployment.EnvironmentRef,
		specJSON, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}

	// Create associated deployment entries
	for _, deployment := range contextObj.Spec.Deployments {
		if deployment.Active {
			err = s.createContextDeployment(ctx, contextObj.Metadata.Name, customerID, deployment)
			if err != nil {
				return fmt.Errorf("failed to create context deployment: %w", err)
			}
		}
	}

	return nil
}

// Get retrieves a Context by name and customer ID
func (s *ContextStore) Get(ctx context.Context, name, customerID string) (*models.Context, error) {
	var appRef, envRef string
	var specJSON []byte
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT app_reference, environment_reference, spec, created_at, updated_at 
		 FROM contexts WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	).Scan(&appRef, &envRef, &specJSON, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	contextObj := &models.Context{
		APIVersion: "contextops/v1",
		Kind:       "Context",
		Metadata: models.ContextMetadata{
			Name:      name,
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}

	if err := json.Unmarshal(specJSON, &contextObj.Spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context spec: %w", err)
	}

	return contextObj, nil
}

// Update updates an existing Context
func (s *ContextStore) Update(ctx context.Context, contextObj *models.Context, customerID string) error {
	specJSON, err := json.Marshal(contextObj.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal context spec: %w", err)
	}

	now := time.Now()
	contextObj.Metadata.UpdatedAt = &now

	// Extract app and environment references from first deployment
	if len(contextObj.Spec.Deployments) == 0 {
		return fmt.Errorf("context must have at least one deployment")
	}

	firstDeployment := contextObj.Spec.Deployments[0]

	result, err := s.db.ExecContext(ctx,
		`UPDATE contexts SET app_reference = $1, environment_reference = $2, spec = $3, updated_at = $4 
		 WHERE name = $5 AND customer_id = $6`,
		firstDeployment.AppRef, firstDeployment.EnvironmentRef, specJSON, now, contextObj.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	// Update associated deployments (delete and recreate for simplicity)
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM context_deployments WHERE context_name = $1 AND customer_id = $2`,
		contextObj.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old context deployments: %w", err)
	}

	for _, deployment := range contextObj.Spec.Deployments {
		if deployment.Active {
			err = s.createContextDeployment(ctx, contextObj.Metadata.Name, customerID, deployment)
			if err != nil {
				return fmt.Errorf("failed to create context deployment: %w", err)
			}
		}
	}

	return nil
}

// Delete deletes a Context
func (s *ContextStore) Delete(ctx context.Context, name, customerID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM contexts WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// List lists all Contexts for a customer
func (s *ContextStore) List(ctx context.Context, customerID string) ([]*models.Context, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, app_reference, environment_reference, spec, created_at, updated_at 
		 FROM contexts WHERE customer_id = $1 ORDER BY name`,
		customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}
	defer rows.Close()

	var contexts []*models.Context
	for rows.Next() {
		var name, appRef, envRef string
		var specJSON []byte
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&name, &appRef, &envRef, &specJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan context row: %w", err)
		}

		contextObj := &models.Context{
			APIVersion: "contextops/v1",
			Kind:       "Context",
			Metadata: models.ContextMetadata{
				Name:      name,
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
			},
		}

		if err := json.Unmarshal(specJSON, &contextObj.Spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context spec: %w", err)
		}

		contexts = append(contexts, contextObj)
	}

	return contexts, nil
}

// GetByAppAndEnvironment retrieves contexts that reference a specific app and environment
func (s *ContextStore) GetByAppAndEnvironment(ctx context.Context, appRef, envRef, customerID string) ([]*models.Context, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, app_reference, environment_reference, spec, created_at, updated_at 
		 FROM contexts WHERE app_reference = $1 AND environment_reference = $2 AND customer_id = $3 
		 ORDER BY name`,
		appRef, envRef, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get contexts by app and environment: %w", err)
	}
	defer rows.Close()

	var contexts []*models.Context
	for rows.Next() {
		var name, appRefDB, envRefDB string
		var specJSON []byte
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&name, &appRefDB, &envRefDB, &specJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan context row: %w", err)
		}

		contextObj := &models.Context{
			APIVersion: "contextops/v1",
			Kind:       "Context",
			Metadata: models.ContextMetadata{
				Name:      name,
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
			},
		}

		if err := json.Unmarshal(specJSON, &contextObj.Spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context spec: %w", err)
		}

		contexts = append(contexts, contextObj)
	}

	return contexts, nil
}

// createContextDeployment creates a context deployment entry
func (s *ContextStore) createContextDeployment(ctx context.Context, contextName, customerID string, deployment models.ContextDeployment) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO context_deployments (context_name, customer_id, environment, deployment_status) 
		 VALUES ($1, $2, $3, $4)`,
		contextName, customerID, deployment.Environment, "unknown",
	)
	return err
}