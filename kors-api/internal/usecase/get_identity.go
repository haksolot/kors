package usecase

import (
    "context"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/identity"
)

type GetIdentityUseCase struct {
    Repo identity.Repository
}

func (uc *GetIdentityUseCase) Execute(ctx context.Context, id uuid.UUID) (*identity.Identity, error) {
    return uc.Repo.GetByID(ctx, id)
}
