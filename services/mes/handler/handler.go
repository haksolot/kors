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

// RoutingRepository is the read-only persistence interface for Routing templates.
type RoutingRepository interface {
	FindRoutingByID(ctx context.Context, id string) (*domain.Routing, error)
	FindRoutingsByProductID(ctx context.Context, productID string) ([]*domain.Routing, error)
}

// DispatchRepository adds the dispatch list query to the order repository.
type DispatchRepository interface {
	OrderRepository
	DispatchList(ctx context.Context, limit int) ([]*domain.Order, error)
}

// QualificationRepository is the read-only interface for operator qualifications (AS9100D §7.2).
type QualificationRepository interface {
	FindQualificationByID(ctx context.Context, id string) (*domain.Qualification, error)
	ListQualificationsByOperator(ctx context.Context, operatorID string) ([]*domain.Qualification, error)
	ListActiveSkills(ctx context.Context, operatorID string, now time.Time) ([]string, error)
	ListExpiringQualifications(ctx context.Context, warningDays int, now time.Time) ([]*domain.Qualification, error)
}

// WorkstationRepository is the read-only interface for workstations.
type WorkstationRepository interface {
	FindWorkstationByID(ctx context.Context, id string) (*domain.Workstation, error)
	ListWorkstations(ctx context.Context, limit, offset int) ([]*domain.Workstation, error)
}

// TimeTrackingRepository is the read-only interface for time logs and downtimes.
type TimeTrackingRepository interface {
	FindDowntimeByID(ctx context.Context, id string) (*domain.DowntimeEvent, error)
	FindOngoingDowntime(ctx context.Context, workstationID string) (*domain.DowntimeEvent, error)
	ListTimeLogsByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.TimeLog, error)
	ListDowntimesByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.DowntimeEvent, error)
}

// ToolRepository is the read-only interface for tools and gauges.
type ToolRepository interface {
	FindToolByID(ctx context.Context, id string) (*domain.Tool, error)
	FindToolBySerialNumber(ctx context.Context, sn string) (*domain.Tool, error)
	ListTools(ctx context.Context, limit, offset int) ([]*domain.Tool, error)
	ListToolsByOperation(ctx context.Context, operationID string) ([]*domain.Tool, error)
}

// MaterialRepository is the read-only interface for material tracking.
type MaterialRepository interface {
	FindOngoingTOEExposure(ctx context.Context, lotID string) (*domain.TOEExposureLog, error)
	ListConsumptionsByOperation(ctx context.Context, operationID string) ([]*domain.ConsumptionRecord, error)
	ListTransfersByEntity(ctx context.Context, entityID string) ([]*domain.LocationTransfer, error)
}

// QualityRepository is the read-only interface for inline quality.
type QualityRepository interface {
	FindCharacteristicByID(ctx context.Context, id string) (*domain.ControlCharacteristic, error)
	ListCharacteristicsByStep(ctx context.Context, stepID string) ([]*domain.ControlCharacteristic, error)
	ListCharacteristicsByOperation(ctx context.Context, operationID string) ([]*domain.ControlCharacteristic, error)
	ListMeasurementsByOperation(ctx context.Context, operationID string) ([]*domain.Measurement, error)
	ListMeasurementsByCharacteristic(ctx context.Context, characteristicID string, limit int) ([]*domain.Measurement, error)
}

// AlertRepository is the read-only interface for alerts.
type AlertRepository interface {
	FindAlertByID(ctx context.Context, id string) (*domain.Alert, error)
	ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error)
}

// Handler processes NATS request-reply messages for the MES service.
// All state-changing operations use domain.Transactor to guarantee atomicity
// between business data and the outbox entry (ADR-004).
type Handler struct {
	orders       DispatchRepository
	ops          OperationRepository
	trace        TraceabilityRepository
	routings     RoutingRepository
	quals        QualificationRepository
	workstations WorkstationRepository
	time         TimeTrackingRepository
	tools        ToolRepository
	materials    MaterialRepository
	quality      QualityRepository
	alerts       AlertRepository
	store        domain.Transactor
	log          *zerolog.Logger
	reqTotal     *prometheus.CounterVec
	reqDuration  *prometheus.HistogramVec
}

