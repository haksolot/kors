package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// OrderRepository is the read-only persistence interface for ManufacturingOrders.
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Order, error)
	FindByReference(ctx context.Context, ref string) (*domain.Order, error)
	List(ctx context.Context, f domain.ListOrdersFilter) ([]*domain.Order, error)
}

// OperationRepository is the read-only persistence interface for Operations.
type OperationRepository interface {
	FindOperationByID(ctx context.Context, id string) (*domain.Operation, error)
	FindOperationsByOFID(ctx context.Context, ofID string) ([]*domain.Operation, error)
}

// TraceabilityRepository is the read-only persistence interface for lots, serial numbers and genealogy.
type TraceabilityRepository interface {
	FindLotByID(ctx context.Context, id string) (*domain.Lot, error)
	FindSNByID(ctx context.Context, id string) (*domain.SerialNumber, error)
	FindSNBySN(ctx context.Context, sn string) (*domain.SerialNumber, error)
	GetGenealogyByParentSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error)
	GetGenealogyByChildSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error)
}

// Handler processes NATS request-reply messages for the MES service.
// All state-changing operations use domain.Transactor to guarantee atomicity
// between business data and the outbox entry (ADR-004).
type Handler struct {
	orders  OrderRepository
	ops     OperationRepository
	trace   TraceabilityRepository
	store   domain.Transactor
	log     *zerolog.Logger
	reqTotal *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}

// New returns a Handler with the provided dependencies injected.
// reg is used to register Prometheus metrics; pass prometheus.DefaultRegisterer in production.
func New(
	orders OrderRepository,
	ops OperationRepository,
	trace TraceabilityRepository,
	store domain.Transactor,
	reg prometheus.Registerer,
	log *zerolog.Logger,
) *Handler {
	return &Handler{
		orders:      orders,
		ops:         ops,
		trace:       trace,
		store:       store,
		log:         log,
		reqTotal:    core.NewCounter(reg, "mes", "handler_requests", "Total NATS handler invocations", []string{"subject", "status"}),
		reqDuration: core.NewHistogram(reg, "mes", "handler_duration_seconds", "NATS handler latency", []string{"subject"}, nil),
	}
}

// ── Orders ────────────────────────────────────────────────────────────────────

// CreateOrder handles kors.mes.of.create.
func (h *Handler) CreateOrder(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateOrder")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFCreate, start, fmt.Errorf("CreateOrder: unmarshal: %w", err))
	}

	order, err := domain.NewOrder(req.Reference, req.ProductId, int(req.Quantity))
	if err != nil {
		return h.fail(domain.SubjectOFCreate, start, fmt.Errorf("CreateOrder: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OFCreatedEvent{
		EventId:   uuid.NewString(),
		OfId:      order.ID,
		Reference: order.Reference,
		ProductId: order.ProductID,
		Quantity:  int32(order.Quantity),
		CreatedAt: timestamppb.New(order.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFCreate, start, fmt.Errorf("CreateOrder: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveOrder(ctx, order); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFCreated",
			Subject:   domain.SubjectOFCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFCreate, start, fmt.Errorf("CreateOrder: tx: %w", err))
	}

	h.log.Info().Str("of_id", order.ID).Str("reference", order.Reference).Msg("manufacturing order created")
	h.succeed(domain.SubjectOFCreate, start)
	return proto.Marshal(&pbmes.CreateOrderResponse{Order: orderToProto(order)})
}

// GetOrder handles kors.mes.of.get.
func (h *Handler) GetOrder(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetOrder")
	defer span.End()
	start := time.Now()

	var req pbmes.GetOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFGet, start, fmt.Errorf("GetOrder: unmarshal: %w", err))
	}

	order, err := h.orders.FindByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectOFGet, start, fmt.Errorf("GetOrder: %w", err))
	}

	h.succeed(domain.SubjectOFGet, start)
	return proto.Marshal(&pbmes.GetOrderResponse{Order: orderToProto(order)})
}

