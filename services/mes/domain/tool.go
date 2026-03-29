package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ToolStatus string

const (
	ToolStatusValid          ToolStatus = "VALID"
	ToolStatusExpired        ToolStatus = "EXPIRED"
	ToolStatusBlocked        ToolStatus = "BLOCKED"
	ToolStatusDecommissioned ToolStatus = "DECOMMISSIONED"
)

type Tool struct {
	ID                string
	SerialNumber      string
	Name              string
	Description       string
	Category          string
	Status            ToolStatus
	LastCalibrationAt *time.Time
	NextCalibrationAt *time.Time
	CurrentCycles     int
	MaxCycles         int // 0 if unlimited
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewTool(sn, name, desc, category string, lastCal, nextCal *time.Time, maxCycles int) (*Tool, error) {
	if sn == "" || name == "" {
		return nil, ErrInvalidToolInput
	}
	if maxCycles < 0 {
		return nil, ErrInvalidToolCycles
	}

	now := time.Now().UTC()
	return &Tool{
		ID:                uuid.NewString(),
		SerialNumber:      sn,
		Name:              name,
		Description:       desc,
		Category:          category,
		Status:            ToolStatusValid,
		LastCalibrationAt: lastCal,
		NextCalibrationAt: nextCal,
		CurrentCycles:     0,
		MaxCycles:         maxCycles,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (t *Tool) IsCalibrationValid(now time.Time) bool {
	if t.NextCalibrationAt == nil {
		return true // No calibration required
	}
	return t.NextCalibrationAt.After(now)
}

func (t *Tool) HasRemainingLife() bool {
	if t.MaxCycles == 0 {
		return true
	}
	return t.CurrentCycles < t.MaxCycles
}

func (t *Tool) RecordUsage(cycles int) error {
	if cycles < 0 {
		return fmt.Errorf("cycles must be positive")
	}
	t.CurrentCycles += cycles
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Tool) Calibrate(last, next time.Time) {
	t.LastCalibrationAt = &last
	t.NextCalibrationAt = &next
	t.Status = ToolStatusValid
	t.UpdatedAt = time.Now().UTC()
}

func (t *Tool) Block() {
	t.Status = ToolStatusBlocked
	t.UpdatedAt = time.Now().UTC()
}
