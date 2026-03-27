package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OrderStatus represents the lifecycle state of a ManufacturingOrder.
type OrderStatus string

const (
	OrderStatusPlanned    OrderStatus = "planned"
	OrderStatusInProgress OrderStatus = "in_progress"
	OrderStatusCompleted  OrderStatus = "completed"
	OrderStatusSuspended  OrderStatus = "suspended"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

// Order is the central aggregate of the MES domain.
// All state transitions go through methods — never mutate fields directly.
type Order struct {
	ID          string
	Reference   string
	ProductID   string
	Quantity    int
	Status      OrderStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// NewOrder creates a new ManufacturingOrder in Planned status.
// Validates all required fields before creating the aggregate.
func NewOrder(reference, productID string, quantity int) (*Order, error) {
	if reference == "" {
		return nil, ErrInvalidReference
	}
	if productID == "" {
		return nil, ErrInvalidProductID
	}
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	now := time.Now().UTC()
	return &Order{
		ID:        uuid.NewString(),
		Reference: reference,
		ProductID: productID,
		Quantity:  quantity,
		Status:    OrderStatusPlanned,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Start transitions the order from Planned (or Suspended) to InProgress.
func (o *Order) Start() error {
	switch o.Status {
	case OrderStatusInProgress:
		return ErrOrderAlreadyStarted
	case OrderStatusCompleted:
		return fmt.Errorf("Start: %w", ErrInvalidTransition)
	case OrderStatusCancelled:
		return ErrOrderCancelled
	case OrderStatusPlanned, OrderStatusSuspended:
		// valid transitions
	default:
		return fmt.Errorf("Start: unknown status %q: %w", o.Status, ErrInvalidTransition)
	}

	now := time.Now().UTC()
	if o.StartedAt == nil {
		o.StartedAt = &now
	}
	o.Status = OrderStatusInProgress
	o.UpdatedAt = now
	return nil
}

// Complete transitions the order from InProgress to Completed.
func (o *Order) Complete() error {
	switch o.Status {
	case OrderStatusCompleted:
		return ErrOrderAlreadyComplete
	case OrderStatusInProgress:
		// valid
	default:
		return ErrOrderNotInProgress
	}

	now := time.Now().UTC()
	o.Status = OrderStatusCompleted
	o.CompletedAt = &now
	o.UpdatedAt = now
	return nil
}

// Suspend transitions the order from InProgress to Suspended.
func (o *Order) Suspend(_ string) error {
	if o.Status != OrderStatusInProgress {
		return ErrOrderNotInProgress
	}

	o.Status = OrderStatusSuspended
	o.UpdatedAt = time.Now().UTC()
	return nil
}

// Resume transitions the order from Suspended back to InProgress.
func (o *Order) Resume() error {
	if o.Status != OrderStatusSuspended {
		return fmt.Errorf("Resume: order is %q, must be suspended: %w", o.Status, ErrInvalidTransition)
	}

	now := time.Now().UTC()
	o.Status = OrderStatusInProgress
	o.UpdatedAt = now
	return nil
}

// Cancel transitions the order to Cancelled from any non-terminal status.
func (o *Order) Cancel(_ string) error {
	switch o.Status {
	case OrderStatusCompleted:
		return fmt.Errorf("Cancel: %w", ErrInvalidTransition)
	case OrderStatusCancelled:
		return fmt.Errorf("Cancel: %w", ErrInvalidTransition)
	}

	o.Status = OrderStatusCancelled
	o.UpdatedAt = time.Now().UTC()
	return nil
}