// New returns a Handler with the provided dependencies injected.
// reg is used to register Prometheus metrics; pass prometheus.DefaultRegisterer in production.
func New(
	orders DispatchRepository,
	ops OperationRepository,
	trace TraceabilityRepository,
	routings RoutingRepository,
	quals QualificationRepository,
	workstations WorkstationRepository,
	timeRepo TimeTrackingRepository,
	toolRepo ToolRepository,
	materialRepo MaterialRepository,
	qualityRepo QualityRepository,
	alertRepo AlertRepository,
	store domain.Transactor,
	reg prometheus.Registerer,
	log *zerolog.Logger,
) *Handler {
	return &Handler{
		orders:       orders,
		ops:          ops,
		trace:        trace,
		routings:     routings,
		quals:        quals,
		workstations: workstations,
		time:         timeRepo,
		tools:        toolRepo,
		materials:    materialRepo,
		quality:      qualityRepo,
		alerts:       alertRepo,
		store:        store,
		log:          log,
		reqTotal:     core.NewCounter(reg, "mes", "handler_requests", "Total NATS handler invocations", []string{"subject", "status"}),
		reqDuration:  core.NewHistogram(reg, "mes", "handler_duration_seconds", "NATS handler latency", []string{"subject"}, nil),
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

	order.IsFAI = req.IsFai
	if req.Priority > 0 || req.DueDate != nil {
		priority := int(req.Priority)
		if priority == 0 {
			priority = 50
		}
		var dueDate *time.Time
		if req.DueDate != nil {
			t := req.DueDate.AsTime()
			dueDate = &t
		}
		if err := order.SetPlanning(dueDate, priority); err != nil {
			return h.fail(domain.SubjectOFCreate, start, fmt.Errorf("CreateOrder: planning: %w", err))
		}
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
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: find op: %w", err))
	}

	order, err := h.orders.FindByID(ctx, op.OFID)
	if err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: find order: %w", err))
	}

	// Interlock (AS9100D §7.2 + §13 NADCAP): load the operator's currently valid DB
	// qualifications when the operation requires a skill or is a special process.
	// mergedRoles = JWT roles + DB skill codes (for RequiredSkill check).
	// nadcapSkills = DB skill codes only (for NADCAP process code check — §13).
	mergedRoles := req.OperatorRoles
	var nadcapSkills []string
	if op.RequiredSkill != "" || op.IsSpecialProcess {
		activeSkills, err := h.quals.ListActiveSkills(ctx, req.OperatorId, time.Now().UTC())
		if err != nil {
			return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: list active skills: %w", err))
		}
		mergedRoles = append(mergedRoles, activeSkills...)
		nadcapSkills = activeSkills
	}

	// Tool Interlock (BLOC 8): verify all tools assigned to this operation are valid.
	tools, err := h.tools.ListToolsByOperation(ctx, op.ID)
	if err == nil {
		now := time.Now().UTC()
		for _, t := range tools {
			if !t.IsCalibrationValid(now) {
				return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("tool %s (%s) is expired: %w", t.Name, t.SerialNumber, domain.ErrToolExpired))
			}
			if t.Status == domain.ToolStatusBlocked {
				return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("tool %s (%s) is blocked: %w", t.Name, t.SerialNumber, domain.ErrToolBlocked))
			}
			if !t.HasRemainingLife() {
				return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("tool %s (%s) has reached max cycles: %w", t.Name, t.SerialNumber, domain.ErrToolMaxCyclesReached))
			}
		}
	}

	// Quality Interlock (BLOC 10): verify previous mandatory characteristics are PASS.
	allOps, err := h.ops.FindOperationsByOFID(ctx, op.OFID)
	if err == nil {
		for _, prevOp := range allOps {
			if prevOp.StepNumber < op.StepNumber {
				chars, _ := h.quality.ListCharacteristicsByOperation(ctx, prevOp.ID)
				meas, _ := h.quality.ListMeasurementsByOperation(ctx, prevOp.ID)
				
				measMap := make(map[string]*domain.Measurement)
				for _, m := range meas {
					measMap[m.CharacteristicID] = m
				}

				for _, c := range chars {
					if c.IsMandatory {
						m, exists := measMap[c.ID]
						if !exists {
							return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("mandatory quality check '%s' missing in previous step %d", c.Name, prevOp.StepNumber))
						}
						if m.Status != domain.MeasurementStatusPass {
							return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("mandatory quality check '%s' failed in previous step %d", c.Name, prevOp.StepNumber))
						}
					}
				}
			}
		}
	}

	if err := op.Start(req.OperatorId, mergedRoles, nadcapSkills); err != nil {
		return h.fail(domain.SubjectOperationStart, start, err)
	}

	// Automatic state transition for the parent order (ADR-004 side-effect)
	orderStarted := false
	if order.Status == domain.OrderStatusPlanned || order.Status == domain.OrderStatusSuspended {
		if err := order.Start(); err != nil {
			return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: start order: %w", err))
		}
		orderStarted = true
	}

	opEvt, err := proto.Marshal(&pbmes.OperationStartedEvent{
		EventId:     uuid.NewString(),
		OperationId: op.ID,
		OfId:        op.OFID,
		OperatorId:  op.OperatorID,
		StepNumber:  int32(op.StepNumber),
		StartedAt:   timestamppb.New(*op.StartedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: marshal op event: %w", err))
	}

	var ofEvt []byte
	if orderStarted {
		ofEvt, err = proto.Marshal(&pbmes.OFStartedEvent{
			EventId:   uuid.NewString(),
			OfId:      order.ID,
			StartedAt: timestamppb.New(*order.StartedAt),
		})
		if err != nil {
			return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: marshal of event: %w", err))
		}
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		if orderStarted {
			if err := tx.UpdateOrder(ctx, order); err != nil {
				return err
			}
		}

		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationStarted",
			Subject:   domain.SubjectOperationStarted,
			Payload:   opEvt,
		}); err != nil {
			return err
		}

		if orderStarted {
			return tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "OFStarted",
				Subject:   domain.SubjectOFStarted,
				Payload:   ofEvt,
			})
		}
		return nil
	}); err != nil {
		return h.fail(domain.SubjectOperationStart, start, fmt.Errorf("StartOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Str("of_id", op.OFID).Msg("operation and order started")
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

	// Automatic state transition for the parent order if all operations are terminal
	orderCompleted := false
	var order *domain.Order
	allOps, err := h.ops.FindOperationsByOFID(ctx, op.OFID)
	if err == nil {
		allDone := true
		for _, o := range allOps {
			// Check if this specific operation is done (the one we just updated)
			// or if other operations are already in terminal states
			status := o.Status
			if o.ID == op.ID {
				status = op.Status
			}
			if status != domain.OperationStatusCompleted &&
				status != domain.OperationStatusSkipped &&
				status != domain.OperationStatusReleased {
				allDone = false
				break
			}
		}

		if allDone {
			order, err = h.orders.FindByID(ctx, op.OFID)
			if err == nil && order.Status == domain.OrderStatusInProgress {
				if err := order.Complete(); err == nil {
					orderCompleted = true
				}
			}
		}
	}

	opEvt, err := proto.Marshal(&pbmes.OperationCompletedEvent{
		EventId:                uuid.NewString(),
		OperationId:            op.ID,
		OfId:                   op.OFID,
		OperatorId:             op.OperatorID,
		StepNumber:             int32(op.StepNumber),
		CompletedAt:            timestamppb.New(*op.CompletedAt),
		PlannedDurationSeconds: int32(op.PlannedDurationSeconds),
		ActualDurationSeconds:  int32(op.ActualDurationSeconds),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: marshal event: %w", err))
	}

	var ofEvt []byte
	if orderCompleted {
		ofEvt, err = proto.Marshal(&pbmes.OFCompletedEvent{
			EventId:     uuid.NewString(),
			OfId:        order.ID,
			CompletedAt: timestamppb.New(*order.CompletedAt),
		})
		if err != nil {
			return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: marshal of event: %w", err))
		}
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		if orderCompleted {
			if err := tx.UpdateOrder(ctx, order); err != nil {
				return err
			}
		}

		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationCompleted",
			Subject:   domain.SubjectOperationCompleted,
			Payload:   opEvt,
		}); err != nil {
			return err
		}

		if orderCompleted {
			return tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "OFCompleted",
				Subject:   domain.SubjectOFCompleted,
				Payload:   ofEvt,
			})
		}
		return nil
	}); err != nil {
		return h.fail(domain.SubjectOperationComplete, start, fmt.Errorf("CompleteOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", op.ID).Msg("operation completed")
	if orderCompleted {
		h.log.Info().Str("of_id", order.ID).Msg("manufacturing order completed")
	}

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

// ── Quality handlers (BLOC 4) ─────────────────────────────────────────────────

// SignOffOperation handles kors.mes.operation.sign_off (AS9100D §8.6 hold point).
// The BFF must extract inspector_id from the JWT and verify the quality_inspector role before calling.
func (h *Handler) SignOffOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.SignOffOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.SignOffOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationSignOff, start, fmt.Errorf("unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationSignOff, start, err)
	}

	if err := op.SignOff(req.InspectorId); err != nil {
		return h.fail(domain.SubjectOperationSignOff, start, err)
	}

	evt, err := proto.Marshal(&pbmes.OperationSignedOffEvent{
		EventId:     uuid.NewString(),
		OperationId: op.ID,
		OfId:        op.OFID,
		StepNumber:  int32(op.StepNumber),
		InspectorId: op.SignedOffBy,
		SignedOffAt: timestamppb.New(*op.SignedOffAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationSignOff, start, fmt.Errorf("marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationSignedOff",
			Subject:   domain.SubjectOperationSignedOff,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOperationSignOff, start, err)
	}

	h.succeed(domain.SubjectOperationSignOff, start)
	return proto.Marshal(&pbmes.SignOffOperationResponse{Operation: operationToProto(op)})
}

// DeclareNC handles kors.mes.operation.declare_nc (AS9100D §8.7).
// Publishes kors.mes.nc.declared via outbox for the QMS service to consume.
// No NC table is created in MES (event-driven decoupling).
func (h *Handler) DeclareNC(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.DeclareNC")
	defer span.End()
	start := time.Now()

	var req pbmes.DeclareNCRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationDeclareNC, start, fmt.Errorf("unmarshal: %w", err))
	}
	if req.OperationId == "" || req.OfId == "" || req.DefectCode == "" {
		return h.fail(domain.SubjectOperationDeclareNC, start, fmt.Errorf("operation_id, of_id and defect_code are required"))
	}

	eventID := uuid.NewString()
	now := time.Now().UTC()

	ncEvt, err := proto.Marshal(&pbmes.NCDeclaredEvent{
		EventId:          eventID,
		OperationId:      req.OperationId,
		OfId:             req.OfId,
		DefectCode:       req.DefectCode,
		Description:      req.Description,
		AffectedQuantity: req.AffectedQuantity,
		SerialNumbers:    req.SerialNumbers,
		DeclaredBy:       req.DeclaredBy,
		DeclaredAt:       timestamppb.New(now),
	})
	if err != nil {
		return h.fail(domain.SubjectOperationDeclareNC, start, fmt.Errorf("marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "NCDeclared",
			Subject:   domain.SubjectNCDeclared,
			Payload:   ncEvt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOperationDeclareNC, start, err)
	}

	h.succeed(domain.SubjectOperationDeclareNC, start)
	return proto.Marshal(&pbmes.DeclareNCResponse{EventId: eventID})
}

// ApproveFAI handles kors.mes.of.fai_approve (AS9100D §8.6 FAI).
// The BFF must extract approver_id from the JWT and verify the quality_manager role before calling.
func (h *Handler) ApproveFAI(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ApproveFAI")
	defer span.End()
	start := time.Now()

	var req pbmes.ApproveFAIRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFFAIApprove, start, fmt.Errorf("unmarshal: %w", err))
	}

	o, err := h.orders.FindByID(ctx, req.OfId)
	if err != nil {
		return h.fail(domain.SubjectOFFAIApprove, start, err)
	}

	if err := o.ApproveFAI(req.ApproverId); err != nil {
		return h.fail(domain.SubjectOFFAIApprove, start, err)
	}

	faiEvt, err := proto.Marshal(&pbmes.OFFAIApprovedEvent{
		EventId:    uuid.NewString(),
		OfId:       o.ID,
		ApproverId: o.FAIApprovedBy,
		ApprovedAt: timestamppb.New(*o.FAIApprovedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFFAIApprove, start, fmt.Errorf("marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOrder(ctx, o); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFFAIApproved",
			Subject:   domain.SubjectOFFAIApproved,
			Payload:   faiEvt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFFAIApprove, start, err)
	}

	h.succeed(domain.SubjectOFFAIApprove, start)
	return proto.Marshal(&pbmes.ApproveFAIResponse{Order: orderToProto(o)})
}

// AttachInstructions handles kors.mes.operation.attach_instructions.
// Associates a MinIO work instruction URL with an operation.
func (h *Handler) AttachInstructions(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.AttachInstructions")
	defer span.End()
	start := time.Now()

	var req pbmes.AttachInstructionsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationAttachInstructions, start, fmt.Errorf("unmarshal: %w", err))
	}

	op, err := h.ops.FindOperationByID(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationAttachInstructions, start, err)
	}

	op.AttachInstructions(req.InstructionsUrl)

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateOperation(ctx, op)
	}); err != nil {
		return h.fail(domain.SubjectOperationAttachInstructions, start, err)
	}

	h.succeed(domain.SubjectOperationAttachInstructions, start)
	return proto.Marshal(&pbmes.AttachInstructionsResponse{Operation: operationToProto(op)})
}

