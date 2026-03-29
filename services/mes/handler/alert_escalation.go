package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// HandleEscalationRequest consumes internal escalation timer events.
func (h *Handler) HandleEscalationRequest(ctx context.Context, payload []byte) ([]byte, error) {
	var req pbmes.EscalationRequestedEvent
	if err := proto.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("HandleEscalationRequest: unmarshal: %w", err)
	}

	alert, err := h.alerts.FindAlertByID(ctx, req.AlertId)
	if err != nil {
		return nil, fmt.Errorf("HandleEscalationRequest: find alert: %w", err)
	}

	// 1. Check if alert is still ACTIVE. If acknowledged or resolved, stop escalation.
	if alert.Status != domain.AlertStatusActive {
		h.log.Debug().Str("alert_id", alert.ID).Str("status", string(alert.Status)).Msg("skipping escalation: alert no longer active")
		return nil, nil
	}

	oldLevel := string(alert.Level)
	if err := alert.Escalate(); err != nil {
		// Level 3 already reached, stop.
		return nil, nil
	}

	evt, err := proto.Marshal(&pbmes.AlertEscalatedEvent{
		EventId:  uuid.NewString(),
		AlertId:  alert.ID,
		OldLevel: oldLevel,
		NewLevel: string(alert.Level),
	})
	if err != nil {
		return nil, fmt.Errorf("HandleEscalationRequest: marshal event: %w", err)
	}

	// If still not at L3, schedule another escalation
	var nextEscReq []byte
	if alert.Level != domain.AlertLevelL3Director {
		nextEscReq, _ = proto.Marshal(&pbmes.EscalationRequestedEvent{
			AlertId:     alert.ID,
			TargetLevel: pbmes.AlertLevel_ALERT_LEVEL_L3_DIRECTOR,
		})
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateAlert(ctx, alert); err != nil {
			return err
		}
		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "AlertEscalated",
			Subject:   domain.SubjectAlertEscalated,
			Payload:   evt,
		}); err != nil {
			return err
		}
		if nextEscReq != nil {
			return tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "EscalationRequested",
				Subject:   domain.SubjectAlertEscalationRequested,
				Payload:   nextEscReq,
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("HandleEscalationRequest: tx: %w", err)
	}

	h.log.Info().Str("alert_id", alert.ID).Str("from", oldLevel).Str("to", string(alert.Level)).Msg("alert escalated")
	return nil, nil
}
