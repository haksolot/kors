package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	Pool *pgxpool.Pool
}

func (r *EventRepository) IsProcessed(ctx context.Context, natsMessageID uuid.UUID) (bool, error) {
	var count int
	query := "SELECT count(*) FROM kors.events WHERE id = $1"
	err := r.Pool.QueryRow(ctx, query, natsMessageID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
