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

// ── Materials & WIP (BLOC 9) ──────────────────────────────────────────────────

// ConsumeMaterial handles kors.mes.material.consume.
func (h *Handler) ConsumeMaterial(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ConsumeMaterial")
	defer span.End()
	start := time.Now()

	var req pbmes.ConsumeMaterialRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("ConsumeMaterial: unmarshal: %w", err))
	}

	// 1. Verify Lot exists and is valid for consumption
	lot, err := h.trace.FindLotByID(ctx, req.LotId)
	if err != nil {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("ConsumeMaterial: find lot: %w", err))
	}

	now := time.Now().UTC()
	if lot.IsBlocked() {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("lot %s is BLOCKED", lot.Reference))
	}
	if lot.IsExpired(now) {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("lot %s is EXPIRED", lot.Reference))
	}
	if lot.TOERemainingMinutes() <= 0 {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("lot %s has exceeded TOE limit", lot.Reference))
	}

	// 2. Create consumption record
	rec, err := domain.NewConsumptionRecord(req.LotId, req.OperationId, int(req.Quantity), req.OperatorId)
	if err != nil {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("ConsumeMaterial: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.MaterialConsumedEvent{
		EventId:     uuid.NewString(),
		LotId:       rec.LotID,
		OperationId: rec.OperationID,
		Quantity:    int32(rec.Quantity),
	})
	if err != nil {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("ConsumeMaterial: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveConsumptionRecord(ctx, rec); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "MaterialConsumed",
			Subject:   domain.SubjectMaterialConsumed,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectMaterialConsume, start, fmt.Errorf("ConsumeMaterial: tx: %w", err))
	}

	h.log.Info().Str("lot_id", rec.LotID).Str("op_id", rec.OperationID).Int("qty", rec.Quantity).Msg("material consumed")
	h.succeed(domain.SubjectMaterialConsume, start)
	return proto.Marshal(&pbmes.ConsumeMaterialResponse{Record: consumptionToProto(rec)})
}

// StartTOEExposure handles kors.mes.material.toe.start.
func (h *Handler) StartTOEExposure(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.StartTOEExposure")
	defer span.End()
	start := time.Now()

	var req pbmes.StartTOEExposureRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMaterialTOEStart, start, fmt.Errorf("StartTOEExposure: unmarshal: %w", err))
	}

	// Check if already exposing
	ongoing, err := h.materials.FindOngoingTOEExposure(ctx, req.LotId)
	if err != nil {
		return h.fail(domain.SubjectMaterialTOEStart, start, fmt.Errorf("StartTOEExposure: check ongoing: %w", err))
	}
	if ongoing != nil {
		return h.fail(domain.SubjectMaterialTOEStart, start, fmt.Errorf("lot already has an ongoing TOE exposure cycle"))
	}

	log, err := domain.NewTOEExposureLog(req.LotId, req.OperatorId)
	if err != nil {
		return h.fail(domain.SubjectMaterialTOEStart, start, fmt.Errorf("StartTOEExposure: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveTOEExposureLog(ctx, log)
	}); err != nil {
		return h.fail(domain.SubjectMaterialTOEStart, start, fmt.Errorf("StartTOEExposure: tx: %w", err))
	}

	h.log.Info().Str("lot_id", log.LotID).Msg("TOE exposure started")
	h.succeed(domain.SubjectMaterialTOEStart, start)
	return proto.Marshal(&pbmes.StartTOEExposureResponse{Log: toeLogToProto(log)})
}

// EndTOEExposure handles kors.mes.material.toe.stop.
func (h *Handler) EndTOEExposure(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.EndTOEExposure")
	defer span.End()
	start := time.Now()

	var req pbmes.EndTOEExposureRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMaterialTOEStop, start, fmt.Errorf("EndTOEExposure: unmarshal: %w", err))
	}

	log, err := h.materials.FindOngoingTOEExposure(ctx, req.LotId)
	if err != nil || log == nil {
		return h.fail(domain.SubjectMaterialTOEStop, start, fmt.Errorf("no ongoing TOE exposure found for lot"))
	}

	log.End()
	durationMinutes := int(log.Duration().Minutes())

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateTOEExposureLog(ctx, log); err != nil {
			return err
		}
		// Update cumulative exposure on the Lot
		lot, err := h.trace.FindLotByID(ctx, log.LotID)
		if err != nil {
			return err
		}
		lot.TOEExposureMinutes += durationMinutes
		return tx.UpdateLot(ctx, lot)
	}); err != nil {
		return h.fail(domain.SubjectMaterialTOEStop, start, fmt.Errorf("EndTOEExposure: tx: %w", err))
	}

	h.log.Info().Str("lot_id", log.LotID).Int("added_minutes", durationMinutes).Msg("TOE exposure ended")
	h.succeed(domain.SubjectMaterialTOEStop, start)
	return proto.Marshal(&pbmes.EndTOEExposureResponse{Log: toeLogToProto(log)})
}

