package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CAPAStatus represents the lifecycle state of a CAPA record.
type CAPAStatus string

const (
	CAPAStatusOpen      CAPAStatus = "open"
	CAPAStatusInProgress CAPAStatus = "in_progress"
	CAPAStatusCompleted CAPAStatus = "completed"
	CAPAStatusCancelled CAPAStatus = "cancelled"
)

// CAPAActionType distinguishes corrective from preventive actions.
type CAPAActionType string

const (
	CAPAActionUnspecified  CAPAActionType = ""
	CAPAActionCorrective   CAPAActionType = "corrective"
	CAPAActionPreventive   CAPAActionType = "preventive"
)

// CAPA (Corrective And Preventive Action) is linked to a NonConformity.
type CAPA struct {
	ID          string
	NCID        string
	ActionType  CAPAActionType
	Description string
	OwnerID     string
	Status      CAPAStatus
	DueDate     *time.Time

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// NewCAPA creates a new CAPA in OPEN state.
func NewCAPA(ncID string, actionType CAPAActionType, description, ownerID string, dueDate *time.Time) (*CAPA, error) {
	if ncID == "" {
		return nil, fmt.Errorf("NewCAPA: %w", ErrInvalidNCID)
	}
	if actionType == CAPAActionUnspecified {
		return nil, fmt.Errorf("NewCAPA: %w", ErrInvalidCAPAActionType)
	}
	if description == "" {
		return nil, fmt.Errorf("NewCAPA: %w", ErrInvalidCAPADescription)
	}
	if ownerID == "" {
		return nil, fmt.Errorf("NewCAPA: %w", ErrInvalidCAPAOwner)
	}

	now := time.Now().UTC()
	return &CAPA{
		ID:          uuid.NewString(),
		NCID:        ncID,
		ActionType:  actionType,
		Description: description,
		OwnerID:     ownerID,
		Status:      CAPAStatusOpen,
		DueDate:     dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Start transitions OPEN → IN_PROGRESS.
func (c *CAPA) Start() error {
	if c.Status != CAPAStatusOpen {
		return fmt.Errorf("Start CAPA: status is %s: %w", c.Status, ErrCAPAInvalidTransition)
	}
	c.Status = CAPAStatusInProgress
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// Complete transitions IN_PROGRESS → COMPLETED.
func (c *CAPA) Complete() error {
	if c.Status != CAPAStatusInProgress {
		return fmt.Errorf("Complete CAPA: status is %s: %w", c.Status, ErrCAPAInvalidTransition)
	}
	now := time.Now().UTC()
	c.Status = CAPAStatusCompleted
	c.CompletedAt = &now
	c.UpdatedAt = now
	return nil
}

// Cancel transitions OPEN|IN_PROGRESS → CANCELLED.
func (c *CAPA) Cancel() error {
	if c.Status != CAPAStatusOpen && c.Status != CAPAStatusInProgress {
		return fmt.Errorf("Cancel CAPA: status is %s: %w", c.Status, ErrCAPAInvalidTransition)
	}
	c.Status = CAPAStatusCancelled
	c.UpdatedAt = time.Now().UTC()
	return nil
}
