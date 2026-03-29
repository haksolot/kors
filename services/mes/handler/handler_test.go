package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
)

// ── CreateOrder ───────────────────────────────────────────────────────────────

func TestHandler_CreateOrder(t *testing.T) {
	tests := []struct {
		name      string
		req       *pbmes.CreateOrderRequest
		setupMock func(*mockTransactor)
		wantErr   bool
	}{
		{
			name: "valid request creates order and writes outbox",
			req:  &pbmes.CreateOrderRequest{Reference: "OF-001", ProductId: "prod-1", Quantity: 10},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(nil)
				st.Ops.On("SaveOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
				st.Ops.On("InsertOutbox", mock.Anything, "OFCreated").Return(nil)
			},
		},
		{
			name:      "empty reference returns error before tx",
			req:       &pbmes.CreateOrderRequest{Reference: "", ProductId: "prod-1", Quantity: 10},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
		{
			name:      "zero quantity returns error before tx",
			req:       &pbmes.CreateOrderRequest{Reference: "OF-002", ProductId: "prod-1", Quantity: 0},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
		{
			name: "db error is propagated",
			req:  &pbmes.CreateOrderRequest{Reference: "OF-003", ProductId: "prod-1", Quantity: 5},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(errDB)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockTransactor()
			tc.setupMock(store)
			h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)

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

			var response pbmes.CreateOrderResponse
			require.NoError(t, proto.Unmarshal(resp, &response))
			assert.Equal(t, tc.req.Reference, response.Order.Reference)
			assert.Equal(t, pbmes.OrderStatus_ORDER_STATUS_PLANNED, response.Order.Status)
			store.AssertExpectations(t)
			store.Ops.AssertExpectations(t)
		})
	}
}

// ── GetOrder ──────────────────────────────────────────────────────────────────

func TestHandler_GetOrder(t *testing.T) {
	t.Run("existing order is returned", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		h := newTestHandler(orders, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.GetOrderRequest{Id: order.ID})

		resp, err := h.GetOrder(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.GetOrderResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, order.ID, response.Order.Id)
		orders.AssertExpectations(t)
	})

	t.Run("not found returns error", func(t *testing.T) {
		orders := &mockOrderRepo{}
		orders.On("FindByID", mock.Anything, "missing-id").Return(nil, domain.ErrOrderNotFound)

		h := newTestHandler(orders, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.GetOrderRequest{Id: "missing-id"})

		_, err := h.GetOrder(context.Background(), payload)
		require.Error(t, err)
		orders.AssertExpectations(t)
	})
}

// ── SuspendOrder ──────────────────────────────────────────────────────────────

func TestHandler_SuspendOrder(t *testing.T) {
	t.Run("in-progress order can be suspended", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		_ = order.Start()
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFSuspended").Return(nil)

		h := newTestHandler(orders, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.SuspendOrderRequest{Id: order.ID, Reason: "shortage"})

		resp, err := h.SuspendOrder(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.SuspendOrderResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OrderStatus_ORDER_STATUS_SUSPENDED, response.Order.Status)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})

	t.Run("planned order cannot be suspended", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-002", "prod-1", 10)
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		h := newTestHandler(orders, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.SuspendOrderRequest{Id: order.ID, Reason: "test"})

		_, err := h.SuspendOrder(context.Background(), payload)
		require.Error(t, err)
	})
}

// ── ResumeOrder ───────────────────────────────────────────────────────────────

func TestHandler_ResumeOrder(t *testing.T) {
	t.Run("suspended order can be resumed", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		_ = order.Start()
		_ = order.Suspend("shortage")
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFResumed").Return(nil)

		h := newTestHandler(orders, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.ResumeOrderRequest{Id: order.ID})

		resp, err := h.ResumeOrder(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.ResumeOrderResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OrderStatus_ORDER_STATUS_IN_PROGRESS, response.Order.Status)
		store.AssertExpectations(t)
	})
}

// ── CancelOrder ───────────────────────────────────────────────────────────────

