package handler

import (
	"net/http"
	"time"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ── Dashboards & Metrics (§16) ──────────────────────────────────────────────

func (h *Handler) getSupervisorDashboard(w http.ResponseWriter, r *http.Request) {
	var resp pbmes.GetSupervisorDashboardResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectDashboardSupervisorGet, &pbmes.GetSupervisorDashboardRequest{}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) getTRSByPeriod(w http.ResponseWriter, r *http.Request) {
	workstationID := r.URL.Query().Get("workstation_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	granularityStr := r.URL.Query().Get("granularity")

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -7) // default 7 days
	}

	granularity := pbmes.TRSPeriodGranularity_TRS_GRANULARITY_DAY
	switch granularityStr {
	case "WEEK":
		granularity = pbmes.TRSPeriodGranularity_TRS_GRANULARITY_WEEK
	case "MONTH":
		granularity = pbmes.TRSPeriodGranularity_TRS_GRANULARITY_MONTH
	}

	req := &pbmes.GetTRSByPeriodRequest{
		WorkstationId: workstationID,
		From:          timestamppb.New(from),
		To:            timestamppb.New(to),
		Granularity:   granularity,
	}

	var resp pbmes.GetTRSByPeriodResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMetricsTRSByPeriod, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) getDowntimeCauses(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -30) // default 30 days
	}

	req := &pbmes.GetDowntimeCausesRequest{
		From: timestamppb.New(from),
		To:   timestamppb.New(to),
	}

	var resp pbmes.GetDowntimeCausesResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMetricsDowntimeCauses, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) getProductionProgress(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -7) // default 7 days
	}

	req := &pbmes.GetProductionProgressRequest{
		From: timestamppb.New(from),
		To:   timestamppb.New(to),
	}

	var resp pbmes.GetProductionProgressResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMetricsProductionProgress, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
