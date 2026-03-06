package resource

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Resource represents a KORS envelope for a business entity.
type Resource struct {
	ID        uuid.UUID
	TypeID    uuid.UUID
	State     string
	Metadata  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Repository defines the contract for persisting and retrieving Resources.
type Repository interface {
	Create(ctx context.Context, res *Resource) error
	GetByID(ctx context.Context, id uuid.UUID) (*Resource, error)
	Update(ctx context.Context, res *Resource) error
}
