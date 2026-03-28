package handler_test

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/haksolot/kors/services/qms/domain"
	"github.com/haksolot/kors/services/qms/handler"
)

// newTestHandler constructs a Handler wired with the given mocks and a no-op registry.
func newTestHandler(ncs *mockNCRepo, capas *mockCAPARepo, store *mockTransactor) *handler.Handler {
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	return handler.New(ncs, capas, store, reg, &log)
}

// ── NC repo mock ──────────────────────────────────────────────────────────────

type mockNCRepo struct{ mock.Mock }

func (m *mockNCRepo) FindNCByID(ctx context.Context, id string) (*domain.NonConformity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NonConformity), args.Error(1)
}

func (m *mockNCRepo) ListNCs(ctx context.Context, filter domain.ListNCsFilter) ([]*domain.NonConformity, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.NonConformity), args.Error(1)
}

// ── CAPA repo mock ────────────────────────────────────────────────────────────

type mockCAPARepo struct{ mock.Mock }

func (m *mockCAPARepo) FindCAPAByID(ctx context.Context, id string) (*domain.CAPA, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CAPA), args.Error(1)
}

func (m *mockCAPARepo) ListCAPAs(ctx context.Context, filter domain.ListCAPAsFilter) ([]*domain.CAPA, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CAPA), args.Error(1)
}

// ── Transactor mock ───────────────────────────────────────────────────────────

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

func (m *mockTxOps) SaveNC(ctx context.Context, nc *domain.NonConformity) error {
	return m.Called(ctx, nc).Error(0)
}

func (m *mockTxOps) UpdateNC(ctx context.Context, nc *domain.NonConformity) error {
	return m.Called(ctx, nc).Error(0)
}

func (m *mockTxOps) SaveCAPA(ctx context.Context, c *domain.CAPA) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockTxOps) UpdateCAPA(ctx context.Context, c *domain.CAPA) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockTxOps) InsertOutbox(ctx context.Context, entry domain.OutboxEntry) error {
	return m.Called(ctx, entry.EventType).Error(0)
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var errDB = errors.New("db error")
