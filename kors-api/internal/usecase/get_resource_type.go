package usecase

import (
    "context"
    "fmt"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

type GetResourceTypeUseCase struct {
    Repo resourcetype.Repository
}

func (uc *GetResourceTypeUseCase) ExecuteByName(ctx context.Context, name string) (*resourcetype.ResourceType, error) {
    rt, err := uc.Repo.GetByName(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource type: %w", err)
    }
    return rt, nil
}

func (uc *GetResourceTypeUseCase) ExecuteList(ctx context.Context) ([]*resourcetype.ResourceType, error) {
    rts, err := uc.Repo.List(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list resource types: %w", err)
    }
    return rts, nil
}