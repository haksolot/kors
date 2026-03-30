package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// QueryAuditTrail handles kors.mes.audit.query.
// Requires role: admin or quality_manager (enforced at BFF layer).
// Returns audit entries matching the filter, ordered by created_at DESC.
func (h *Handler) QueryAuditTrail(ctx context.Context, payload []byte) ([]byte, error) {
	start := time.Now()

	var req pbmes.QueryAuditTrailRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectAuditQuery, start, fmt.Errorf("QueryAuditTrail: unmarshal: %w", err))
	}

	f := domain.AuditFilter{
		ActorID:    req.ActorId,
		EntityID:   req.EntityId,
		PageSize:   int(req.PageSize),
	}
	if req.EntityType != "" {
		f.EntityType = domain.AuditEntityType(req.EntityType)
	}
	if req.Action != "" {
		f.Action = domain.AuditAction(req.Action)
	}
	if req.From != nil {
		t := req.From.AsTime()
		f.From = &t
	}
	if req.To != nil {
		t := req.To.AsTime()
		f.To = &t
	}

	entries, err := h.audit.QueryAuditTrail(ctx, f)
	if err != nil {
		return h.fail(domain.SubjectAuditQuery, start, fmt.Errorf("QueryAuditTrail: query: %w", err))
	}

	pb := make([]*pbmes.AuditEntry, 0, len(entries))
	for _, e := range entries {
		pb = append(pb, auditEntryToProto(e))
	}

	h.succeed(domain.SubjectAuditQuery, start)
	return proto.Marshal(&pbmes.QueryAuditTrailResponse{Entries: pb})
}

// GetAsBuilt handles kors.mes.compliance.as_built.get.
// Returns the complete As-Built dossier for the given OF (§13 — Dossier Industriel Numérique).
// Requires role: admin, quality_manager, or prod_manager (enforced at BFF layer).
func (h *Handler) GetAsBuilt(ctx context.Context, payload []byte) ([]byte, error) {
	start := time.Now()

	var req pbmes.GetAsBuiltRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectAsBuiltGet, start, fmt.Errorf("GetAsBuilt: unmarshal: %w", err))
	}

	if req.OfId == "" {
		return h.fail(domain.SubjectAsBuiltGet, start, fmt.Errorf("GetAsBuilt: %w", domain.ErrInvalidProductID))
	}

	report, err := h.compliance.GetAsBuiltByOFID(ctx, req.OfId)
	if err != nil {
		if errors.Is(err, domain.ErrAsBuiltNotFound) {
			return h.fail(domain.SubjectAsBuiltGet, start, err)
		}
		return h.fail(domain.SubjectAsBuiltGet, start, fmt.Errorf("GetAsBuilt: %w", err))
	}

	h.succeed(domain.SubjectAsBuiltGet, start)
	return proto.Marshal(asBuiltToProto(report))
}

// ── Proto converters ──────────────────────────────────────────────────────────

func auditEntryToProto(e *domain.AuditEntry) *pbmes.AuditEntry {
	pb := &pbmes.AuditEntry{
		Id:            e.ID,
		ActorId:       e.ActorID,
		ActorRole:     e.ActorRole,
		Action:        string(e.Action),
		EntityType:    string(e.EntityType),
		EntityId:      e.EntityID,
		WorkstationId: e.WorkstationID,
		Notes:         e.Notes,
		CreatedAt:     timestamppb.New(e.CreatedAt),
	}
	return pb
}

func asBuiltToProto(r *domain.AsBuiltReport) *pbmes.GetAsBuiltResponse {
	pb := &pbmes.GetAsBuiltResponse{
		GeneratedAt: timestamppb.New(r.GeneratedAt),
		OrderId:     r.OrderID,
		Reference:   r.Reference,
		ProductId:   r.ProductID,
		Quantity:    int32(r.Quantity),
		Status:      string(r.Status),
		IsFai:       r.IsFAI,
	}
	if r.FAIApprovedBy != "" {
		pb.FaiApprovedBy = r.FAIApprovedBy
	}
	if r.FAIApprovedAt != nil {
		pb.FaiApprovedAt = timestamppb.New(*r.FAIApprovedAt)
	}
	if r.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*r.StartedAt)
	}
	if r.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*r.CompletedAt)
	}
	for _, op := range r.Operations {
		pb.Operations = append(pb.Operations, asBuiltOpToProto(op))
	}
	for _, sn := range r.SerialNumbers {
		pb.SerialNumbers = append(pb.SerialNumbers, &pbmes.AsBuiltSerialNumber{
			Id:        sn.ID,
			Sn:        sn.SN,
			LotId:     sn.LotID,
			ProductId: sn.ProductID,
			Status:    string(sn.Status),
		})
	}
	return pb
}

func asBuiltOpToProto(op *domain.AsBuiltOperation) *pbmes.AsBuiltOperation {
	pb := &pbmes.AsBuiltOperation{
		OperationId:            op.OperationID,
		StepNumber:             int32(op.StepNumber),
		Name:                   op.Name,
		Status:                 string(op.Status),
		OperatorId:             op.OperatorID,
		WorkstationId:          op.WorkstationID,
		RequiresSignOff:        op.RequiresSignOff,
		SignedOffBy:            op.SignedOffBy,
		IsSpecialProcess:       op.IsSpecialProcess,
		NadcapProcessCode:      op.NADCAPProcessCode,
		PlannedDurationSeconds: int32(op.PlannedDurationSeconds),
		ActualDurationSeconds:  int32(op.ActualDurationSeconds),
	}
	if op.SignedOffAt != nil {
		pb.SignedOffAt = timestamppb.New(*op.SignedOffAt)
	}
	if op.StartedAt != nil {
		pb.StartedAt = timestamppb.New(*op.StartedAt)
	}
	if op.CompletedAt != nil {
		pb.CompletedAt = timestamppb.New(*op.CompletedAt)
	}
	for _, m := range op.Measurements {
		pb.Measurements = append(pb.Measurements, &pbmes.AsBuiltMeasurement{
			CharacteristicId: m.CharacteristicID,
			Value:            m.Value,
			Status:           string(m.Status),
			OperatorId:       m.OperatorID,
			RecordedAt:       timestamppb.New(m.RecordedAt),
		})
	}
	for _, c := range op.ConsumedLots {
		pb.ConsumedLots = append(pb.ConsumedLots, &pbmes.AsBuiltConsumedLot{
			LotId:    c.LotID,
			Quantity: int32(c.Quantity),
		})
	}
	for _, t := range op.Tools {
		tool := &pbmes.AsBuiltTool{
			ToolId:       t.ToolID,
			SerialNumber: t.SerialNumber,
			Name:         t.Name,
		}
		if t.CalibrationExpiry != nil {
			tool.CalibrationExpiry = timestamppb.New(*t.CalibrationExpiry)
		}
		pb.Tools = append(pb.Tools, tool)
	}
	return pb
}
