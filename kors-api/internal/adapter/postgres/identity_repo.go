package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
)

type IdentityRepository struct {
	Pool *pgxpool.Pool
}

func (r *IdentityRepository) Create(ctx context.Context, id *identity.Identity) error {
	query := `
		INSERT INTO kors.identities (id, external_id, name, type, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.Pool.Exec(ctx, query,
		id.ID,
		id.ExternalID,
		id.Name,
		id.Type,
		id.Metadata,
		id.CreatedAt,
		id.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert identity: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetByExternalID(ctx context.Context, externalID string) (*identity.Identity, error) {
	query := `
		SELECT id, external_id, name, type, metadata, created_at, updated_at
		FROM kors.identities
		WHERE external_id = $1
	`
	var id identity.Identity
	err := r.Pool.QueryRow(ctx, query, externalID).Scan(
		&id.ID,
		&id.ExternalID,
		&id.Name,
		&id.Type,
		&id.Metadata,
		&id.CreatedAt,
		&id.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get identity by external id: %w", err)
	}
	return &id, nil
}
