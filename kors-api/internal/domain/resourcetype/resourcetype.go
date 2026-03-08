package resourcetype

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
)

// ResourceType represents a registered type in KORS with its validation rules and lifecycle.
type ResourceType struct {
	ID          uuid.UUID
	Name        string
	Description string
	JSONSchema  map[string]interface{}
	Transitions map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateMetadata validates the provided metadata against the type's JSONSchema.
// Returns nil if schema is empty or validation passes.
func (rt *ResourceType) ValidateMetadata(metadata map[string]interface{}) error {
	if len(rt.JSONSchema) == 0 {
		return nil
	}
	schemaLoader := gojsonschema.NewGoLoader(rt.JSONSchema)
	documentLoader := gojsonschema.NewGoLoader(metadata)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}
	if !result.Valid() {
		errs := make([]string, len(result.Errors()))
		for i, e := range result.Errors() {
			errs[i] = e.String()
		}
		return fmt.Errorf("metadata does not match schema: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CanTransitionTo checks if a transition from fromState to toState is allowed.
func (rt *ResourceType) CanTransitionTo(fromState, toState string) bool {
	// Simple validation for now: check if the toState exists in the allowed transitions for fromState.
	// transitions map format: { "fromState": ["toState1", "toState2"] }
	allowed, ok := rt.Transitions[fromState].([]interface{})
	if !ok {
		return false
	}

	for _, s := range allowed {
		if sStr, ok := s.(string); ok && sStr == toState {
			return true
		}
	}

	return false
}

// Repository defines the contract for persisting and retrieving ResourceTypes.
type Repository interface {
	Create(ctx context.Context, rt *ResourceType) error
	GetByID(ctx context.Context, id uuid.UUID) (*ResourceType, error)
	GetByName(ctx context.Context, name string) (*ResourceType, error)
	List(ctx context.Context) ([]*ResourceType, error)
}
