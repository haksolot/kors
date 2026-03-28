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
	return handler.New(orders, ops, store, reg, &log)
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

func (m *mockTxOps) InsertOutbox(ctx context.Context, entry domain.OutboxEntry) error {
	return m.Called(ctx, entry.EventType).Error(0)
}

// ── Sentinel errors used in tests ────────────────────────────────────────────

var errDB = errors.New("db error")
