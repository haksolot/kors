package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLocked = fmt.Errorf("resource is locked by another worker")

type PermissionRepository struct {
	Pool *pgxpool.Pool
}

func (r *PermissionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	// 1. Démarrer une transaction
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// 2. Tenter de prendre un verrou consultatif unique (ID arbitraire 12345)
	// pg_try_advisory_xact_lock renvoie true si le verrou est acquis.
	var acquired bool
	err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(12345)").Scan(&acquired)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	if !acquired {
		// Quelqu'un d'autre travaille déjà, on passe notre tour
		return 0, ErrLocked
	}

	// 3. Exécuter le nettoyage
	query := "DELETE FROM kors.permissions WHERE expires_at IS NOT NULL AND expires_at < NOW()"
	cmd, err := tx.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired permissions: %w", err)
	}

	// 4. Valider (relâche automatiquement le verrou)
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return cmd.RowsAffected(), nil
}
