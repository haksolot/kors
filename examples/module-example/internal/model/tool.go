package model

import (
	"github.com/google/uuid"
)

type Tool struct {
	ID           uuid.UUID // ID Universel KORS
	SerialNumber string
	Model        string
	Diameter     float64
	Length       float64
}
