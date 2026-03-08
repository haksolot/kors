package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Event represents an immutable record of an action in the system.
type Event struct {
	ID            uuid.UUID
	ResourceID    *uuid.UUID // Optional, can be null for global events
	IdentityID    uuid.UUID
	Type          string // e.g., 'resource.created', 'resource.state_changed'
	Payload       map[string]interface{}
	NatsMessageID *uuid.UUID // For idempotency
	CreatedAt     time.Time
}

type ListFilter struct {
	ResourceID *uuid.UUID
	IdentityID *uuid.UUID
	Type       *string
}

// Repository defines the contract for persisting and retrieving events.
type Repository interface {
	Create(ctx context.Context, e *Event) error
	CreateWithTx(ctx context.Context, tx pgx.Tx, e *Event) error
	List(ctx context.Context, filter ListFilter, first int, after *uuid.UUID) ([]*Event, bool, int, error) // NOUVEAU
	GetByID(ctx context.Context, id uuid.UUID) (*Event, error)                                            // NOUVEAU
}

// Publisher defines the contract for broadcasting events to the bus.
type Publisher interface {
	Publish(ctx context.Context, e *Event) error
}
