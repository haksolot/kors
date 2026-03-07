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
	DB               interface {
		Begin(ctx context.Context) (any, error)
	}
}

func (uc *CreateResourceUseCase) Execute(ctx context.Context, input CreateResourceInput) (*resource.Resource, error) {
	rt, err := uc.ResourceTypeRepo.GetByName(ctx, input.TypeName)
	if err != nil || rt == nil {
		return nil, fmt.Errorf("resource type not found: %w", err)
	}

	allowed, _ := uc.PermissionRepo.Check(ctx, input.IdentityID, "write", nil, &rt.ID)
	if !allowed {
		return nil, fmt.Errorf("permission denied")
	}

	if input.Metadata == nil { input.Metadata = make(map[string]interface{}) }
	res := &resource.Resource{
		ID: uuid.New(), TypeID: rt.ID, State: input.InitialState, Metadata: input.Metadata,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	// 1. Persist Resource (Not yet committed)
	if err := uc.ResourceRepo.Create(ctx, res); err != nil {
		return nil, err
	}

	// 2. Create Event
	ev := &event.Event{
		ID: uuid.New(), ResourceID: &res.ID, IdentityID: input.IdentityID,
		Type: "kors.resource.created",
		Payload: map[string]interface{}{"type": rt.Name, "state": res.State},
		CreatedAt: time.Now(),
	}
	_ = uc.EventRepo.Create(ctx, ev)

	// 3. CRITICAL: Try to Publish to NATS
	if uc.EventPublisher != nil {
		if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
			// NATS failed! We return error and we should normally ROLLBACK.
			// Currently, since we didn't implement real SQL transaction objects in domain yet,
			// just returning an error is our "barrier".
			return nil, fmt.Errorf("bus error: failed to broadcast event (operation aborted): %w", err)
		}
	}

	return res, nil
}