func TestHandler_CancelOrder(t *testing.T) {
	t.Run("planned order can be cancelled", func(t *testing.T) {
		orders := &mockOrderRepo{}
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		orders.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFCancelled").Return(nil)

		h := newTestHandler(orders, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.CancelOrderRequest{Id: order.ID, Reason: "obsolete"})

		resp, err := h.CancelOrder(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.CancelOrderResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OrderStatus_ORDER_STATUS_CANCELLED, response.Order.Status)
	})
}

// ── CreateOperation ───────────────────────────────────────────────────────────

func TestHandler_CreateOperation(t *testing.T) {
	tests := []struct {
		name      string
		req       *pbmes.CreateOperationRequest
		setupMock func(*mockTransactor)
		wantErr   bool
	}{
		{
			name: "valid request creates operation",
			req:  &pbmes.CreateOperationRequest{OfId: "of-1", StepNumber: 1, Name: "Découpe"},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(nil)
				st.Ops.On("SaveOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
			},
		},
		{
			name:      "empty name returns error",
			req:       &pbmes.CreateOperationRequest{OfId: "of-1", StepNumber: 1, Name: ""},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
		{
			name:      "zero step number returns error",
			req:       &pbmes.CreateOperationRequest{OfId: "of-1", StepNumber: 0, Name: "Découpe"},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockTransactor()
			tc.setupMock(store)
			h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)

			payload, _ := proto.Marshal(tc.req)
			resp, err := h.CreateOperation(context.Background(), payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)

			var response pbmes.CreateOperationResponse
			require.NoError(t, proto.Unmarshal(resp, &response))
			assert.Equal(t, tc.req.Name, response.Operation.Name)
			assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_PENDING, response.Operation.Status)
			store.AssertExpectations(t)
			store.Ops.AssertExpectations(t)
		})
	}
}

// ── GetOperation ──────────────────────────────────────────────────────────────

func TestHandler_GetOperation(t *testing.T) {
	t.Run("existing operation is returned", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op, _ := domain.NewOperation("of-1", 1, "Découpe")
		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		h := newTestHandler(&mockOrderRepo{}, ops, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.GetOperationRequest{Id: op.ID})

		resp, err := h.GetOperation(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.GetOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, op.ID, response.Operation.Id)
		ops.AssertExpectations(t)
	})
}

// ── ListOperations ────────────────────────────────────────────────────────────

func TestHandler_ListOperations(t *testing.T) {
	t.Run("returns all operations for an OF", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op1, _ := domain.NewOperation("of-1", 1, "Découpe")
		op2, _ := domain.NewOperation("of-1", 2, "Soudure")
		ops.On("FindOperationsByOFID", mock.Anything, "of-1").Return([]*domain.Operation{op1, op2}, nil)

		h := newTestHandler(&mockOrderRepo{}, ops, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.ListOperationsRequest{OfId: "of-1"})

		resp, err := h.ListOperations(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.ListOperationsResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Len(t, response.Operations, 2)
		ops.AssertExpectations(t)
	})
}

// ── StartOperation ────────────────────────────────────────────────────────────

func TestHandler_StartOperation(t *testing.T) {
	t.Run("pending operation can be started and order status updated", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op, _ := domain.NewOperation("of-1", 1, "Découpe")
		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		order, _ := domain.NewOrder("OF-1", "prod-1", 10)
		order.ID = "of-1"
		orders := &mockOrderRepo{}
		orders.On("FindByID", mock.Anything, "of-1").Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OperationStarted").Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFStarted").Return(nil)

		h := newTestHandler(orders, ops, store)
		payload, _ := proto.Marshal(&pbmes.StartOperationRequest{OperationId: op.ID, OperatorId: "op-1"})

		resp, err := h.StartOperation(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.StartOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_IN_PROGRESS, response.Operation.Status)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
		orders.AssertExpectations(t)
	})
}

// ── CompleteOperation ─────────────────────────────────────────────────────────

