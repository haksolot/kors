package handler_test

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
)

// newTestHandler constructs a Handler wired with the given mocks and a no-op registry.
func newTestHandler(orders *mockOrderRepo, ops *mockOperationRepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	return handler.New(orders, ops, &mockTraceabilityRepo{}, &mockRoutingRepo{}, store, reg, &log)
}

// newTestHandlerWithTrace is like newTestHandler but with an explicit trace repo mock.
func newTestHandlerWithTrace(orders *mockOrderRepo, ops *mockOperationRepo, trace *mockTraceabilityRepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	return handler.New(orders, ops, trace, &mockRoutingRepo{}, store, reg, &log)
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

// ── Sentinel errors used in tests ────────────────────────────────────────────

var errDB = errors.New("db error")
