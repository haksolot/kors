package handler_test

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
)

func newTestHandler(orders *mockOrderRepo, ops *mockOperationRepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	tools := &mockToolRepo{}
	tools.On("ListToolsByOperation", mock.Anything, mock.Anything).Return([]*domain.Tool{}, nil).Maybe()
	quality := &mockQualityRepo{}
	quality.On("ListCharacteristicsByOperation", mock.Anything, mock.Anything).Return([]*domain.ControlCharacteristic{}, nil).Maybe()
	quality.On("ListMeasurementsByOperation", mock.Anything, mock.Anything).Return([]*domain.Measurement{}, nil).Maybe()
	ops.On("FindOperationsByOFID", mock.Anything, mock.Anything).Return([]*domain.Operation{}, nil).Maybe()
	return handler.New(orders, ops, &mockTraceabilityRepo{}, &mockRoutingRepo{}, &mockQualificationRepo{}, &mockWorkstationRepo{}, &mockTimeTrackingRepo{}, tools, &mockMaterialRepo{}, quality, &mockAlertRepo{}, &mockAuditRepo{}, &mockComplianceRepo{}, store, reg, &log)
}

// newTestHandlerWithTrace is like newTestHandler but with an explicit trace repo mock.
func newTestHandlerWithTrace(orders *mockOrderRepo, ops *mockOperationRepo, trace *mockTraceabilityRepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	tools := &mockToolRepo{}
	tools.On("ListToolsByOperation", mock.Anything, mock.Anything).Return([]*domain.Tool{}, nil).Maybe()
	quality := &mockQualityRepo{}
	quality.On("ListCharacteristicsByOperation", mock.Anything, mock.Anything).Return([]*domain.ControlCharacteristic{}, nil).Maybe()
	quality.On("ListMeasurementsByOperation", mock.Anything, mock.Anything).Return([]*domain.Measurement{}, nil).Maybe()
	ops.On("FindOperationsByOFID", mock.Anything, mock.Anything).Return([]*domain.Operation{}, nil).Maybe()
	return handler.New(orders, ops, trace, &mockRoutingRepo{}, &mockQualificationRepo{}, &mockWorkstationRepo{}, &mockTimeTrackingRepo{}, tools, &mockMaterialRepo{}, quality, &mockAlertRepo{}, &mockAuditRepo{}, &mockComplianceRepo{}, store, reg, &log)
}

// newTestHandlerWithQuals is like newTestHandler but with an explicit qualification repo mock.
func newTestHandlerWithQuals(orders *mockOrderRepo, ops *mockOperationRepo, quals *mockQualificationRepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	tools := &mockToolRepo{}
	tools.On("ListToolsByOperation", mock.Anything, mock.Anything).Return([]*domain.Tool{}, nil).Maybe()
	quality := &mockQualityRepo{}
	quality.On("ListCharacteristicsByOperation", mock.Anything, mock.Anything).Return([]*domain.ControlCharacteristic{}, nil).Maybe()
	quality.On("ListMeasurementsByOperation", mock.Anything, mock.Anything).Return([]*domain.Measurement{}, nil).Maybe()
	ops.On("FindOperationsByOFID", mock.Anything, mock.Anything).Return([]*domain.Operation{}, nil).Maybe()
	return handler.New(orders, ops, &mockTraceabilityRepo{}, &mockRoutingRepo{}, quals, &mockWorkstationRepo{}, &mockTimeTrackingRepo{}, tools, &mockMaterialRepo{}, quality, &mockAlertRepo{}, &mockAuditRepo{}, &mockComplianceRepo{}, store, reg, &log)
}

// ── Order repo mock ───────────────────────────────────────────────────────────

type mockOrderRepo struct{ mock.Mock }

func (m *mockOrderRepo) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}

func (m *mockOrderRepo) FindByReference(ctx context.Context, ref string) (*domain.Order, error) {
	args := m.Called(ctx, ref)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Order), args.Error(1)
}

func (m *mockOrderRepo) List(ctx context.Context, f domain.ListOrdersFilter) ([]*domain.Order, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Order), args.Error(1)
}

func (m *mockOrderRepo) DispatchList(ctx context.Context, limit int) ([]*domain.Order, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Order), args.Error(1)
}

// ── Routing repo mock ─────────────────────────────────────────────────────────

type mockRoutingRepo struct{ mock.Mock }