func TestHandler_CompleteOperation(t *testing.T) {
	t.Run("in-progress operation can be completed and order status updated", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op, _ := domain.NewOperation("of-1", 1, "Découpe")
		_ = op.Start("op-1", nil)
		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)
		// Mock FindOperationsByOFID returning only this operation (so allDone is true)
		ops.On("FindOperationsByOFID", mock.Anything, "of-1").Return([]*domain.Operation{op}, nil)

		order, _ := domain.NewOrder("OF-1", "prod-1", 10)
		order.ID = "of-1"
		_ = order.Start()
		orders := &mockOrderRepo{}
		orders.On("FindByID", mock.Anything, "of-1").Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OperationCompleted").Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFCompleted").Return(nil)

		h := newTestHandler(orders, ops, store)
		payload, _ := proto.Marshal(&pbmes.CompleteOperationRequest{OperationId: op.ID, OperatorId: "op-1"})

		resp, err := h.CompleteOperation(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.CompleteOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_COMPLETED, response.Operation.Status)
		store.AssertExpectations(t)
		ops.AssertExpectations(t)
		orders.AssertExpectations(t)
	})
}

// ── SkipOperation ─────────────────────────────────────────────────────────────

func TestHandler_SkipOperation(t *testing.T) {
	t.Run("pending operation can be skipped with reason", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op, _ := domain.NewOperation("of-1", 1, "Contrôle visuel")
		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OperationSkipped").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, ops, store)
		payload, _ := proto.Marshal(&pbmes.SkipOperationRequest{OperationId: op.ID, Reason: "non applicable"})

		resp, err := h.SkipOperation(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.SkipOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_SKIPPED, response.Operation.Status)
		store.AssertExpectations(t)
	})

	t.Run("skip without reason returns error", func(t *testing.T) {
		ops := &mockOperationRepo{}
		op, _ := domain.NewOperation("of-1", 1, "Contrôle visuel")
		ops.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		h := newTestHandler(&mockOrderRepo{}, ops, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.SkipOperationRequest{OperationId: op.ID, Reason: ""})

		_, err := h.SkipOperation(context.Background(), payload)
		require.Error(t, err)
	})
}

// ── CreateLot ─────────────────────────────────────────────────────────────────

func TestHandler_CreateLot(t *testing.T) {
	t.Run("valid lot is saved with outbox entry", func(t *testing.T) {
		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("SaveLot", mock.Anything, mock.AnythingOfType("*domain.Lot")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "LotCreated").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.CreateLotRequest{
			Reference: "LOT-001", ProductId: "prod-1", Quantity: 50,
		})

		resp, err := h.CreateLot(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.CreateLotResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, "LOT-001", response.Lot.Reference)
		store.AssertExpectations(t)
	})

	t.Run("empty reference returns error", func(t *testing.T) {
		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.CreateLotRequest{Reference: "", ProductId: "prod-1", Quantity: 10})
		_, err := h.CreateLot(context.Background(), payload)
		require.Error(t, err)
	})
}

// ── RegisterSN ────────────────────────────────────────────────────────────────

func TestHandler_RegisterSN(t *testing.T) {
	t.Run("registers a serial number", func(t *testing.T) {
		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("SaveSerialNumber", mock.Anything, mock.AnythingOfType("*domain.SerialNumber")).Return(nil)

		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.RegisterSNRequest{
			Sn: "SN-0042", ProductId: "prod-1", OfId: "of-1",
		})

		resp, err := h.RegisterSN(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.RegisterSNResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, "SN-0042", response.SerialNumber.Sn)
		assert.Equal(t, pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_PRODUCED, response.SerialNumber.Status)
		store.AssertExpectations(t)
	})
}

// ── ReleaseSN ─────────────────────────────────────────────────────────────────

