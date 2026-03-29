package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Qualifications (AS9100D §7.2) ─────────────────────────────────────────────

// createQualification handles POST /api/v1/qualifications.
// granted_by is always taken from the JWT subject, never from the request body.
func (h *Handler) createQualification(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	var req pbmes.CreateQualificationRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.GrantedBy = claims.Subject
	var resp pbmes.CreateQualificationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

// getQualification handles GET /api/v1/qualifications/{id}.
func (h *Handler) getQualification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetQualificationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationGet,
		&pbmes.GetQualificationRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// listQualifications handles GET /api/v1/qualifications?operator_id={uuid}.
func (h *Handler) listQualifications(w http.ResponseWriter, r *http.Request) {
	operatorID := r.URL.Query().Get("operator_id")
	if operatorID == "" {
		writeError(w, http.StatusBadRequest, "operator_id query parameter is required")
		return
	}
	var resp pbmes.ListQualificationsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationList,
		&pbmes.ListQualificationsRequest{OperatorId: operatorID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// listExpiringQualifications handles GET /api/v1/qualifications/expiring?days={n}.
func (h *Handler) listExpiringQualifications(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days := int32(mesdomain.DefaultExpiryWarningDays)
	if daysStr != "" {
		n, err := strconv.Atoi(daysStr)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		days = int32(n)
	}
	var resp pbmes.ListExpiringQualificationsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationListExpiring,
		&pbmes.ListExpiringQualificationsRequest{WarningDays: days}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// renewQualification handles POST /api/v1/qualifications/{id}/renew.
// renewed_by is always taken from the JWT subject.
func (h *Handler) renewQualification(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var req pbmes.RenewQualificationRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Id = id
	req.RenewedBy = claims.Subject
	var resp pbmes.RenewQualificationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationRenew, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// revokeQualification handles POST /api/v1/qualifications/{id}/revoke.
// revoked_by is always taken from the JWT subject.
func (h *Handler) revokeQualification(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var req pbmes.RevokeQualificationRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Id = id
	req.RevokedBy = claims.Subject
	var resp pbmes.RevokeQualificationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectQualificationRevoke, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
