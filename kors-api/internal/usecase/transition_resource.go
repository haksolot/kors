package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/event"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

type TransitionResourceInput struct {
	ResourceID uuid.UUID
	ToState    string
	Metadata   map[string]interface{}
	IdentityID uuid.UUID // ID de l'acteur qui effectue la transition
}

type TransitionResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
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
	oldState := res.State
	if !rt.CanTransitionTo(res.State, input.ToState) {
		return nil, fmt.Errorf("transition from '%s' to '%s' is not allowed for type '%s'", res.State, input.ToState, rt.Name)
	}

	// 4. Update Resource
	res.State = input.ToState
	res.UpdatedAt = time.Now()
	// Merge metadata if provided
	if input.Metadata != nil {
		if res.Metadata == nil {
			res.Metadata = make(map[string]interface{})
		}
		for k, v := range input.Metadata {
			res.Metadata[k] = v
		}
	}

	if err := uc.ResourceRepo.Update(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to update resource state: %w", err)
	}

	// 5. Publish Event
	ev := &event.Event{
		ID:         uuid.New(),
		ResourceID: &res.ID,
		IdentityID: input.IdentityID,
		Type:       "kors.resource.state_changed",
		Payload: map[string]interface{}{
			"from_state": oldState,
			"to_state":   res.State,
		},
		CreatedAt: time.Now(),
	}
	if err := uc.EventRepo.Create(ctx, ev); err != nil {
		fmt.Printf("Warning: failed to record event for resource transition: %v\n", err)
	}

	return res, nil
}