func TestHandler_ReleaseSN(t *testing.T) {
	t.Run("produced SN is released with outbox entry", func(t *testing.T) {
		sn, _ := domain.NewSerialNumber("SN-1", "", "prod-1", "of-1")

		trace := &mockTraceabilityRepo{}
		trace.On("FindSNByID", mock.Anything, sn.ID).Return(sn, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateSerialNumber", mock.Anything, mock.AnythingOfType("*domain.SerialNumber")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "SNReleased").Return(nil)

		h := newTestHandlerWithTrace(&mockOrderRepo{}, &mockOperationRepo{}, trace, store)
		payload, _ := proto.Marshal(&pbmes.ReleaseSNRequest{Id: sn.ID})

		resp, err := h.ReleaseSN(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.ReleaseSNResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_RELEASED, response.SerialNumber.Status)
		store.AssertExpectations(t)
	})
}

// ── ScrapSN ───────────────────────────────────────────────────────────────────

func TestHandler_ScrapSN(t *testing.T) {
	t.Run("produced SN can be scrapped", func(t *testing.T) {
		sn, _ := domain.NewSerialNumber("SN-2", "", "prod-1", "of-1")

		trace := &mockTraceabilityRepo{}
		trace.On("FindSNByID", mock.Anything, sn.ID).Return(sn, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateSerialNumber", mock.Anything, mock.AnythingOfType("*domain.SerialNumber")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "SNScrapped").Return(nil)

		h := newTestHandlerWithTrace(&mockOrderRepo{}, &mockOperationRepo{}, trace, store)
		payload, _ := proto.Marshal(&pbmes.ScrapSNRequest{Id: sn.ID})

		resp, err := h.ScrapSN(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.ScrapSNResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_SCRAPPED, response.SerialNumber.Status)
	})
}

// ── AddGenealogyEntry ─────────────────────────────────────────────────────────

func TestHandler_AddGenealogyEntry(t *testing.T) {
	t.Run("genealogy entry is saved with outbox", func(t *testing.T) {
		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("SaveGenealogyEntry", mock.Anything, mock.AnythingOfType("*domain.GenealogyEntry")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "GenealogyEntryAdded").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.AddGenealogyEntryRequest{
			ParentSnId: "parent-sn-id",
			ChildSnId:  "child-sn-id",
			OfId:       "of-id",
		})

		resp, err := h.AddGenealogyEntry(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.AddGenealogyEntryResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, "parent-sn-id", response.Entry.ParentSnId)
		store.AssertExpectations(t)
	})

	t.Run("same parent and child returns error", func(t *testing.T) {
		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.AddGenealogyEntryRequest{
			ParentSnId: "same-id", ChildSnId: "same-id", OfId: "of-id",
		})
		_, err := h.AddGenealogyEntry(context.Background(), payload)
		require.Error(t, err)
	})
}

// ── SignOffOperation ───────────────────────────────────────────────────────────

func TestHandler_SignOffOperation(t *testing.T) {
	t.Run("sign-off transitions operation to released", func(t *testing.T) {
		op := &domain.Operation{
			ID:              "00000000-0000-0000-0000-000000000001",
			OFID:            "00000000-0000-0000-0000-000000000002",
			StepNumber:      1,
			Name:            "Weld joint",
			Status:          domain.OperationStatusPendingSignOff,
			RequiresSignOff: true,
		}

		opsRepo := &mockOperationRepo{}
		opsRepo.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OperationSignedOff").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, opsRepo, store)
		payload, _ := proto.Marshal(&pbmes.SignOffOperationRequest{
			OperationId: op.ID,
			InspectorId: "00000000-0000-0000-0000-000000000020",
		})

		resp, err := h.SignOffOperation(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.SignOffOperationResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_RELEASED, response.Operation.Status)
		store.AssertExpectations(t)
	})

	t.Run("sign-off on non-pending_sign_off operation returns error", func(t *testing.T) {
		op := &domain.Operation{
			ID:     "00000000-0000-0000-0000-000000000001",
			Status: domain.OperationStatusCompleted,
		}

		opsRepo := &mockOperationRepo{}
		opsRepo.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		h := newTestHandler(&mockOrderRepo{}, opsRepo, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.SignOffOperationRequest{
			OperationId: op.ID,
			InspectorId: "00000000-0000-0000-0000-000000000020",
		})

		_, err := h.SignOffOperation(context.Background(), payload)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotPendingSignOff)
	})
}

// ── DeclareNC ─────────────────────────────────────────────────────────────────

func TestHandler_DeclareNC(t *testing.T) {
	t.Run("nc declared writes outbox event", func(t *testing.T) {
		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "NCDeclared").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.DeclareNCRequest{
			OperationId:      "00000000-0000-0000-0000-000000000001",
			OfId:             "00000000-0000-0000-0000-000000000002",
			DefectCode:       "DIM_OUT_OF_TOLERANCE",
			Description:      "Diameter 0.2mm over tolerance",
			AffectedQuantity: 1,
			DeclaredBy:       "00000000-0000-0000-0000-000000000010",
		})

		resp, err := h.DeclareNC(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.DeclareNCResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.NotEmpty(t, response.EventId)
		store.AssertExpectations(t)
	})

	t.Run("missing required fields returns error", func(t *testing.T) {
		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.DeclareNCRequest{
			OperationId: "00000000-0000-0000-0000-000000000001",
			// missing of_id and defect_code
		})
		_, err := h.DeclareNC(context.Background(), payload)
		require.Error(t, err)
	})
}

