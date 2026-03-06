package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

type ResourceTypeRepository struct {
	Pool *pgxpool.Pool
}

func (r *ResourceTypeRepository) Create(ctx context.Context, rt *resourcetype.ResourceType) error {
	query := `
		INSERT INTO kors.resource_types (id, name, description, json_schema, transitions, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.Pool.Exec(ctx, query,
		rt.ID,
		rt.Name,
		rt.Description,
		rt.JSONSchema,
		rt.Transitions,
		rt.CreatedAt,
		rt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert resource type: %w", err)
	}
	return nil
}

func (r *ResourceTypeRepository) GetByID(ctx context.Context, id uuid.UUID) (*resourcetype.ResourceType, error) {
	query := `
		SELECT id, name, description, json_schema, transitions, created_at, updated_at
		FROM kors.resource_types
		WHERE id = $1
	`
	var rt resourcetype.ResourceType
	err := r.Pool.QueryRow(ctx, query, id).Scan(
		&rt.ID,
		&rt.Name,
		&rt.Description,
		&rt.JSONSchema,
		&rt.Transitions,
		&rt.CreatedAt,
		&rt.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get resource type by id: %w", err)
	}
	return &rt, nil
}

func (r *ResourceTypeRepository) GetByName(ctx context.Context, name string) (*resourcetype.ResourceType, error) {
	query := `
		SELECT id, name, description, json_schema, transitions, created_at, updated_at
		FROM kors.resource_types
		WHERE name = $1
	`
	var rt resourcetype.ResourceType
	err := r.Pool.QueryRow(ctx, query, name).Scan(
		&rt.ID,
		&rt.Name,
		&rt.Description,
		&rt.JSONSchema,
		&rt.Transitions,
		&rt.CreatedAt,
		&rt.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil if not found
		}
		return nil, fmt.Errorf("failed to get resource type by name: %w", err)
	}
	return &rt, nil
}

func (r *ResourceTypeRepository) List(ctx context.Context) ([]*resourcetype.ResourceType, error) {
	query := `
		SELECT id, name, description, json_schema, transitions, created_at, updated_at
		FROM kors.resource_types
		ORDER BY name ASC
	`
	rows, err := r.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list resource types: %w", err)
	}
	defer rows.Close()

	var results []*resourcetype.ResourceType
	for rows.Next() {
		var rt resourcetype.ResourceType
		err := rows.Scan(
			&rt.ID,
			&rt.Name,
			&rt.Description,
			&rt.JSONSchema,
			&rt.Transitions,
			&rt.CreatedAt,
			&rt.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan resource type row: %w", err)
		}
		results = append(results, &rt)
	}
	return results, nil
}