func (m *mockRoutingRepo) FindRoutingByID(ctx context.Context, id string) (*domain.Routing, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Routing), args.Error(1)
}

func (m *mockRoutingRepo) FindRoutingsByProductID(ctx context.Context, productID string) ([]*domain.Routing, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Routing), args.Error(1)
}

// ── Operation repo mock ───────────────────────────────────────────────────────

type mockOperationRepo struct{ mock.Mock }

func (m *mockOperationRepo) FindOperationByID(ctx context.Context, id string) (*domain.Operation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Operation), args.Error(1)
}

func (m *mockOperationRepo) FindOperationsByOFID(ctx context.Context, ofID string) ([]*domain.Operation, error) {
	args := m.Called(ctx, ofID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Operation), args.Error(1)
}

// ── Traceability repo mock ────────────────────────────────────────────────────

type mockTraceabilityRepo struct{ mock.Mock }

func (m *mockTraceabilityRepo) FindLotByID(ctx context.Context, id string) (*domain.Lot, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lot), args.Error(1)
}

func (m *mockTraceabilityRepo) FindSNByID(ctx context.Context, id string) (*domain.SerialNumber, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SerialNumber), args.Error(1)
}

func (m *mockTraceabilityRepo) FindSNBySN(ctx context.Context, sn string) (*domain.SerialNumber, error) {
	args := m.Called(ctx, sn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SerialNumber), args.Error(1)
}

func (m *mockTraceabilityRepo) GetGenealogyByParentSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error) {
	args := m.Called(ctx, snID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.GenealogyEntry), args.Error(1)
}

func (m *mockTraceabilityRepo) GetGenealogyByChildSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error) {
	args := m.Called(ctx, snID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.GenealogyEntry), args.Error(1)
}

// ── Transactor mock ───────────────────────────────────────────────────────────

// mockTransactor executes fn immediately with a mockTxOps.
// Tests configure expected calls on mockTxOps via the Ops field.
type mockTransactor struct {
	mock.Mock
	Ops *mockTxOps
}

func newMockTransactor() *mockTransactor {
	return &mockTransactor{Ops: &mockTxOps{}}
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(domain.TxOps) error) error {
	args := m.Called(ctx)
	if args.Error(0) != nil {
		return args.Error(0)
	}
	return fn(m.Ops)
}

// ── TxOps mock ────────────────────────────────────────────────────────────────

type mockTxOps struct{ mock.Mock }

func (m *mockTxOps) SaveOrder(ctx context.Context, o *domain.Order) error {
	return m.Called(ctx, o).Error(0)
}

func (m *mockTxOps) UpdateOrder(ctx context.Context, o *domain.Order) error {
	return m.Called(ctx, o).Error(0)
}

func (m *mockTxOps) SaveOperation(ctx context.Context, op *domain.Operation) error {
	return m.Called(ctx, op).Error(0)
}

func (m *mockTxOps) UpdateOperation(ctx context.Context, op *domain.Operation) error {
	return m.Called(ctx, op).Error(0)
}

func (m *mockTxOps) SaveRouting(ctx context.Context, r *domain.Routing) error {
	return m.Called(ctx, r).Error(0)
}

func (m *mockTxOps) SaveRoutingStep(ctx context.Context, step *domain.RoutingStep) error {
	return m.Called(ctx, step).Error(0)
}

func (m *mockTxOps) SaveLot(ctx context.Context, l *domain.Lot) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockTxOps) UpdateLot(ctx context.Context, l *domain.Lot) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockTxOps) SaveSerialNumber(ctx context.Context, sn *domain.SerialNumber) error {
	return m.Called(ctx, sn).Error(0)
}

func (m *mockTxOps) UpdateSerialNumber(ctx context.Context, sn *domain.SerialNumber) error {
	return m.Called(ctx, sn).Error(0)
}

func (m *mockTxOps) SaveGenealogyEntry(ctx context.Context, e *domain.GenealogyEntry) error {
	return m.Called(ctx, e).Error(0)
}

func (m *mockTxOps) InsertOutbox(ctx context.Context, entry domain.OutboxEntry) error {
	return m.Called(ctx, entry.EventType).Error(0)
}

func (m *mockTxOps) SaveQualification(ctx context.Context, q *domain.Qualification) error {
	return m.Called(ctx, q).Error(0)
}

