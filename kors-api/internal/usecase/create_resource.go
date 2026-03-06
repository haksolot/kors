package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

type CreateResourceInput struct {
	TypeName     string
	InitialState string
	Metadata     map[string]interface{}
}

type CreateResourceUseCase struct {
	ResourceRepo     resource.Repository
	ResourceTypeRepo resourcetype.Repository
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

	return res, nil
}
