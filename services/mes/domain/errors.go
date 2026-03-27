package domain

import "errors"

// Sentinel errors for the MES domain.
// Use errors.Is(err, ErrXxx) to check for specific domain failures.
var (
	// Order errors
	ErrOrderNotFound        = errors.New("manufacturing order not found")
	ErrOrderAlreadyExists   = errors.New("manufacturing order reference already exists")
	ErrInvalidReference     = errors.New("order reference must not be empty")
	ErrInvalidQuantity      = errors.New("order quantity must be greater than zero")
	ErrInvalidProductID     = errors.New("product ID must not be empty")

	// State machine errors
	ErrInvalidTransition    = errors.New("invalid order status transition")
	ErrOrderNotInProgress   = errors.New("order must be in progress to perform this action")
	ErrOrderAlreadyStarted  = errors.New("order is already in progress")
	ErrOrderAlreadyComplete = errors.New("order is already completed")
	ErrOrderCancelled       = errors.New("order has been cancelled")

	// Operation errors
	ErrOperationNotFound      = errors.New("operation not found")
	ErrOperationAlreadyStarted = errors.New("operation is already in progress")
	ErrOperationNotStarted    = errors.New("operation must be started before completing")
	ErrInvalidStepNumber      = errors.New("step number must be greater than zero")
	ErrInvalidOperationName   = errors.New("operation name must not be empty")
	ErrSkipReasonRequired     = errors.New("skip reason is required when skipping an operation")
)
