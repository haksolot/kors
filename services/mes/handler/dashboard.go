package handler

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// GetSupervisorDashboard handles kors.mes.dashboard.supervisor.get.
func (h *Handler) GetSupervisorDashboard(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetSupervisorDashboard")
	defer span.End()
	start := time.Now()

	snapshot, err := h.dashboards.GetSupervisorSnapshot(ctx)
	if err != nil {
		return h.fail(domain.SubjectDashboardSupervisorGet, start, fmt.Errorf("GetSupervisorDashboard: %w", err))
	}

	resp := &pbmes.GetSupervisorDashboardResponse{}

	for _, o := range snapshot.ActiveOrders {
		resp.ActiveOrders = append(resp.ActiveOrders, orderToProto(o))
	}

	for _, ws := range snapshot.Workstations {
		resp.Workstations = append(resp.Workstations, &pbmes.WorkstationSnapshot{
			WorkstationId:   ws.WorkstationID,
			WorkstationName: ws.WorkstationName,
			Status:          domainWorkstationStatusToProto(ws.Status),
			CurrentOfId:     ws.CurrentOFID,
			CurrentOfReference: ws.CurrentOFRef,
			Trs:             ws.OEE.TRS,
			Availability:    ws.OEE.Availability,
			Performance:     ws.OEE.Performance,
			Quality:         ws.OEE.Quality,
		})
	}

	for _, a := range snapshot.ActiveAlerts {
		resp.ActiveAlerts = append(resp.ActiveAlerts, alertToProto(a))
	}

	h.succeed(domain.SubjectDashboardSupervisorGet, start)
	return proto.Marshal(resp)
}

// GetTRSByPeriod handles kors.mes.metrics.trs.by_period.
func (h *Handler) GetTRSByPeriod(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetTRSByPeriod")
	defer span.End()
	start := time.Now()

	var req pbmes.GetTRSByPeriodRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMetricsTRSByPeriod, start, fmt.Errorf("GetTRSByPeriod: unmarshal: %w", err))
	}

	filter := domain.TRSFilter{
		WorkstationID: req.WorkstationId,
		From:          req.From.AsTime(),
		To:            req.To.AsTime(),
		Granularity:   protoTRSGranularityToDomain(req.Granularity),
	}

	points, err := h.dashboards.GetTRSByPeriod(ctx, filter)
	if err != nil {
		return h.fail(domain.SubjectMetricsTRSByPeriod, start, fmt.Errorf("GetTRSByPeriod: %w", err))
	}

	resp := &pbmes.GetTRSByPeriodResponse{}
	for _, p := range points {
		resp.Points = append(resp.Points, &pbmes.TRSDataPoint{
			Period:       p.Period,
			Trs:          p.TRS,
			Availability: p.Availability,
			Performance:  p.Performance,
			Quality:      p.Quality,
		})
	}

	h.succeed(domain.SubjectMetricsTRSByPeriod, start)
	return proto.Marshal(resp)
}

// GetDowntimeCauses handles kors.mes.metrics.downtime_causes.
func (h *Handler) GetDowntimeCauses(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetDowntimeCauses")
	defer span.End()
	start := time.Now()

	var req pbmes.GetDowntimeCausesRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMetricsDowntimeCauses, start, fmt.Errorf("GetDowntimeCauses: unmarshal: %w", err))
	}

	from := req.From.AsTime()
	to := req.To.AsTime()

	causes, err := h.dashboards.GetDowntimeCauses(ctx, from, to)
	if err != nil {
		return h.fail(domain.SubjectMetricsDowntimeCauses, start, fmt.Errorf("GetDowntimeCauses: %w", err))
	}

	resp := &pbmes.GetDowntimeCausesResponse{}
	for _, c := range causes {
		resp.Causes = append(resp.Causes, &pbmes.DowntimeCause{
			Reason:               c.Reason,
			TotalDurationSeconds: c.TotalDurationSeconds,
			OccurrenceCount:      int32(c.OccurrenceCount),
		})
	}

	h.succeed(domain.SubjectMetricsDowntimeCauses, start)
	return proto.Marshal(resp)
}

// GetProductionProgress handles kors.mes.metrics.production_progress.
func (h *Handler) GetProductionProgress(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetProductionProgress")
	defer span.End()
	start := time.Now()

	var req pbmes.GetProductionProgressRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectMetricsProductionProgress, start, fmt.Errorf("GetProductionProgress: unmarshal: %w", err))
	}

	from := req.From.AsTime()
	to := req.To.AsTime()

	lines, err := h.dashboards.GetProductionProgress(ctx, from, to)
	if err != nil {
		return h.fail(domain.SubjectMetricsProductionProgress, start, fmt.Errorf("GetProductionProgress: %w", err))
	}

	resp := &pbmes.GetProductionProgressResponse{}
	for _, l := range lines {
		resp.Lines = append(resp.Lines, &pbmes.ProductionProgressLine{
			OfId:                 l.OFID,
			OfReference:          l.OFReference,
			ProductId:            l.ProductID,
			PlannedQuantity:      int32(l.PlannedQuantity),
			GoodQuantity:         int32(l.GoodQuantity),
			ScrapQuantity:        int32(l.ScrapQuantity),
			CompletionPercentage: l.CompletionPercentage,
		})
	}

	h.succeed(domain.SubjectMetricsProductionProgress, start)
	return proto.Marshal(resp)
}

// ── Converters ────────────────────────────────────────────────────────────────

func protoTRSGranularityToDomain(g pbmes.TRSPeriodGranularity) domain.TRSPeriodGranularity {
	switch g {
	case pbmes.TRSPeriodGranularity_TRS_GRANULARITY_DAY:
		return domain.TRSPeriodDay
	case pbmes.TRSPeriodGranularity_TRS_GRANULARITY_WEEK:
		return domain.TRSPeriodWeek
	case pbmes.TRSPeriodGranularity_TRS_GRANULARITY_MONTH:
		return domain.TRSPeriodMonth
	default:
		return domain.TRSPeriodDay
	}
}