// TransferEntity handles kors.mes.entity.transfer.
func (h *Handler) TransferEntity(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.TransferEntity")
	defer span.End()
	start := time.Now()

	var req pbmes.TransferEntityRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectEntityTransfer, start, fmt.Errorf("TransferEntity: unmarshal: %w", err))
	}

	var fromWS *string
	eType := domain.EntityTypeLot
	if req.EntityType == pbmes.EntityType_ENTITY_TYPE_SERIAL {
		eType = domain.EntityTypeSerial
		// In a real system, we'd fetch the current location of the SN here.
	} else {
		lot, err := h.trace.FindLotByID(ctx, req.EntityId)
		if err == nil {
			fromWS = lot.CurrentWorkstationID
		}
	}

	tr, err := domain.NewLocationTransfer(req.EntityId, eType, fromWS, &req.ToWorkstationId, req.TransferredBy)
	if err != nil {
		return h.fail(domain.SubjectEntityTransfer, start, fmt.Errorf("TransferEntity: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.LocationTransferredEvent{
		EventId:         uuid.NewString(),
		EntityId:        tr.EntityID,
		EntityType:      string(tr.EntityType),
		ToWorkstationId: tr.ToWorkstationID,
	})
	if err != nil {
		return h.fail(domain.SubjectEntityTransfer, start, fmt.Errorf("TransferEntity: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveLocationTransfer(ctx, tr); err != nil {
			return err
		}
		// Update entity current location
		if tr.EntityType == domain.EntityTypeLot {
			lot, err := h.trace.FindLotByID(ctx, tr.EntityID)
			if err == nil {
				lot.CurrentWorkstationID = &tr.ToWorkstationID
				return tx.UpdateLot(ctx, lot)
			}
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "LocationTransferred",
			Subject:   domain.SubjectLocationTransferred,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectEntityTransfer, start, fmt.Errorf("TransferEntity: tx: %w", err))
	}

	h.log.Info().Str("entity", tr.EntityID).Str("to", tr.ToWorkstationID).Msg("entity transferred")
	h.succeed(domain.SubjectEntityTransfer, start)
	return proto.Marshal(&pbmes.TransferEntityResponse{Transfer: transferToProto(tr)})
}

// ── Converters ────────────────────────────────────────────────────────────────

func consumptionToProto(r *domain.ConsumptionRecord) *pbmes.ConsumptionRecord {
	return &pbmes.ConsumptionRecord{
		Id:          r.ID,
		LotId:       r.LotID,
		OperationId: r.OperationID,
		Quantity:    int32(r.Quantity),
		OperatorId:  r.OperatorID,
		ConsumedAt:  timestamppb.New(r.ConsumedAt),
	}
}

func toeLogToProto(l *domain.TOEExposureLog) *pbmes.TOEExposureLog {
	pb := &pbmes.TOEExposureLog{
		Id:         l.ID,
		LotId:      l.LotID,
		StartTime:  timestamppb.New(l.StartTime),
		OperatorId: l.OperatorID,
	}
	if l.EndTime != nil {
		pb.EndTime = timestamppb.New(*l.EndTime)
	}
	return pb
}

func transferToProto(tr *domain.LocationTransfer) *pbmes.LocationTransfer {
	pb := &pbmes.LocationTransfer{
		Id:              tr.ID,
		EntityId:        tr.EntityID,
		ToWorkstationId: tr.ToWorkstationID,
		TransferredBy:   tr.TransferredBy,
		TransferredAt:   timestamppb.New(tr.TransferredAt),
	}
	if tr.EntityType == domain.EntityTypeLot {
		pb.EntityType = pbmes.EntityType_ENTITY_TYPE_LOT
	} else {
		pb.EntityType = pbmes.EntityType_ENTITY_TYPE_SERIAL
	}
	if tr.FromWorkstationID != nil {
		pb.FromWorkstationId = *tr.FromWorkstationID
	}
	return pb
}