// ListOrders handles kors.mes.of.list.
func (h *Handler) ListOrders(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListOrders")
	defer span.End()
	start := time.Now()

	var req pbmes.ListOrdersRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFList, start, fmt.Errorf("ListOrders: unmarshal: %w", err))
	}

	filter := domain.ListOrdersFilter{PageSize: int(req.PageSize)}
	if req.StatusFilter != pbmes.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		s := protoStatusToDomain(req.StatusFilter)
		filter.Status = &s
	}

	orders, err := h.orders.List(ctx, filter)
	if err != nil {
		return h.fail(domain.SubjectOFList, start, fmt.Errorf("ListOrders: %w", err))
	}

	pbOrders := make([]*pbmes.ManufacturingOrder, 0, len(orders))
	for _, o := range orders {
		pbOrders = append(pbOrders, orderToProto(o))
	}
	h.succeed(domain.SubjectOFList, start)
	return proto.Marshal(&pbmes.ListOrdersResponse{Orders: pbOrders})
}

// SuspendOrder handles kors.mes.of.suspend.
func (h *Handler) SuspendOrder(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.SuspendOrder")
	defer span.End()
	start := time.Now()

	var req pbmes.SuspendOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFSuspend, start, fmt.Errorf("SuspendOrder: unmarshal: %w", err))
	}

	order, err := h.orders.FindByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectOFSuspend, start, fmt.Errorf("SuspendOrder: find: %w", err))
	}

	if err := order.Suspend(req.Reason); err != nil {
		return h.fail(domain.SubjectOFSuspend, start, fmt.Errorf("SuspendOrder: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OFSuspendedEvent{
		EventId:     uuid.NewString(),
		OfId:        order.ID,
		Reason:      req.Reason,
		SuspendedAt: timestamppb.New(order.UpdatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFSuspend, start, fmt.Errorf("SuspendOrder: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOrder(ctx, order); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFSuspended",
			Subject:   domain.SubjectOFSuspended,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFSuspend, start, fmt.Errorf("SuspendOrder: tx: %w", err))
	}

	h.log.Info().Str("of_id", order.ID).Msg("manufacturing order suspended")
	h.succeed(domain.SubjectOFSuspend, start)
	return proto.Marshal(&pbmes.SuspendOrderResponse{Order: orderToProto(order)})
}

// ResumeOrder handles kors.mes.of.resume.
func (h *Handler) ResumeOrder(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ResumeOrder")
	defer span.End()
	start := time.Now()

	var req pbmes.ResumeOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFResume, start, fmt.Errorf("ResumeOrder: unmarshal: %w", err))
	}

	order, err := h.orders.FindByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectOFResume, start, fmt.Errorf("ResumeOrder: find: %w", err))
	}

	if err := order.Resume(); err != nil {
		return h.fail(domain.SubjectOFResume, start, fmt.Errorf("ResumeOrder: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OFResumedEvent{
		EventId:   uuid.NewString(),
		OfId:      order.ID,
		ResumedAt: timestamppb.New(order.UpdatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFResume, start, fmt.Errorf("ResumeOrder: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOrder(ctx, order); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFResumed",
			Subject:   domain.SubjectOFResumed,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFResume, start, fmt.Errorf("ResumeOrder: tx: %w", err))
	}

	h.log.Info().Str("of_id", order.ID).Msg("manufacturing order resumed")
	h.succeed(domain.SubjectOFResume, start)
	return proto.Marshal(&pbmes.ResumeOrderResponse{Order: orderToProto(order)})
}

// CancelOrder handles kors.mes.of.cancel.
func (h *Handler) CancelOrder(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CancelOrder")
	defer span.End()
	start := time.Now()

	var req pbmes.CancelOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFCancel, start, fmt.Errorf("CancelOrder: unmarshal: %w", err))
	}

	order, err := h.orders.FindByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectOFCancel, start, fmt.Errorf("CancelOrder: find: %w", err))
	}

	if err := order.Cancel(req.Reason); err != nil {
		return h.fail(domain.SubjectOFCancel, start, fmt.Errorf("CancelOrder: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OFCancelledEvent{
		EventId:     uuid.NewString(),
		OfId:        order.ID,
		Reason:      req.Reason,
		CancelledAt: timestamppb.New(order.UpdatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFCancel, start, fmt.Errorf("CancelOrder: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOrder(ctx, order); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFCancelled",
			Subject:   domain.SubjectOFCancelled,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFCancel, start, fmt.Errorf("CancelOrder: tx: %w", err))
	}

	h.log.Info().Str("of_id", order.ID).Msg("manufacturing order cancelled")
	h.succeed(domain.SubjectOFCancel, start)
	return proto.Marshal(&pbmes.CancelOrderResponse{Order: orderToProto(order)})
}

// ── Operations ────────────────────────────────────────────────────────────────

// CreateOperation handles kors.mes.operation.create.
func (h *Handler) CreateOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationCreate, start, fmt.Errorf("CreateOperation: unmarshal: %w", err))
	}

	op, err := domain.NewOperation(req.OfId, int(req.StepNumber), req.Name)
	if err != nil {
		return h.fail(domain.SubjectOperationCreate, start, fmt.Errorf("CreateOperation: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveOperation(ctx, op)
	}); err != nil {
		return h.fail(domain.SubjectOperationCreate, start, fmt.Errorf("CreateOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Str("of_id", op.OFID).Int("step", op.StepNumber).Msg("operation created")
	h.succeed(domain.SubjectOperationCreate, start)
	return proto.Marshal(&pbmes.CreateOperationResponse{Operation: operationToProto(op)})
}

// GetOperation handles kors.mes.operation.get.
func (h *Handler) GetOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.GetOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationGet, start, fmt.Errorf("GetOperation: unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectOperationGet, start, fmt.Errorf("GetOperation: %w", err))
	}

	h.succeed(domain.SubjectOperationGet, start)
	return proto.Marshal(&pbmes.GetOperationResponse{Operation: operationToProto(op)})
}

// ListOperations handles kors.mes.operation.list.
func (h *Handler) ListOperations(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListOperations")
	defer span.End()
	start := time.Now()

	var req pbmes.ListOperationsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationList, start, fmt.Errorf("ListOperations: unmarshal: %w", err))
	}

	ops, err := h.ops.FindOperationsByOFID(ctx, req.OfId)
	if err != nil {
		return h.fail(domain.SubjectOperationList, start, fmt.Errorf("ListOperations: %w", err))
	}

	pbOps := make([]*pbmes.Operation, 0, len(ops))
	for _, op := range ops {
		pbOps = append(pbOps, operationToProto(op))
	}
	h.succeed(domain.SubjectOperationList, start)
	return proto.Marshal(&pbmes.ListOperationsResponse{Operations: pbOps})
}

