package usecase

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/event"
    "github.com/haksolot/kors/shared/pagination"
)

type ListEventsInput struct {
    ResourceID *uuid.UUID
    IdentityID *uuid.UUID
    Type       *string
    First      int
    After      *string
}

type ListEventsResult struct {
    Events      []*event.Event
    HasNextPage bool
    TotalCount  int
}

type ListEventsUseCase struct {
    Repo event.Repository
}

func (uc *ListEventsUseCase) Execute(ctx context.Context, input ListEventsInput) (*ListEventsResult, error) {
    if input.First == 0 { input.First = 20 }
    var after *uuid.UUID
    if input.After != nil && *input.After != "" {
        raw, err := pagination.DecodeCursor(*input.After)
        if err != nil { return nil, fmt.Errorf("invalid cursor: %w", err) }
        id, err := uuid.Parse(raw)
        if err != nil { return nil, fmt.Errorf("invalid cursor id: %w", err) }
        after = &id
    }
    filter := event.ListFilter{ResourceID: input.ResourceID, IdentityID: input.IdentityID, Type: input.Type}
    events, hasNext, total, err := uc.Repo.List(ctx, filter, input.First, after)
    if err != nil { return nil, err }
    return &ListEventsResult{Events: events, HasNextPage: hasNext, TotalCount: total}, nil
}
