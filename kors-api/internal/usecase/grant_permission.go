package usecase

import (
	"context"
	"fmt"
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

var validActions = map[string]bool{
    "read": true, "write": true, "transition": true, "admin": true,
}

func (uc *GrantPermissionUseCase) Execute(ctx context.Context, input GrantPermissionInput) (*permission.Permission, error) {
	if !validActions[input.Action] {
        return nil, fmt.Errorf("invalid action %q: must be one of read, write, transition, admin", input.Action)
    }
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