// StartOperation handles kors.mes.operation.start.
// operator_id is taken from the request payload; it is the BFF's responsibility
// to populate it from validated JWT claims before sending the NATS request.
func (h *Handler) StartOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.StartOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.StartOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: find: %w", err))
	}

	if err := op.Start(req.OperatorId); err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OperationStartedEvent{
		EventId:     uuid.NewString(),
		OperationId: op.ID,
		OfId:        op.OFID,
		OperatorId:  op.OperatorID,
		StepNumber:  int32(op.StepNumber),
		StartedAt:   timestamppb.New(*op.StartedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationStarted",
			Subject:   domain.SubjectOperationStarted,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Str("operator_id", op.OperatorID).Msg("operation started")
	h.succeed(domain.SubjectOperationStart, start)
	return proto.Marshal(&pbmes.StartOperationResponse{Operation: operationToProto(op)})
}

// CompleteOperation handles kors.mes.operation.complete.
func (h *Handler) CompleteOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CompleteOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.CompleteOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: find: %w", err))
	}

	if err := op.Complete(req.OperatorId); err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.OperationCompletedEvent{
		EventId:     uuid.NewString(),
		OperationId: op.ID,
		OfId:        op.OFID,
		OperatorId:  op.OperatorID,
		StepNumber:  int32(op.StepNumber),
		CompletedAt: timestamppb.New(*op.CompletedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationCompleted",
			Subject:   domain.SubjectOperationCompleted,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Msg("operation completed")
	h.succeed(domain.SubjectOperationComplete, start)
	return proto.Marshal(&pbmes.CompleteOperationResponse{Operation: operationToProto(op)})
}

// SkipOperation handles kors.mes.operation.skip.
func (h *Handler) SkipOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.SkipOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.SkipOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationSkip, start, fmt.Errorf("SkipOperation: unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationSkip, start, fmt.Errorf("SkipOperation: find: %w", err))
	}

	if err := op.Skip(req.Reason); err != nil {
		return h.fail(domain.SubjectOperationSkip, start, fmt.Errorf("SkipOperation: %w", err))
	}

	now := time.Now().UTC()
	evt, err := proto.Marshal(&pbmes.OperationSkippedEvent{
		EventId:     uuid.NewString(),
		OperationId: op.ID,
		OfId:        op.OFID,
		StepNumber:  int32(op.StepNumber),
		Reason:      op.SkipReason,
		SkippedAt:   timestamppb.New(now),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationSkip, start, fmt.Errorf("SkipOperation: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationSkipped",
			Subject:   domain.SubjectOperationSkipped,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOperationSkip, start, fmt.Errorf("SkipOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Str("reason", op.SkipReason).Msg("operation skipped")
	h.succeed(domain.SubjectOperationSkip, start)
	return proto.Marshal(&pbmes.SkipOperationResponse{Operation: operationToProto(op)})
}

// ── Traceability ──────────────────────────────────────────────────────────────

// CreateLot handles kors.mes.lot.create.
func (h *Handler) CreateLot(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateLot")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateLotRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectLotCreate, start, fmt.Errorf("CreateLot: unmarshal: %w", err))
	}

	lot, err := domain.NewLot(req.Reference, req.ProductId, int(req.Quantity))
	if err != nil {
		return h.fail(domain.SubjectLotCreate, start, fmt.Errorf("CreateLot: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.Lot{
		Id:        lot.ID,
		Reference: lot.Reference,
		ProductId: lot.ProductID,
		Quantity:  int32(lot.Quantity),
	})
	if err != nil {
		return h.fail(domain.SubjectLotCreate, start, fmt.Errorf("CreateLot: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveLot(ctx, lot); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "LotCreated",
			Subject:   domain.SubjectLotCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectLotCreate, start, fmt.Errorf("CreateLot: tx: %w", err))
	}

	h.log.Info().Str("lot_id", lot.ID).Str("reference", lot.Reference).Msg("lot created")
	h.succeed(domain.SubjectLotCreate, start)
	return proto.Marshal(&pbmes.CreateLotResponse{Lot: lotToProto(lot)})
}

// GetLot handles kors.mes.lot.get.
func (h *Handler) GetLot(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetLot")
	defer span.End()
	start := time.Now()

	var req pbmes.GetLotRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectLotGet, start, fmt.Errorf("GetLot: unmarshal: %w", err))
	}

	lot, err := h.trace.FindLotByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectLotGet, start, fmt.Errorf("GetLot: %w", err))
	}

	h.succeed(domain.SubjectLotGet, start)
	return proto.Marshal(&pbmes.GetLotResponse{Lot: lotToProto(lot)})
}

// RegisterSN handles kors.mes.sn.register — creates a new serial number for a produced part.
func (h *Handler) RegisterSN(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RegisterSN")
	defer span.End()
	start := time.Now()

	var req pbmes.RegisterSNRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectSNRegister, start, fmt.Errorf("RegisterSN: unmarshal: %w", err))
	}

	sn, err := domain.NewSerialNumber(req.Sn, req.LotId, req.ProductId, req.OfId)
	if err != nil {
		return h.fail(domain.SubjectSNRegister, start, fmt.Errorf("RegisterSN: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveSerialNumber(ctx, sn)
	}); err != nil {
		return h.fail(domain.SubjectSNRegister, start, fmt.Errorf("RegisterSN: tx: %w", err))
	}

	h.log.Info().Str("sn_id", sn.ID).Str("sn", sn.SN).Msg("serial number registered")
	h.succeed(domain.SubjectSNRegister, start)
	return proto.Marshal(&pbmes.RegisterSNResponse{SerialNumber: snToProto(sn)})
}