// ── Routing & Planning handlers (BLOC 5) ──────────────────────────────────────

// CreateRouting handles kors.mes.routing.create — saves a routing template with its steps.
func (h *Handler) CreateRouting(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateRouting")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateRoutingRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectRoutingCreate, start, fmt.Errorf("unmarshal: %w", err))
	}

	rt, err := domain.NewRouting(req.ProductId, req.Name, int(req.Version))
	if err != nil {
		return h.fail(domain.SubjectRoutingCreate, start, err)
	}

	for _, s := range req.Steps {
		step, err := rt.AddStep(int(s.StepNumber), s.Name, int(s.PlannedDurationSeconds))
		if err != nil {
			return h.fail(domain.SubjectRoutingCreate, start, err)
		}
		step.RequiredSkill = s.RequiredSkill
		step.InstructionsURL = s.InstructionsUrl
		step.RequiresSignOff = s.RequiresSignOff
	}

	if req.Activate {
		if err := rt.Activate(); err != nil {
			return h.fail(domain.SubjectRoutingCreate, start, err)
		}
	}

	evt, err := proto.Marshal(&pbmes.RoutingCreatedEvent{
		EventId:   uuid.NewString(),
		RoutingId: rt.ID,
		ProductId: rt.ProductID,
		Version:   int32(rt.Version),
		Name:      rt.Name,
		CreatedAt: timestamppb.New(rt.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectRoutingCreate, start, fmt.Errorf("marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveRouting(ctx, rt); err != nil {
			return err
		}
		for _, step := range rt.Steps {
			if err := tx.SaveRoutingStep(ctx, step); err != nil {
				return err
			}
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "RoutingCreated",
			Subject:   domain.SubjectRoutingCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectRoutingCreate, start, err)
	}

	h.log.Info().Str("routing_id", rt.ID).Str("name", rt.Name).Int("steps", len(rt.Steps)).Msg("routing created")
	h.succeed(domain.SubjectRoutingCreate, start)
	return proto.Marshal(&pbmes.CreateRoutingResponse{Routing: routingToProto(rt)})
}

// GetRouting handles kors.mes.routing.get.
func (h *Handler) GetRouting(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetRouting")
	defer span.End()
	start := time.Now()

	var req pbmes.GetRoutingRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectRoutingGet, start, fmt.Errorf("unmarshal: %w", err))
	}

	rt, err := h.routings.FindRoutingByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectRoutingGet, start, err)
	}

	h.succeed(domain.SubjectRoutingGet, start)
	return proto.Marshal(&pbmes.GetRoutingResponse{Routing: routingToProto(rt)})
}

