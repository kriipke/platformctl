package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kriipke/platformctl/internal/models"
)

type EnvironmentStore struct {
	db *DB
}

func NewEnvironmentStore(db *DB) *EnvironmentStore {
	return &EnvironmentStore{db: db}
}

// Create creates a new Environment manifest
func (s *EnvironmentStore) Create(ctx context.Context, env *models.Environment, customerID string) error {
	specJSON, err := json.Marshal(env.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal environment spec: %w", err)
	}

	now := time.Now()
	env.Metadata.CreatedAt = &now
	env.Metadata.UpdatedAt = &now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO environments (name, customer_id, spec, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5)`,
		env.Metadata.Name, customerID, specJSON, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	// Create associated Vault source
	err = s.createVaultSource(ctx, env.Metadata.Name, customerID, env.Spec.Vault)
	if err != nil {
		return fmt.Errorf("failed to create vault source: %w", err)
	}

	// Create associated Vault static secrets
	for _, vaultSecret := range env.Spec.VaultSecrets {
		err = s.createVaultStaticSecret(ctx, env.Metadata.Name, customerID, vaultSecret)
		if err != nil {
			return fmt.Errorf("failed to create vault static secret: %w", err)
		}
	}

	// Create associated cluster configuration
	err = s.createClusterConfig(ctx, env.Metadata.Name, customerID, env.Spec.Environment)
	if err != nil {
		return fmt.Errorf("failed to create cluster config: %w", err)
	}

	return nil
}

// Get retrieves an Environment manifest by name and customer ID
func (s *EnvironmentStore) Get(ctx context.Context, name, customerID string) (*models.Environment, error) {
	var specJSON []byte
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT spec, created_at, updated_at FROM environments WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	).Scan(&specJSON, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	env := &models.Environment{
		APIVersion: "contextops/v1",
		Kind:       "Environment",
		Metadata: models.EnvironmentMetadata{
			Name:      name,
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}

	if err := json.Unmarshal(specJSON, &env.Spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal environment spec: %w", err)
	}

	return env, nil
}

// Update updates an existing Environment manifest
func (s *EnvironmentStore) Update(ctx context.Context, env *models.Environment, customerID string) error {
	specJSON, err := json.Marshal(env.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal environment spec: %w", err)
	}

	now := time.Now()
	env.Metadata.UpdatedAt = &now

	result, err := s.db.ExecContext(ctx,
		`UPDATE environments SET spec = $1, updated_at = $2 WHERE name = $3 AND customer_id = $4`,
		specJSON, now, env.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update environment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	// Update associated resources (delete and recreate for simplicity)
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM vault_sources WHERE environment_name = $1 AND customer_id = $2`,
		env.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old vault sources: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`DELETE FROM vault_static_secrets WHERE environment_name = $1 AND customer_id = $2`,
		env.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old vault static secrets: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`DELETE FROM cluster_configs WHERE environment_name = $1 AND customer_id = $2`,
		env.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old cluster configs: %w", err)
	}

	// Recreate associated resources
	err = s.createVaultSource(ctx, env.Metadata.Name, customerID, env.Spec.Vault)
	if err != nil {
		return fmt.Errorf("failed to create vault source: %w", err)
	}

	for _, vaultSecret := range env.Spec.VaultSecrets {
		err = s.createVaultStaticSecret(ctx, env.Metadata.Name, customerID, vaultSecret)
		if err != nil {
			return fmt.Errorf("failed to create vault static secret: %w", err)
		}
	}

	err = s.createClusterConfig(ctx, env.Metadata.Name, customerID, env.Spec.Environment)
	if err != nil {
		return fmt.Errorf("failed to create cluster config: %w", err)
	}

	return nil
}

// Delete deletes an Environment manifest
func (s *EnvironmentStore) Delete(ctx context.Context, name, customerID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM environments WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
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

// List lists all Environment manifests for a customer
func (s *EnvironmentStore) List(ctx context.Context, customerID string) ([]*models.Environment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, spec, created_at, updated_at FROM environments WHERE customer_id = $1 ORDER BY name`,
		customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	var envs []*models.Environment
	for rows.Next() {
		var name string
		var specJSON []byte
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&name, &specJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan environment row: %w", err)
		}

		env := &models.Environment{
			APIVersion: "contextops/v1",
			Kind:       "Environment",
			Metadata: models.EnvironmentMetadata{
				Name:      name,
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
			},
		}

		if err := json.Unmarshal(specJSON, &env.Spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal environment spec: %w", err)
		}

		envs = append(envs, env)
	}

	return envs, nil
}

// createVaultSource creates a Vault source entry
func (s *EnvironmentStore) createVaultSource(ctx context.Context, envName, customerID string, vault models.EnvironmentVaultConfig) error {
	authConfigJSON, err := json.Marshal(vault.Auth)
	if err != nil {
		return fmt.Errorf("failed to marshal auth config: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO vault_sources (environment_name, customer_id, vault_address, vault_namespace, auth_method, auth_config) 
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		envName, customerID, vault.Address, vault.Namespace, vault.Auth.Method, authConfigJSON,
	)
	return err
}

// createVaultStaticSecret creates a Vault static secret entry
func (s *EnvironmentStore) createVaultStaticSecret(ctx context.Context, envName, customerID string, secret models.VaultStaticSecret) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vault_static_secrets (environment_name, customer_id, name, vault_path, destination_secret, required_keys) 
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		envName, customerID, secret.Name, secret.VaultPath, secret.DestinationSecret, secret.RequiredKeys,
	)
	return err
}

// createClusterConfig creates a cluster configuration entry
func (s *EnvironmentStore) createClusterConfig(ctx context.Context, envName, customerID string, envConfig models.EnvironmentConfig) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cluster_configs (environment_name, customer_id, cluster_name, namespace, kubeconfig_vault_path, kubeconfig_vault_key) 
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		envName, customerID, envConfig.Name, envConfig.Namespace,
		envConfig.Cluster.KubeconfigSecretRef.Vault, envConfig.Cluster.KubeconfigSecretRef.Key,
	)
	return err
}