package domain

import (
	"time"

	"github.com/google/uuid"
)

type TimeLogType string

const (
	TimeLogTypeSetup TimeLogType = "SETUP"
	TimeLogTypeRun   TimeLogType = "RUN"
)

type DowntimeCategory string

const (
	DowntimeCategoryMachineFailure        DowntimeCategory = "MACHINE_FAILURE"
	DowntimeCategoryPreventiveMaintenance DowntimeCategory = "PREVENTIVE_MAINTENANCE"
	DowntimeCategoryMaterialShortage      DowntimeCategory = "MATERIAL_SHORTAGE"
	DowntimeCategoryQualityHold           DowntimeCategory = "QUALITY_HOLD"
	DowntimeCategoryChangeover            DowntimeCategory = "CHANGEOVER"
	DowntimeCategoryRegulatoryPause       DowntimeCategory = "REGULATORY_PAUSE"
	DowntimeCategoryUnjustified           DowntimeCategory = "UNJUSTIFIED"
)

type TimeLog struct {
	ID            string
	OperationID   string
	WorkstationID string
	OperatorID    string
	LogType       TimeLogType
	StartTime     time.Time
	EndTime       time.Time
	GoodQuantity  int
	ScrapQuantity int
	CreatedAt     time.Time
}

// Duration returns the duration of the time log.
func (t *TimeLog) Duration() time.Duration {
	return t.EndTime.Sub(t.StartTime)
}

func NewTimeLog(operationID, workstationID, operatorID string, logType TimeLogType, start, end time.Time, good, scrap int) (*TimeLog, error) {
	if operationID == "" || workstationID == "" || operatorID == "" {
		return nil, ErrInvalidTimeLogInput
	}
	if end.Before(start) {
		return nil, ErrInvalidTimeLogDates
	}
	if good < 0 || scrap < 0 {
		return nil, ErrInvalidTimeLogQuantities
	}
	switch logType {
	case TimeLogTypeSetup, TimeLogTypeRun:
	default:
		return nil, ErrInvalidTimeLogType
	}

	return &TimeLog{
		ID:            uuid.NewString(),
		OperationID:   operationID,
		WorkstationID: workstationID,
		OperatorID:    operatorID,
		LogType:       logType,
		StartTime:     start.UTC(),
		EndTime:       end.UTC(),
		GoodQuantity:  good,
		ScrapQuantity: scrap,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

type DowntimeEvent struct {
	ID            string
	WorkstationID string
	OperationID   *string // optional
	Category      DowntimeCategory
	Description   string
	StartTime     time.Time
	EndTime       *time.Time // null if ongoing
	ReportedBy    string
	CreatedAt     time.Time
}

func NewDowntimeEvent(workstationID string, operationID *string, category DowntimeCategory, description, reportedBy string) (*DowntimeEvent, error) {
	if workstationID == "" || reportedBy == "" {
		return nil, ErrInvalidDowntimeInput
	}
	switch category {
	case DowntimeCategoryMachineFailure, DowntimeCategoryPreventiveMaintenance, DowntimeCategoryMaterialShortage,
		DowntimeCategoryQualityHold, DowntimeCategoryChangeover, DowntimeCategoryRegulatoryPause, DowntimeCategoryUnjustified:
	default:
		return nil, ErrInvalidDowntimeCategory
	}

	return &DowntimeEvent{
		ID:            uuid.NewString(),
		WorkstationID: workstationID,
		OperationID:   operationID,
		Category:      category,
		Description:   description,
		StartTime:     time.Now().UTC(),
		ReportedBy:    reportedBy,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (d *DowntimeEvent) End() error {
	if d.EndTime != nil {
		return ErrDowntimeAlreadyEnded
	}
	now := time.Now().UTC()
	d.EndTime = &now
	return nil
}

func (d *DowntimeEvent) Duration() time.Duration {
	if d.EndTime == nil {
		return time.Since(d.StartTime)
	}
	return d.EndTime.Sub(d.StartTime)
}
