package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/revision"
)

type RevisionRepository struct {
	Pool *pgxpool.Pool
}

func (r *RevisionRepository) Create(ctx context.Context, rev *revision.Revision) error {
	query := `
		INSERT INTO kors.revisions (id, resource_id, identity_id, snapshot, file_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.Pool.Exec(ctx, query,
		rev.ID,
		rev.ResourceID,
		rev.IdentityID,
		rev.Snapshot,
		rev.FilePath,
		rev.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert revision: %w", err)
	}
	return nil
}

func (r *RevisionRepository) GetByID(ctx context.Context, id uuid.UUID) (*revision.Revision, error) {
	query := `
		SELECT id, resource_id, identity_id, snapshot, file_path, created_at
		FROM kors.revisions
		WHERE id = $1
	`
	var rev revision.Revision
	err := r.Pool.QueryRow(ctx, query, id).Scan(
		&rev.ID,
		&rev.ResourceID,
		&rev.IdentityID,
		&rev.Snapshot,
		&rev.FilePath,
		&rev.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}
	return &rev, nil
}

func (r *RevisionRepository) ListByResource(ctx context.Context, resourceID uuid.UUID) ([]*revision.Revision, error) {
	query := `
		SELECT id, resource_id, identity_id, snapshot, file_path, created_at
		FROM kors.revisions
		WHERE resource_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.Pool.Query(ctx, query, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*revision.Revision
	for rows.Next() {
		var rev revision.Revision
		err := rows.Scan(&rev.ID, &rev.ResourceID, &rev.IdentityID, &rev.Snapshot, &rev.FilePath, &rev.CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, &rev)
	}
	return results, nil
}

func (r *RevisionRepository) ListByResourcePaginated(ctx context.Context, resourceID uuid.UUID, first int, after *uuid.UUID) ([]*revision.Revision, bool, int, error) {
	// 1. Total count
	var totalCount int
	err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM kors.revisions WHERE resource_id = $1", resourceID).Scan(&totalCount)
	if err != nil {
		return nil, false, 0, err
	}

	// 2. Query
	query := `
		SELECT id, resource_id, identity_id, snapshot, file_path, created_at
		FROM kors.revisions
		WHERE resource_id = $1
	`
	args := []interface{}{resourceID}
	argIdx := 2

	if after != nil {
		query += fmt.Sprintf(" AND created_at < (SELECT created_at FROM kors.revisions WHERE id = $%d)", argIdx)
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

	var results []*revision.Revision
	for rows.Next() {
		var rev revision.Revision
		err := rows.Scan(&rev.ID, &rev.ResourceID, &rev.IdentityID, &rev.Snapshot, &rev.FilePath, &rev.CreatedAt)
		if err != nil {
			return nil, false, 0, err
		}
		results = append(results, &rev)
	}

	hasNextPage := len(results) > first
	if hasNextPage {
		results = results[:first]
	}

	return results, hasNextPage, totalCount, nil
}
