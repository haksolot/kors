package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

type TransitionResourceInput struct {
	ResourceID uuid.UUID
	ToState    string
	Metadata   map[string]interface{}
}

type TransitionResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
}

func (uc *TransitionResourceUseCase) Execute(ctx context.Context, input TransitionResourceInput) (*resource.Resource, error) {
	// 1. Get Resource
	res, err := uc.ResourceRepo.GetByID(ctx, input.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("resource not found")
	}

	// 2. Get ResourceType
	rt, err := uc.ResourceTypeRepo.GetByID(ctx, res.TypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource type: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("resource type not found")
	}

	// 3. Validate Transition
	if !rt.CanTransitionTo(res.State, input.ToState) {
		return nil, fmt.Errorf("transition from '%s' to '%s' is not allowed for type '%s'", res.State, input.ToState, rt.Name)
	}

	// 4. Update Resource
	res.State = input.ToState
	res.UpdatedAt = time.Now()
	// Merge metadata if provided
	if input.Metadata != nil {
		for k, v := range input.Metadata {
			res.Metadata[k] = v
		}
	}

	if err := uc.ResourceRepo.Update(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to update resource state: %w", err)
	}

	return res, nil
}