// ListRoutings handles kors.mes.routing.list — returns all routings for a product.
func (h *Handler) ListRoutings(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListRoutings")
	defer span.End()
	start := time.Now()

	var req pbmes.ListRoutingsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectRoutingList, start, fmt.Errorf("unmarshal: %w", err))
	}

	routings, err := h.routings.FindRoutingsByProductID(ctx, req.ProductId)
	if err != nil {
		return h.fail(domain.SubjectRoutingList, start, err)
	}

	pbRoutings := make([]*pbmes.Routing, 0, len(routings))
	for _, rt := range routings {
		pbRoutings = append(pbRoutings, routingToProto(rt))
	}
	h.succeed(domain.SubjectRoutingList, start)
	return proto.Marshal(&pbmes.ListRoutingsResponse{Routings: pbRoutings})
}

// CreateFromRouting handles kors.mes.of.create_from_routing — creates an OF and all its
// operations from a routing template in a single transaction (ADR-004).
func (h *Handler) CreateFromRouting(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateFromRouting")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateFromRoutingRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, fmt.Errorf("unmarshal: %w", err))
	}

	rt, err := h.routings.FindRoutingByID(ctx, req.RoutingId)
	if err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, err)
	}

	order, err := domain.NewOrder(req.Reference, rt.ProductID, int(req.Quantity))
	if err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, err)
	}
	order.IsFAI = req.IsFai

	if req.Priority > 0 || req.DueDate != nil {
		priority := int(req.Priority)
		if priority == 0 {
			priority = 50
		}
		var dueDate *time.Time
		if req.DueDate != nil {
			t := req.DueDate.AsTime()
			dueDate = &t
		}
		if err := order.SetPlanning(dueDate, priority); err != nil {
			return h.fail(domain.SubjectOFCreateFromRouting, start, err)
		}
	}

	ops, err := rt.InstantiateOperations(order.ID)
	if err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, err)
	}

	ofEvt, err := proto.Marshal(&pbmes.OFCreatedEvent{
		EventId:   uuid.NewString(),
		OfId:      order.ID,
		Reference: order.Reference,
		ProductId: order.ProductID,
		Quantity:  int32(order.Quantity),
		CreatedAt: timestamppb.New(order.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, fmt.Errorf("marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveOrder(ctx, order); err != nil {
			return err
		}
		for _, op := range ops {
			if err := tx.SaveOperation(ctx, op); err != nil {
				return err
			}
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFCreated",
			Subject:   domain.SubjectOFCreated,
			Payload:   ofEvt,
		})
	}); err != nil {
		return h.fail(domain.SubjectOFCreateFromRouting, start, err)
	}

	pbOps := make([]*pbmes.Operation, 0, len(ops))
	for _, op := range ops {
		pbOps = append(pbOps, operationToProto(op))
	}

	h.log.Info().Str("of_id", order.ID).Str("routing_id", rt.ID).Int("ops", len(ops)).Msg("order created from routing")
	h.succeed(domain.SubjectOFCreateFromRouting, start)
	return proto.Marshal(&pbmes.CreateFromRoutingResponse{
		Order:      orderToProto(order),
		Operations: pbOps,
	})
}

