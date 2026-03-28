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
	pbqms "github.com/haksolot/kors/proto/gen/qms"
	"github.com/haksolot/kors/services/qms/domain"
)

// NCRepository is the read-only persistence interface for NonConformities.
type NCRepository interface {
	FindNCByID(ctx context.Context, id string) (*domain.NonConformity, error)
	ListNCs(ctx context.Context, filter domain.ListNCsFilter) ([]*domain.NonConformity, error)
}

// CAPARepository is the read-only persistence interface for CAPAs.
type CAPARepository interface {
	FindCAPAByID(ctx context.Context, id string) (*domain.CAPA, error)
	ListCAPAs(ctx context.Context, filter domain.ListCAPAsFilter) ([]*domain.CAPA, error)
}

// Handler processes NATS request-reply messages for the QMS service.
// All state-changing operations use domain.Transactor to guarantee atomicity
// between business data and the outbox entry (ADR-004).
type Handler struct {
	ncs   NCRepository
	capas CAPARepository
	store domain.Transactor
	log   *zerolog.Logger

	reqTotal    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}

// New returns a Handler with the provided dependencies injected.
func New(
	ncs NCRepository,
	capas CAPARepository,
	store domain.Transactor,
	reg prometheus.Registerer,
	log *zerolog.Logger,
) *Handler {
	return &Handler{
		ncs:         ncs,
		capas:       capas,
		store:       store,
		log:         log,
		reqTotal:    core.NewCounter(reg, "qms", "handler_requests", "Total NATS handler invocations", []string{"subject", "status"}),
		reqDuration: core.NewHistogram(reg, "qms", "handler_duration_seconds", "NATS handler latency", []string{"subject"}, nil),
	}
}

// ── NonConformities ───────────────────────────────────────────────────────────

// GetNC handles kors.qms.nc.get.
func (h *Handler) GetNC(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetNC")
	defer span.End()
	start := time.Now()

	var req pbqms.GetNCRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectNCGet, start, fmt.Errorf("GetNC: unmarshal: %w", err))
	}

	nc, err := h.ncs.FindNCByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectNCGet, start, fmt.Errorf("GetNC: %w", err))
	}

	h.succeed(domain.SubjectNCGet, start)
	return proto.Marshal(&pbqms.GetNCResponse{Nc: ncToProto(nc)})
}

// ListNCs handles kors.qms.nc.list.
func (h *Handler) ListNCs(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListNCs")
	defer span.End()
	start := time.Now()

	var req pbqms.ListNCsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectNCList, start, fmt.Errorf("ListNCs: unmarshal: %w", err))
	}

	filter := domain.ListNCsFilter{PageSize: int(req.PageSize)}
	if req.StatusFilter != pbqms.NCStatus_NC_STATUS_UNSPECIFIED {
		s := protoStatusToDomain(req.StatusFilter)
		filter.Status = &s
	}

	ncs, err := h.ncs.ListNCs(ctx, filter)
	if err != nil {
		return h.fail(domain.SubjectNCList, start, fmt.Errorf("ListNCs: %w", err))
	}

	resp := &pbqms.ListNCsResponse{}
	for _, nc := range ncs {
		resp.Ncs = append(resp.Ncs, ncToProto(nc))
	}
	h.succeed(domain.SubjectNCList, start)
	return proto.Marshal(resp)
}

// StartAnalysis handles kors.qms.nc.analyse — OPEN → UNDER_ANALYSIS.
func (h *Handler) StartAnalysis(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.StartAnalysis")
	defer span.End()
	start := time.Now()

	var req pbqms.StartAnalysisRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectNCAnalyse, start, fmt.Errorf("StartAnalysis: unmarshal: %w", err))
	}

	nc, err := h.ncs.FindNCByID(ctx, req.NcId)
	if err != nil {
		return h.fail(domain.SubjectNCAnalyse, start, fmt.Errorf("StartAnalysis: %w", err))
	}

	if err := nc.StartAnalysis(req.AnalystId); err != nil {
		return h.fail(domain.SubjectNCAnalyse, start, fmt.Errorf("StartAnalysis: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateNC(ctx, nc)
	}); err != nil {
		return h.fail(domain.SubjectNCAnalyse, start, fmt.Errorf("StartAnalysis: tx: %w", err))
	}

	h.log.Info().Str("nc_id", nc.ID).Str("analyst_id", req.AnalystId).Msg("NC analysis started")
	h.succeed(domain.SubjectNCAnalyse, start)
	return proto.Marshal(&pbqms.StartAnalysisResponse{Nc: ncToProto(nc)})
}