// GetSN handles kors.mes.sn.get — fetch by human-readable SN string.
func (h *Handler) GetSN(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetSN")
	defer span.End()
	start := time.Now()

	var req pbmes.GetSNRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectSNGet, start, fmt.Errorf("GetSN: unmarshal: %w", err))
	}

	sn, err := h.trace.FindSNBySN(ctx, req.Sn)
	if err != nil {
		return h.fail(domain.SubjectSNGet, start, fmt.Errorf("GetSN: %w", err))
	}

	h.succeed(domain.SubjectSNGet, start)
	return proto.Marshal(&pbmes.GetSNResponse{SerialNumber: snToProto(sn)})
}

// ReleaseSN handles kors.mes.sn.release — quality check passed, SN is released.
func (h *Handler) ReleaseSN(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ReleaseSN")
	defer span.End()
	start := time.Now()

	var req pbmes.ReleaseSNRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectSNRelease, start, fmt.Errorf("ReleaseSN: unmarshal: %w", err))
	}

	sn, err := h.trace.FindSNByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectSNRelease, start, fmt.Errorf("ReleaseSN: find: %w", err))
	}

	if err := sn.Release(); err != nil {
		return h.fail(domain.SubjectSNRelease, start, fmt.Errorf("ReleaseSN: %w", err))
	}

	evt, err := proto.Marshal(snToProto(sn))
	if err != nil {
		return h.fail(domain.SubjectSNRelease, start, fmt.Errorf("ReleaseSN: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateSerialNumber(ctx, sn); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "SNReleased",
			Subject:   domain.SubjectSNReleased,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectSNRelease, start, fmt.Errorf("ReleaseSN: tx: %w", err))
	}

	h.log.Info().Str("sn_id", sn.ID).Str("sn", sn.SN).Msg("serial number released")
	h.succeed(domain.SubjectSNRelease, start)
	return proto.Marshal(&pbmes.ReleaseSNResponse{SerialNumber: snToProto(sn)})
}

