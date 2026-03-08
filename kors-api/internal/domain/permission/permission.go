package permission

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Permission defines who can do what on which resource or type.
type Permission struct {
	ID             uuid.UUID
	IdentityID     uuid.UUID
	ResourceID     *uuid.UUID
	ResourceTypeID *uuid.UUID
	Action         string // 'read', 'write', 'transition', 'admin'
	ExpiresAt      *time.Time
	CreatedAt      time.Time
}

// IsExpired checks if the permission has reached its expiration date.
func (p *Permission) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*p.ExpiresAt)
}

// Repository defines the contract for permission persistence.
type Repository interface {
	Create(ctx context.Context, p *Permission) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteForIdentity(ctx context.Context, identityID uuid.UUID) error
	FindForIdentity(ctx context.Context, identityID uuid.UUID) ([]*Permission, error)
	Check(ctx context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error)
	List(ctx context.Context, identityID *uuid.UUID, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) ([]*Permission, error) // NOUVEAU
	GetByID(ctx context.Context, id uuid.UUID) (*Permission, error) // NOUVEAU
}
