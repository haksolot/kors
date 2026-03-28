package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	pbqms "github.com/haksolot/kors/proto/gen/qms"
	qmsdomain "github.com/haksolot/kors/services/qms/domain"
)

// ── Non-Conformities ───────────────────────────────────────────────────────────

func (h *Handler) getNC(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbqms.GetNCResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectNCGet, &pbqms.GetNCRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listNCs(w http.ResponseWriter, r *http.Request) {
	var req pbqms.ListNCsRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbqms.ListNCsResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectNCList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) startAnalysis(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var resp pbqms.StartAnalysisResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectNCAnalyse, &pbqms.StartAnalysisRequest{
		NcId:      id,
		AnalystId: claims.Subject,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) proposeDisposition(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var req pbqms.ProposeDispositionRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.NcId = id
	req.AnalystId = claims.Subject
	var resp pbqms.ProposeDispositionResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectNCProposeDisposition, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) closeNC(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var resp pbqms.CloseNCResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectNCClose, &pbqms.CloseNCRequest{
		NcId:     id,
		ClosedBy: claims.Subject,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// ── CAPAs ──────────────────────────────────────────────────────────────────────

func (h *Handler) createCAPA(w http.ResponseWriter, r *http.Request) {
	var req pbqms.CreateCAPARequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbqms.CreateCAPAResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectCAPACreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getCAPA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbqms.GetCAPAResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectCAPAGet, &pbqms.GetCAPARequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listCAPAs(w http.ResponseWriter, r *http.Request) {
	var req pbqms.ListCAPAsRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbqms.ListCAPAsResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectCAPAList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) startCAPA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbqms.StartCAPAResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectCAPAStart, &pbqms.StartCAPARequest{CapaId: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) completeCAPA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbqms.CompleteCAPAResponse
	if err := h.natsReq(r.Context(), qmsdomain.SubjectCAPAComplete, &pbqms.CompleteCAPARequest{CapaId: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
