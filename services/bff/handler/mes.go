package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Manufacturing Orders ───────────────────────────────────────────────────────

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateOrderRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateOrderResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetOrderResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFGet, &pbmes.GetOrderRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	var req pbmes.ListOrdersRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.ListOrdersResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) suspendOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.SuspendOrderResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFSuspend, &pbmes.SuspendOrderRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) resumeOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.ResumeOrderResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFResume, &pbmes.ResumeOrderRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.CancelOrderResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFCancel, &pbmes.CancelOrderRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) approveFAI(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var resp pbmes.ApproveFAIResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFFAIApprove, &pbmes.ApproveFAIRequest{
		OfId:       id,
		ApproverId: claims.Subject,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) setPlanning(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req pbmes.SetPlanningRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.OfId = id
	var resp pbmes.SetPlanningResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFSetPlanning, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) getDispatchList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var resp pbmes.DispatchListResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFDispatchList, &pbmes.DispatchListRequest{
		Limit: int32(limit),
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// ── Routings ───────────────────────────────────────────────────────────────────

func (h *Handler) createRouting(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateRoutingRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateRoutingResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectRoutingCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getRouting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetRoutingResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectRoutingGet, &pbmes.GetRoutingRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listRoutings(w http.ResponseWriter, r *http.Request) {
	var req pbmes.ListRoutingsRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.ListRoutingsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectRoutingList, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) createFromRouting(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateFromRoutingRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateFromRoutingResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOFCreateFromRouting, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

// ── Operations ─────────────────────────────────────────────────────────────────

func (h *Handler) getOperation(w http.ResponseWriter, r *http.Request) {
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.GetOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationGet, &pbmes.GetOperationRequest{Id: opID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) listOperations(w http.ResponseWriter, r *http.Request) {
	ofID := chi.URLParam(r, "id")
	var resp pbmes.ListOperationsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationList, &pbmes.ListOperationsRequest{OfId: ofID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) startOperation(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.StartOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationStart, &pbmes.StartOperationRequest{
		OperationId:   opID,
		OperatorId:    claims.Subject,
		OperatorRoles: claims.Roles,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) completeOperation(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.CompleteOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationComplete, &pbmes.CompleteOperationRequest{
		OperationId: opID,
		OperatorId:  claims.Subject,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) skipOperation(w http.ResponseWriter, r *http.Request) {
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.SkipOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationSkip, &pbmes.SkipOperationRequest{OperationId: opID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) signOffOperation(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	opID := chi.URLParam(r, "op_id")
	var resp pbmes.SignOffOperationResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationSignOff, &pbmes.SignOffOperationRequest{
		OperationId: opID,
		InspectorId: claims.Subject,
	}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) declareNC(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	opID := chi.URLParam(r, "op_id")
	ofID := chi.URLParam(r, "id")
	var req pbmes.DeclareNCRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.OperationId = opID
	req.OfId = ofID
	req.DeclaredBy = claims.Subject
	var resp pbmes.DeclareNCResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationDeclareNC, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) attachInstructions(w http.ResponseWriter, r *http.Request) {
	opID := chi.URLParam(r, "op_id")
	var req pbmes.AttachInstructionsRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.OperationId = opID
	var resp pbmes.AttachInstructionsResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectOperationAttachInstructions, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// ── Lots ───────────────────────────────────────────────────────────────────────

func (h *Handler) createLot(w http.ResponseWriter, r *http.Request) {
	var req pbmes.CreateLotRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.CreateLotResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectLotCreate, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getLot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var resp pbmes.GetLotResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectLotGet, &pbmes.GetLotRequest{Id: id}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// ── Serial numbers ─────────────────────────────────────────────────────────────

func (h *Handler) registerSN(w http.ResponseWriter, r *http.Request) {
	var req pbmes.RegisterSNRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp pbmes.RegisterSNResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectSNRegister, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getSN(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	var resp pbmes.GetSNResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectSNGet, &pbmes.GetSNRequest{Sn: sn}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) releaseSN(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	var resp pbmes.ReleaseSNResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectSNRelease, &pbmes.ReleaseSNRequest{Id: sn}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

func (h *Handler) scrapSN(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	var resp pbmes.ScrapSNResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectSNScrap, &pbmes.ScrapSNRequest{Id: sn}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// ── Genealogy ──────────────────────────────────────────────────────────────────

func (h *Handler) addGenealogyEntry(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	var req pbmes.AddGenealogyEntryRequest
	if err := unmarshalBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.ParentSnId = sn
	var resp pbmes.AddGenealogyEntryResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectGenealogyAdd, &req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &resp)
}

func (h *Handler) getGenealogy(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	var resp pbmes.GetGenealogyResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectGenealogyGet, &pbmes.GetGenealogyRequest{SnId: sn}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
