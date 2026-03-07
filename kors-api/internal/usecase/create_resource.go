package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/event"
	"github.com/safran-ls/kors/kors-api/internal/domain/permission"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

type CreateResourceInput struct {
	TypeName     string
	InitialState string
	Metadata     map[string]interface{}
	IdentityID   uuid.UUID
}

type CreateResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
	PermissionRepo   permission.Repository
	EventPublisher   event.Publisher
}

func (uc *CreateResourceUseCase) Execute(ctx context.Context, input CreateResourceInput) (*resource.Resource, error) {
	// 1. Verify ResourceType exists
	rt, err := uc.ResourceTypeRepo.GetByName(ctx, input.TypeName)
	if err != nil {
		return nil, fmt.Errorf("failed to check resource type: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("resource type '%s' not found", input.TypeName)
	}

	// 2. Check Permission (Identity must have 'write' on this ResourceType)
	allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "write", nil, &rt.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("identity %s does not have 'write' permission on type '%s'", input.IdentityID, rt.Name)
	}

	// 2. Create Resource domain object
	if input.Metadata == nil {
		input.Metadata = make(map[string]interface{})
	}
	res := &resource.Resource{
		ID:        uuid.New(),
		TypeID:    rt.ID,
		State:     input.InitialState,
		Metadata:  input.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}


	// 3. Persist Resource
	if err := uc.ResourceRepo.Create(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 4. Create Audit Event
	ev := &event.Event{
		ID:         uuid.New(),
		ResourceID: &res.ID,
		IdentityID: input.IdentityID,
		Type:       "kors.resource.created",
		Payload: map[string]interface{}{
			"type":  rt.Name,
			"state": res.State,
		},
		CreatedAt: time.Now(),
	}

	// 5. Persist Event
	if err := uc.EventRepo.Create(ctx, ev); err != nil {
		fmt.Printf("Warning: failed to record event for resource creation: %v\n", err)
	}

	// 6. Broadcast to NATS bus
	if uc.EventPublisher != nil {
		if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
			fmt.Printf("Warning: failed to broadcast event on NATS: %v\n", err)
		}
	}

	return res, nil
}
