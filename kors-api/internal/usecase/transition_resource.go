package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/event"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
	"github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
	"github.com/rs/zerolog"
)

type TransitionResourceInput struct {
	ResourceID uuid.UUID
	ToState    string
	Metadata   map[string]interface{}
	IdentityID uuid.UUID
}

type TransitionResourceUseCase struct {
	Pool             *pgxpool.Pool // NOUVEAU
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
	PermissionRepo   permission.Repository
	EventPublisher   event.Publisher
	Logger           zerolog.Logger
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

	// Merge existing metadata with input
	mergedMetadata := make(map[string]interface{})
	if res.Metadata != nil {
		for k, v := range res.Metadata {
			mergedMetadata[k] = v
		}
	}
	if input.Metadata != nil {
		for k, v := range input.Metadata {
			mergedMetadata[k] = v
		}
	}

	// Validate against schema
	if err := rt.ValidateMetadata(mergedMetadata); err != nil {
		return nil, fmt.Errorf("metadata validation failed: %w", err)
	}

	// 5. Update Resource
	res.State = input.ToState
	res.UpdatedAt = time.Now()
	res.Metadata = mergedMetadata

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

	// 7. Persist Resource and Event in an atomic transaction
	tx, err := uc.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := uc.ResourceRepo.UpdateWithTx(ctx, tx, res); err != nil {
		return nil, fmt.Errorf("failed to update resource: %w", err)
	}
	if err := uc.EventRepo.CreateWithTx(ctx, tx, ev); err != nil {
		return nil, fmt.Errorf("failed to persist transition event: %w", err)
	}

	// 8. Broadcast to NATS bus
	if uc.EventPublisher != nil {
		if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
			// NATS is not transactional. We log the failure but commit anyway
			// so we don't lose the transition. A retry worker can recover.
			uc.Logger.Warn().Err(err).Msg("failed to broadcast transition event on NATS")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transition: %w", err)
	}

	return res, nil
}
