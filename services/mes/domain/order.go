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
	// IsFAI: when true, this is a First Article Inspection order (AS9100D §8.6).
	// An FAI order must be explicitly approved by a quality_manager via ApproveFAI.
	IsFAI          bool
	FAIApprovedBy  string     // UUID of the quality_manager who approved the FAI
	FAIApprovedAt  *time.Time // nil until ApproveFAI is called
	// Planning fields (BLOC 5)
	DueDate  *time.Time // Target completion date; drives dispatch list ordering
	Priority int        // 1 (lowest) – 100 (highest); defaults to 50
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
		Priority:  50, // default mid-priority
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SetPlanning sets due_date and priority on the order.
// priority must be in [1, 100]; dueDate may be nil.
func (o *Order) SetPlanning(dueDate *time.Time, priority int) error {
	if priority < 1 || priority > 100 {
		return ErrInvalidPriority
	}
	o.DueDate = dueDate
	o.Priority = priority
	o.UpdatedAt = time.Now().UTC()
	return nil
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

// ApproveFAI records the quality_manager approval for a First Article Inspection order.
// The caller must verify that approverID belongs to a user with role quality_manager.
func (o *Order) ApproveFAI(approverID string) error {
	if approverID == "" {
		return ErrUnauthorizedRole
	}
	if !o.IsFAI {
		return ErrNotFAIOrder
	}
	if o.FAIApprovedAt != nil {
		return ErrFAIAlreadyApproved
	}
	now := time.Now().UTC()
	o.FAIApprovedBy = approverID
	o.FAIApprovedAt = &now
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
