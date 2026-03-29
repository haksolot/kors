package handler

import (
	"net/http"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Materials & WIP ──────────────────────────────────────────────────────────

func (h *Handler) consumeMaterial(w http.ResponseWriter, r *http.Request) {
	var req pbmes.ConsumeMaterialRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Extract operator_id from JWT
	claims := claimsFromCtx(r)
	if claims != nil {
		req.OperatorId = claims.Subject
	}

	var resp pbmes.ConsumeMaterialResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMaterialConsume, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) startTOEExposure(w http.ResponseWriter, r *http.Request) {
	var req pbmes.StartTOEExposureRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	claims := claimsFromCtx(r)
	if claims != nil {
		req.OperatorId = claims.Subject
	}

	var resp pbmes.StartTOEExposureResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMaterialTOEStart, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) stopTOEExposure(w http.ResponseWriter, r *http.Request) {
	var req pbmes.EndTOEExposureRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	claims := claimsFromCtx(r)
	if claims != nil {
		req.OperatorId = claims.Subject
	}

	var resp pbmes.EndTOEExposureResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectMaterialTOEStop, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) transferEntity(w http.ResponseWriter, r *http.Request) {
	var req pbmes.TransferEntityRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	claims := claimsFromCtx(r)
	if claims != nil {
		req.TransferredBy = claims.Subject
	}

	var resp pbmes.TransferEntityResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectEntityTransfer, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