// ScrapSN handles kors.mes.sn.scrap — marks a serial number as scrapped.
func (h *Handler) ScrapSN(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ScrapSN")
	defer span.End()
	start := time.Now()

	var req pbmes.ScrapSNRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectSNScrap, start, fmt.Errorf("ScrapSN: unmarshal: %w", err))
	}

	sn, err := h.trace.FindSNByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectSNScrap, start, fmt.Errorf("ScrapSN: find: %w", err))
	}

	if err := sn.Scrap(); err != nil {
		return h.fail(domain.SubjectSNScrap, start, fmt.Errorf("ScrapSN: %w", err))
	}

	evt, err := proto.Marshal(snToProto(sn))
	if err != nil {
		return h.fail(domain.SubjectSNScrap, start, fmt.Errorf("ScrapSN: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateSerialNumber(ctx, sn); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "SNScrapped",
			Subject:   domain.SubjectSNScrapped,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectSNScrap, start, fmt.Errorf("ScrapSN: tx: %w", err))
	}

	h.log.Info().Str("sn_id", sn.ID).Str("sn", sn.SN).Msg("serial number scrapped")
	h.succeed(domain.SubjectSNScrap, start)
	return proto.Marshal(&pbmes.ScrapSNResponse{SerialNumber: snToProto(sn)})
}