// GetDispatchList handles kors.mes.of.dispatch_list — PLANNED+IN_PROGRESS orders sorted by priority/due_date.
func (h *Handler) GetDispatchList(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetDispatchList")
	defer span.End()
	start := time.Now()

	var req pbmes.DispatchListRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFDispatchList, start, fmt.Errorf("unmarshal: %w", err))
	}

	orders, err := h.orders.DispatchList(ctx, int(req.Limit))
	if err != nil {
		return h.fail(domain.SubjectOFDispatchList, start, err)
	}

	pbOrders := make([]*pbmes.ManufacturingOrder, 0, len(orders))
	for _, o := range orders {
		pbOrders = append(pbOrders, orderToProto(o))
	}
	h.succeed(domain.SubjectOFDispatchList, start)
	return proto.Marshal(&pbmes.DispatchListResponse{Orders: pbOrders})
}

// SetPlanning handles kors.mes.of.set_planning — updates due_date and priority on an existing order.
func (h *Handler) SetPlanning(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.SetPlanning")
	defer span.End()
	start := time.Now()

	var req pbmes.SetPlanningRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOFSetPlanning, start, fmt.Errorf("unmarshal: %w", err))
	}

	order, err := h.orders.FindByID(ctx, req.OfId)
	if err != nil {
		return h.fail(domain.SubjectOFSetPlanning, start, err)
	}

	var dueDate *time.Time
	if req.DueDate != nil {
		t := req.DueDate.AsTime()
		dueDate = &t
	}
	if err := order.SetPlanning(dueDate, int(req.Priority)); err != nil {
		return h.fail(domain.SubjectOFSetPlanning, start, err)
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateOrder(ctx, order)
	}); err != nil {
		return h.fail(domain.SubjectOFSetPlanning, start, err)
	}

	h.succeed(domain.SubjectOFSetPlanning, start)
	return proto.Marshal(&pbmes.SetPlanningResponse{Order: orderToProto(order)})
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
		IsFai:     o.IsFAI,
		Priority:  int32(o.Priority),
	}
	if o.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*o.StartedAt)
	}
	if o.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*o.CompletedAt)
	}
	if o.FAIApprovedAt != nil {
		pb.FaiApprovedBy = o.FAIApprovedBy
		pb.FaiApprovedAt = timestamppb.New(*o.FAIApprovedAt)
	}
	if o.DueDate != nil {
		pb.DueDate = timestamppb.New(*o.DueDate)
	}
	return pb
}