// ── ApproveFAI ────────────────────────────────────────────────────────────────

func TestHandler_ApproveFAI(t *testing.T) {
	t.Run("fai approved writes outbox event", func(t *testing.T) {
		order := &domain.Order{
			ID:        "00000000-0000-0000-0000-000000000001",
			Reference: "OF-FAI-001",
			ProductID: "00000000-0000-0000-0000-000000000002",
			Quantity:  1,
			Status:    domain.OrderStatusInProgress,
			IsFAI:     true,
		}

		ordersRepo := &mockOrderRepo{}
		ordersRepo.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "OFFAIApproved").Return(nil)

		h := newTestHandler(ordersRepo, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.ApproveFAIRequest{
			OfId:       order.ID,
			ApproverId: "00000000-0000-0000-0000-000000000030",
		})

		resp, err := h.ApproveFAI(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.ApproveFAIResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.True(t, response.Order.IsFai)
		assert.NotEmpty(t, response.Order.FaiApprovedBy)
		store.AssertExpectations(t)
	})

	t.Run("non-fai order returns error", func(t *testing.T) {
		order := &domain.Order{
			ID:    "00000000-0000-0000-0000-000000000001",
			IsFAI: false,
		}

		ordersRepo := &mockOrderRepo{}
		ordersRepo.On("FindByID", mock.Anything, order.ID).Return(order, nil)

		h := newTestHandler(ordersRepo, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.ApproveFAIRequest{
			OfId:       order.ID,
			ApproverId: "00000000-0000-0000-0000-000000000030",
		})

		_, err := h.ApproveFAI(context.Background(), payload)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFAIOrder)
	})
}

// ── AttachInstructions ────────────────────────────────────────────────────────

func TestHandler_AttachInstructions(t *testing.T) {
	t.Run("instructions url is persisted", func(t *testing.T) {
		op := &domain.Operation{
			ID:         "00000000-0000-0000-0000-000000000001",
			OFID:       "00000000-0000-0000-0000-000000000002",
			StepNumber: 1,
			Name:       "Inspect surface",
			Status:     domain.OperationStatusPending,
		}

		opsRepo := &mockOperationRepo{}
		opsRepo.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)

		h := newTestHandler(&mockOrderRepo{}, opsRepo, store)
		payload, _ := proto.Marshal(&pbmes.AttachInstructionsRequest{
			OperationId:    op.ID,
			InstructionsUrl: "minio://instructions/sop-weld-v3.pdf",
		})

		resp, err := h.AttachInstructions(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.AttachInstructionsResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, "minio://instructions/sop-weld-v3.pdf", response.Operation.InstructionsUrl)
		store.AssertExpectations(t)
	})
}

// ── BLOC 5 — Routing & Planning ───────────────────────────────────────────────

func TestHandler_CreateRouting(t *testing.T) {
	t.Run("routing created and persisted with outbox", func(t *testing.T) {
		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("SaveRouting", mock.Anything, mock.AnythingOfType("*domain.Routing")).Return(nil)
		store.Ops.On("SaveRoutingStep", mock.Anything, mock.AnythingOfType("*domain.RoutingStep")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "RoutingCreated").Return(nil)

		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.CreateRoutingRequest{
			ProductId: "00000000-0000-0000-0000-000000000001",
			Name:      "Frame Assembly v1",
			Version:   1,
			Steps: []*pbmes.CreateRoutingStepInput{
				{StepNumber: 1, Name: "Cut", PlannedDurationSeconds: 120},
			},
			Activate: true,
		})

		resp, err := h.CreateRouting(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.CreateRoutingResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, "Frame Assembly v1", response.Routing.Name)
		assert.True(t, response.Routing.IsActive)
		assert.Len(t, response.Routing.Steps, 1)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})

	t.Run("no steps with activate returns error before tx", func(t *testing.T) {
		store := newMockTransactor()
		h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.CreateRoutingRequest{
			ProductId: "00000000-0000-0000-0000-000000000001",
			Name:      "Empty Routing",
			Version:   1,
			Activate:  true,
		})

		_, err := h.CreateRouting(context.Background(), payload)
		require.Error(t, err)
	})
}