// AddGenealogyEntry handles kors.mes.genealogy.add.
func (h *Handler) AddGenealogyEntry(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.AddGenealogyEntry")
	defer span.End()
	start := time.Now()

	var req pbmes.AddGenealogyEntryRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectGenealogyAdd, start, fmt.Errorf("AddGenealogyEntry: unmarshal: %w", err))
	}

	entry, err := domain.NewGenealogyEntry(req.ParentSnId, req.ChildSnId, req.OfId, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectGenealogyAdd, start, fmt.Errorf("AddGenealogyEntry: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveGenealogyEntry(ctx, entry); err != nil {
			return err
		}
		evt, err := proto.Marshal(genealogyEntryToProto(entry))
		if err != nil {
			return fmt.Errorf("AddGenealogyEntry: marshal event: %w", err)
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "GenealogyEntryAdded",
			Subject:   domain.SubjectGenealogyEntryAdded,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectGenealogyAdd, start, fmt.Errorf("AddGenealogyEntry: tx: %w", err))
	}

	h.log.Info().Str("entry_id", entry.ID).Str("parent", entry.ParentSNID).Str("child", entry.ChildSNID).Msg("genealogy entry added")
	h.succeed(domain.SubjectGenealogyAdd, start)
	return proto.Marshal(&pbmes.AddGenealogyEntryResponse{Entry: genealogyEntryToProto(entry)})
}

// GetGenealogy handles kors.mes.genealogy.get — returns all genealogy entries for an SN (parent + child).
func (h *Handler) GetGenealogy(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetGenealogy")
	defer span.End()
	start := time.Now()

	var req pbmes.GetGenealogyRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectGenealogyGet, start, fmt.Errorf("GetGenealogy: unmarshal: %w", err))
	}

	parents, err := h.trace.GetGenealogyByParentSN(ctx, req.SnId)
	if err != nil {
		return h.fail(domain.SubjectGenealogyGet, start, fmt.Errorf("GetGenealogy: parent query: %w", err))
	}
	children, err := h.trace.GetGenealogyByChildSN(ctx, req.SnId)
	if err != nil {
		return h.fail(domain.SubjectGenealogyGet, start, fmt.Errorf("GetGenealogy: child query: %w", err))
	}

	all := make([]*pbmes.GenealogyEntry, 0, len(parents)+len(children))
	for _, e := range parents {
		all = append(all, genealogyEntryToProto(e))
	}
	for _, e := range children {
		all = append(all, genealogyEntryToProto(e))
	}

	h.succeed(domain.SubjectGenealogyGet, start)
	return proto.Marshal(&pbmes.GetGenealogyResponse{Entries: all})
}

// ── Metrics helpers ───────────────────────────────────────────────────────────

func (h *Handler) succeed(subject string, start time.Time) {
	h.reqTotal.WithLabelValues(subject, "ok").Inc()
	h.reqDuration.WithLabelValues(subject).Observe(time.Since(start).Seconds())
}

func (h *Handler) fail(subject string, start time.Time, err error) ([]byte, error) {
	h.reqTotal.WithLabelValues(subject, "error").Inc()
	h.reqDuration.WithLabelValues(subject).Observe(time.Since(start).Seconds())
	return nil, err
}

// ── Converters ────────────────────────────────────────────────────────────────

func orderToProto(o *domain.Order) *pbmes.ManufacturingOrder {
	pb := &pbmes.ManufacturingOrder{
		Id:        o.ID,
		Reference: o.Reference,
		ProductId: o.ProductID,
		Quantity:  int32(o.Quantity),
		Status:    domainStatusToProto(o.Status),
		CreatedAt: timestamppb.New(o.CreatedAt),
		UpdatedAt: timestamppb.New(o.UpdatedAt),
	}
	if o.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*o.StartedAt)
	}
	if o.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*o.CompletedAt)
	}
	return pb
}

func operationToProto(op *domain.Operation) *pbmes.Operation {
	pb := &pbmes.Operation{
		Id:         op.ID,
		OfId:       op.OFID,
		StepNumber: int32(op.StepNumber),
		Name:       op.Name,
		OperatorId: op.OperatorID,
		Status:     domainOpStatusToProto(op.Status),
		SkipReason: op.SkipReason,
		CreatedAt:  timestamppb.New(op.CreatedAt),
	}
	if op.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*op.StartedAt)
	}
	if op.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*op.CompletedAt)
	}
	return pb
}

