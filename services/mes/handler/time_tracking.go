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

// ── Time Tracking & OEE (BLOC 5) ──────────────────────────────────────────────

// RecordTimeLog handles kors.mes.time_log.record.
func (h *Handler) RecordTimeLog(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.RecordTimeLog")
	defer span.End()
	start := time.Now()

	var req pbmes.RecordTimeLogRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectTimeLogRecord, start, fmt.Errorf("RecordTimeLog: unmarshal: %w", err))
	}

	logType := domain.TimeLogTypeSetup
	if req.LogType == pbmes.TimeLogType_TIME_LOG_TYPE_RUN {
		logType = domain.TimeLogTypeRun
	}

	startTime := req.StartTime.AsTime()
	endTime := req.EndTime.AsTime()

	tlog, err := domain.NewTimeLog(
		req.OperationId,
		req.WorkstationId,
		req.OperatorId,
		logType,
		startTime,
		endTime,
		int(req.GoodQuantity),
		int(req.ScrapQuantity),
	)
	if err != nil {
		return h.fail(domain.SubjectTimeLogRecord, start, fmt.Errorf("RecordTimeLog: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.TimeLogRecordedEvent{
		EventId:         uuid.NewString(),
		TimeLogId:       tlog.ID,
		OperationId:     tlog.OperationID,
		WorkstationId:   tlog.WorkstationID,
		DurationSeconds: int32(tlog.Duration().Seconds()),
		GoodQuantity:    int32(tlog.GoodQuantity),
		ScrapQuantity:   int32(tlog.ScrapQuantity),
	})
	if err != nil {
		return h.fail(domain.SubjectTimeLogRecord, start, fmt.Errorf("RecordTimeLog: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveTimeLog(ctx, tlog); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "TimeLogRecorded",
			Subject:   domain.SubjectTimeLogRecorded,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectTimeLogRecord, start, fmt.Errorf("RecordTimeLog: tx: %w", err))
	}

	h.log.Info().Str("time_log_id", tlog.ID).Str("workstation", tlog.WorkstationID).Msg("time log recorded")
	h.succeed(domain.SubjectTimeLogRecord, start)
	return proto.Marshal(&pbmes.RecordTimeLogResponse{TimeLog: timeLogToProto(tlog)})
}

// StartDowntime handles kors.mes.downtime.start.
func (h *Handler) StartDowntime(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.StartDowntime")
	defer span.End()
	start := time.Now()

	var req pbmes.StartDowntimeRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: unmarshal: %w", err))
	}

	// Check if already down
	ongoing, err := h.time.FindOngoingDowntime(ctx, req.WorkstationId)
	if err != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: check ongoing: %w", err))
	}
	if ongoing != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: workstation already has an ongoing downtime event"))
	}

	var opID *string
	if req.OperationId != "" {
		opID = &req.OperationId
	}

	category := protoDowntimeCategoryToDomain(req.Category)

	dt, err := domain.NewDowntimeEvent(req.WorkstationId, opID, category, req.Description, req.ReportedBy)
	if err != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.DowntimeStartedEvent{
		EventId:       uuid.NewString(),
		DowntimeId:    dt.ID,
		WorkstationId: dt.WorkstationID,
		Category:      string(dt.Category),
		StartTime:     timestamppb.New(dt.StartTime),
	})
	if err != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: marshal event: %w", err))
	}

	alert, _ := domain.NewAlert(domain.AlertCategoryMachine, &dt.WorkstationID, dt.OperationID, fmt.Sprintf("Machine down: %s (%s)", dt.WorkstationID, dt.Category))
	alertEvt, _ := proto.Marshal(&pbmes.AlertRaisedEvent{
		EventId:  uuid.NewString(),
		AlertId:  alert.ID,
		Category: string(alert.Category),
		Level:    string(alert.Level),
		Message:  alert.Message,
	})
	escReq, _ := proto.Marshal(&pbmes.EscalationRequestedEvent{
		AlertId:     alert.ID,
		TargetLevel: pbmes.AlertLevel_ALERT_LEVEL_L2_MANAGER,
	})

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveDowntimeEvent(ctx, dt); err != nil {
			return err
		}
		if err := tx.SaveAlert(ctx, alert); err != nil {
			return err
		}
		// Also update workstation status to DOWN
		ws, err := h.workstations.FindWorkstationByID(ctx, dt.WorkstationID)
		if err == nil && ws.Status != domain.WorkstationStatusDown {
			ws.UpdateStatus(domain.WorkstationStatusDown)
			if err := tx.UpdateWorkstation(ctx, ws); err != nil {
				return err
			}
		}
		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "DowntimeStarted",
			Subject:   domain.SubjectDowntimeStarted,
			Payload:   evt,
		}); err != nil {
			return err
		}
		if err := tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "AlertRaised",
			Subject:   domain.SubjectAlertRaised,
			Payload:   alertEvt,
		}); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "EscalationRequested",
			Subject:   domain.SubjectAlertEscalationRequested,
			Payload:   escReq,
		})
	}); err != nil {
		return h.fail(domain.SubjectDowntimeStart, start, fmt.Errorf("StartDowntime: tx: %w", err))
	}

	h.log.Info().Str("downtime_id", dt.ID).Str("workstation", dt.WorkstationID).Msg("downtime started")
	h.succeed(domain.SubjectDowntimeStart, start)
	return proto.Marshal(&pbmes.StartDowntimeResponse{Downtime: downtimeToProto(dt)})
}

