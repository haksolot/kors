package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AlertStatus string

const (
	AlertStatusActive      AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved    AlertStatus = "RESOLVED"
)

type AlertLevel string

const (
	AlertLevelL1Supervisor AlertLevel = "L1_SUPERVISOR"
	AlertLevelL2Manager    AlertLevel = "L2_MANAGER"
	AlertLevelL3Director   AlertLevel = "L3_DIRECTOR"
)

type AlertCategory string

const (
	AlertCategoryMachine   AlertCategory = "MACHINE"
	AlertCategoryQuality   AlertCategory = "QUALITY"
	AlertCategoryPlanning  AlertCategory = "PLANNING"
	AlertCategoryLogistics AlertCategory = "LOGISTICS"
)

type Alert struct {
	ID              string
	Category        AlertCategory
	Level           AlertLevel
	Status          AlertStatus
	WorkstationID   *string
	OperationID     *string
	Message         string
	EscalationCount int
	AcknowledgedBy  *string
	AcknowledgedAt  *time.Time
	ResolvedBy      *string
	ResolvedAt      *time.Time
	ResolutionNotes string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewAlert(category AlertCategory, wsID, opID *string, message string) (*Alert, error) {
	if message == "" {
		return nil, fmt.Errorf("alert message is required")
	}
	now := time.Now().UTC()
	return &Alert{
		ID:              uuid.NewString(),
		Category:        category,
		Level:           AlertLevelL1Supervisor,
		Status:          AlertStatusActive,
		WorkstationID:   wsID,
		OperationID:     opID,
		Message:         message,
		EscalationCount: 0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (a *Alert) Acknowledge(userID string) error {
	if a.Status != AlertStatusActive {
		return fmt.Errorf("only active alerts can be acknowledged")
	}
	now := time.Now().UTC()
	a.Status = AlertStatusAcknowledged
	a.AcknowledgedBy = &userID
	a.AcknowledgedAt = &now
	a.UpdatedAt = now
	return nil
}

func (a *Alert) Resolve(userID, notes string) error {
	if a.Status == AlertStatusResolved {
		return fmt.Errorf("alert is already resolved")
	}
	if notes == "" {
		return fmt.Errorf("resolution notes are mandatory")
	}
	now := time.Now().UTC()
	a.Status = AlertStatusResolved
	a.ResolvedBy = &userID
	a.ResolvedAt = &now
	a.ResolutionNotes = notes
	a.UpdatedAt = now
	return nil
}

func (a *Alert) Escalate() error {
	if a.Status != AlertStatusActive {
		return fmt.Errorf("only active alerts can be escalated")
	}
	
	switch a.Level {
	case AlertLevelL1Supervisor:
		a.Level = AlertLevelL2Manager
	case AlertLevelL2Manager:
		a.Level = AlertLevelL3Director
	case AlertLevelL3Director:
		return fmt.Errorf("alert already at maximum escalation level")
	}

	a.EscalationCount++
	a.UpdatedAt = time.Now().UTC()
	return nil
}
