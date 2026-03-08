package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
)

type ListPermissionsInput struct {
    CallerID       uuid.UUID
    IdentityID     *uuid.UUID
    ResourceID     *uuid.UUID
    ResourceTypeID *uuid.UUID
}

type ListPermissionsUseCase struct {
    Repo permission.Repository
}

func (uc *ListPermissionsUseCase) Execute(ctx context.Context, input ListPermissionsInput) ([]*permission.Permission, error) {
    // Permission check: admin can list anything, others can only list for themselves
    isAdmin, err := uc.Repo.Check(ctx, input.CallerID, "admin", nil, nil)
    if err != nil { return nil, fmt.Errorf("failed to check admin permission: %w", err) }

    if !isAdmin {
        if input.IdentityID != nil && *input.IdentityID != input.CallerID {
            return nil, fmt.Errorf("non-admin identities can only list their own permissions")
        }
        // Force filter to self if no identity specified
        if input.IdentityID == nil {
            input.IdentityID = &input.CallerID
        }
    }

    return uc.Repo.List(ctx, input.IdentityID, input.ResourceID, input.ResourceTypeID)
}
