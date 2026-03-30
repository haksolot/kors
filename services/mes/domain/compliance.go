package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── AuditEntry ────────────────────────────────────────────────────────────────
// AuditEntry is an append-only record of every state-changing action in the MES.
// It satisfies §13 (EN9100 audit trail) and §12 (electronic signatures for critical steps).
// Entries are NEVER updated or deleted — the audit trail is immutable by convention
// and enforced at the repository layer (no UPDATE/DELETE on audit_trail).

// AuditAction enumerates the actions that must appear in the audit trail.
type AuditAction string

const (
	AuditActionOFCreated          AuditAction = "OF_CREATED"
	AuditActionOFStarted          AuditAction = "OF_STARTED"
	AuditActionOFSuspended        AuditAction = "OF_SUSPENDED"
	AuditActionOFCompleted        AuditAction = "OF_COMPLETED"
	AuditActionOFCancelled        AuditAction = "OF_CANCELLED"
	AuditActionOFFAIApproved      AuditAction = "OF_FAI_APPROVED"
	AuditActionOperationStarted   AuditAction = "OPERATION_STARTED"
	AuditActionOperationCompleted AuditAction = "OPERATION_COMPLETED"
	AuditActionOperationSignedOff AuditAction = "OPERATION_SIGNED_OFF"
	AuditActionOperationSkipped   AuditAction = "OPERATION_SKIPPED"
	AuditActionNCDeclared         AuditAction = "NC_DECLARED"
	AuditActionLotBlocked         AuditAction = "LOT_BLOCKED"
	AuditActionSNReleased         AuditAction = "SN_RELEASED"
	AuditActionSNScrapped         AuditAction = "SN_SCRAPPED"
	// NADCAP-specific (§13 Special Processes)
	AuditActionNADCAPOperationStarted AuditAction = "NADCAP_OPERATION_STARTED"
)

// AuditEntityType identifies the primary entity concerned by an audit entry.
type AuditEntityType string

const (
	AuditEntityOrder     AuditEntityType = "manufacturing_order"
	AuditEntityOperation AuditEntityType = "operation"
	AuditEntityLot       AuditEntityType = "lot"
	AuditEntitySN        AuditEntityType = "serial_number"
)

// AuditEntry records a single immutable event in the audit trail.
// ActorID is always taken from the validated JWT subject — never from the request body.
type AuditEntry struct {
	ID             string
	ActorID        string          // UUID of the user who performed the action
	ActorRole      string          // Role claim from the JWT at the time of the action
	Action         AuditAction     // What happened
	EntityType     AuditEntityType // Which entity was affected
	EntityID       string          // UUID of the affected entity
	WorkstationID  string          // optional — workstation from which the action was performed
	Notes          string          // optional — free-text context (e.g. suspension reason)
	CreatedAt      time.Time
}

// NewAuditEntry constructs a valid AuditEntry. Returns an error if required fields are missing.
func NewAuditEntry(actorID, actorRole string, action AuditAction, entityType AuditEntityType, entityID string) (*AuditEntry, error) {
	if actorID == "" {
		return nil, ErrInvalidAuditActor
	}
	if action == "" {
		return nil, ErrInvalidAuditAction
	}
	if entityType == "" {
		return nil, ErrInvalidAuditEntityType
	}
	return &AuditEntry{
		ID:         uuid.NewString(),
		ActorID:    actorID,
		ActorRole:  actorRole,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// ── AuditFilter ───────────────────────────────────────────────────────────────

// AuditFilter defines optional criteria when querying the audit trail.
// Used by AuditRepository.QueryAuditTrail (defined in repository.go).
type AuditFilter struct {
	ActorID    string          // filter by user
	EntityType AuditEntityType // filter by entity type
	EntityID   string          // filter by specific entity UUID
	Action     AuditAction     // filter by action type
	From       *time.Time      // inclusive lower bound
	To         *time.Time      // inclusive upper bound
	PageSize   int             // 0 defaults to 50, max 200
	PageToken  string          // opaque cursor; empty means first page
}

// ── AsBuiltReport ─────────────────────────────────────────────────────────────
// AsBuiltReport is the "Dossier Industriel Numérique" (§13 — As-Built Record).
// It aggregates all traceability data for a given OF or serial number into a single
// structure that can be exported for audit purposes (EN9100 §8.5.2, §8.6).

// AsBuiltOperation summarises one operation within the As-Built record.
type AsBuiltOperation struct {
	OperationID            string
	StepNumber             int
	Name                   string
	Status                 OperationStatus
	OperatorID             string
	WorkstationID          string
	RequiresSignOff        bool
	SignedOffBy            string
	SignedOffAt            *time.Time
	IsSpecialProcess       bool
	NADCAPProcessCode      string
	PlannedDurationSeconds int
	ActualDurationSeconds  int
	StartedAt              *time.Time
	CompletedAt            *time.Time
	// Inline quality results recorded during this operation
	Measurements []*Measurement
	// Material lots consumed during this operation
	ConsumedLots []*ConsumptionRecord
	// Tools and gauges used during this operation
	Tools []AsBuiltTool
}

// AsBuiltTool is a lightweight summary of a tool used in the As-Built record.
type AsBuiltTool struct {
	ToolID           string
	SerialNumber     string
	Name             string
	CalibrationExpiry *time.Time
}

// AsBuiltReport is the complete, reconstructed production dossier for one OF.
type AsBuiltReport struct {
	GeneratedAt time.Time
	// Order summary
	OrderID       string
	Reference     string
	ProductID     string
	Quantity      int
	Status        OrderStatus
	IsFAI         bool
	FAIApprovedBy string
	FAIApprovedAt *time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	// Per-operation detail (ordered by StepNumber)
	Operations []*AsBuiltOperation
	// Top-level serial numbers produced by this OF
	SerialNumbers []*SerialNumber
}
