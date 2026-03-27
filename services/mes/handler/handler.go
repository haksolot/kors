package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// OrderRepository is the persistence interface consumed by Handler.
type OrderRepository interface {
	Save(ctx context.Context, o *domain.Order) error
	FindByID(ctx context.Context, id string) (*domain.Order, error)
	FindByReference(ctx context.Context, ref string) (*domain.Order, error)
	Update(ctx context.Context, o *domain.Order) error
	List(ctx context.Context, f domain.ListOrdersFilter) ([]*domain.Order, error)
}

// OperationRepository is the persistence interface consumed by Handler.
type OperationRepository interface {
	SaveOperation(ctx context.Context, op *domain.Operation) error
	FindOperationByID(ctx context.Context, id string) (*domain.Operation, error)
	FindOperationsByOFID(ctx context.Context, ofID string) ([]*domain.Operation, error)
	UpdateOperation(ctx context.Context, op *domain.Operation) error
}

// Handler processes NATS request-reply messages for the MES service.
// Each method receives a raw Protobuf payload and returns a serialized response.
type Handler struct {
	orders OrderRepository
	ops    OperationRepository
	log    *zerolog.Logger
}

// New returns a Handler with the provided dependencies injected.
func New(orders OrderRepository, ops OperationRepository, log *zerolog.Logger) *Handler {
	return &Handler{orders: orders, ops: ops, log: log}
}

// CreateOrder handles kors.mes.of.create requests.
// Decodes a CreateOrderRequest, creates a new Order, persists it, and returns CreateOrderResponse.
func (h *Handler) CreateOrder(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.CreateOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("CreateOrder: unmarshal request: %w", err)
	}

	order, err := domain.NewOrder(req.Reference, req.ProductId, int(req.Quantity))
	if err != nil {
		return nil, fmt.Errorf("CreateOrder: %w", err)
	}

	if err := h.orders.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("CreateOrder: save: %w", err)
	}

	h.log.Info().
		Str("of_id", order.ID).
		Str("reference", order.Reference).
		Msg("manufacturing order created")

	return proto.Marshal(&pbmes.CreateOrderResponse{
		Order: orderToProto(order),
	})
}

// GetOrder handles kors.mes.of.get requests.
func (h *Handler) GetOrder(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.GetOrderRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("GetOrder: unmarshal request: %w", err)
	}

	order, err := h.orders.FindByID(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("GetOrder: %w", err)
	}

	return proto.Marshal(&pbmes.GetOrderResponse{
		Order: orderToProto(order),
	})
}

// ListOrders handles kors.mes.of.list requests.
func (h *Handler) ListOrders(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.ListOrdersRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("ListOrders: unmarshal request: %w", err)
	}

	filter := domain.ListOrdersFilter{
		PageSize: int(req.PageSize),
	}
	if req.StatusFilter != pbmes.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		s := protoStatusToDomain(req.StatusFilter)
		filter.Status = &s
	}

	orders, err := h.orders.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("ListOrders: %w", err)
	}

	pbOrders := make([]*pbmes.ManufacturingOrder, 0, len(orders))
	for _, o := range orders {
		pbOrders = append(pbOrders, orderToProto(o))
	}
	return proto.Marshal(&pbmes.ListOrdersResponse{Orders: pbOrders})
}

// StartOperation handles kors.mes.operation.start requests.
func (h *Handler) StartOperation(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.StartOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("StartOperation: unmarshal request: %w", err)
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return nil, fmt.Errorf("StartOperation: find: %w", err)
	}

	if err := op.Start(req.OperatorId); err != nil {
		return nil, fmt.Errorf("StartOperation: %w", err)
	}

	if err := h.ops.UpdateOperation(ctx, op); err != nil {
		return nil, fmt.Errorf("StartOperation: update: %w", err)
	}

	h.log.Info().
		Str("operation_id", op.ID).
		Str("operator_id", op.OperatorID).
		Msg("operation started")

	return proto.Marshal(&pbmes.StartOperationResponse{
		Operation: operationToProto(op),
	})
}

// CompleteOperation handles kors.mes.operation.complete requests.
func (h *Handler) CompleteOperation(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.CompleteOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("CompleteOperation: unmarshal request: %w", err)
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return nil, fmt.Errorf("CompleteOperation: find: %w", err)
	}

	if err := op.Complete(req.OperatorId); err != nil {
		return nil, fmt.Errorf("CompleteOperation: %w", err)
	}

	if err := h.ops.UpdateOperation(ctx, op); err != nil {
		return nil, fmt.Errorf("CompleteOperation: update: %w", err)
	}

	h.log.Info().
		Str("operation_id", op.ID).
		Msg("operation completed")

	return proto.Marshal(&pbmes.CompleteOperationResponse{
		Operation: operationToProto(op),
	})
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

// IsNotFound returns true if the error wraps a domain "not found" sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, domain.ErrOrderNotFound) ||
		errors.Is(err, domain.ErrOperationNotFound)
}
