package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ConsumptionRecord struct {
	ID          string
	LotID       string
	OperationID string
	Quantity    int
	OperatorID  string
	ConsumedAt  time.Time
}

func NewConsumptionRecord(lotID, operationID string, quantity int, operatorID string) (*ConsumptionRecord, error) {
	if lotID == "" || operationID == "" || operatorID == "" {
		return nil, fmt.Errorf("lot, operation and operator IDs are required for consumption")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("consumption quantity must be positive")
	}
	return &ConsumptionRecord{
		ID:          uuid.NewString(),
		LotID:       lotID,
		OperationID: operationID,
		Quantity:    quantity,
		OperatorID:  operatorID,
		ConsumedAt:  time.Now().UTC(),
	}, nil
}

type TOEExposureLog struct {
	ID         string
	LotID      string
	StartTime  time.Time
	EndTime    *time.Time
	OperatorID string
}

func NewTOEExposureLog(lotID, operatorID string) (*TOEExposureLog, error) {
	if lotID == "" || operatorID == "" {
		return nil, fmt.Errorf("lot and operator IDs are required for TOE log")
	}
	return &TOEExposureLog{
		ID:         uuid.NewString(),
		LotID:      lotID,
		StartTime:  time.Now().UTC(),
		OperatorID: operatorID,
	}, nil
}

func (l *TOEExposureLog) End() {
	now := time.Now().UTC()
	l.EndTime = &now
}

func (l *TOEExposureLog) Duration() time.Duration {
	if l.EndTime == nil {
		return time.Since(l.StartTime)
	}
	return l.EndTime.Sub(l.StartTime)
}

type EntityType string

const (
	EntityTypeLot    EntityType = "LOT"
	EntityTypeSerial EntityType = "SERIAL"
)

type LocationTransfer struct {
	ID                string
	EntityID          string
	EntityType        EntityType
	FromWorkstationID *string
	ToWorkstationID   string
	TransferredBy     string
	TransferredAt     time.Time
}

func NewLocationTransfer(entityID string, entityType EntityType, fromWS, toWS *string, transferredBy string) (*LocationTransfer, error) {
	if entityID == "" || toWS == nil || *toWS == "" || transferredBy == "" {
		return nil, fmt.Errorf("entity ID, target workstation and operator ID are required for transfer")
	}
	return &LocationTransfer{
		ID:                uuid.NewString(),
		EntityID:          entityID,
		EntityType:        entityType,
		FromWorkstationID: fromWS,
		ToWorkstationID:   *toWS,
		TransferredBy:     transferredBy,
		TransferredAt:     time.Now().UTC(),
	}, nil
}
