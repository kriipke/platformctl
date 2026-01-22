package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"platformctl/internal/models"
)

var ErrNotFound = errors.New("record not found")

// GitOpsContextStore handles persistence for GitOps contexts.
type GitOpsContextStore struct {
	db *sqlx.DB
}

type contextRecord struct {
	Name      string          `db:"name"`
	Customer  string          `db:"customer_id"`
	Spec      json.RawMessage `db:"spec"`
	CreatedAt sql.NullTime    `db:"created_at"`
	UpdatedAt sql.NullTime    `db:"updated_at"`
}

func NewPostgresStore(db *sqlx.DB) *GitOpsContextStore {
	return &GitOpsContextStore{db: db}
}

func OpenPostgres(ctx context.Context, dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func (s *GitOpsContextStore) CreateContext(ctx context.Context, customerID string, contextModel *models.Context) error {
	payload, err := json.Marshal(contextModel)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO contexts (name, customer_id, spec) VALUES ($1, $2, $3)`,
		contextModel.Metadata.Name,
		customerID,
		payload,
	)
	return err
}

func (s *GitOpsContextStore) GetContext(ctx context.Context, customerID, name string) (*models.Context, error) {
	var record contextRecord
	if err := s.db.GetContext(
		ctx,
		&record,
		`SELECT name, customer_id, spec, created_at, updated_at FROM contexts WHERE name = $1 AND customer_id = $2`,
		name,
		customerID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var contextModel models.Context
	if err := json.Unmarshal(record.Spec, &contextModel); err != nil {
		return nil, err
	}
	return &contextModel, nil
}

func (s *GitOpsContextStore) ListContexts(ctx context.Context, customerID string) ([]models.Context, error) {
	var records []contextRecord
	if err := s.db.SelectContext(
		ctx,
		&records,
		`SELECT name, customer_id, spec, created_at, updated_at FROM contexts WHERE customer_id = $1 ORDER BY name`,
		customerID,
	); err != nil {
		return nil, err
	}

	contexts := make([]models.Context, 0, len(records))
	for _, record := range records {
		var contextModel models.Context
		if err := json.Unmarshal(record.Spec, &contextModel); err != nil {
			return nil, err
		}
		contexts = append(contexts, contextModel)
	}
	return contexts, nil
}

func (s *GitOpsContextStore) UpdateContext(ctx context.Context, customerID string, contextModel *models.Context) error {
	payload, err := json.Marshal(contextModel)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE contexts SET spec = $1 WHERE name = $2 AND customer_id = $3`,
		payload,
		contextModel.Metadata.Name,
		customerID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *GitOpsContextStore) DeleteContext(ctx context.Context, customerID, name string) error {
	result, err := s.db.ExecContext(
		ctx,
		`DELETE FROM contexts WHERE name = $1 AND customer_id = $2`,
		name,
		customerID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
