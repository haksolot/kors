package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ── Time Tracking & Downtimes ────────────────────────────────────────────────

func (h *Handler) recordTimeLog(w http.ResponseWriter, r *http.Request) {
	var req pbmes.RecordTimeLogRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.RecordTimeLogResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectTimeLogRecord, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) startDowntime(w http.ResponseWriter, r *http.Request) {
	var req pbmes.StartDowntimeRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.StartDowntimeResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectDowntimeStart, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) endDowntime(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.EndDowntimeRequest
	req.Id = id
	var resp pbmes.EndDowntimeResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectDowntimeEnd, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) getWorkstationOEE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req pbmes.GetWorkstationOEERequest
	req.WorkstationId = id

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	// Default to last 24 hours if not provided
	toTime := time.Now().UTC()
	fromTime := toTime.Add(-24 * time.Hour)

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			fromTime = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			toTime = t
		}
	}

	req.From = timestamppb.New(fromTime)
	req.To = timestamppb.New(toTime)

	var resp pbmes.GetWorkstationOEEResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationOEEGet, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