func operationToProto(op *domain.Operation) *pbmes.Operation {
	pb := &pbmes.Operation{
		Id:                     op.ID,
		OfId:                   op.OFID,
		StepNumber:             int32(op.StepNumber),
		Name:                   op.Name,
		OperatorId:             op.OperatorID,
		Status:                 domainOpStatusToProto(op.Status),
		SkipReason:             op.SkipReason,
		CreatedAt:              timestamppb.New(op.CreatedAt),
		RequiresSignOff:        op.RequiresSignOff,
		InstructionsUrl:        op.InstructionsURL,
		PlannedDurationSeconds: int32(op.PlannedDurationSeconds),
		ActualDurationSeconds:  int32(op.ActualDurationSeconds),
		RequiredSkill:          op.RequiredSkill,
	}
	if op.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*op.StartedAt)
	}
	if op.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*op.CompletedAt)
	}
	if op.SignedOffAt != nil {
		pb.SignedOffBy = op.SignedOffBy
		pb.SignedOffAt = timestamppb.New(*op.SignedOffAt)
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
	case domain.OperationStatusPendingSignOff:
		return pbmes.OperationStatus_OPERATION_STATUS_PENDING_SIGN_OFF
	case domain.OperationStatusReleased:
		return pbmes.OperationStatus_OPERATION_STATUS_RELEASED
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

func routingToProto(rt *domain.Routing) *pbmes.Routing {
	pb := &pbmes.Routing{
		Id:        rt.ID,
		ProductId: rt.ProductID,
		Version:   int32(rt.Version),
		Name:      rt.Name,
		IsActive:  rt.IsActive,
		CreatedAt: timestamppb.New(rt.CreatedAt),
	}
	for _, step := range rt.Steps {
		pb.Steps = append(pb.Steps, routingStepToProto(step))
	}
	return pb
}

func routingStepToProto(step *domain.RoutingStep) *pbmes.RoutingStep {
	return &pbmes.RoutingStep{
		Id:                     step.ID,
		RoutingId:              step.RoutingID,
		StepNumber:             int32(step.StepNumber),
		Name:                   step.Name,
		PlannedDurationSeconds: int32(step.PlannedDurationSeconds),
		RequiredSkill:          step.RequiredSkill,
		InstructionsUrl:        step.InstructionsURL,
		RequiresSignOff:        step.RequiresSignOff,
	}
}

// ── Qualifications (AS9100D §7.2) ────────────────────────────────────────────

// CreateQualification handles kors.mes.qualification.create.
// granted_by must be set by the BFF from the validated JWT claims — never from the client body.
func (h *Handler) CreateQualification(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateQualification")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateQualificationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationCreate, start, fmt.Errorf("CreateQualification: unmarshal: %w", err))
	}

	var issuedAt, expiresAt time.Time
	if req.IssuedAt != nil {
		issuedAt = req.IssuedAt.AsTime()
	}
	if req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt.AsTime()
	}

	q, err := domain.NewQualification(req.OperatorId, req.Skill, req.Label, issuedAt, expiresAt, req.GrantedBy)
	if err != nil {
		return h.fail(domain.SubjectQualificationCreate, start, fmt.Errorf("CreateQualification: %w", err))
	}
	if req.CertificateUrl != "" {
		q.AttachCertificate(req.CertificateUrl)
	}

	evt, err := proto.Marshal(&pbmes.QualificationCreatedEvent{
		EventId:         uuid.NewString(),
		QualificationId: q.ID,
		OperatorId:      q.OperatorID,
		Skill:           q.Skill,
		GrantedBy:       q.GrantedBy,
		ExpiresAt:       timestamppb.New(q.ExpiresAt),
		CreatedAt:       timestamppb.New(q.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectQualificationCreate, start, fmt.Errorf("CreateQualification: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveQualification(ctx, q); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "QualificationCreated",
			Subject:   domain.SubjectQualificationCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectQualificationCreate, start, fmt.Errorf("CreateQualification: tx: %w", err))
	}

	h.log.Info().Str("qualification_id", q.ID).Str("operator_id", q.OperatorID).Str("skill", q.Skill).Msg("qualification created")
	h.succeed(domain.SubjectQualificationCreate, start)
	return proto.Marshal(&pbmes.CreateQualificationResponse{Qualification: qualificationToProto(q, time.Now().UTC())})
}

// GetQualification handles kors.mes.qualification.get.
func (h *Handler) GetQualification(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetQualification")
	defer span.End()
	start := time.Now()

	var req pbmes.GetQualificationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationGet, start, fmt.Errorf("GetQualification: unmarshal: %w", err))
	}

	q, err := h.quals.FindQualificationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectQualificationGet, start, fmt.Errorf("GetQualification: %w", err))
	}

	h.succeed(domain.SubjectQualificationGet, start)
	return proto.Marshal(&pbmes.GetQualificationResponse{Qualification: qualificationToProto(q, time.Now().UTC())})
}