// EndDowntime handles kors.mes.downtime.end.
func (h *Handler) EndDowntime(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.EndDowntime")
	defer span.End()
	start := time.Now()

	var req pbmes.EndDowntimeRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectDowntimeEnd, start, fmt.Errorf("EndDowntime: unmarshal: %w", err))
	}

	dt, err := h.time.FindDowntimeByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectDowntimeEnd, start, fmt.Errorf("EndDowntime: find: %w", err))
	}

	if err := dt.End(); err != nil {
		return h.fail(domain.SubjectDowntimeEnd, start, fmt.Errorf("EndDowntime: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.DowntimeEndedEvent{
		EventId:         uuid.NewString(),
		DowntimeId:      dt.ID,
		WorkstationId:   dt.WorkstationID,
		DurationSeconds: int32(dt.Duration().Seconds()),
		EndTime:         timestamppb.New(*dt.EndTime),
	})
	if err != nil {
		return h.fail(domain.SubjectDowntimeEnd, start, fmt.Errorf("EndDowntime: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateDowntimeEvent(ctx, dt); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "DowntimeEnded",
			Subject:   domain.SubjectDowntimeEnded,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectDowntimeEnd, start, fmt.Errorf("EndDowntime: tx: %w", err))
	}

	h.log.Info().Str("downtime_id", dt.ID).Str("workstation", dt.WorkstationID).Msg("downtime ended")
	h.succeed(domain.SubjectDowntimeEnd, start)
	return proto.Marshal(&pbmes.EndDowntimeResponse{Downtime: downtimeToProto(dt)})
}

// GetWorkstationOEE handles kors.mes.workstation.oee.get.
func (h *Handler) GetWorkstationOEE(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetWorkstationOEE")
	defer span.End()
	start := time.Now()

	var req pbmes.GetWorkstationOEERequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectWorkstationOEEGet, start, fmt.Errorf("GetWorkstationOEE: unmarshal: %w", err))
	}

	from := req.From.AsTime()
	to := req.To.AsTime()
	if to.IsZero() {
		to = time.Now().UTC()
	}

	logs, err := h.time.ListTimeLogsByWorkstation(ctx, req.WorkstationId, from, to)
	if err != nil {
		return h.fail(domain.SubjectWorkstationOEEGet, start, fmt.Errorf("GetWorkstationOEE: logs query: %w", err))
	}

	downtimes, err := h.time.ListDowntimesByWorkstation(ctx, req.WorkstationId, from, to)
	if err != nil {
		return h.fail(domain.SubjectWorkstationOEEGet, start, fmt.Errorf("GetWorkstationOEE: downtime query: %w", err))
	}

	ws, err := h.workstations.FindWorkstationByID(ctx, req.WorkstationId)
	if err != nil {
		return h.fail(domain.SubjectWorkstationOEEGet, start, fmt.Errorf("GetWorkstationOEE: find workstation: %w", err))
	}

	var operatingTime time.Duration
	var downtime time.Duration
	var goodQty, scrapQty int

	for _, l := range logs {
		if l.LogType == domain.TimeLogTypeRun {
			operatingTime += l.Duration()
		}
		goodQty += l.GoodQuantity
		scrapQty += l.ScrapQuantity
	}

	for _, dt := range downtimes {
		// Cap downtime to the requested period
		dtStart := dt.StartTime
		if dtStart.Before(from) {
			dtStart = from
		}
		dtEnd := to
		if dt.EndTime != nil && dt.EndTime.Before(to) {
			dtEnd = *dt.EndTime
		}
		if dtEnd.After(dtStart) {
			downtime += dtEnd.Sub(dtStart)
		}
	}

	plannedTime := to.Sub(from)

	// Workstation NominalRate is typically pieces per hour, so ideal cycle time is 3600 / NominalRate
	var idealCycleTime float64 = 0
	if ws.NominalRate > 0 {
		idealCycleTime = 3600.0 / ws.NominalRate
	}

	oeeData := domain.CalculateOEE(plannedTime, operatingTime, downtime, goodQty, scrapQty, idealCycleTime)

	pbOEE := &pbmes.OEEData{
		Availability:          oeeData.Availability,
		Performance:           oeeData.Performance,
		Quality:               oeeData.Quality,
		Trs:                   oeeData.TRS,
		TotalDowntimeSeconds:  int32(oeeData.TotalDowntimeSeconds),
		TotalOperatingSeconds: int32(oeeData.TotalOperatingSeconds),
		TotalGoodQuantity:     int32(oeeData.TotalGoodQuantity),
		TotalScrapQuantity:    int32(oeeData.TotalScrapQuantity),
	}

	h.succeed(domain.SubjectWorkstationOEEGet, start)
	return proto.Marshal(&pbmes.GetWorkstationOEEResponse{
		WorkstationId: req.WorkstationId,
		Oee:           pbOEE,
	})
}

