package resourcetype

import (
	"context"
	"time"

	"github.com/google/uuid"
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
