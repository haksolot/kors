package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WorkstationStatus represents the current state of a workstation/machine.
type WorkstationStatus string

const (
	WorkstationStatusAvailable    WorkstationStatus = "AVAILABLE"
	WorkstationStatusInProduction WorkstationStatus = "IN_PRODUCTION"
	WorkstationStatusDown         WorkstationStatus = "DOWN"
	WorkstationStatusMaintenance  WorkstationStatus = "MAINTENANCE"
)

// Workstation represents a physical machine or workspace where operations are executed.
type Workstation struct {
	ID          string
	Name        string
	Description string
	Capacity    int
	NominalRate float64
	Status      WorkstationStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewWorkstation creates a new Workstation.
// Validates all required fields before creating the aggregate.
func NewWorkstation(name, description string, capacity int, nominalRate float64) (*Workstation, error) {
	if name == "" {
		return nil, ErrInvalidWorkstationName
	}
	if capacity <= 0 {
		return nil, ErrInvalidWorkstationCapacity
	}
	if nominalRate < 0 {
		return nil, ErrInvalidWorkstationRate
	}

	now := time.Now().UTC()
	return &Workstation{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		Capacity:    capacity,
		NominalRate: nominalRate,
		Status:      WorkstationStatusAvailable,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// UpdateStatus changes the workstation's status.
func (w *Workstation) UpdateStatus(newStatus WorkstationStatus) error {
	switch newStatus {
	case WorkstationStatusAvailable, WorkstationStatusInProduction, WorkstationStatusDown, WorkstationStatusMaintenance:
		// Valid statuses
	default:
		return fmt.Errorf("invalid workstation status %s: %w", newStatus, ErrInvalidWorkstationStatus)
	}

	w.Status = newStatus
	w.UpdatedAt = time.Now().UTC()
	return nil
}
