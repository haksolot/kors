package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
	"github.com/haksolot/kors/shared/pagination"
)

type ListResourcesInput struct {
	First         int
	After         *string // Base64 cursor
	TypeName      *string
	State         *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
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
	if input.After != nil && *input.After != "" {
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

	filter := resource.ListFilter{
		TypeName:      input.TypeName,
		State:         input.State,
		CreatedAfter:  input.CreatedAfter,
		CreatedBefore: input.CreatedBefore,
	}

	res, hasNext, total, err := uc.Repo.List(ctx, input.First, afterID, filter)
	if err != nil {
		return nil, err
	}

	return &ListResourcesResult{
		Resources:   res,
		HasNextPage: hasNext,
		TotalCount:  total,
	}, nil
}
