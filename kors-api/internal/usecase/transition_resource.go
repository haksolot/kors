package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/event"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
	"github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

type TransitionResourceInput struct {
	ResourceID uuid.UUID
	ToState    string
	Metadata   map[string]interface{}
	IdentityID uuid.UUID
}

type TransitionResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
	PermissionRepo   permission.Repository
	EventPublisher   event.Publisher
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

	// 2. Check Permission (Identity must have 'transition' on this Resource or its Type)
	allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "transition", &res.ID, &res.TypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("identity %s does not have 'transition' permission on resource %s", input.IdentityID, res.ID)
	}

	// 3. Get ResourceType
	rt, err := uc.ResourceTypeRepo.GetByID(ctx, res.TypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource type: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("resource type not found")
	}

	// 4. Validate Transition
	oldState := res.State
	if !rt.CanTransitionTo(oldState, input.ToState) {
		return nil, fmt.Errorf("transition from '%s' to '%s' not allowed for type '%s'", oldState, input.ToState, rt.Name)
	}

	// 5. Update Resource
	res.State = input.ToState
	res.UpdatedAt = time.Now()
	if input.Metadata != nil {
		if res.Metadata == nil {
			res.Metadata = make(map[string]interface{})
		}
		for k, v := range input.Metadata {
			res.Metadata[k] = v
		}
	}

	if err := uc.ResourceRepo.Update(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to update resource: %w", err)
	}

	// 6. Create Audit Event
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

	// 7. Persist Event
	if err := uc.EventRepo.Create(ctx, ev); err != nil {
		fmt.Printf("Warning: failed to record event for transition: %v\n", err)
	}

	// 8. Broadcast to NATS bus
	if uc.EventPublisher != nil {
		if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
			fmt.Printf("Warning: failed to broadcast event on NATS: %v\n", err)
		}
	}

	return res, nil
}