// ProposeDisposition handles kors.qms.nc.propose_disposition — UNDER_ANALYSIS → PENDING_DISPOSITION.
func (h *Handler) ProposeDisposition(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ProposeDisposition")
	defer span.End()
	start := time.Now()

	var req pbqms.ProposeDispositionRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectNCProposeDisposition, start, fmt.Errorf("ProposeDisposition: unmarshal: %w", err))
	}

	nc, err := h.ncs.FindNCByID(ctx, req.NcId)
	if err != nil {
		return h.fail(domain.SubjectNCProposeDisposition, start, fmt.Errorf("ProposeDisposition: %w", err))
	}

	disposition := protoDispositionToDomain(req.Disposition)
	if err := nc.ProposeDisposition(disposition, req.AnalystId); err != nil {
		return h.fail(domain.SubjectNCProposeDisposition, start, fmt.Errorf("ProposeDisposition: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateNC(ctx, nc)
	}); err != nil {
		return h.fail(domain.SubjectNCProposeDisposition, start, fmt.Errorf("ProposeDisposition: tx: %w", err))
	}

	h.log.Info().Str("nc_id", nc.ID).Str("disposition", string(disposition)).Msg("NC disposition proposed")
	h.succeed(domain.SubjectNCProposeDisposition, start)
	return proto.Marshal(&pbqms.ProposeDispositionResponse{Nc: ncToProto(nc)})
}

// CloseNC handles kors.qms.nc.close — PENDING_DISPOSITION → CLOSED.
func (h *Handler) CloseNC(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CloseNC")
	defer span.End()
	start := time.Now()

	var req pbqms.CloseNCRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectNCClose, start, fmt.Errorf("CloseNC: unmarshal: %w", err))
	}

	nc, err := h.ncs.FindNCByID(ctx, req.NcId)
	if err != nil {
		return h.fail(domain.SubjectNCClose, start, fmt.Errorf("CloseNC: %w", err))
	}

	if err := nc.Close(req.ClosedBy); err != nil {
		return h.fail(domain.SubjectNCClose, start, fmt.Errorf("CloseNC: %w", err))
	}

	evt, err := proto.Marshal(&pbqms.NCClosedEvent{
		EventId:     uuid.NewString(),
		NcId:        nc.ID,
		ClosedBy:    nc.ClosedBy,
		Disposition: domainDispositionToProto(nc.Disposition),
		ClosedAt:    timestamppb.New(*nc.ClosedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectNCClose, start, fmt.Errorf("CloseNC: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateNC(ctx, nc); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "NCClosed",
			Subject:   domain.SubjectNCClosed,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectNCClose, start, fmt.Errorf("CloseNC: tx: %w", err))
	}

	h.log.Info().Str("nc_id", nc.ID).Str("closed_by", nc.ClosedBy).Msg("NC closed")
	h.succeed(domain.SubjectNCClose, start)
	return proto.Marshal(&pbqms.CloseNCResponse{Nc: ncToProto(nc)})
}

// ── CAPAs ─────────────────────────────────────────────────────────────────────

// CreateCAPA handles kors.qms.capa.create.
func (h *Handler) CreateCAPA(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateCAPA")
	defer span.End()
	start := time.Now()

	var req pbqms.CreateCAPARequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectCAPACreate, start, fmt.Errorf("CreateCAPA: unmarshal: %w", err))
	}

	actionType := protoActionTypeToDomain(req.ActionType)

	var dueDate *time.Time
	if req.DueDate != nil {
		t := req.DueDate.AsTime()
		dueDate = &t
	}

	capa, err := domain.NewCAPA(req.NcId, actionType, req.Description, req.OwnerId, dueDate)
	if err != nil {
		return h.fail(domain.SubjectCAPACreate, start, fmt.Errorf("CreateCAPA: %w", err))
	}

	evt, err := proto.Marshal(&pbqms.CAPACreatedEvent{
		EventId:    uuid.NewString(),
		CapaId:     capa.ID,
		NcId:       capa.NCID,
		ActionType: req.ActionType,
		OwnerId:    capa.OwnerID,
		CreatedAt:  timestamppb.New(capa.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectCAPACreate, start, fmt.Errorf("CreateCAPA: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveCAPA(ctx, capa); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "CAPACreated",
			Subject:   domain.SubjectCAPACreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectCAPACreate, start, fmt.Errorf("CreateCAPA: tx: %w", err))
	}

	h.log.Info().Str("capa_id", capa.ID).Str("nc_id", capa.NCID).Msg("CAPA created")
	h.succeed(domain.SubjectCAPACreate, start)
	return proto.Marshal(&pbqms.CreateCAPAResponse{Capa: capaToProto(capa)})
}

// GetCAPA handles kors.qms.capa.get.
func (h *Handler) GetCAPA(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetCAPA")
	defer span.End()
	start := time.Now()

	var req pbqms.GetCAPARequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectCAPAGet, start, fmt.Errorf("GetCAPA: unmarshal: %w", err))
	}

	capa, err := h.capas.FindCAPAByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectCAPAGet, start, fmt.Errorf("GetCAPA: %w", err))
	}

	h.succeed(domain.SubjectCAPAGet, start)
	return proto.Marshal(&pbqms.GetCAPAResponse{Capa: capaToProto(capa)})
}

