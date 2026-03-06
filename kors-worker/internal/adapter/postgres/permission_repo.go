package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PermissionRepository struct {
	Pool *pgxpool.Pool
}

func (r *PermissionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	query := "DELETE FROM kors.permissions WHERE expires_at IS NOT NULL AND expires_at < NOW()"
	cmd, err := r.Pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired permissions: %w", err)
	}
	return cmd.RowsAffected(), nil
}