// ListQualifications handles kors.mes.qualification.list.
// Returns all qualifications (all statuses) for the operator — used for audit history.
func (h *Handler) ListQualifications(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListQualifications")
	defer span.End()
	start := time.Now()

	var req pbmes.ListQualificationsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationList, start, fmt.Errorf("ListQualifications: unmarshal: %w", err))
	}

	quals, err := h.quals.ListQualificationsByOperator(ctx, req.OperatorId)
	if err != nil {
		return h.fail(domain.SubjectQualificationList, start, fmt.Errorf("ListQualifications: %w", err))
	}

	now := time.Now().UTC()
	pbQuals := make([]*pbmes.Qualification, 0, len(quals))
	for _, q := range quals {
		pbQuals = append(pbQuals, qualificationToProto(q, now))
	}
	h.succeed(domain.SubjectQualificationList, start)
	return proto.Marshal(&pbmes.ListQualificationsResponse{Qualifications: pbQuals})
}

// RenewQualification handles kors.mes.qualification.renew.
// renewed_by must be set by the BFF from the validated JWT claims.
func (h *Handler) RenewQualification(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RenewQualification")
	defer span.End()
	start := time.Now()

	var req pbmes.RenewQualificationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationRenew, start, fmt.Errorf("RenewQualification: unmarshal: %w", err))
	}

	q, err := h.quals.FindQualificationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectQualificationRenew, start, fmt.Errorf("RenewQualification: find: %w", err))
	}

	var newExpires time.Time
	if req.NewExpiresAt != nil {
		newExpires = req.NewExpiresAt.AsTime()
	}
	if err := q.Renew(newExpires, req.RenewedBy); err != nil {
		return h.fail(domain.SubjectQualificationRenew, start, fmt.Errorf("RenewQualification: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.QualificationRenewedEvent{
		EventId:         uuid.NewString(),
		QualificationId: q.ID,
		OperatorId:      q.OperatorID,
		Skill:           q.Skill,
		RenewedBy:       req.RenewedBy,
		NewExpiresAt:    timestamppb.New(q.ExpiresAt),
		RenewedAt:       timestamppb.New(q.UpdatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectQualificationRenew, start, fmt.Errorf("RenewQualification: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateQualification(ctx, q); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "QualificationRenewed",
			Subject:   domain.SubjectQualificationRenewed,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectQualificationRenew, start, fmt.Errorf("RenewQualification: tx: %w", err))
	}

	h.log.Info().Str("qualification_id", q.ID).Str("operator_id", q.OperatorID).Msg("qualification renewed")
	h.succeed(domain.SubjectQualificationRenew, start)
	return proto.Marshal(&pbmes.RenewQualificationResponse{Qualification: qualificationToProto(q, time.Now().UTC())})
}

// RevokeQualification handles kors.mes.qualification.revoke.
// revoked_by must be set by the BFF from the validated JWT claims.
func (h *Handler) RevokeQualification(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RevokeQualification")
	defer span.End()
	start := time.Now()

	var req pbmes.RevokeQualificationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationRevoke, start, fmt.Errorf("RevokeQualification: unmarshal: %w", err))
	}

	q, err := h.quals.FindQualificationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectQualificationRevoke, start, fmt.Errorf("RevokeQualification: find: %w", err))
	}

	if err := q.Revoke(req.RevokedBy, req.Reason); err != nil {
		return h.fail(domain.SubjectQualificationRevoke, start, fmt.Errorf("RevokeQualification: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.QualificationRevokedEvent{
		EventId:         uuid.NewString(),
		QualificationId: q.ID,
		OperatorId:      q.OperatorID,
		Skill:           q.Skill,
		RevokedBy:       q.RevokedBy,
		Reason:          q.RevokeReason,
		RevokedAt:       timestamppb.New(*q.RevokedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectQualificationRevoke, start, fmt.Errorf("RevokeQualification: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateQualification(ctx, q); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "QualificationRevoked",
			Subject:   domain.SubjectQualificationRevoked,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectQualificationRevoke, start, fmt.Errorf("RevokeQualification: tx: %w", err))
	}

	h.log.Info().Str("qualification_id", q.ID).Str("operator_id", q.OperatorID).Msg("qualification revoked")
	h.succeed(domain.SubjectQualificationRevoke, start)
	return proto.Marshal(&pbmes.RevokeQualificationResponse{Qualification: qualificationToProto(q, time.Now().UTC())})
}

// ListExpiringQualifications handles kors.mes.qualification.list_expiring.
func (h *Handler) ListExpiringQualifications(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListExpiringQualifications")
	defer span.End()
	start := time.Now()

	var req pbmes.ListExpiringQualificationsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectQualificationListExpiring, start, fmt.Errorf("ListExpiringQualifications: unmarshal: %w", err))
	}

	warningDays := int(req.WarningDays)
	if warningDays <= 0 {
		warningDays = domain.DefaultExpiryWarningDays
	}

	now := time.Now().UTC()
	quals, err := h.quals.ListExpiringQualifications(ctx, warningDays, now)
	if err != nil {
		return h.fail(domain.SubjectQualificationListExpiring, start, fmt.Errorf("ListExpiringQualifications: %w", err))
	}

	pbQuals := make([]*pbmes.Qualification, 0, len(quals))
	for _, q := range quals {
		pbQuals = append(pbQuals, qualificationToProto(q, now))
	}
	h.succeed(domain.SubjectQualificationListExpiring, start)
	return proto.Marshal(&pbmes.ListExpiringQualificationsResponse{Qualifications: pbQuals})
}

