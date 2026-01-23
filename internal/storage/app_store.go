package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/contextops/platformctl/internal/models"
)

type AppStore struct {
	db *DB
}

func NewAppStore(db *DB) *AppStore {
	return &AppStore{db: db}
}

// Create creates a new App manifest
func (s *AppStore) Create(ctx context.Context, app *models.App, customerID string) error {
	specJSON, err := json.Marshal(app.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal app spec: %w", err)
	}

	now := time.Now()
	app.Metadata.CreatedAt = &now
	app.Metadata.UpdatedAt = &now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO apps (name, customer_id, spec, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5)`,
		app.Metadata.Name, customerID, specJSON, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// Create associated Helm sources
	for i, helmSource := range app.Spec.Helm.Sources {
		isDefault := i == app.Spec.Helm.DefaultSource
		err = s.createHelmSource(ctx, app.Metadata.Name, customerID, helmSource, isDefault)
		if err != nil {
			return fmt.Errorf("failed to create helm source: %w", err)
		}
	}

	// Create associated ApplicationSets
	for _, appSet := range app.Spec.ArgoCD.ApplicationSets {
		err = s.createApplicationSet(ctx, app.Metadata.Name, customerID, appSet)
		if err != nil {
			return fmt.Errorf("failed to create applicationset: %w", err)
		}
	}

	return nil
}

// Get retrieves an App manifest by name and customer ID
func (s *AppStore) Get(ctx context.Context, name, customerID string) (*models.App, error) {
	var specJSON []byte
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT spec, created_at, updated_at FROM apps WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	).Scan(&specJSON, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	app := &models.App{
		APIVersion: "contextops/v1",
		Kind:       "App",
		Metadata: models.AppMetadata{
			Name:      name,
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}

	if err := json.Unmarshal(specJSON, &app.Spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app spec: %w", err)
	}

	return app, nil
}

// Update updates an existing App manifest
func (s *AppStore) Update(ctx context.Context, app *models.App, customerID string) error {
	specJSON, err := json.Marshal(app.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal app spec: %w", err)
	}

	now := time.Now()
	app.Metadata.UpdatedAt = &now

	result, err := s.db.ExecContext(ctx,
		`UPDATE apps SET spec = $1, updated_at = $2 WHERE name = $3 AND customer_id = $4`,
		specJSON, now, app.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	// Update associated Helm sources (delete and recreate for simplicity)
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM helm_sources WHERE app_name = $1 AND customer_id = $2`,
		app.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old helm sources: %w", err)
	}

	for i, helmSource := range app.Spec.Helm.Sources {
		isDefault := i == app.Spec.Helm.DefaultSource
		err = s.createHelmSource(ctx, app.Metadata.Name, customerID, helmSource, isDefault)
		if err != nil {
			return fmt.Errorf("failed to create helm source: %w", err)
		}
	}

	// Update associated ApplicationSets (delete and recreate)
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM applicationsets WHERE app_name = $1 AND customer_id = $2`,
		app.Metadata.Name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old applicationsets: %w", err)
	}

	for _, appSet := range app.Spec.ArgoCD.ApplicationSets {
		err = s.createApplicationSet(ctx, app.Metadata.Name, customerID, appSet)
		if err != nil {
			return fmt.Errorf("failed to create applicationset: %w", err)
		}
	}

	return nil
}

// Delete deletes an App manifest
func (s *AppStore) Delete(ctx context.Context, name, customerID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM apps WHERE name = $1 AND customer_id = $2`,
		name, customerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
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

// List lists all App manifests for a customer
func (s *AppStore) List(ctx context.Context, customerID string) ([]*models.App, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, spec, created_at, updated_at FROM apps WHERE customer_id = $1 ORDER BY name`,
		customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	defer rows.Close()

	var apps []*models.App
	for rows.Next() {
		var name string
		var specJSON []byte
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&name, &specJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan app row: %w", err)
		}

		app := &models.App{
			APIVersion: "contextops/v1",
			Kind:       "App",
			Metadata: models.AppMetadata{
				Name:      name,
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
			},
		}

		if err := json.Unmarshal(specJSON, &app.Spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal app spec: %w", err)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// createHelmSource creates a Helm source entry
func (s *AppStore) createHelmSource(ctx context.Context, appName, customerID string, source models.HelmSource, isDefault bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO helm_sources (app_name, customer_id, source_type, registry, chart, version, repository, path, ref, is_default) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		appName, customerID, source.Type, source.Registry, source.Chart,
		source.Version, source.Repository, source.Path, source.Ref, isDefault,
	)
	return err
}

// createApplicationSet creates an ApplicationSet entry
func (s *AppStore) createApplicationSet(ctx context.Context, appName, customerID string, appSet models.ApplicationSetConfig) error {
	generatorConfigJSON, err := json.Marshal(map[string]interface{}{
		"type": appSet.Generator.Type,
		"git":  appSet.Generator.Git,
		"list": appSet.Generator.List,
		"clusters": appSet.Generator.Clusters,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal generator config: %w", err)
	}

	templateConfigJSON, err := json.Marshal(appSet.Template)
	if err != nil {
		return fmt.Errorf("failed to marshal template config: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO applicationsets (app_name, customer_id, name, namespace, generator_type, generator_config, template_config) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		appName, customerID, appSet.Name, appSet.Namespace, appSet.Generator.Type, generatorConfigJSON, templateConfigJSON,
	)
	return err
}