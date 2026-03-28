package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ── Lot ───────────────────────────────────────────────────────────────────────

// Lot represents a batch of raw material or components received from a supplier.
// Material certificates (AS9100D §8.5.3) are stored in MinIO; only the URL is held here.
type Lot struct {
	ID              string
	Reference       string // unique human-readable lot number (e.g. "LOT-2026-001")
	ProductID       string // UUID of the product this lot is for
	Quantity        int
	MaterialCertURL string // MinIO object URL, empty until certificate is attached
	ReceivedAt      time.Time
}

// NewLot creates a Lot with the given reference, product, and quantity.
func NewLot(reference, productID string, quantity int) (*Lot, error) {
	if reference == "" {
		return nil, ErrInvalidLotReference
	}
	if productID == "" {
		return nil, ErrInvalidProductID
	}
	if quantity <= 0 {
		return nil, ErrInvalidLotQuantity
	}
	return &Lot{
		ID:         uuid.NewString(),
		Reference:  reference,
		ProductID:  productID,
		Quantity:   quantity,
		ReceivedAt: time.Now().UTC(),
	}, nil
}

// AttachCertificate records the MinIO URL of the material certificate.
func (l *Lot) AttachCertificate(url string) {
	l.MaterialCertURL = url
}

// ── SerialNumber ──────────────────────────────────────────────────────────────

// SerialNumberStatus is the lifecycle state of a serialized part (AS9100D §8.5.2).
type SerialNumberStatus string

const (
	SNStatusProduced SerialNumberStatus = "produced"
	SNStatusReleased SerialNumberStatus = "released"
	SNStatusScrapped SerialNumberStatus = "scrapped"
)

// SerialNumber represents a unique, traceable instance of a product.
// It is created when a part is produced during an OF and tracks its entire lifecycle.
type SerialNumber struct {
	ID        string
	SN        string             // human-readable serial (e.g. "SN-2026-0042")
	LotID     string             // source material lot
	ProductID string             // UUID of the product
	OFID      string             // OF that produced this SN
	Status    SerialNumberStatus
	CreatedAt time.Time
}

// NewSerialNumber registers a new serial number produced by a manufacturing order.
func NewSerialNumber(sn, lotID, productID, ofID string) (*SerialNumber, error) {
	if sn == "" {
		return nil, ErrInvalidSerialNumber
	}
	if productID == "" {
		return nil, ErrInvalidProductID
	}
	if ofID == "" {
		return nil, fmt.Errorf("NewSerialNumber: ofID: %w", ErrInvalidProductID)
	}
	return &SerialNumber{
		ID:        uuid.NewString(),
		SN:        sn,
		LotID:     lotID,
		ProductID: productID,
		OFID:      ofID,
		Status:    SNStatusProduced,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Release transitions the serial number from Produced to Released (quality check passed).
func (s *SerialNumber) Release() error {
	switch s.Status {
	case SNStatusReleased:
		return ErrSNAlreadyReleased
	case SNStatusScrapped:
		return fmt.Errorf("Release: %w", ErrSNInvalidTransition)
	case SNStatusProduced:
		// valid
	default:
		return fmt.Errorf("Release: unknown status %q: %w", s.Status, ErrSNInvalidTransition)
	}
	s.Status = SNStatusReleased
	return nil
}

// Scrap marks the serial number as scrapped (defective, removed from production).
// Both Produced and Released SNs can be scrapped.
func (s *SerialNumber) Scrap() error {
	if s.Status == SNStatusScrapped {
		return ErrSNAlreadyScrapped
	}
	s.Status = SNStatusScrapped
	return nil
}

// ── GenealogyEntry ────────────────────────────────────────────────────────────

// GenealogyEntry records that a child serial number was consumed into a parent
// assembly during a specific operation (AS9100D §8.5.2 — full genealogy traceability).
type GenealogyEntry struct {
	ID          string
	ParentSNID  string // assembly receiving the component
	ChildSNID   string // component being consumed
	OFID        string
	OperationID string
	RecordedAt  time.Time
}

// NewGenealogyEntry creates a genealogy link between a parent and child serial number.
func NewGenealogyEntry(parentSNID, childSNID, ofID, operationID string) (*GenealogyEntry, error) {
	if parentSNID == "" || childSNID == "" {
		return nil, fmt.Errorf("NewGenealogyEntry: parent and child SN IDs are required: %w", ErrInvalidProductID)
	}
	if parentSNID == childSNID {
		return nil, fmt.Errorf("NewGenealogyEntry: parent and child SN IDs must differ: %w", ErrSNInvalidTransition)
	}
	if ofID == "" {
		return nil, fmt.Errorf("NewGenealogyEntry: ofID is required: %w", ErrInvalidProductID)
	}
	return &GenealogyEntry{
		ID:          uuid.NewString(),
		ParentSNID:  parentSNID,
		ChildSNID:   childSNID,
		OFID:        ofID,
		OperationID: operationID,
		RecordedAt:  time.Now().UTC(),
	}, nil
}
