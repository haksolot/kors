package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Tools & Gauges ────────────────────────────────────────────────────────────

func (h *Handler) createTool(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateToolRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateToolResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetToolResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolGet, &pbmes.GetToolRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listTools(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var req pbmes.ListToolsRequest
	req.Limit = int32(limit)
	req.Offset = int32(offset)

	var resp pbmes.ListToolsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) calibrateTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.CalibrateToolRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Id = id
	// Extract performed_by from JWT
	claims := claimsFromCtx(r)
	if claims != nil {
		req.PerformedBy = claims.Subject
	}

	var resp pbmes.CalibrateToolResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolCalibrate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) assignToolToOperation(w http.ResponseWriter, r *http.Request) {
	opID := chi.URLParam(r, "op_id")
	var req pbmes.AssignToolToOperationRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.OperationId = opID

	var resp pbmes.AssignToolToOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolAssignToOperation, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listOperationTools(w http.ResponseWriter, r *http.Request) {
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.GetOperationToolsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationToolsList, &pbmes.GetOperationToolsRequest{OperationId: opID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
