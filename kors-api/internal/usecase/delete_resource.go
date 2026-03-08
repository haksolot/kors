package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
)

type DeleteResourceUseCase struct {
    ResourceRepo   resource.Repository
    PermissionRepo permission.Repository
}

func (uc *DeleteResourceUseCase) Execute(ctx context.Context, id uuid.UUID, identityID uuid.UUID) error {
    res, err := uc.ResourceRepo.GetByID(ctx, id)
    if err != nil {
        return fmt.Errorf("failed to get resource: %w", err)
    }
    if res == nil {
        return fmt.Errorf("resource not found")
    }

    allowed, err := uc.PermissionRepo.Check(ctx, identityID, "admin", &res.ID, &res.TypeID)
    if err != nil {
        return fmt.Errorf("failed to check permission: %w", err)
    }
    if !allowed {
        return fmt.Errorf("admin permission required to delete resource")
    }

    if err := uc.ResourceRepo.SoftDelete(ctx, id); err != nil {
        return fmt.Errorf("failed to delete resource: %w", err)
    }
    return nil
}
