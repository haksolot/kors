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

type CreateResourceInput struct {
	TypeName     string
	InitialState string
	Metadata     map[string]interface{}
	IdentityID   uuid.UUID // ID de l'acteur qui crée la ressource
}

type CreateResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
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

	// 2. Create Resource domain object
	res := &resource.Resource{
		ID:        uuid.New(),
		TypeID:    rt.ID,
		State:     input.InitialState,
		Metadata:  input.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. Persist
	if err := uc.ResourceRepo.Create(ctx, res); err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 4. Publish Event
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
	if err := uc.EventRepo.Create(ctx, ev); err != nil {
		// Log error but don't fail the operation (or choose strict mode)
		// For now, let's keep it robust
		fmt.Printf("Warning: failed to record event for resource creation: %v\n", err)
	}

	return res, nil
}
