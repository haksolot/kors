package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OperationStatus represents the lifecycle state of a production Operation.
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "pending"
	OperationStatusInProgress OperationStatus = "in_progress"
	OperationStatusCompleted  OperationStatus = "completed"
	OperationStatusSkipped    OperationStatus = "skipped"
)

// Operation is a single production step within a ManufacturingOrder.
// Steps are ordered by StepNumber (1-based, sequential execution).
type Operation struct {
	ID          string
	OFID        string // Parent ManufacturingOrder ID
	StepNumber  int
	Name        string
	OperatorID  string
	Status      OperationStatus
	SkipReason  string
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
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
func (op *Operation) Start(operatorID string) error {
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

	now := time.Now().UTC()
	op.OperatorID = operatorID
	op.Status = OperationStatusInProgress
	op.StartedAt = &now
	return nil
}

// Complete transitions the operation from InProgress to Completed.
func (op *Operation) Complete(operatorID string) error {
	if op.Status != OperationStatusInProgress {
		return ErrOperationNotStarted
	}

	now := time.Now().UTC()
	op.OperatorID = operatorID
	op.Status = OperationStatusCompleted
	op.CompletedAt = &now
	return nil
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
