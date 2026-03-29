package domain

import (
	"context"
	"time"
)

// OrderRepository defines the read-only persistence contract for ManufacturingOrders.
// Write operations (create/update) must go through Transactor to ensure atomicity with outbox.
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*Order, error)
	FindByReference(ctx context.Context, reference string) (*Order, error)
	List(ctx context.Context, filter ListOrdersFilter) ([]*Order, error)
	DispatchList(ctx context.Context, limit int) ([]*Order, error)
}

// RoutingRepository defines read-only persistence for Routing templates.
type RoutingRepository interface {
	FindRoutingByID(ctx context.Context, id string) (*Routing, error)
	FindRoutingsByProductID(ctx context.Context, productID string) ([]*Routing, error)
}

// OperationRepository defines the read-only persistence contract for Operations.
// Write operations must go through Transactor to ensure atomicity with outbox.
type OperationRepository interface {
	FindOperationByID(ctx context.Context, id string) (*Operation, error)
	FindOperationsByOFID(ctx context.Context, ofID string) ([]*Operation, error)
}

// LotRepository defines read-only persistence for Lots.
type LotRepository interface {
	FindLotByID(ctx context.Context, id string) (*Lot, error)
}

// TraceabilityRepository defines read-only persistence for serial numbers and genealogy.
type TraceabilityRepository interface {
	FindSNByID(ctx context.Context, id string) (*SerialNumber, error)
	FindSNBySN(ctx context.Context, sn string) (*SerialNumber, error)
	GetGenealogyByParentSN(ctx context.Context, snID string) ([]*GenealogyEntry, error)
	GetGenealogyByChildSN(ctx context.Context, snID string) ([]*GenealogyEntry, error)
}

// QualificationRepository defines read-only persistence for operator qualifications (AS9100D §7.2).
// Write operations (create/renew/revoke) must go through Transactor to ensure atomicity with outbox.
type QualificationRepository interface {
	FindQualificationByID(ctx context.Context, id string) (*Qualification, error)
	ListQualificationsByOperator(ctx context.Context, operatorID string) ([]*Qualification, error)
	// ListActiveSkills returns the skill strings for all non-revoked, non-expired qualifications
	// for the given operator at time now. This is the hot path for the StartOperation interlock.
	ListActiveSkills(ctx context.Context, operatorID string, now time.Time) ([]string, error)
	// ListExpiringQualifications returns qualifications expiring within warningDays from now.
	ListExpiringQualifications(ctx context.Context, warningDays int, now time.Time) ([]*Qualification, error)
}

// WorkstationRepository defines read-only persistence for workstations (BLOC 6).
// Write operations (create/update status) must go through Transactor to ensure atomicity with outbox.
type WorkstationRepository interface {
	FindWorkstationByID(ctx context.Context, id string) (*Workstation, error)
	ListWorkstations(ctx context.Context, limit, offset int) ([]*Workstation, error)
}

// TimeTrackingRepository defines read-only persistence for time logs and downtimes (BLOC 5).
type TimeTrackingRepository interface {
	FindDowntimeByID(ctx context.Context, id string) (*DowntimeEvent, error)
	FindOngoingDowntime(ctx context.Context, workstationID string) (*DowntimeEvent, error)
	ListTimeLogsByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*TimeLog, error)
	ListDowntimesByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*DowntimeEvent, error)
}

// ToolRepository defines read-only persistence for tools and gauges (BLOC 8).
type ToolRepository interface {
	FindToolByID(ctx context.Context, id string) (*Tool, error)
	FindToolBySerialNumber(ctx context.Context, sn string) (*Tool, error)
	ListTools(ctx context.Context, limit, offset int) ([]*Tool, error)
	ListToolsByOperation(ctx context.Context, operationID string) ([]*Tool, error)
}

// MaterialRepository defines read-only persistence for material tracking (BLOC 9).
type MaterialRepository interface {
	FindOngoingTOEExposure(ctx context.Context, lotID string) (*TOEExposureLog, error)
	ListConsumptionsByOperation(ctx context.Context, operationID string) ([]*ConsumptionRecord, error)
	ListTransfersByEntity(ctx context.Context, entityID string) ([]*LocationTransfer, error)
}

