package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Inline Quality ────────────────────────────────────────────────────────────

func (h *Handler) recordMeasurement(w http.ResponseWriter, r *http.Request) {
	var req pbmes.RecordMeasurementRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	claims := claimsFromCtx(r)
	if claims != nil {
		req.OperatorId = claims.Subject
	}

	var resp pbmes.RecordMeasurementResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMeasurementRecord, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getOperationCharacteristics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetOperationCharacteristicsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationCharacteristicsList, &pbmes.GetOperationCharacteristicsRequest{OperationId: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
Applied fuzzy match at line 1-3.