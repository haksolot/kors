package event

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// Repository defines the contract for persisting and retrieving events.
type Repository interface {
	Create(ctx context.Context, e *Event) error
}

// Publisher defines the contract for broadcasting events to the bus.
type Publisher interface {
	Publish(ctx context.Context, e *Event) error
}
