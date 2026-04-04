package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Alerts ───────────────────────────────────────────────────────────────────

func (h *Handler) raiseAlert(w http.ResponseWriter, r *http.Request) {
	var req pbmes.RaiseAlertRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.RaiseAlertResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAlertRaise, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) listActiveAlerts(w http.ResponseWriter, r *http.Request) {
	var resp pbmes.ListActiveAlertsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAlertListActive, &pbmes.ListActiveAlertsRequest{}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.AcknowledgeAlertRequest
	req.Id = id
	
	claims := claimsFromCtx(r)
	if claims != nil {
		req.UserId = claims.Subject
	}

	var resp pbmes.AcknowledgeAlertResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAlertAcknowledge, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.ResolveAlertRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Id = id
	
	claims := claimsFromCtx(r)
	if claims != nil {
		req.UserId = claims.Subject
	}

	var resp pbmes.ResolveAlertResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAlertResolve, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