// ── Converters ────────────────────────────────────────────────────────────────

func timeLogToProto(l *domain.TimeLog) *pbmes.TimeLog {
	pb := &pbmes.TimeLog{
		Id:            l.ID,
		OperationId:   l.OperationID,
		WorkstationId: l.WorkstationID,
		OperatorId:    l.OperatorID,
		StartTime:     timestamppb.New(l.StartTime),
		EndTime:       timestamppb.New(l.EndTime),
		GoodQuantity:  int32(l.GoodQuantity),
		ScrapQuantity: int32(l.ScrapQuantity),
		CreatedAt:     timestamppb.New(l.CreatedAt),
	}
	if l.LogType == domain.TimeLogTypeSetup {
		pb.LogType = pbmes.TimeLogType_TIME_LOG_TYPE_SETUP
	} else {
		pb.LogType = pbmes.TimeLogType_TIME_LOG_TYPE_RUN
	}
	return pb
}

func downtimeToProto(d *domain.DowntimeEvent) *pbmes.DowntimeEvent {
	pb := &pbmes.DowntimeEvent{
		Id:            d.ID,
		WorkstationId: d.WorkstationID,
		Category:      domainDowntimeCategoryToProto(d.Category),
		Description:   d.Description,
		StartTime:     timestamppb.New(d.StartTime),
		ReportedBy:    d.ReportedBy,
		CreatedAt:     timestamppb.New(d.CreatedAt),
	}
	if d.OperationID != nil {
		pb.OperationId = *d.OperationID
	}
	if d.EndTime != nil {
		pb.EndTime = timestamppb.New(*d.EndTime)
	}
	return pb
}

func protoDowntimeCategoryToDomain(c pbmes.DowntimeCategory) domain.DowntimeCategory {
	switch c {
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_MACHINE_FAILURE:
		return domain.DowntimeCategoryMachineFailure
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_PREVENTIVE_MAINTENANCE:
		return domain.DowntimeCategoryPreventiveMaintenance
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_MATERIAL_SHORTAGE:
		return domain.DowntimeCategoryMaterialShortage
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_QUALITY_HOLD:
		return domain.DowntimeCategoryQualityHold
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_CHANGEOVER:
		return domain.DowntimeCategoryChangeover
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_REGULATORY_PAUSE:
		return domain.DowntimeCategoryRegulatoryPause
	case pbmes.DowntimeCategory_DOWNTIME_CATEGORY_UNJUSTIFIED:
		return domain.DowntimeCategoryUnjustified
	default:
		return domain.DowntimeCategoryUnjustified
	}
}

func domainDowntimeCategoryToProto(c domain.DowntimeCategory) pbmes.DowntimeCategory {
	switch c {
	case domain.DowntimeCategoryMachineFailure:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_MACHINE_FAILURE
	case domain.DowntimeCategoryPreventiveMaintenance:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_PREVENTIVE_MAINTENANCE
	case domain.DowntimeCategoryMaterialShortage:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_MATERIAL_SHORTAGE
	case domain.DowntimeCategoryQualityHold:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_QUALITY_HOLD
	case domain.DowntimeCategoryChangeover:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_CHANGEOVER
	case domain.DowntimeCategoryRegulatoryPause:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_REGULATORY_PAUSE
	case domain.DowntimeCategoryUnjustified:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_UNJUSTIFIED
	default:
		return pbmes.DowntimeCategory_DOWNTIME_CATEGORY_UNSPECIFIED
	}
}
