package resource

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type ListFilter struct {
	TypeName      *string
	State         *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// Repository defines the contract for persisting and retrieving Resources.
type Repository interface {
	Create(ctx context.Context, res *Resource) error
	CreateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error // NOUVEAU
	GetByID(ctx context.Context, id uuid.UUID) (*Resource, error)
	Update(ctx context.Context, res *Resource) error
	UpdateWithTx(ctx context.Context, tx pgx.Tx, res *Resource) error // NOUVEAU
	List(ctx context.Context, first int, after *uuid.UUID, filter ListFilter) ([]*Resource, bool, int, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
