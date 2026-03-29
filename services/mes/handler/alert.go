package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// ── Alerts (BLOC 11) ──────────────────────────────────────────────────────────

// RaiseAlert handles kors.mes.alert.raise.
func (h *Handler) RaiseAlert(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RaiseAlert")
	defer span.End()
	start := time.Now()

	var req pbmes.RaiseAlertRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectAlertRaise, start, fmt.Errorf("RaiseAlert: unmarshal: %w", err))
	}

	cat := domain.AlertCategoryMachine
	switch req.Category {
	case pbmes.AlertCategory_ALERT_CATEGORY_QUALITY:
		cat = domain.AlertCategoryQuality
	case pbmes.AlertCategory_ALERT_CATEGORY_PLANNING:
		cat = domain.AlertCategoryPlanning
	case pbmes.AlertCategory_ALERT_CATEGORY_LOGISTICS:
		cat = domain.AlertCategoryLogistics
	}

	var wsID, opID *string
	if req.WorkstationId != "" {
		wsID = &req.WorkstationId
	}
	if req.OperationId != "" {
		opID = &req.OperationId
	}

	alert, err := domain.NewAlert(cat, wsID, opID, req.Message)
	if err != nil {
		return h.fail(domain.SubjectAlertRaise, start, fmt.Errorf("RaiseAlert: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.AlertRaisedEvent{
		EventId:   uuid.NewString(),
		AlertId:   alert.ID,
		Category:  string(alert.Category),
		Level:     string(alert.Level),
		Message:   alert.Message,
	})
	if err != nil {
		return h.fail(domain.SubjectAlertRaise, start, fmt.Errorf("RaiseAlert: marshal event: %w", err))
	}

	// Internal event for escalation timer
	escReq, err := proto.Marshal(&pbmes.EscalationRequestedEvent{
		AlertId:     alert.ID,
		TargetLevel: pbmes.AlertLevel_ALERT_LEVEL_L2_MANAGER,
	})
	if err != nil {
		return h.fail(domain.SubjectAlertRaise, start, fmt.Errorf("RaiseAlert: marshal escalation req: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveAlert(ctx, alert); err != nil {
			return err
		}
		// Notification event
		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "AlertRaised",
			Subject:   domain.SubjectAlertRaised,
			Payload:   evt,
		}); err != nil {
			return err
		}
		// Escalation timer event (published with delay in a real system, 
		// here we rely on the consumer logic or headers if supported by infrastructure)
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "EscalationRequested",
			Subject:   domain.SubjectAlertEscalationRequested,
			Payload:   escReq,
		})
	}); err != nil {
		return h.fail(domain.SubjectAlertRaise, start, fmt.Errorf("RaiseAlert: tx: %w", err))
	}

	h.log.Info().Str("alert_id", alert.ID).Str("level", string(alert.Level)).Msg("alert raised")
	h.succeed(domain.SubjectAlertRaise, start)
	return proto.Marshal(&pbmes.RaiseAlertResponse{Alert: alertToProto(alert)})
}

// AcknowledgeAlert handles kors.mes.alert.acknowledge.
func (h *Handler) AcknowledgeAlert(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.AcknowledgeAlert")
	defer span.End()
	start := time.Now()

	var req pbmes.AcknowledgeAlertRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectAlertAcknowledge, start, fmt.Errorf("AcknowledgeAlert: unmarshal: %w", err))
	}

	alert, err := h.alerts.FindAlertByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectAlertAcknowledge, start, fmt.Errorf("AcknowledgeAlert: %w", err))
	}

	if err := alert.Acknowledge(req.UserId); err != nil {
		return h.fail(domain.SubjectAlertAcknowledge, start, fmt.Errorf("AcknowledgeAlert: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateAlert(ctx, alert)
	}); err != nil {
		return h.fail(domain.SubjectAlertAcknowledge, start, fmt.Errorf("AcknowledgeAlert: tx: %w", err))
	}

	h.log.Info().Str("alert_id", alert.ID).Str("user", req.UserId).Msg("alert acknowledged")
	h.succeed(domain.SubjectAlertAcknowledge, start)
	return proto.Marshal(&pbmes.AcknowledgeAlertResponse{Alert: alertToProto(alert)})
}

