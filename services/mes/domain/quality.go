package domain

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type CharacteristicType string

const (
	CharacteristicTypeQuantitative CharacteristicType = "QUANTITATIVE"
	CharacteristicTypeQualitative  CharacteristicType = "QUALITATIVE"
)

type MeasurementStatus string

const (
	MeasurementStatusPass    MeasurementStatus = "PASS"
	MeasurementStatusFail    MeasurementStatus = "FAIL"
	MeasurementStatusWarning MeasurementStatus = "WARNING"
)

type ControlCharacteristic struct {
	ID             string
	StepID         string
	Name           string
	Type           CharacteristicType
	Unit           string
	NominalValue   *float64
	UpperTolerance *float64
	LowerTolerance *float64
	IsMandatory    bool
}

type Measurement struct {
	ID               string
	OperationID      string
	CharacteristicID string
	Value            string
	Status           MeasurementStatus
	OperatorID       string
	RecordedAt       time.Time
}

func NewMeasurement(opID, charID, value, operatorID string, char *ControlCharacteristic) (*Measurement, error) {
	if opID == "" || charID == "" || operatorID == "" {
		return nil, fmt.Errorf("operation, characteristic and operator IDs are required")
	}

	status := MeasurementStatusPass
	if char.Type == CharacteristicTypeQuantitative {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid numeric value for quantitative characteristic: %w", err)
		}
		if char.UpperTolerance != nil && v > *char.UpperTolerance {
			status = MeasurementStatusFail
		}
		if char.LowerTolerance != nil && v < *char.LowerTolerance {
			status = MeasurementStatusFail
		}
	} else if char.Type == CharacteristicTypeQualitative {
		if value != "PASS" && value != "OK" && value != "YES" {
			status = MeasurementStatusFail
		}
	}

	return &Measurement{
		ID:               uuid.NewString(),
		OperationID:      opID,
		CharacteristicID: charID,
		Value:            value,
		Status:           status,
		OperatorID:       operatorID,
		RecordedAt:       time.Now().UTC(),
	}, nil
}

// CheckSPCDrift looks for Nelson/Western Electric rules.
// Simple implementation: check if last 3 measurements are all trending in same direction.
func CheckSPCDrift(history []*Measurement) bool {
	if len(history) < 3 {
		return false
	}
	// Simplified: check for 3 consecutive FAIL or WARNING
	count := 0
	for _, m := range history {
		if m.Status != MeasurementStatusPass {
			count++
		} else {
			break
		}
	}
	return count >= 3
}
