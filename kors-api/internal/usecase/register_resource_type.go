package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kors-project/kors/kors-api/internal/domain/permission"
	"github.com/kors-project/kors/kors-api/internal/domain/resourcetype"
)

type RegisterResourceTypeInput struct {
	Name        string
	Description string
	JSONSchema  map[string]interface{}
	Transitions map[string]interface{}
	IdentityID  uuid.UUID
}

type RegisterResourceTypeUseCase struct {
	Repo           resourcetype.Repository
	PermissionRepo permission.Repository
}

func (uc *RegisterResourceTypeUseCase) Execute(ctx context.Context, input RegisterResourceTypeInput) (*resourcetype.ResourceType, error) {
	// Sanity checks
	if uc.Repo == nil {
		return nil, fmt.Errorf("internal error: resource type repository is not initialized")
	}
	if uc.PermissionRepo == nil {
		return nil, fmt.Errorf("internal error: permission repository is not initialized")
	}

	if input.Name == "" {
		return nil, fmt.Errorf("resource type name is required")
	}

	// 1. Check Permission (admin required)
	allowed, err := uc.PermissionRepo.Check(ctx, input.IdentityID, "admin", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("identity %s is not authorized to register types (admin role required)", input.IdentityID)
	}

	// 2. Create domain object
	rt := &resourcetype.ResourceType{
		ID:          uuid.New(),
		Name:        input.Name,
		Description: input.Description,
		JSONSchema:  input.JSONSchema,
		Transitions: input.Transitions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 3. Persist
	if err := uc.Repo.Create(ctx, rt); err != nil {
		return nil, fmt.Errorf("failed to register resource type: %w", err)
	}

	return rt, nil
}