// qualificationToProto converts a domain Qualification to its Protobuf representation.
// now is passed in to compute the status without mutating the domain struct.
func qualificationToProto(q *domain.Qualification, now time.Time) *pbmes.Qualification {
	pb := &pbmes.Qualification{
		Id:             q.ID,
		OperatorId:     q.OperatorID,
		Skill:          q.Skill,
		Label:          q.Label,
		Status:         domainQualStatusToProto(q.Status(now)),
		GrantedBy:      q.GrantedBy,
		CertificateUrl: q.CertificateURL,
		IsRevoked:      q.IsRevoked,
		RevokedBy:      q.RevokedBy,
		RevokeReason:   q.RevokeReason,
		IssuedAt:       timestamppb.New(q.IssuedAt),
		ExpiresAt:      timestamppb.New(q.ExpiresAt),
		CreatedAt:      timestamppb.New(q.CreatedAt),
		UpdatedAt:      timestamppb.New(q.UpdatedAt),
	}
	if q.RevokedAt != nil {
		pb.RevokedAt = timestamppb.New(*q.RevokedAt)
	}
	return pb
}

func domainQualStatusToProto(s domain.QualificationStatus) pbmes.QualificationStatus {
	switch s {
	case domain.QualificationStatusActive:
		return pbmes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE
	case domain.QualificationStatusExpiring:
		return pbmes.QualificationStatus_QUALIFICATION_STATUS_EXPIRING
	case domain.QualificationStatusExpired:
		return pbmes.QualificationStatus_QUALIFICATION_STATUS_EXPIRED
	case domain.QualificationStatusRevoked:
		return pbmes.QualificationStatus_QUALIFICATION_STATUS_REVOKED
	default:
		return pbmes.QualificationStatus_QUALIFICATION_STATUS_UNSPECIFIED
	}
}

// IsNotFound returns true if the error wraps a domain "not found" sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, domain.ErrOrderNotFound) ||
		errors.Is(err, domain.ErrOperationNotFound) ||
		errors.Is(err, domain.ErrLotNotFound) ||
		errors.Is(err, domain.ErrSerialNumberNotFound) ||
		errors.Is(err, domain.ErrRoutingNotFound) ||
		errors.Is(err, domain.ErrQualificationNotFound)
}