// ResolveAlert handles kors.mes.alert.resolve.
func (h *Handler) ResolveAlert(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ResolveAlert")
	defer span.End()
	start := time.Now()

	var req pbmes.ResolveAlertRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectAlertResolve, start, fmt.Errorf("ResolveAlert: unmarshal: %w", err))
	}

	alert, err := h.alerts.FindAlertByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectAlertResolve, start, fmt.Errorf("ResolveAlert: %w", err))
	}

	if err := alert.Resolve(req.UserId, req.Notes); err != nil {
		return h.fail(domain.SubjectAlertResolve, start, fmt.Errorf("ResolveAlert: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.AlertResolvedEvent{
		EventId:    uuid.NewString(),
		AlertId:    alert.ID,
		ResolvedBy: req.UserId,
	})
	if err != nil {
		return h.fail(domain.SubjectAlertResolve, start, fmt.Errorf("ResolveAlert: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateAlert(ctx, alert); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "AlertResolved",
			Subject:   domain.SubjectAlertResolved,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectAlertResolve, start, fmt.Errorf("ResolveAlert: tx: %w", err))
	}

	h.log.Info().Str("alert_id", alert.ID).Str("user", req.UserId).Msg("alert resolved")
	h.succeed(domain.SubjectAlertResolve, start)
	return proto.Marshal(&pbmes.ResolveAlertResponse{Alert: alertToProto(alert)})
}

// ListActiveAlerts handles kors.mes.alert.list_active.
func (h *Handler) ListActiveAlerts(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListActiveAlerts")
	defer span.End()
	start := time.Now()

	alerts, err := h.alerts.ListActiveAlerts(ctx)
	if err != nil {
		return h.fail(domain.SubjectAlertListActive, start, fmt.Errorf("ListActiveAlerts: %w", err))
	}

	pbAlerts := make([]*pbmes.Alert, 0, len(alerts))
	for _, a := range alerts {
		pbAlerts = append(pbAlerts, alertToProto(a))
	}
	h.succeed(domain.SubjectAlertListActive, start)
	return proto.Marshal(&pbmes.ListActiveAlertsResponse{Alerts: pbAlerts})
}

// ── Converters ────────────────────────────────────────────────────────────────

func alertToProto(a *domain.Alert) *pbmes.Alert {
	pb := &pbmes.Alert{
		Id:              a.ID,
		Category:        domainAlertCategoryToProto(a.Category),
		Level:           domainAlertLevelToProto(a.Level),
		Status:          domainAlertStatusToProto(a.Status),
		Message:         a.Message,
		EscalationCount: int32(a.EscalationCount),
		ResolutionNotes: a.ResolutionNotes,
		CreatedAt:       timestamppb.New(a.CreatedAt),
		UpdatedAt:       timestamppb.New(a.UpdatedAt),
	}
	if a.WorkstationID != nil {
		pb.WorkstationId = *a.WorkstationID
	}
	if a.OperationID != nil {
		pb.OperationId = *a.OperationID
	}
	if a.AcknowledgedBy != nil {
		pb.AcknowledgedBy = *a.AcknowledgedBy
		pb.AcknowledgedAt = timestamppb.New(*a.AcknowledgedAt)
	}
	if a.ResolvedBy != nil {
		pb.ResolvedBy = *a.ResolvedBy
		pb.ResolvedAt = timestamppb.New(*a.ResolvedAt)
	}
	return pb
}

func domainAlertCategoryToProto(c domain.AlertCategory) pbmes.AlertCategory {
	switch c {
	case domain.AlertCategoryMachine:
		return pbmes.AlertCategory_ALERT_CATEGORY_MACHINE
	case domain.AlertCategoryQuality:
		return pbmes.AlertCategory_ALERT_CATEGORY_QUALITY
	case domain.AlertCategoryPlanning:
		return pbmes.AlertCategory_ALERT_CATEGORY_PLANNING
	case domain.AlertCategoryLogistics:
		return pbmes.AlertCategory_ALERT_CATEGORY_LOGISTICS
	default:
		return pbmes.AlertCategory_ALERT_CATEGORY_UNSPECIFIED
	}
}

func domainAlertLevelToProto(l domain.AlertLevel) pbmes.AlertLevel {
	switch l {
	case domain.AlertLevelL1Supervisor:
		return pbmes.AlertLevel_ALERT_LEVEL_L1_SUPERVISOR
	case domain.AlertLevelL2Manager:
		return pbmes.AlertLevel_ALERT_LEVEL_L2_MANAGER
	case domain.AlertLevelL3Director:
		return pbmes.AlertLevel_ALERT_LEVEL_L3_DIRECTOR
	default:
		return pbmes.AlertLevel_ALERT_LEVEL_UNSPECIFIED
	}
}

func domainAlertStatusToProto(s domain.AlertStatus) pbmes.AlertStatus {
	switch s {
	case domain.AlertStatusActive:
		return pbmes.AlertStatus_ALERT_STATUS_ACTIVE
	case domain.AlertStatusAcknowledged:
		return pbmes.AlertStatus_ALERT_STATUS_ACKNOWLEDGED
	case domain.AlertStatusResolved:
		return pbmes.AlertStatus_ALERT_STATUS_RESOLVED
	default:
		return pbmes.AlertStatus_ALERT_STATUS_UNSPECIFIED
	}
}