// ListCAPAs handles kors.qms.capa.list.
func (h *Handler) ListCAPAs(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListCAPAs")
	defer span.End()
	start := time.Now()

	var req pbqms.ListCAPAsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectCAPAList, start, fmt.Errorf("ListCAPAs: unmarshal: %w", err))
	}

	filter := domain.ListCAPAsFilter{NCID: req.NcId, PageSize: int(req.PageSize)}
	if req.StatusFilter != pbqms.CAPAStatus_CAPA_STATUS_UNSPECIFIED {
		s := protoCAPAStatusToDomain(req.StatusFilter)
		filter.Status = &s
	}

	capas, err := h.capas.ListCAPAs(ctx, filter)
	if err != nil {
		return h.fail(domain.SubjectCAPAList, start, fmt.Errorf("ListCAPAs: %w", err))
	}

	resp := &pbqms.ListCAPAsResponse{}
	for _, c := range capas {
		resp.Capas = append(resp.Capas, capaToProto(c))
	}
	h.succeed(domain.SubjectCAPAList, start)
	return proto.Marshal(resp)
}

// StartCAPA handles kors.qms.capa.start — OPEN → IN_PROGRESS.
func (h *Handler) StartCAPA(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.StartCAPA")
	defer span.End()
	start := time.Now()

	var req pbqms.StartCAPARequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectCAPAStart, start, fmt.Errorf("StartCAPA: unmarshal: %w", err))
	}

	capa, err := h.capas.FindCAPAByID(ctx, req.CapaId)
	if err != nil {
		return h.fail(domain.SubjectCAPAStart, start, fmt.Errorf("StartCAPA: %w", err))
	}

	if err := capa.Start(); err != nil {
		return h.fail(domain.SubjectCAPAStart, start, fmt.Errorf("StartCAPA: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateCAPA(ctx, capa)
	}); err != nil {
		return h.fail(domain.SubjectCAPAStart, start, fmt.Errorf("StartCAPA: tx: %w", err))
	}

	h.log.Info().Str("capa_id", capa.ID).Msg("CAPA started")
	h.succeed(domain.SubjectCAPAStart, start)
	return proto.Marshal(&pbqms.StartCAPAResponse{Capa: capaToProto(capa)})
}

