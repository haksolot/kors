package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
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

func (r *ResourceRepository) List(ctx context.Context, first int, after *uuid.UUID, typeName *string) ([]*resource.Resource, bool, int, error) {
	// 1. Get Total Count
	var totalCount int
	countQuery := "SELECT count(*) FROM kors.resources r"
	if typeName != nil {
		countQuery += " JOIN kors.resource_types rt ON r.type_id = rt.id WHERE rt.name = $1"
		err := r.Pool.QueryRow(ctx, countQuery, *typeName).Scan(&totalCount)
		if err != nil {
			return nil, false, 0, err
		}
	} else {
		err := r.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
		if err != nil {
			return nil, false, 0, err
		}
	}

	// 2. Fetch Data
	// We fetch first + 1 to know if there is a next page
	query := `
		SELECT r.id, r.type_id, r.state, r.metadata, r.created_at, r.updated_at, r.deleted_at
		FROM kors.resources r
		LEFT JOIN kors.resource_types rt ON r.type_id = rt.id
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if typeName != nil {
		query += fmt.Sprintf(" AND rt.name = $%d", argIdx)
		args = append(args, *typeName)
		argIdx++
	}

	if after != nil {
		// Seek pagination logic (using created_at or ID for stable sorting)
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
