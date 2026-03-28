package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NCStatus represents the lifecycle state of a NonConformity.
type NCStatus string

const (
	NCStatusOpen               NCStatus = "open"
	NCStatusUnderAnalysis      NCStatus = "under_analysis"
	NCStatusPendingDisposition NCStatus = "pending_disposition"
	NCStatusClosed             NCStatus = "closed"
)

// NCDisposition is the final decision on a NonConformity.
type NCDisposition string

const (
	NCDispositionUnspecified       NCDisposition = ""
	NCDispositionRework            NCDisposition = "rework"
	NCDispositionScrap             NCDisposition = "scrap"
	NCDispositionUseAsIs           NCDisposition = "use_as_is"
	NCDispositionReturnToSupplier  NCDisposition = "return_to_supplier"
)

// NonConformity is the central QMS aggregate (AS9100D §8.7).
// Created automatically when kors.mes.nc.declared is consumed.
type NonConformity struct {
	ID               string
	OperationID      string // dedup key — one NC per MES operation
	OFID             string
	DefectCode       string
	Description      string
	AffectedQuantity int
	SerialNumbers    []string
	DeclaredBy       string
	Status           NCStatus
	Disposition      NCDisposition
	ClosedBy         string

	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  *time.Time
}

// NewNC creates a new NonConformity in OPEN state.
// Typically called by the subscriber when kors.mes.nc.declared is received.
func NewNC(operationID, ofID, defectCode, description string, affectedQuantity int, serialNumbers []string, declaredBy string) (*NonConformity, error) {
	if operationID == "" {
		return nil, fmt.Errorf("NewNC: %w", ErrInvalidOperationID)
	}
	if ofID == "" {
		return nil, fmt.Errorf("NewNC: %w", ErrInvalidOFID)
	}
	if defectCode == "" {
		return nil, fmt.Errorf("NewNC: %w", ErrInvalidDefectCode)
	}
	if affectedQuantity < 1 {
		return nil, fmt.Errorf("NewNC: %w", ErrInvalidAffectedQuantity)
	}
	if declaredBy == "" {
		return nil, fmt.Errorf("NewNC: %w", ErrInvalidDeclaredBy)
	}

	now := time.Now().UTC()
	return &NonConformity{
		ID:               uuid.NewString(),
		OperationID:      operationID,
		OFID:             ofID,
		DefectCode:       defectCode,
		Description:      description,
		AffectedQuantity: affectedQuantity,
		SerialNumbers:    serialNumbers,
		DeclaredBy:       declaredBy,
		Status:           NCStatusOpen,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// StartAnalysis transitions OPEN → UNDER_ANALYSIS.
func (nc *NonConformity) StartAnalysis(analystID string) error {
	if analystID == "" {
		return fmt.Errorf("StartAnalysis: %w", ErrUnauthorizedActor)
	}
	if nc.Status != NCStatusOpen {
		return fmt.Errorf("StartAnalysis: status is %s: %w", nc.Status, ErrNCInvalidTransition)
	}
	nc.Status = NCStatusUnderAnalysis
	nc.UpdatedAt = time.Now().UTC()
	return nil
}

// ProposeDisposition transitions UNDER_ANALYSIS → PENDING_DISPOSITION and records the disposition.
func (nc *NonConformity) ProposeDisposition(disposition NCDisposition, analystID string) error {
	if analystID == "" {
		return fmt.Errorf("ProposeDisposition: %w", ErrUnauthorizedActor)
	}
	if disposition == NCDispositionUnspecified {
		return fmt.Errorf("ProposeDisposition: %w", ErrInvalidDisposition)
	}
	if nc.Status != NCStatusUnderAnalysis {
		return fmt.Errorf("ProposeDisposition: status is %s: %w", nc.Status, ErrNCInvalidTransition)
	}
	nc.Status = NCStatusPendingDisposition
	nc.Disposition = disposition
	nc.UpdatedAt = time.Now().UTC()
	return nil
}

// Close transitions PENDING_DISPOSITION → CLOSED. Requires quality_manager actor (enforced by BFF/handler).
func (nc *NonConformity) Close(closedBy string) error {
	if closedBy == "" {
		return fmt.Errorf("Close: %w", ErrUnauthorizedActor)
	}
	if nc.Status != NCStatusPendingDisposition {
		return fmt.Errorf("Close: status is %s: %w", nc.Status, ErrNCInvalidTransition)
	}
	now := time.Now().UTC()
	nc.Status = NCStatusClosed
	nc.ClosedBy = closedBy
	nc.ClosedAt = &now
	nc.UpdatedAt = now
	return nil
}