// CompleteCAPA handles kors.qms.capa.complete — IN_PROGRESS → COMPLETED.
func (h *Handler) CompleteCAPA(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CompleteCAPA")
	defer span.End()
	start := time.Now()

	var req pbqms.CompleteCAPARequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectCAPAComplete, start, fmt.Errorf("CompleteCAPA: unmarshal: %w", err))
	}

	capa, err := h.capas.FindCAPAByID(ctx, req.CapaId)
	if err != nil {
		return h.fail(domain.SubjectCAPAComplete, start, fmt.Errorf("CompleteCAPA: %w", err))
	}

	if err := capa.Complete(); err != nil {
		return h.fail(domain.SubjectCAPAComplete, start, fmt.Errorf("CompleteCAPA: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateCAPA(ctx, capa)
	}); err != nil {
		return h.fail(domain.SubjectCAPAComplete, start, fmt.Errorf("CompleteCAPA: tx: %w", err))
	}

	h.log.Info().Str("capa_id", capa.ID).Msg("CAPA completed")
	h.succeed(domain.SubjectCAPAComplete, start)
	return proto.Marshal(&pbqms.CompleteCAPAResponse{Capa: capaToProto(capa)})
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

// IsNotFound returns true if the error wraps a domain "not found" sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, domain.ErrNCNotFound) ||
		errors.Is(err, domain.ErrCAPANotFound)
}

// ── Converters ────────────────────────────────────────────────────────────────

func ncToProto(nc *domain.NonConformity) *pbqms.NonConformity {
	pb := &pbqms.NonConformity{
		Id:               nc.ID,
		OperationId:      nc.OperationID,
		OfId:             nc.OFID,
		DefectCode:       nc.DefectCode,
		Description:      nc.Description,
		AffectedQuantity: int32(nc.AffectedQuantity),
		SerialNumbers:    nc.SerialNumbers,
		DeclaredBy:       nc.DeclaredBy,
		Status:           domainNCStatusToProto(nc.Status),
		Disposition:      domainDispositionToProto(nc.Disposition),
		ClosedBy:         nc.ClosedBy,
		CreatedAt:        timestamppb.New(nc.CreatedAt),
		UpdatedAt:        timestamppb.New(nc.UpdatedAt),
	}
	if nc.ClosedAt != nil {
		pb.ClosedAt = timestamppb.New(*nc.ClosedAt)
	}
	return pb
}

func capaToProto(c *domain.CAPA) *pbqms.CAPA {
	pb := &pbqms.CAPA{
		Id:          c.ID,
		NcId:        c.NCID,
		ActionType:  domainActionTypeToProto(c.ActionType),
		Description: c.Description,
		OwnerId:     c.OwnerID,
		Status:      domainCAPAStatusToProto(c.Status),
		CreatedAt:   timestamppb.New(c.CreatedAt),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
	}
	if c.DueDate != nil {
		pb.DueDate = timestamppb.New(*c.DueDate)
	}
	if c.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*c.CompletedAt)
	}
	return pb
}

