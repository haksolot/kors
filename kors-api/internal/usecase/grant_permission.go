package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
)

type GrantPermissionInput struct {
	CallerID       uuid.UUID // NOUVEAU
	IdentityID     uuid.UUID
	ResourceID     *uuid.UUID
	ResourceTypeID *uuid.UUID
	Action         string
	ExpiresAt      *time.Time
}

type GrantPermissionUseCase struct {
	Repo           permission.Repository
	PermissionRepo permission.Repository // pour checker les droits du caller
}

var validActions = map[string]bool{
	"read": true, "write": true, "transition": true, "admin": true,
}

func (uc *GrantPermissionUseCase) Execute(ctx context.Context, input GrantPermissionInput) (*permission.Permission, error) {
	if !validActions[input.Action] {
		return nil, fmt.Errorf("invalid action %q: must be one of read, write, transition, admin", input.Action)
	}

	if input.Action == "admin" {
		// Seul un admin global peut accorder admin
		allowed, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check caller permission: %w", err)
		}
		if !allowed {
			return nil, fmt.Errorf("only global admins can grant admin permissions")
		}
	} else {
		// write ou admin suffisent pour deleguer les autres permissions
		isAdmin, _ := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
		hasWrite, _ := uc.PermissionRepo.Check(ctx, input.CallerID, "write", nil, nil)
		if !isAdmin && !hasWrite {
			return nil, fmt.Errorf("write or admin permission required to grant permissions")
		}
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