func (m *mockTxOps) UpdateQualification(ctx context.Context, q *domain.Qualification) error {
	return m.Called(ctx, q).Error(0)
}

func (m *mockTxOps) SaveWorkstation(ctx context.Context, w *domain.Workstation) error {
	return m.Called(ctx, w).Error(0)
}

func (m *mockTxOps) UpdateWorkstation(ctx context.Context, w *domain.Workstation) error {
	return m.Called(ctx, w).Error(0)
}

func (m *mockTxOps) SaveTimeLog(ctx context.Context, l *domain.TimeLog) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockTxOps) SaveDowntimeEvent(ctx context.Context, d *domain.DowntimeEvent) error {
	return m.Called(ctx, d).Error(0)
}

func (m *mockTxOps) UpdateDowntimeEvent(ctx context.Context, d *domain.DowntimeEvent) error {
	return m.Called(ctx, d).Error(0)
}

func (m *mockTxOps) SaveTool(ctx context.Context, t *domain.Tool) error {
	return m.Called(ctx, t).Error(0)
}

func (m *mockTxOps) UpdateTool(ctx context.Context, t *domain.Tool) error {
	return m.Called(ctx, t).Error(0)
}

func (m *mockTxOps) LinkToolToOperation(ctx context.Context, operationID, toolID string) error {
	return m.Called(ctx, operationID, toolID).Error(0)
}

func (m *mockTxOps) SaveConsumptionRecord(ctx context.Context, r *domain.ConsumptionRecord) error {
	return m.Called(ctx, r).Error(0)
}

func (m *mockTxOps) SaveTOEExposureLog(ctx context.Context, l *domain.TOEExposureLog) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockTxOps) UpdateTOEExposureLog(ctx context.Context, l *domain.TOEExposureLog) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockTxOps) SaveLocationTransfer(ctx context.Context, t *domain.LocationTransfer) error {
	return m.Called(ctx, t).Error(0)
}

func (m *mockTxOps) SaveControlCharacteristic(ctx context.Context, c *domain.ControlCharacteristic) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockTxOps) SaveMeasurement(ctx context.Context, meas *domain.Measurement) error {
	return m.Called(ctx, meas).Error(0)
}

func (m *mockTxOps) SaveAlert(ctx context.Context, a *domain.Alert) error {
	return m.Called(ctx, a).Error(0)
}

func (m *mockTxOps) UpdateAlert(ctx context.Context, a *domain.Alert) error {
	return m.Called(ctx, a).Error(0)
}

func (m *mockTxOps) AppendAuditEntry(ctx context.Context, e *domain.AuditEntry) error {
	return m.Called(ctx, e).Error(0)
}

// ── Qualification repo mock ───────────────────────────────────────────────────

type mockQualificationRepo struct{ mock.Mock }

func (m *mockQualificationRepo) FindQualificationByID(ctx context.Context, id string) (*domain.Qualification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Qualification), args.Error(1)
}

func (m *mockQualificationRepo) ListQualificationsByOperator(ctx context.Context, operatorID string) ([]*domain.Qualification, error) {
	args := m.Called(ctx, operatorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Qualification), args.Error(1)
}

func (m *mockQualificationRepo) ListActiveSkills(ctx context.Context, operatorID string, now time.Time) ([]string, error) {
	args := m.Called(ctx, operatorID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockQualificationRepo) ListExpiringQualifications(ctx context.Context, warningDays int, now time.Time) ([]*domain.Qualification, error) {
	args := m.Called(ctx, warningDays, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Qualification), args.Error(1)
}

// ── Sentinel errors used in tests ────────────────────────────────────────────

var errDB = errors.New("db error")

// ── Workstation repo mock ─────────────────────────────────────────────────────

type mockWorkstationRepo struct{ mock.Mock }

func (m *mockWorkstationRepo) FindWorkstationByID(ctx context.Context, id string) (*domain.Workstation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Workstation), args.Error(1)
}

func (m *mockWorkstationRepo) ListWorkstations(ctx context.Context, limit, offset int) ([]*domain.Workstation, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Workstation), args.Error(1)
}

// ── Time Tracking repo mock ───────────────────────────────────────────────────

type mockTimeTrackingRepo struct{ mock.Mock }

func (m *mockTimeTrackingRepo) FindDowntimeByID(ctx context.Context, id string) (*domain.DowntimeEvent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DowntimeEvent), args.Error(1)
}