func domainNCStatusToProto(s domain.NCStatus) pbqms.NCStatus {
	switch s {
	case domain.NCStatusOpen:
		return pbqms.NCStatus_NC_STATUS_OPEN
	case domain.NCStatusUnderAnalysis:
		return pbqms.NCStatus_NC_STATUS_UNDER_ANALYSIS
	case domain.NCStatusPendingDisposition:
		return pbqms.NCStatus_NC_STATUS_PENDING_DISPOSITION
	case domain.NCStatusClosed:
		return pbqms.NCStatus_NC_STATUS_CLOSED
	default:
		return pbqms.NCStatus_NC_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomain(s pbqms.NCStatus) domain.NCStatus {
	switch s {
	case pbqms.NCStatus_NC_STATUS_OPEN:
		return domain.NCStatusOpen
	case pbqms.NCStatus_NC_STATUS_UNDER_ANALYSIS:
		return domain.NCStatusUnderAnalysis
	case pbqms.NCStatus_NC_STATUS_PENDING_DISPOSITION:
		return domain.NCStatusPendingDisposition
	case pbqms.NCStatus_NC_STATUS_CLOSED:
		return domain.NCStatusClosed
	default:
		return domain.NCStatusOpen
	}
}

func domainDispositionToProto(d domain.NCDisposition) pbqms.NCDisposition {
	switch d {
	case domain.NCDispositionRework:
		return pbqms.NCDisposition_NC_DISPOSITION_REWORK
	case domain.NCDispositionScrap:
		return pbqms.NCDisposition_NC_DISPOSITION_SCRAP
	case domain.NCDispositionUseAsIs:
		return pbqms.NCDisposition_NC_DISPOSITION_USE_AS_IS
	case domain.NCDispositionReturnToSupplier:
		return pbqms.NCDisposition_NC_DISPOSITION_RETURN_TO_SUPPLIER
	default:
		return pbqms.NCDisposition_NC_DISPOSITION_UNSPECIFIED
	}
}

func protoDispositionToDomain(d pbqms.NCDisposition) domain.NCDisposition {
	switch d {
	case pbqms.NCDisposition_NC_DISPOSITION_REWORK:
		return domain.NCDispositionRework
	case pbqms.NCDisposition_NC_DISPOSITION_SCRAP:
		return domain.NCDispositionScrap
	case pbqms.NCDisposition_NC_DISPOSITION_USE_AS_IS:
		return domain.NCDispositionUseAsIs
	case pbqms.NCDisposition_NC_DISPOSITION_RETURN_TO_SUPPLIER:
		return domain.NCDispositionReturnToSupplier
	default:
		return domain.NCDispositionUnspecified
	}
}

func domainCAPAStatusToProto(s domain.CAPAStatus) pbqms.CAPAStatus {
	switch s {
	case domain.CAPAStatusOpen:
		return pbqms.CAPAStatus_CAPA_STATUS_OPEN
	case domain.CAPAStatusInProgress:
		return pbqms.CAPAStatus_CAPA_STATUS_IN_PROGRESS
	case domain.CAPAStatusCompleted:
		return pbqms.CAPAStatus_CAPA_STATUS_COMPLETED
	case domain.CAPAStatusCancelled:
		return pbqms.CAPAStatus_CAPA_STATUS_CANCELLED
	default:
		return pbqms.CAPAStatus_CAPA_STATUS_UNSPECIFIED
	}
}

func protoCAPAStatusToDomain(s pbqms.CAPAStatus) domain.CAPAStatus {
	switch s {
	case pbqms.CAPAStatus_CAPA_STATUS_OPEN:
		return domain.CAPAStatusOpen
	case pbqms.CAPAStatus_CAPA_STATUS_IN_PROGRESS:
		return domain.CAPAStatusInProgress
	case pbqms.CAPAStatus_CAPA_STATUS_COMPLETED:
		return domain.CAPAStatusCompleted
	case pbqms.CAPAStatus_CAPA_STATUS_CANCELLED:
		return domain.CAPAStatusCancelled
	default:
		return domain.CAPAStatusOpen
	}
}

func domainActionTypeToProto(a domain.CAPAActionType) pbqms.CAPAActionType {
	switch a {
	case domain.CAPAActionCorrective:
		return pbqms.CAPAActionType_CAPA_ACTION_TYPE_CORRECTIVE
	case domain.CAPAActionPreventive:
		return pbqms.CAPAActionType_CAPA_ACTION_TYPE_PREVENTIVE
	default:
		return pbqms.CAPAActionType_CAPA_ACTION_TYPE_UNSPECIFIED
	}
}

func protoActionTypeToDomain(a pbqms.CAPAActionType) domain.CAPAActionType {
	switch a {
	case pbqms.CAPAActionType_CAPA_ACTION_TYPE_CORRECTIVE:
		return domain.CAPAActionCorrective
	case pbqms.CAPAActionType_CAPA_ACTION_TYPE_PREVENTIVE:
		return domain.CAPAActionPreventive
	default:
		return domain.CAPAActionUnspecified
	}
}
