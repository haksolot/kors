package handler_test

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/haksolot/kors/services/mes/domain"
)

// mockOrderRepo is a testify mock for domain.OrderRepository.
type mockOrderRepo struct{ mock.Mock }

func (m *mockOrderRepo) Save(ctx context.Context, o *domain.Order) error {
	return m.Called(ctx, o).Error(0)
}
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
func (m *mockOrderRepo) Update(ctx context.Context, o *domain.Order) error {
	return m.Called(ctx, o).Error(0)
}
func (m *mockOrderRepo) List(ctx context.Context, f domain.ListOrdersFilter) ([]*domain.Order, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Order), args.Error(1)
}

// mockOperationRepo is a testify mock for handler.OperationRepository.
type mockOperationRepo struct{ mock.Mock }

func (m *mockOperationRepo) SaveOperation(ctx context.Context, op *domain.Operation) error {
	return m.Called(ctx, op).Error(0)
}
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
func (m *mockOperationRepo) UpdateOperation(ctx context.Context, op *domain.Operation) error {
	return m.Called(ctx, op).Error(0)
}
