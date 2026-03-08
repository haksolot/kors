package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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

func (r *EventRepository) GetByID(ctx context.Context, id uuid.UUID) (*event.Event, error) {
	query := `
		SELECT id, resource_id, identity_id, type, payload, nats_message_id, created_at
		FROM kors.events
		WHERE id = $1
	`
	var e event.Event
	err := r.Pool.QueryRow(ctx, query, id).Scan(
		&e.ID,
		&e.ResourceID,
		&e.IdentityID,
		&e.Type,
		&e.Payload,
		&e.NatsMessageID,
		&e.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event by id: %w", err)
	}
	return &e, nil
}

func (r *EventRepository) List(ctx context.Context, filter event.ListFilter, first int, after *uuid.UUID) ([]*event.Event, bool, int, error) {
	// 1. Get Total Count
	var totalCount int
	countQuery := "SELECT count(*) FROM kors.events WHERE 1=1"
	argsCount := []interface{}{}
	argIdx := 1

	if filter.ResourceID != nil {
		countQuery += fmt.Sprintf(" AND resource_id = $%d", argIdx)
		argsCount = append(argsCount, *filter.ResourceID)
		argIdx++
	}
	if filter.IdentityID != nil {
		countQuery += fmt.Sprintf(" AND identity_id = $%d", argIdx)
		argsCount = append(argsCount, *filter.IdentityID)
		argIdx++
	}
	if filter.Type != nil {
		countQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		argsCount = append(argsCount, *filter.Type)
		argIdx++
	}

	err := r.Pool.QueryRow(ctx, countQuery, argsCount...).Scan(&totalCount)
	if err != nil {
		return nil, false, 0, err
	}

	// 2. Fetch Data
	query := `
		SELECT id, resource_id, identity_id, type, payload, nats_message_id, created_at
		FROM kors.events
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx = 1

	if filter.ResourceID != nil {
		query += fmt.Sprintf(" AND resource_id = $%d", argIdx)
		args = append(args, *filter.ResourceID)
		argIdx++
	}
	if filter.IdentityID != nil {
		query += fmt.Sprintf(" AND identity_id = $%d", argIdx)
		args = append(args, *filter.IdentityID)
		argIdx++
	}
	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *filter.Type)
		argIdx++
	}

	if after != nil {
		query += fmt.Sprintf(" AND created_at < (SELECT created_at FROM kors.events WHERE id = $%d)", argIdx)
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

	var results []*event.Event
	for rows.Next() {
		var e event.Event
		err := rows.Scan(&e.ID, &e.ResourceID, &e.IdentityID, &e.Type, &e.Payload, &e.NatsMessageID, &e.CreatedAt)
		if err != nil {
			return nil, false, 0, err
		}
		results = append(results, &e)
	}

	hasNextPage := len(results) > first
	if hasNextPage {
		results = results[:first]
	}

	return results, hasNextPage, totalCount, nil
}
