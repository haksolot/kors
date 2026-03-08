package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
)

type ListResourcesInput struct {
	First    int
	After    *string // Base64 cursor
	TypeName *string
}

type ListResourcesResult struct {
	Resources   []*resource.Resource
	HasNextPage bool
	TotalCount  int
}

type ListResourcesUseCase struct {
	Repo resource.Repository
}

func (uc *ListResourcesUseCase) Execute(ctx context.Context, input ListResourcesInput) (*ListResourcesResult, error) {
	var afterID *uuid.UUID
	if input.After != nil {
		// Logic to decode cursor would go here if we used the shared pagination helper
		// For now, we'll assume the cursor is just the raw UUID for simplicity in this turn
		id, err := uuid.Parse(*input.After)
		if err == nil {
			afterID = &id
		}
	}

	if input.First <= 0 {
		input.First = 20 // Default limit
	}

	res, hasNext, total, err := uc.Repo.List(ctx, input.First, afterID, input.TypeName)
	if err != nil {
		return nil, err
	}

	return &ListResourcesResult{
		Resources:   res,
		HasNextPage: hasNext,
		TotalCount:  total,
	}, nil
}