func (m *mockTimeTrackingRepo) FindOngoingDowntime(ctx context.Context, workstationID string) (*domain.DowntimeEvent, error) {
	args := m.Called(ctx, workstationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DowntimeEvent), args.Error(1)
}

func (m *mockTimeTrackingRepo) ListTimeLogsByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.TimeLog, error) {
	args := m.Called(ctx, workstationID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TimeLog), args.Error(1)
}

func (m *mockTimeTrackingRepo) ListDowntimesByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.DowntimeEvent, error) {
	args := m.Called(ctx, workstationID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DowntimeEvent), args.Error(1)
}

// ── Tool repo mock ────────────────────────────────────────────────────────────

type mockToolRepo struct{ mock.Mock }

func (m *mockToolRepo) FindToolByID(ctx context.Context, id string) (*domain.Tool, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tool), args.Error(1)
}

func (m *mockToolRepo) FindToolBySerialNumber(ctx context.Context, sn string) (*domain.Tool, error) {
	args := m.Called(ctx, sn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tool), args.Error(1)
}

func (m *mockToolRepo) ListTools(ctx context.Context, limit, offset int) ([]*domain.Tool, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tool), args.Error(1)
}

func (m *mockToolRepo) ListToolsByOperation(ctx context.Context, operationID string) ([]*domain.Tool, error) {
	args := m.Called(ctx, operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tool), args.Error(1)
}

// ── Material repo mock ────────────────────────────────────────────────────────

type mockMaterialRepo struct{ mock.Mock }

func (m *mockMaterialRepo) FindOngoingTOEExposure(ctx context.Context, lotID string) (*domain.TOEExposureLog, error) {
	args := m.Called(ctx, lotID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TOEExposureLog), args.Error(1)
}

func (m *mockMaterialRepo) ListConsumptionsByOperation(ctx context.Context, operationID string) ([]*domain.ConsumptionRecord, error) {
	args := m.Called(ctx, operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ConsumptionRecord), args.Error(1)
}

func (m *mockMaterialRepo) ListTransfersByEntity(ctx context.Context, entityID string) ([]*domain.LocationTransfer, error) {
	args := m.Called(ctx, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LocationTransfer), args.Error(1)
}

// ── Quality repo mock ─────────────────────────────────────────────────────────

type mockQualityRepo struct{ mock.Mock }

func (m *mockQualityRepo) FindCharacteristicByID(ctx context.Context, id string) (*domain.ControlCharacteristic, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ControlCharacteristic), args.Error(1)
}

func (m *mockQualityRepo) ListCharacteristicsByStep(ctx context.Context, stepID string) ([]*domain.ControlCharacteristic, error) {
	args := m.Called(ctx, stepID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ControlCharacteristic), args.Error(1)
}

func (m *mockQualityRepo) ListCharacteristicsByOperation(ctx context.Context, operationID string) ([]*domain.ControlCharacteristic, error) {
	args := m.Called(ctx, operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ControlCharacteristic), args.Error(1)
}

func (m *mockQualityRepo) ListMeasurementsByOperation(ctx context.Context, operationID string) ([]*domain.Measurement, error) {
	args := m.Called(ctx, operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Measurement), args.Error(1)
}

func (m *mockQualityRepo) ListMeasurementsByCharacteristic(ctx context.Context, characteristicID string, limit int) ([]*domain.Measurement, error) {
	args := m.Called(ctx, characteristicID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Measurement), args.Error(1)
}

// ── Alert repo mock ───────────────────────────────────────────────────────────

type mockAlertRepo struct{ mock.Mock }

func (m *mockAlertRepo) FindAlertByID(ctx context.Context, id string) (*domain.Alert, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *mockAlertRepo) ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

// ── Audit repo mock (§13) ─────────────────────────────────────────────────────

type mockAuditRepo struct{ mock.Mock }

func (m *mockAuditRepo) QueryAuditTrail(ctx context.Context, f domain.AuditFilter) ([]*domain.AuditEntry, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditEntry), args.Error(1)
}

// ── Compliance repo mock (§13) ────────────────────────────────────────────────

type mockComplianceRepo struct{ mock.Mock }

func (m *mockComplianceRepo) GetAsBuiltByOFID(ctx context.Context, ofID string) (*domain.AsBuiltReport, error) {
	args := m.Called(ctx, ofID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AsBuiltReport), args.Error(1)
}
