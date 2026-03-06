package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
)

type ResourceRepository struct {
	Pool *pgxpool.Pool
}

func (r *ResourceRepository) Create(ctx context.Context, res *resource.Resource) error {
	query := `
		INSERT INTO kors.resources (id, type_id, state, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.Pool.Exec(ctx, query,
		res.ID,
		res.TypeID,
		res.State,
		res.Metadata,
		res.CreatedAt,
		res.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert resource: %w", err)
	}
	return nil
}

func (r *ResourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*resource.Resource, error) {
	query := `
		SELECT id, type_id, state, metadata, created_at, updated_at, deleted_at
		FROM kors.resources
		WHERE id = $1
	`
	var res resource.Resource
	err := r.Pool.QueryRow(ctx, query, id).Scan(
		&res.ID,
		&res.TypeID,
		&res.State,
		&res.Metadata,
		&res.CreatedAt,
		&res.UpdatedAt,
		&res.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get resource by id: %w", err)
	}
	return &res, nil
}

func (r *ResourceRepository) Update(ctx context.Context, res *resource.Resource) error {
	query := `
		UPDATE kors.resources
		SET state = $1, metadata = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.Pool.Exec(ctx, query,
		res.State,
		res.Metadata,
		res.UpdatedAt,
		res.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}
	return nil
}
