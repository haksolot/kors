package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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

func (r *IdentityRepository) GetByID(ctx context.Context, internalID uuid.UUID) (*identity.Identity, error) {
	query := `
		SELECT id, external_id, name, type, metadata, created_at, updated_at
		FROM kors.identities
		WHERE id = $1
	`
	var id identity.Identity
	err := r.Pool.QueryRow(ctx, query, internalID).Scan(
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
		return nil, fmt.Errorf("failed to get identity by id: %w", err)
	}
	return &id, nil
}

func (r *IdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM kors.identities WHERE id = $1", id)
	return err
}

func (r *IdentityRepository) List(ctx context.Context, identityType *string, first int, after *uuid.UUID) ([]*identity.Identity, bool, int, error) {
	// 1. Get Total Count
	var totalCount int
	countQuery := "SELECT count(*) FROM kors.identities WHERE 1=1"
	argsCount := []interface{}{}
	argIdx := 1

	if identityType != nil {
		countQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		argsCount = append(argsCount, *identityType)
		argIdx++
	}

	err := r.Pool.QueryRow(ctx, countQuery, argsCount...).Scan(&totalCount)
	if err != nil {
		return nil, false, 0, err
	}

	// 2. Fetch Data
	query := `
		SELECT id, external_id, name, type, metadata, created_at, updated_at
		FROM kors.identities
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx = 1

	if identityType != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *identityType)
		argIdx++
	}

	if after != nil {
		query += fmt.Sprintf(" AND created_at < (SELECT created_at FROM kors.identities WHERE id = $%d)", argIdx)
		args = append(args, *after)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, first+1)

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var results []*identity.Identity
	for rows.Next() {
		var id identity.Identity
		err := rows.Scan(&id.ID, &id.ExternalID, &id.Name, &id.Type, &id.Metadata, &id.CreatedAt, &id.UpdatedAt)
		if err != nil {
			return nil, false, 0, err
		}
		results = append(results, &id)
	}

	hasNextPage := len(results) > first
	if hasNextPage {
		results = results[:first]
	}

	return results, hasNextPage, totalCount, nil
}
