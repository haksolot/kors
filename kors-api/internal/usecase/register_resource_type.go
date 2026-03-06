package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/resourcetype"
)

// RegisterResourceTypeInput is the data needed to register a new ResourceType.
type RegisterResourceTypeInput struct {
	Name        string
	Description string
	JSONSchema  map[string]interface{}
	Transitions map[string]interface{}
}

// RegisterResourceTypeUseCase orchestrates the registration of a new ResourceType.
type RegisterResourceTypeUseCase struct {
	Repo resourcetype.Repository
}

// Execute performs the registration.
func (uc *RegisterResourceTypeUseCase) Execute(ctx context.Context, input RegisterResourceTypeInput) (*resourcetype.ResourceType, error) {
	// Simple validation
	if input.Name == "" {
		return nil, fmt.Errorf("resource type name is required")
	}

	// Create domain object
	rt := &resourcetype.ResourceType{
		ID:          uuid.New(),
		Name:        input.Name,
		Description: input.Description,
		JSONSchema:  input.JSONSchema,
		Transitions: input.Transitions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Persist
	if err := uc.Repo.Create(ctx, rt); err != nil {
		return nil, fmt.Errorf("failed to register resource type: %w", err)
	}

	return rt, nil
}
