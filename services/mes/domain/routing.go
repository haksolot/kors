package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Routing is a manufacturing route template attached to a product.
// It defines the ordered sequence of steps (RoutingStep) to produce the product.
// When an OF is created from a routing, all steps are instantiated as Operations.
type Routing struct {
	ID        string
	ProductID string
	Version   int
	Name      string
	IsActive  bool
	Steps     []*RoutingStep
	CreatedAt time.Time
}

// RoutingStep is a single step in a Routing template.
// It becomes an Operation when the routing is applied to an OF.
type RoutingStep struct {
	ID                     string
	RoutingID              string
	StepNumber             int
	Name                   string
	PlannedDurationSeconds int    // Expected duration — used for TRS/cycle-time comparison
	RequiredSkill          string // JWT role required by the operator (AS9100D §7.2)
	InstructionsURL        string // MinIO reference, optional
	RequiresSignOff        bool   // If true, the instantiated operation will require sign-off
	// Special Process fields (§13 — EN9100 / NADCAP compliance).
	// IsSpecialProcess flags this step as subject to NADCAP qualification.
	// NADCAPProcessCode is the specific process code (e.g. "NADCAP-WELD", "NADCAP-NDT").
	// The operator must hold a non-expired Qualification with SkillCode == NADCAPProcessCode.
	IsSpecialProcess  bool
	NADCAPProcessCode string
}

// NewRouting creates a new Routing template in inactive state.
// Activate it explicitly after steps are added.
func NewRouting(productID, name string, version int) (*Routing, error) {
	if productID == "" {
		return nil, ErrInvalidProductID
	}
	if name == "" {
		return nil, ErrInvalidRoutingName
	}
	if version <= 0 {
		return nil, ErrInvalidRoutingVersion
	}
	return &Routing{
		ID:        uuid.NewString(),
		ProductID: productID,
		Version:   version,
		Name:      name,
		IsActive:  false,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// AddStep appends a step to the routing.
// Steps must be added in order (step_number must be sequential starting at 1).
func (r *Routing) AddStep(stepNumber int, name string, plannedDurationSeconds int) (*RoutingStep, error) {
	if stepNumber <= 0 {
		return nil, ErrInvalidStepNumber
	}
	if name == "" {
		return nil, ErrInvalidOperationName
	}
	if plannedDurationSeconds < 0 {
		return nil, ErrInvalidPlannedDuration
	}
	step := &RoutingStep{
		ID:                     uuid.NewString(),
		RoutingID:              r.ID,
		StepNumber:             stepNumber,
		Name:                   name,
		PlannedDurationSeconds: plannedDurationSeconds,
	}
	r.Steps = append(r.Steps, step)
	return step, nil
}

// Activate marks the routing as active.
// A routing must have at least one step before it can be activated.
func (r *Routing) Activate() error {
	if len(r.Steps) == 0 {
		return ErrRoutingHasNoSteps
	}
	r.IsActive = true
	return nil
}

// InstantiateOperations creates a slice of Operations from this routing's steps,
// associated with the given OF. All operations start in Pending status.
// Returns an error if the routing has no steps or is not active.
func (r *Routing) InstantiateOperations(ofID string) ([]*Operation, error) {
	if !r.IsActive {
		return nil, ErrRoutingNotActive
	}
	if len(r.Steps) == 0 {
		return nil, ErrRoutingHasNoSteps
	}
	if ofID == "" {
		return nil, fmt.Errorf("InstantiateOperations: ofID is required: %w", ErrInvalidProductID)
	}

	ops := make([]*Operation, 0, len(r.Steps))
	now := time.Now().UTC()
	for _, step := range r.Steps {
		op := &Operation{
			ID:                     uuid.NewString(),
			OFID:                   ofID,
			StepNumber:             step.StepNumber,
			Name:                   step.Name,
			Status:                 OperationStatusPending,
			RequiredSkill:          step.RequiredSkill,
			PlannedDurationSeconds: step.PlannedDurationSeconds,
			InstructionsURL:        step.InstructionsURL,
			RequiresSignOff:        step.RequiresSignOff,
			IsSpecialProcess:       step.IsSpecialProcess,
			NADCAPProcessCode:      step.NADCAPProcessCode,
			CreatedAt:              now,
		}
		ops = append(ops, op)
	}
	return ops, nil
}