func TestHandler_GetRouting(t *testing.T) {
	t.Run("returns routing by id", func(t *testing.T) {
		rt := &domain.Routing{
			ID:        "00000000-0000-0000-0000-000000000001",
			ProductID: "00000000-0000-0000-0000-000000000002",
			Version:   1,
			Name:      "Airframe Assembly",
			IsActive:  true,
		}

		routingRepo := &mockRoutingRepo{}
		routingRepo.On("FindRoutingByID", mock.Anything, rt.ID).Return(rt, nil)

		log := zerolog.Nop()
		reg := prometheus.NewRegistry()
		h := handler.New(&mockOrderRepo{}, &mockOperationRepo{}, &mockTraceabilityRepo{}, routingRepo, &mockQualificationRepo{}, &mockWorkstationRepo{}, &mockTimeTrackingRepo{}, &mockToolRepo{}, &mockMaterialRepo{}, newMockTransactor(), reg, &log)

		payload, _ := proto.Marshal(&pbmes.GetRoutingRequest{Id: rt.ID})
		resp, err := h.GetRouting(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.GetRoutingResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, rt.Name, response.Routing.Name)
		routingRepo.AssertExpectations(t)
	})
}

func TestHandler_GetDispatchList(t *testing.T) {
	t.Run("returns dispatch list ordered by priority", func(t *testing.T) {
		o1, _ := domain.NewOrder("OF-001", "prod-1", 5)
		_ = o1.SetPlanning(nil, 90)
		o2, _ := domain.NewOrder("OF-002", "prod-1", 3)
		_ = o2.SetPlanning(nil, 50)

		ordersRepo := &mockOrderRepo{}
		ordersRepo.On("DispatchList", mock.Anything, 50).Return([]*domain.Order{o1, o2}, nil)

		h := newTestHandler(ordersRepo, &mockOperationRepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbmes.DispatchListRequest{Limit: 50})
		resp, err := h.GetDispatchList(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.DispatchListResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		require.Len(t, response.Orders, 2)
		assert.Equal(t, int32(90), response.Orders[0].Priority)
		ordersRepo.AssertExpectations(t)
	})
}

func TestHandler_SetPlanning(t *testing.T) {
	t.Run("updates planning fields on order", func(t *testing.T) {
		o, _ := domain.NewOrder("OF-P-001", "prod-1", 1)

		ordersRepo := &mockOrderRepo{}
		ordersRepo.On("FindByID", mock.Anything, o.ID).Return(o, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)

		h := newTestHandler(ordersRepo, &mockOperationRepo{}, store)
		payload, _ := proto.Marshal(&pbmes.SetPlanningRequest{OfId: o.ID, Priority: 80})
		resp, err := h.SetPlanning(context.Background(), payload)
		require.NoError(t, err)

		var response pbmes.SetPlanningResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, int32(80), response.Order.Priority)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})
}

// ── Qualification handler tests ───────────────────────────────────────────────

