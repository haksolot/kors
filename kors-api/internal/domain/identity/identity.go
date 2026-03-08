package identity

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Identity struct {
	ID         uuid.UUID
	ExternalID string
	Name       string
	Type       string // 'user', 'service', 'system'
	Metadata   map[string]interface{}
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Repository interface {
	Create(ctx context.Context, id *Identity) error
	GetByID(ctx context.Context, id uuid.UUID) (*Identity, error)
	GetByExternalID(ctx context.Context, externalID string) (*Identity, error)
}
