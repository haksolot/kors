package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
	"github.com/haksolot/kors/shared/pagination"
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
		rawID, err := pagination.DecodeCursor(*input.After)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor ID: %w", err)
		}
		afterID = &id
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
