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

// ── Inline Quality (BLOC 10) ──────────────────────────────────────────────────

// RecordMeasurement handles kors.mes.measurement.record.
func (h *Handler) RecordMeasurement(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RecordMeasurement")
	defer span.End()
	start := time.Now()

	var req pbmes.RecordMeasurementRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMeasurementRecord, start, fmt.Errorf("RecordMeasurement: unmarshal: %w", err))
	}

	char, err := h.quality.FindCharacteristicByID(ctx, req.CharacteristicId)
	if err != nil {
		return h.fail(domain.SubjectMeasurementRecord, start, fmt.Errorf("RecordMeasurement: find characteristic: %w", err))
	}

	meas, err := domain.NewMeasurement(req.OperationId, req.CharacteristicId, req.Value, req.OperatorId, char)
	if err != nil {
		return h.fail(domain.SubjectMeasurementRecord, start, fmt.Errorf("RecordMeasurement: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.MeasurementRecordedEvent{
		EventId:          uuid.NewString(),
		MeasurementId:    meas.ID,
		OperationId:      meas.OperationID,
		CharacteristicId: meas.CharacteristicID,
		Value:            meas.Value,
		Status:           string(meas.Status),
	})
	if err != nil {
		return h.fail(domain.SubjectMeasurementRecord, start, fmt.Errorf("RecordMeasurement: marshal event: %w", err))
	}

	// ── Alert & SPC logic ─────────────────────────────────────────────────────
	var alert *domain.Alert
	var alertEvt, escReq []byte

	if meas.Status == domain.MeasurementStatusFail {
		alert, _ = domain.NewAlert(domain.AlertCategoryQuality, nil, &meas.OperationID, fmt.Sprintf("Quality check failed: %s", char.Name))
		alertEvt, _ = proto.Marshal(&pbmes.AlertRaisedEvent{
			EventId:  uuid.NewString(),
			AlertId:  alert.ID,
			Category: string(alert.Category),
			Level:    string(alert.Level),
			Message:  alert.Message,
		})
		escReq, _ = proto.Marshal(&pbmes.EscalationRequestedEvent{
			AlertId:     alert.ID,
			TargetLevel: pbmes.AlertLevel_ALERT_LEVEL_L2_MANAGER,
		})
	}

	// SPC Drift Check
	history, _ := h.quality.ListMeasurementsByCharacteristic(ctx, meas.CharacteristicID, 5)
	driftAlert := domain.CheckSPCDrift(append([]*domain.Measurement{meas}, history...))

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveMeasurement(ctx, meas); err != nil {
			return err
		}
		if alert != nil {
			if err := tx.SaveAlert(ctx, alert); err != nil {
				return err
			}
			if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "AlertRaised",
				Subject:   domain.SubjectAlertRaised,
				Payload:   alertEvt,
			}); err != nil {
				return err
			}
			if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "EscalationRequested",
				Subject:   domain.SubjectAlertEscalationRequested,
				Payload:   escReq,
			}); err != nil {
				return err
			}
		}
		if driftAlert {
			alertEvt, _ := proto.Marshal(&pbmes.QualityAlertRaisedEvent{
				EventId:          uuid.NewString(),
				Type:             "SPC_DRIFT",
				OperationId:      meas.OperationID,
				CharacteristicId: meas.CharacteristicID,
				Message:          fmt.Sprintf("SPC drift detected for %s", char.Name),
			})
			if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
				EventType: "QualityAlertRaised",
				Subject:   domain.SubjectQualityAlertRaised,
				Payload:   alertEvt,
			}); err != nil {
				return err
			}
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "MeasurementRecorded",
			Subject:   domain.SubjectMeasurementRecorded,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectMeasurementRecord, start, fmt.Errorf("RecordMeasurement: tx: %w", err))
	}

	h.log.Info().Str("meas_id", meas.ID).Str("status", string(meas.Status)).Msg("measurement recorded")
	h.succeed(domain.SubjectMeasurementRecord, start)
	return proto.Marshal(&pbmes.RecordMeasurementResponse{Measurement: measurementToProto(meas)})
}

// ListOperationCharacteristics handles kors.mes.operation.characteristics.list.
func (h *Handler) ListOperationCharacteristics(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListOperationCharacteristics")
	defer span.End()
	start := time.Now()

	var req pbmes.GetOperationCharacteristicsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationCharacteristicsList, start, fmt.Errorf("ListOperationCharacteristics: unmarshal: %w", err))
	}

	chars, err := h.quality.ListCharacteristicsByOperation(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationCharacteristicsList, start, fmt.Errorf("ListOperationCharacteristics: %w", err))
	}

	pbChars := make([]*pbmes.ControlCharacteristic, 0, len(chars))
	for _, c := range chars {
		pbChars = append(pbChars, characteristicToProto(c))
	}
	h.succeed(domain.SubjectOperationCharacteristicsList, start)
	return proto.Marshal(&pbmes.GetOperationCharacteristicsResponse{Characteristics: pbChars})
}

// ── Converters ────────────────────────────────────────────────────────────────

func characteristicToProto(c *domain.ControlCharacteristic) *pbmes.ControlCharacteristic {
	pb := &pbmes.ControlCharacteristic{
		Id:          c.ID,
		StepId:      c.StepID,
		Name:        c.Name,
		Unit:        c.Unit,
		IsMandatory: c.IsMandatory,
	}
	if c.Type == domain.CharacteristicTypeQuantitative {
		pb.Type = pbmes.CharacteristicType_CHARACTERISTIC_TYPE_QUANTITATIVE
	} else {
		pb.Type = pbmes.CharacteristicType_CHARACTERISTIC_TYPE_QUALITATIVE
	}
	if c.NominalValue != nil {
		pb.NominalValue = *c.NominalValue
	}
	if c.UpperTolerance != nil {
		pb.UpperTolerance = *c.UpperTolerance
	}
	if c.LowerTolerance != nil {
		pb.LowerTolerance = *c.LowerTolerance
	}
	return pb
}

func measurementToProto(m *domain.Measurement) *pbmes.Measurement {
	pb := &pbmes.Measurement{
		Id:               m.ID,
		OperationId:      m.OperationID,
		CharacteristicId: m.CharacteristicID,
		Value:            m.Value,
		OperatorId:       m.OperatorID,
		RecordedAt:       timestamppb.New(m.RecordedAt),
	}
	switch m.Status {
	case domain.MeasurementStatusPass:
		pb.Status = pbmes.MeasurementStatus_MEASUREMENT_STATUS_PASS
	case domain.MeasurementStatusFail:
		pb.Status = pbmes.MeasurementStatus_MEASUREMENT_STATUS_FAIL
	case domain.MeasurementStatusWarning:
		pb.Status = pbmes.MeasurementStatus_MEASUREMENT_STATUS_WARNING
	}
	return pb
}
