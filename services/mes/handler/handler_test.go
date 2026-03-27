package handler_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
)

func newTestHandler(orders *mockOrderRepo, ops *mockOperationRepo) *handler.Handler {
	log := zerolog.Nop()
	return handler.New(orders, ops, &log)
}

// ── CreateOrder ───────────────────────────────────────────────────────────────

func TestHandler_CreateOrder(t *testing.T) {
	tests := []struct {
		name      string
		req       *mes.CreateOrderRequest
		setupMock func(*mockOrderRepo)
		wantErr   bool
	}{
		{
			name: "valid request creates order and saves it",
			req: &mes.CreateOrderRequest{
				Reference: "OF-001", ProductId: "prod-1", Quantity: 10,
			},
			setupMock: func(m *mockOrderRepo) {
				m.On("Save", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
			},
		},
		{
			name: "empty reference returns error",
			req: &mes.CreateOrderRequest{
				Reference: "", ProductId: "prod-1", Quantity: 10,
			},
			setupMock: func(_ *mockOrderRepo) {},
			wantErr:   true,
		},
		{
			name: "zero quantity returns error",
			req: &mes.CreateOrderRequest{
				Reference: "OF-002", ProductId: "prod-1", Quantity: 0,
			},
			setupMock: func(_ *mockOrderRepo) {},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orders := &mockOrderRepo{}
			ops := &mockOperationRepo{}
			tc.setupMock(orders)
			h := newTestHandler(orders, ops)

			payload, err := proto.Marshal(tc.req)
			require.NoError(t, err)

			resp, err := h.CreateOrder(context.Background(), payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)

			var response mes.CreateOrderResponse
			require.NoError(t, proto.Unmarshal(resp, &response))
			assert.Equal(t, tc.req.Reference, response.Order.Reference)
			assert.Equal(t, mes.OrderStatus_ORDER_STATUS_PLANNED, response.Order.Status)
			orders.AssertExpectations(t)
		})
	}
}

// ── GetOrder ──────────────────────────────────────────────────────────────────

func TestHandler_GetOrder(t *testing.T) {
	t.Run("existing order is returned", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		h := newTestHandler(orders, &mockOperationRepo{})
		req := &mes.GetOrderRequest{Id: order.ID}
		payload, _ := proto.Marshal(req)

		resp, err := h.GetOrder(context.Background(), payload)
		require.NoError(t, err)

		var response mes.GetOrderResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, order.ID, response.Order.Id)
		orders.AssertExpectations(t)
	})

	t.Run("not found returns error", func(t *testing.T) {
		orders := &mockOrderRepo{}
		orders.On("FindByID", mock.Anything, "missing-id").
			Return(nil, domain.ErrOrderNotFound)

		h := newTestHandler(orders, &mockOperationRepo{})
		req := &mes.GetOrderRequest{Id: "missing-id"}
		payload, _ := proto.Marshal(req)

		_, err := h.GetOrder(context.Background(), payload)
		require.Error(t, err)
		orders.AssertExpectations(t)
	})
}

// ── StartOperation ────────────────────────────────────────────────────────────

func TestHandler_StartOperation(t *testing.T) {
	t.Run("pending operation can be started", func(t *testing.T) {
		orders := &mockOrderRepo{}
		ops := &mockOperationRepo{}

		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		_ = order.Start()
		op, _ := domain.NewOperation(order.ID, 1, "Découpe")

		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)
		ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)

		h := newTestHandler(orders, ops)
		req := &mes.StartOperationRequest{OperationId: op.ID, OperatorId: "operator-1"}
		payload, _ := proto.Marshal(req)

		resp, err := h.StartOperation(context.Background(), payload)
		require.NoError(t, err)

		var response mes.StartOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, mes.OperationStatus_OPERATION_STATUS_IN_PROGRESS, response.Operation.Status)
		ops.AssertExpectations(t)
	})
}
