package event

import (
	"context"

	"github.com/google/uuid"
)

type Event struct {
	ID            uuid.UUID
	NatsMessageID *uuid.UUID
	// Autres champs au besoin...
}

type Repository interface {
	IsProcessed(ctx context.Context, natsMessageID uuid.UUID) (bool, error)
}