func domainStatusToProto(s domain.OrderStatus) pbmes.OrderStatus {
	switch s {
	case domain.OrderStatusPlanned:
		return pbmes.OrderStatus_ORDER_STATUS_PLANNED
	case domain.OrderStatusInProgress:
		return pbmes.OrderStatus_ORDER_STATUS_IN_PROGRESS
	case domain.OrderStatusCompleted:
		return pbmes.OrderStatus_ORDER_STATUS_COMPLETED
	case domain.OrderStatusSuspended:
		return pbmes.OrderStatus_ORDER_STATUS_SUSPENDED
	case domain.OrderStatusCancelled:
		return pbmes.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return pbmes.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomain(s pbmes.OrderStatus) domain.OrderStatus {
	switch s {
	case pbmes.OrderStatus_ORDER_STATUS_PLANNED:
		return domain.OrderStatusPlanned
	case pbmes.OrderStatus_ORDER_STATUS_IN_PROGRESS:
		return domain.OrderStatusInProgress
	case pbmes.OrderStatus_ORDER_STATUS_COMPLETED:
		return domain.OrderStatusCompleted
	case pbmes.OrderStatus_ORDER_STATUS_SUSPENDED:
		return domain.OrderStatusSuspended
	case pbmes.OrderStatus_ORDER_STATUS_CANCELLED:
		return domain.OrderStatusCancelled
	default:
		return domain.OrderStatusPlanned
	}
}

func domainOpStatusToProto(s domain.OperationStatus) pbmes.OperationStatus {
	switch s {
	case domain.OperationStatusPending:
		return pbmes.OperationStatus_OPERATION_STATUS_PENDING
	case domain.OperationStatusInProgress:
		return pbmes.OperationStatus_OPERATION_STATUS_IN_PROGRESS
	case domain.OperationStatusCompleted:
		return pbmes.OperationStatus_OPERATION_STATUS_COMPLETED
	case domain.OperationStatusSkipped:
		return pbmes.OperationStatus_OPERATION_STATUS_SKIPPED
	default:
		return pbmes.OperationStatus_OPERATION_STATUS_UNSPECIFIED
	}
}

func lotToProto(l *domain.Lot) *pbmes.Lot {
	pb := &pbmes.Lot{
		Id:              l.ID,
		Reference:       l.Reference,
		ProductId:       l.ProductID,
		Quantity:        int32(l.Quantity),
		MaterialCertUrl: l.MaterialCertURL,
		ReceivedAt:      timestamppb.New(l.ReceivedAt),
	}
	return pb
}

func snToProto(sn *domain.SerialNumber) *pbmes.SerialNumber {
	pb := &pbmes.SerialNumber{
		Id:        sn.ID,
		Sn:        sn.SN,
		LotId:     sn.LotID,
		ProductId: sn.ProductID,
		OfId:      sn.OFID,
		Status:    domainSNStatusToProto(sn.Status),
		CreatedAt: timestamppb.New(sn.CreatedAt),
	}
	return pb
}

func genealogyEntryToProto(e *domain.GenealogyEntry) *pbmes.GenealogyEntry {
	return &pbmes.GenealogyEntry{
		Id:          e.ID,
		ParentSnId:  e.ParentSNID,
		ChildSnId:   e.ChildSNID,
		OfId:        e.OFID,
		OperationId: e.OperationID,
		RecordedAt:  timestamppb.New(e.RecordedAt),
	}
}

func domainSNStatusToProto(s domain.SerialNumberStatus) pbmes.SerialNumberStatus {
	switch s {
	case domain.SNStatusProduced:
		return pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_PRODUCED
	case domain.SNStatusReleased:
		return pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_RELEASED
	case domain.SNStatusScrapped:
		return pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_SCRAPPED
	default:
		return pbmes.SerialNumberStatus_SERIAL_NUMBER_STATUS_UNSPECIFIED
	}
}

// IsNotFound returns true if the error wraps a domain "not found" sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, domain.ErrOrderNotFound) ||
		errors.Is(err, domain.ErrOperationNotFound) ||
		errors.Is(err, domain.ErrLotNotFound) ||
		errors.Is(err, domain.ErrSerialNumberNotFound)
}
