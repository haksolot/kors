package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kors-project/kors/kors-api/internal/domain/permission"
)

type PermissionRepository struct {
	Pool *pgxpool.Pool
}

func (r *PermissionRepository) Create(ctx context.Context, p *permission.Permission) error {
	query := `
		INSERT INTO kors.permissions (id, identity_id, resource_id, resource_type_id, action, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.Pool.Exec(ctx, query,
		p.ID,
		p.IdentityID,
		p.ResourceID,
		p.ResourceTypeID,
		p.Action,
		p.ExpiresAt,
		p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert permission: %w", err)
	}
	return nil
}

func (r *PermissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM kors.permissions WHERE id = $1", id)
	return err
}

func (r *PermissionRepository) FindForIdentity(ctx context.Context, identityID uuid.UUID) ([]*permission.Permission, error) {
	query := `
		SELECT id, identity_id, resource_id, resource_type_id, action, expires_at, created_at
		FROM kors.permissions
		WHERE identity_id = $1 AND (expires_at IS NULL OR expires_at > NOW())
	`
	rows, err := r.Pool.Query(ctx, query, identityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*permission.Permission
	for rows.Next() {
		var p permission.Permission
		err := rows.Scan(&p.ID, &p.IdentityID, &p.ResourceID, &p.ResourceTypeID, &p.Action, &p.ExpiresAt, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, &p)
	}
	return results, nil
}

func (r *PermissionRepository) Check(ctx context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error) {
	// 1. Check for explicit resource permission
	// 2. Check for resource type permission (inherits to all resources of that type)
	// 3. Check for global permission (null resource AND null type)
	query := `
		SELECT EXISTS (
			SELECT 1 FROM kors.permissions
			WHERE identity_id = $1
			AND action = $2
			AND (expires_at IS NULL OR expires_at > NOW())
			AND (
				(resource_id = $3) OR
				(resource_type_id = $4 AND resource_id IS NULL) OR
				(resource_id IS NULL AND resource_type_id IS NULL)
			)
		)
	`
	var exists bool
	err := r.Pool.QueryRow(ctx, query, identityID, action, resourceID, resourceTypeID).Scan(&exists)
	if err != nil && err != pgx.ErrNoRows {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}
	return exists, nil
}
