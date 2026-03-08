package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
)

type GrantPermissionInput struct {
	IdentityID     uuid.UUID
	ResourceID     *uuid.UUID
	ResourceTypeID *uuid.UUID
	Action         string
	ExpiresAt      *time.Time
}

type GrantPermissionUseCase struct {
	Repo permission.Repository
}

func (uc *GrantPermissionUseCase) Execute(ctx context.Context, input GrantPermissionInput) (*permission.Permission, error) {
	p := &permission.Permission{
		ID:             uuid.New(),
		IdentityID:     input.IdentityID,
		ResourceID:     input.ResourceID,
		ResourceTypeID: input.ResourceTypeID,
		Action:         input.Action,
		ExpiresAt:      input.ExpiresAt,
		CreatedAt:      time.Now(),
	}

	if err := uc.Repo.Create(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}
