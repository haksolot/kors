package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	"github.com/haksolot/kors/kors-api/internal/domain/event"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/domain/resource"
	"github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

type CreateResourceInput struct {
	TypeName     string
	InitialState string
	Metadata     map[string]interface{}
	IdentityID   uuid.UUID
}

type CreateResourceUseCase struct {
	Pool             *pgxpool.Pool
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
	EventRepo        event.Repository
	PermissionRepo   permission.Repository
	EventPublisher   event.Publisher
}

func (uc *CreateResourceUseCase) Execute(ctx context.Context, input CreateResourceInput) (*resource.Resource, error) {
	rt, err := uc.ResourceTypeRepo.GetByName(ctx, input.TypeName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup resource type: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("resource type %q not found", input.TypeName)
	}

	allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "write", nil, &rt.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("identity %s does not have 'write' permission on type %s", input.IdentityID, rt.Name)
	}

	if err := rt.ValidateMetadata(input.Metadata); err != nil {
		return nil, fmt.Errorf("metadata validation failed: %w", err)
	}

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

	ev := &event.Event{
		ID:         uuid.New(),
		ResourceID: &res.ID,
		IdentityID: input.IdentityID,
		Type:       "kors.resource.created",
		Payload:    map[string]interface{}{"type": rt.Name, "state": res.State},
		CreatedAt:  time.Now(),
	}

	// Transaction SQL : resource + event en atomique
	tx, err := uc.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // No-op si Commit reussit

	pgRepo, ok := uc.ResourceRepo.(*postgres.ResourceRepository)
	if !ok {
		return nil, fmt.Errorf("internal error: ResourceRepo must be *postgres.ResourceRepository for transactional writes")
	}
	if err := pgRepo.CreateWithTx(ctx, tx, res); err != nil {
		return nil, fmt.Errorf("failed to persist resource: %w", err)
	}

	pgEventRepo, ok := uc.EventRepo.(*postgres.EventRepository)
	if !ok {
		return nil, fmt.Errorf("internal error: EventRepo must be *postgres.EventRepository for transactional writes")
	}
	if err := pgEventRepo.CreateWithTx(ctx, tx, ev); err != nil {
		return nil, fmt.Errorf("failed to persist event: %w", err)
	}

	// Publier sur NATS AVANT le commit : si NATS echoue, on rollback
	if uc.EventPublisher != nil {
		if err := uc.EventPublisher.Publish(ctx, ev); err != nil {
			return nil, fmt.Errorf("bus error: failed to broadcast event (operation aborted): %w", err)
		}
	}

	// Tout s'est bien passe, on commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return res, nil
}
