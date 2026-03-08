package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
)

type ResourceRepository struct {
	Pool *pgxpool.Pool
}

// DBTX is an interface that can be a *pgxpool.Pool or a pgx.Tx
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func (r *ResourceRepository) Create(ctx context.Context, res *resource.Resource) error {
	return r.createWithDB(ctx, r.Pool, res)
}

func (r *ResourceRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, res *resource.Resource) error {
	return r.createWithDB(ctx, tx, res)
}

func (r *ResourceRepository) createWithDB(ctx context.Context, db DBTX, res *resource.Resource) error {
	query := `
		INSERT INTO kors.resources (id, type_id, state, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := db.Exec(ctx, query,
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
		WHERE id = $1 AND deleted_at IS NULL
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
	return r.updateWithDB(ctx, r.Pool, res)
}

func (r *ResourceRepository) UpdateWithTx(ctx context.Context, tx pgx.Tx, res *resource.Resource) error {
	return r.updateWithDB(ctx, tx, res)
}

func (r *ResourceRepository) updateWithDB(ctx context.Context, db DBTX, res *resource.Resource) error {
	query := `
		UPDATE kors.resources
		SET state = $1, metadata = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`
	_, err := db.Exec(ctx, query,
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

func (r *ResourceRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
    _, err := r.Pool.Exec(ctx,
        "UPDATE kors.resources SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL",
        id,
    )
    return err
}

func (r *ResourceRepository) List(ctx context.Context, first int, after *uuid.UUID, filter resource.ListFilter) ([]*resource.Resource, bool, int, error) {
	// 1. Get Total Count
	var totalCount int
	countQuery := "SELECT count(*) FROM kors.resources r"
	if filter.TypeName != nil {
		countQuery += " JOIN kors.resource_types rt ON r.type_id = rt.id"
	}
	countQuery += " WHERE r.deleted_at IS NULL"
	
	argsCount := []interface{}{}
	argIdx := 1

	if filter.TypeName != nil {
		countQuery += fmt.Sprintf(" AND rt.name = $%d", argIdx)
		argsCount = append(argsCount, *filter.TypeName)
		argIdx++
	}
	if filter.State != nil {
		countQuery += fmt.Sprintf(" AND r.state = $%d", argIdx)
		argsCount = append(argsCount, *filter.State)
		argIdx++
	}
	if filter.CreatedAfter != nil {
		countQuery += fmt.Sprintf(" AND r.created_at >= $%d", argIdx)
		argsCount = append(argsCount, *filter.CreatedAfter)
		argIdx++
	}
	if filter.CreatedBefore != nil {
		countQuery += fmt.Sprintf(" AND r.created_at <= $%d", argIdx)
		argsCount = append(argsCount, *filter.CreatedBefore)
		argIdx++
	}

	err := r.Pool.QueryRow(ctx, countQuery, argsCount...).Scan(&totalCount)
	if err != nil {
		return nil, false, 0, err
	}

	// 2. Fetch Data
	query := `
		SELECT r.id, r.type_id, r.state, r.metadata, r.created_at, r.updated_at, r.deleted_at
		FROM kors.resources r
		LEFT JOIN kors.resource_types rt ON r.type_id = rt.id
		WHERE r.deleted_at IS NULL
	`
	args := []interface{}{}
	argIdx = 1

	if filter.TypeName != nil {
		query += fmt.Sprintf(" AND rt.name = $%d", argIdx)
		args = append(args, *filter.TypeName)
		argIdx++
	}
	if filter.State != nil {
		query += fmt.Sprintf(" AND r.state = $%d", argIdx)
		args = append(args, *filter.State)
		argIdx++
	}
	if filter.CreatedAfter != nil {
		query += fmt.Sprintf(" AND r.created_at >= $%d", argIdx)
		args = append(args, *filter.CreatedAfter)
		argIdx++
	}
	if filter.CreatedBefore != nil {
		query += fmt.Sprintf(" AND r.created_at <= $%d", argIdx)
		args = append(args, *filter.CreatedBefore)
		argIdx++
	}

	if after != nil {
		query += fmt.Sprintf(" AND r.created_at < (SELECT created_at FROM kors.resources WHERE id = $%d)", argIdx)
		args = append(args, *after)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY r.created_at DESC LIMIT $%d", argIdx)
	args = append(args, first+1)

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	var results []*resource.Resource
	for rows.Next() {
		var res resource.Resource
		err := rows.Scan(&res.ID, &res.TypeID, &res.State, &res.Metadata, &res.CreatedAt, &res.UpdatedAt, &res.DeletedAt)
		if err != nil {
			return nil, false, 0, err
		}
		results = append(results, &res)
	}

	hasNextPage := len(results) > first
	if hasNextPage {
		results = results[:first]
	}

	return results, hasNextPage, totalCount, nil
}
