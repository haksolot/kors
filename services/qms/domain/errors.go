package domain

import "errors"

var (
	// NC validation
	ErrInvalidOperationID     = errors.New("operation_id must not be empty")
	ErrInvalidOFID            = errors.New("of_id must not be empty")
	ErrInvalidDefectCode      = errors.New("defect_code must not be empty")
	ErrInvalidAffectedQuantity = errors.New("affected_quantity must be at least 1")
	ErrInvalidDeclaredBy      = errors.New("declared_by must not be empty")
	ErrInvalidDisposition     = errors.New("disposition must be specified")
	ErrUnauthorizedActor      = errors.New("actor ID must not be empty")

	// NC state machine
	ErrNCInvalidTransition = errors.New("invalid NC state transition")
	ErrNCNotFound          = errors.New("non-conformity not found")
	ErrNCAlreadyExists     = errors.New("non-conformity already exists for this operation")

	// CAPA validation
	ErrInvalidNCID            = errors.New("nc_id must not be empty")
	ErrInvalidCAPAActionType  = errors.New("action_type must be corrective or preventive")
	ErrInvalidCAPADescription = errors.New("description must not be empty")
	ErrInvalidCAPAOwner       = errors.New("owner_id must not be empty")

	// CAPA state machine
	ErrCAPAInvalidTransition = errors.New("invalid CAPA state transition")
	ErrCAPANotFound          = errors.New("capa not found")
)
