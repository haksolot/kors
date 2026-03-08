package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
)

type GetResourceUseCase struct {
    ResourceRepo   resource.Repository
    PermissionRepo permission.Repository
}

func (uc *GetResourceUseCase) Execute(ctx context.Context, id uuid.UUID, identityID uuid.UUID) (*resource.Resource, error) {
    res, err := uc.ResourceRepo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource: %w", err)
    }
    if res == nil {
        return nil, nil
    }
    allowed, err := uc.PermissionRepo.Check(ctx, identityID, "read", &res.ID, &res.TypeID)
    if err != nil {
        return nil, fmt.Errorf("failed to check permission: %w", err)
    }
    if !allowed {
        return nil, fmt.Errorf("permission denied")
    }
    return res, nil
}