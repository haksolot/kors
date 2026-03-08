package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/identity"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/shared/pagination"
)

type ListIdentitiesInput struct {
    CallerID     uuid.UUID
    IdentityType *string
    First        int
    After        *string
}

type ListIdentitiesResult struct {
    Identities  []*identity.Identity
    HasNextPage bool
    TotalCount  int
}

type ListIdentitiesUseCase struct {
    Repo           identity.Repository
    PermissionRepo permission.Repository
}

func (uc *ListIdentitiesUseCase) Execute(ctx context.Context, input ListIdentitiesInput) (*ListIdentitiesResult, error) {
    // Admin check
    allowed, err := uc.PermissionRepo.Check(ctx, input.CallerID, "admin", nil, nil)
    if err != nil { return nil, fmt.Errorf("failed to check permission: %w", err) }
    if !allowed { return nil, fmt.Errorf("admin permission required to list identities") }

    if input.First == 0 { input.First = 20 }
    var after *uuid.UUID
    if input.After != nil && *input.After != "" {
        raw, err := pagination.DecodeCursor(*input.After)
        if err != nil { return nil, fmt.Errorf("invalid cursor: %w", err) }
        id, err := uuid.Parse(raw)
        if err != nil { return nil, fmt.Errorf("invalid cursor id: %w", err) }
        after = &id
    }

    idents, hasNext, total, err := uc.Repo.List(ctx, input.IdentityType, input.First, after)
    if err != nil { return nil, err }

    return &ListIdentitiesResult{Identities: idents, HasNextPage: hasNext, TotalCount: total}, nil
}