func TestHandler_CreateQualification(t *testing.T) {
	tests := []struct {
		name      string
		req       *pbmes.CreateQualificationRequest
		setupMock func(*mockTransactor)
		wantErr   bool
	}{
		{
			name: "valid request creates qualification and writes outbox",
			req: &pbmes.CreateQualificationRequest{
				OperatorId: "op-1",
				Skill:      "soudure_tig",
				Label:      "Soudure TIG",
				IssuedAt:   timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiresAt:  timestamppb.New(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)),
				GrantedBy:  "mgr-1",
			},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(nil)
				st.Ops.On("SaveQualification", mock.Anything, mock.AnythingOfType("*domain.Qualification")).Return(nil)
				st.Ops.On("InsertOutbox", mock.Anything, "QualificationCreated").Return(nil)
			},
		},
		{
			name: "empty skill returns error",
			req: &pbmes.CreateQualificationRequest{
				OperatorId: "op-1",
				Skill:      "",
				Label:      "Label",
				IssuedAt:   timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiresAt:  timestamppb.New(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)),
				GrantedBy:  "mgr-1",
			},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
		{
			name: "db error is propagated",
			req: &pbmes.CreateQualificationRequest{
				OperatorId: "op-1",
				Skill:      "soudure_tig",
				Label:      "TIG",
				IssuedAt:   timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiresAt:  timestamppb.New(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)),
				GrantedBy:  "mgr-1",
			},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(errDB)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockTransactor()
			tc.setupMock(store)
			h := newTestHandler(&mockOrderRepo{}, &mockOperationRepo{}, store)

			payload, err := proto.Marshal(tc.req)
			require.NoError(t, err)

			resp, err := h.CreateQualification(context.Background(), payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)

			var response pbmes.CreateQualificationResponse
			require.NoError(t, proto.Unmarshal(resp, &response))
			assert.Equal(t, tc.req.Skill, response.Qualification.Skill)
			store.AssertExpectations(t)
			store.Ops.AssertExpectations(t)
		})
	}
}

func TestHandler_StartOperation_InterlocksOnExpiredQualification(t *testing.T) {
	// The interlock check: if an operator has no active DB qualification for the
	// required skill, StartOperation must reject the request.
	op, _ := domain.NewOperation("of-1", 1, "Soudure")
	op.RequiredSkill = "soudure_tig"

	o, _ := domain.NewOrder("OF-001", "prod-1", 10)

	opsRepo := &mockOperationRepo{}
	opsRepo.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

	ordersRepo := &mockOrderRepo{}
	ordersRepo.On("FindByID", mock.Anything, op.OFID).Return(o, nil)

	qualsRepo := &mockQualificationRepo{}
	// No active skills returned — simulates expired or missing qualification.
	qualsRepo.On("ListActiveSkills", mock.Anything, "operator-1", mock.AnythingOfType("time.Time")).
		Return([]string{}, nil)

	store := newMockTransactor()
	h := newTestHandlerWithQuals(ordersRepo, opsRepo, qualsRepo, store)

	payload, _ := proto.Marshal(&pbmes.StartOperationRequest{
		OperationId:   op.ID,
		OperatorId:    "operator-1",
		OperatorRoles: []string{"kors-operateur"}, // does not include "soudure_tig"
	})
	_, err := h.StartOperation(context.Background(), payload)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrOperatorNotQualified)
	qualsRepo.AssertExpectations(t)
}

func TestHandler_StartOperation_PassesWithActiveQualification(t *testing.T) {
	// If ListActiveSkills returns the required skill, StartOperation must succeed.
	op, _ := domain.NewOperation("of-2", 1, "Soudure")
	op.RequiredSkill = "soudure_tig"

	o, _ := domain.NewOrder("OF-002", "prod-1", 10)

	opsRepo := &mockOperationRepo{}
	opsRepo.On("FindOperationByID", mock.Anything, op.ID).Return(op, nil)

	ordersRepo := &mockOrderRepo{}
	ordersRepo.On("FindByID", mock.Anything, op.OFID).Return(o, nil)

	qualsRepo := &mockQualificationRepo{}
	// Active qualification for the required skill.
	qualsRepo.On("ListActiveSkills", mock.Anything, "operator-1", mock.AnythingOfType("time.Time")).
		Return([]string{"soudure_tig"}, nil)

	store := newMockTransactor()
	store.On("WithTx", mock.Anything).Return(nil)
	store.Ops.On("UpdateOperation", mock.Anything, mock.AnythingOfType("*domain.Operation")).Return(nil)
	store.Ops.On("UpdateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).Return(nil)
	store.Ops.On("InsertOutbox", mock.Anything, mock.Anything).Return(nil)

	h := newTestHandlerWithQuals(ordersRepo, opsRepo, qualsRepo, store)

	payload, _ := proto.Marshal(&pbmes.StartOperationRequest{
		OperationId:   op.ID,
		OperatorId:    "operator-1",
		OperatorRoles: []string{"kors-operateur"},
	})
	resp, err := h.StartOperation(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, resp)
	qualsRepo.AssertExpectations(t)
}
