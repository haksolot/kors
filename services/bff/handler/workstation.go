package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Workstations ──────────────────────────────────────────────────────────────

func (h *Handler) createWorkstation(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateWorkstationRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateWorkstationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getWorkstation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetWorkstationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationGet, &pbmes.GetWorkstationRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listWorkstations(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var req pbmes.ListWorkstationsRequest
	req.Limit = int32(limit)
	req.Offset = int32(offset)

	var resp pbmes.ListWorkstationsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) updateWorkstationStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.UpdateWorkstationStatusRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Id = id
	var resp pbmes.UpdateWorkstationStatusResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationUpdateStatus, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
