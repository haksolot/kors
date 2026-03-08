package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/event"
)

type EventRepository struct {
	Pool *pgxpool.Pool
}

func (r *EventRepository) Create(ctx context.Context, e *event.Event) error {
	return r.createWithDB(ctx, r.Pool, e)
}

func (r *EventRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, e *event.Event) error {
	return r.createWithDB(ctx, tx, e)
}

func (r *EventRepository) createWithDB(ctx context.Context, db DBTX, e *event.Event) error {
	query := `
		INSERT INTO kors.events (id, resource_id, identity_id, type, payload, nats_message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := db.Exec(ctx, query,
		e.ID,
		e.ResourceID,
		e.IdentityID,
		e.Type,
		e.Payload,
		e.NatsMessageID,
		e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}
	return nil
}
