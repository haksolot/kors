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

	// Quality / sign-off errors
	ErrSignOffRequired    = errors.New("operation requires quality sign-off before completion")
	ErrNotPendingSignOff  = errors.New("operation is not pending sign-off")
	ErrUnauthorizedRole   = errors.New("caller does not have the required role for this action")
	ErrFAIAlreadyApproved = errors.New("first article inspection already approved")
	ErrNotFAIOrder        = errors.New("manufacturing order is not flagged as a FAI order")

	// Lot errors
	ErrLotNotFound      = errors.New("lot not found")
	ErrLotAlreadyExists = errors.New("lot reference already exists")
	ErrInvalidLotReference = errors.New("lot reference must not be empty")
	ErrInvalidLotQuantity  = errors.New("lot quantity must be greater than zero")

	// SerialNumber errors
	ErrSerialNumberNotFound      = errors.New("serial number not found")
	ErrSerialNumberAlreadyExists = errors.New("serial number already exists")
	ErrInvalidSerialNumber       = errors.New("serial number must not be empty")
	ErrSNAlreadyReleased         = errors.New("serial number is already released")
	ErrSNAlreadyScrapped         = errors.New("serial number is already scrapped")
	ErrSNInvalidTransition       = errors.New("invalid serial number status transition")
)