// QualityRepository defines read-only persistence for inline quality (BLOC 10).
type QualityRepository interface {
	FindCharacteristicByID(ctx context.Context, id string) (*ControlCharacteristic, error)
	ListCharacteristicsByStep(ctx context.Context, stepID string) ([]*ControlCharacteristic, error)
	ListCharacteristicsByOperation(ctx context.Context, operationID string) ([]*ControlCharacteristic, error)
	ListMeasurementsByOperation(ctx context.Context, operationID string) ([]*Measurement, error)
	ListMeasurementsByCharacteristic(ctx context.Context, characteristicID string, limit int) ([]*Measurement, error)
}

// TxOps defines all write operations available within a database transaction.
// Every mutation that triggers a domain event must use TxOps so the outbox entry
// is written in the same transaction as the business data (ADR-004).
type TxOps interface {
	SaveOrder(ctx context.Context, o *Order) error
	UpdateOrder(ctx context.Context, o *Order) error
	SaveOperation(ctx context.Context, op *Operation) error
	UpdateOperation(ctx context.Context, op *Operation) error
	// Routing writes
	SaveRouting(ctx context.Context, r *Routing) error
	SaveRoutingStep(ctx context.Context, step *RoutingStep) error
	// Traceability writes
	SaveLot(ctx context.Context, l *Lot) error
	UpdateLot(ctx context.Context, l *Lot) error
	SaveSerialNumber(ctx context.Context, sn *SerialNumber) error
	UpdateSerialNumber(ctx context.Context, sn *SerialNumber) error
	SaveGenealogyEntry(ctx context.Context, e *GenealogyEntry) error
	InsertOutbox(ctx context.Context, entry OutboxEntry) error
	// Qualification writes (AS9100D §7.2)
	SaveQualification(ctx context.Context, q *Qualification) error
	UpdateQualification(ctx context.Context, q *Qualification) error
	// Workstation writes (BLOC 6)
	SaveWorkstation(ctx context.Context, w *Workstation) error
	UpdateWorkstation(ctx context.Context, w *Workstation) error
	// Time tracking writes (BLOC 5)
	SaveTimeLog(ctx context.Context, l *TimeLog) error
	SaveDowntimeEvent(ctx context.Context, d *DowntimeEvent) error
	UpdateDowntimeEvent(ctx context.Context, d *DowntimeEvent) error
	// Tool writes (BLOC 8)
	SaveTool(ctx context.Context, t *Tool) error
	UpdateTool(ctx context.Context, t *Tool) error
	LinkToolToOperation(ctx context.Context, operationID, toolID string) error
	// Material writes (BLOC 9)
	SaveConsumptionRecord(ctx context.Context, r *ConsumptionRecord) error
	SaveTOEExposureLog(ctx context.Context, l *TOEExposureLog) error
	UpdateTOEExposureLog(ctx context.Context, l *TOEExposureLog) error
	SaveLocationTransfer(ctx context.Context, t *LocationTransfer) error
	// Quality writes (BLOC 10)
	SaveControlCharacteristic(ctx context.Context, c *ControlCharacteristic) error
	SaveMeasurement(ctx context.Context, m *Measurement) error
}

// Transactor manages database transactions and exposes TxOps within them.
// Implementations must begin a transaction, call fn, then commit or rollback.
type Transactor interface {
	WithTx(ctx context.Context, fn func(TxOps) error) error
}

// OutboxEntry holds a single unpublished event from the outbox table.
type OutboxEntry struct {
	ID        int64
	EventType string
	Subject   string
	Payload   []byte
}

// ListOrdersFilter defines optional filtering criteria for Order listings.
type ListOrdersFilter struct {
	Status    *OrderStatus // nil means all statuses
	PageSize  int          // 0 defaults to 50, max 100
	PageToken string       // opaque cursor; empty means first page
}
