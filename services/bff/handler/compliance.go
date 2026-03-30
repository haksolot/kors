package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Compliance & Audit Trail (§13 — EN9100) ───────────────────────────────────

// getAsBuilt handles GET /api/v1/compliance/orders/{id}/as-built.
// Returns the complete Dossier Industriel Numérique for the given OF.
// Roles: quality_manager, production_manager, admin.
func (h *Handler) getAsBuilt(w http.ResponseWriter, r *http.Request) {
	ofID := chi.URLParam(r, "id")
	if ofID == "" {
		writeError(w, http.StatusBadRequest, "order ID is required")
		return
	}

	var resp pbmes.GetAsBuiltResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAsBuiltGet, &pbmes.GetAsBuiltRequest{OfId: ofID}, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// queryAuditTrail handles GET /api/v1/compliance/audit.
// Query parameters (all optional):
//   - actor_id    : filter by user UUID
//   - entity_type : filter by entity type (manufacturing_order, operation, …)
//   - entity_id   : filter by entity UUID
//   - action      : filter by AuditAction value
//   - from        : RFC3339 lower bound (inclusive)
//   - to          : RFC3339 upper bound (inclusive)
//   - page_size   : integer, defaults to 50, max 200
//
// Roles: quality_manager, admin.
func (h *Handler) queryAuditTrail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	req := &pbmes.QueryAuditTrailRequest{
		ActorId:    q.Get("actor_id"),
		EntityType: q.Get("entity_type"),
		EntityId:   q.Get("entity_id"),
		Action:     q.Get("action"),
	}

	if s := q.Get("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			req.From = timestamppb.New(t)
		} else {
			writeError(w, http.StatusBadRequest, "invalid 'from' timestamp: use RFC3339 format")
			return
		}
	}
	if s := q.Get("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			req.To = timestamppb.New(t)
		} else {
			writeError(w, http.StatusBadRequest, "invalid 'to' timestamp: use RFC3339 format")
			return
		}
	}

	var resp pbmes.QueryAuditTrailResponse
	if err := h.natsReq(r.Context(), mesdomain.SubjectAuditQuery, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}
