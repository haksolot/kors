package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OperationStatus represents the lifecycle state of a production Operation.
type OperationStatus string

const (
	OperationStatusPending        OperationStatus = "pending"
	OperationStatusInProgress     OperationStatus = "in_progress"
	OperationStatusCompleted      OperationStatus = "completed"
	OperationStatusSkipped        OperationStatus = "skipped"
	// OperationStatusPendingSignOff: operation work is done but awaits quality sign-off (AS9100D §8.6).
	// Reached when Complete() is called on an operation with RequiresSignOff=true.
	OperationStatusPendingSignOff OperationStatus = "pending_sign_off"
	// OperationStatusReleased: sign-off accepted by a quality_inspector; operation is fully released.
	OperationStatusReleased       OperationStatus = "released"
)

// Operation is a single production step within a ManufacturingOrder.
// Steps are ordered by StepNumber (1-based, sequential execution).
type Operation struct {
	ID             string
	OFID           string // Parent ManufacturingOrder ID
	StepNumber     int
	Name           string
	OperatorID     string
	Status         OperationStatus
	SkipReason     string
	// RequiresSignOff: if true, Complete() moves the operation to PendingSignOff
	// instead of Completed. A quality_inspector must then call SignOff().
	RequiresSignOff bool
	SignedOffBy     string     // UUID of the quality inspector who signed off
	SignedOffAt     *time.Time // nil until sign-off
	// InstructionsURL points to a work instruction document stored in MinIO (ADR-007).
	InstructionsURL string
	// Cycle-time fields (BLOC 5 — TRS). PlannedDurationSeconds comes from the RoutingStep.
	// ActualDurationSeconds is computed at completion: CompletedAt - StartedAt.
	PlannedDurationSeconds int
	ActualDurationSeconds  int
	// RequiredSkill is the JWT role the operator must hold to start this operation (AS9100D §7.2).
	// Empty string means no skill check.
	RequiredSkill string
	// Special Process fields (§13 — EN9100 / NADCAP compliance).
	// IsSpecialProcess flags this operation as a NADCAP-qualified process.
	// NADCAPProcessCode must match a valid Qualification.SkillCode held by the operator.
	// The caller passes nadcapSkills (active qualification codes) to Start() for the interlock.
	IsSpecialProcess  bool
	NADCAPProcessCode string
	CreatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
}

// NewOperation creates a new Operation in Pending status.
func NewOperation(ofID string, stepNumber int, name string) (*Operation, error) {
	if ofID == "" {
		return nil, ErrInvalidProductID // ErrInvalidProductID reused — ofID is a required FK
	}
	if stepNumber <= 0 {
		return nil, ErrInvalidStepNumber
	}
	if name == "" {
		return nil, ErrInvalidOperationName
	}

	return &Operation{
		ID:         uuid.NewString(),
		OFID:       ofID,
		StepNumber: stepNumber,
		Name:       name,
		Status:     OperationStatusPending,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// Start transitions the operation from Pending to InProgress.
// operatorID must be the Subject from a validated JWT — never from the request body.
// operatorRoles is the list of JWT roles for the operator; used to check RequiredSkill (AS9100D §7.2).
// nadcapSkills is the list of active NADCAP qualification codes held by the operator (§13);
// pass nil or empty if the caller has not loaded qualifications.
func (op *Operation) Start(operatorID string, operatorRoles []string, nadcapSkills []string) error {
	if operatorID == "" {
		return ErrInvalidProductID // operatorID is a required field
	}

	switch op.Status {
	case OperationStatusInProgress:
		return ErrOperationAlreadyStarted
	case OperationStatusCompleted, OperationStatusSkipped:
		return fmt.Errorf("Start: operation is %q: %w", op.Status, ErrInvalidTransition)
	case OperationStatusPending:
		// valid
	default:
		return fmt.Errorf("Start: unknown status %q: %w", op.Status, ErrInvalidTransition)
	}

	// Skill qualification check (AS9100D §7.2): if a required skill is set, the operator
	// must hold that role in their JWT claims.
	if op.RequiredSkill != "" && !containsRole(operatorRoles, op.RequiredSkill) {
		return fmt.Errorf("Start: operator lacks required skill %q: %w", op.RequiredSkill, ErrOperatorNotQualified)
	}

	// NADCAP special process interlock (§13 — EN9100 / Special Processes).
	// If the operation is flagged as a special process, the operator must hold a valid,
	// non-expired NADCAP qualification whose SkillCode matches NADCAPProcessCode.
	// The handler is responsible for loading active NADCAP qualifications before calling Start.
	if op.IsSpecialProcess && !containsRole(nadcapSkills, op.NADCAPProcessCode) {
		return fmt.Errorf("Start: operator lacks NADCAP qualification %q: %w", op.NADCAPProcessCode, ErrNADCAPQualificationRequired)
	}

	now := time.Now().UTC()
	op.OperatorID = operatorID
	op.Status = OperationStatusInProgress
	op.StartedAt = &now
	return nil
}

// containsRole reports whether role is in roles.
func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// Complete transitions the operation from InProgress.
// If RequiresSignOff is true, the operation moves to PendingSignOff (AS9100D §8.6 hold point).
// Otherwise it moves directly to Completed.
func (op *Operation) Complete(operatorID string) error {
	if op.Status != OperationStatusInProgress {
		return ErrOperationNotStarted
	}

	now := time.Now().UTC()
	op.OperatorID = operatorID
	op.CompletedAt = &now
	// Compute actual duration if we have a start time.
	if op.StartedAt != nil {
		op.ActualDurationSeconds = int(now.Sub(*op.StartedAt).Seconds())
	}
	if op.RequiresSignOff {
		op.Status = OperationStatusPendingSignOff
	} else {
		op.Status = OperationStatusCompleted
	}
	return nil
}

// SignOff transitions the operation from PendingSignOff to Released.
// inspectorID must be the UUID of a user with role quality_inspector.
// The caller (BFF / handler) is responsible for verifying the role before calling SignOff.
func (op *Operation) SignOff(inspectorID string) error {
	if inspectorID == "" {
		return ErrUnauthorizedRole
	}
	if op.Status != OperationStatusPendingSignOff {
		return ErrNotPendingSignOff
	}
	now := time.Now().UTC()
	op.SignedOffBy = inspectorID
	op.SignedOffAt = &now
	op.Status = OperationStatusReleased
	return nil
}

// AttachInstructions sets the MinIO URL for the work instruction document.
func (op *Operation) AttachInstructions(url string) {
	op.InstructionsURL = url
}

// Skip marks the operation as Skipped with a mandatory justification.
// Only Pending operations can be skipped.
func (op *Operation) Skip(reason string) error {
	if reason == "" {
		return ErrSkipReasonRequired
	}
	switch op.Status {
	case OperationStatusPending:
		// valid
	default:
		return fmt.Errorf("Skip: operation is %q: %w", op.Status, ErrInvalidTransition)
	}

	op.Status = OperationStatusSkipped
	op.SkipReason = reason
	return nil
}
