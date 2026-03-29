package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"

	"github.com/haksolot/kors/libs/core"
)

// Handler is the BFF HTTP handler. It holds the NATS connection, JWT validator,
// WebSocket hub, logger, and metrics registry.
type Handler struct {
	nc        *nats.Conn
	validator *core.JWTValidator
	hub       *Hub
	log       zerolog.Logger
	reg       prometheus.Registerer
}

// New creates a Handler and starts the WebSocket hub goroutine.
// ctx controls the hub lifetime — cancel it to stop accepting WS connections.
func New(
	ctx context.Context,
	nc *nats.Conn,
	validator *core.JWTValidator,
	reg prometheus.Registerer,
	log zerolog.Logger,
) *Handler {
	hub := newHub(log)
	go hub.Run(ctx)
	return &Handler{
		nc:        nc,
		validator: validator,
		hub:       hub,
		log:       log,
		reg:       reg,
	}
}

// Hub returns the Hub so the caller (cmd/main.go) can subscribe NATS events.
func (h *Handler) Hub() *Hub { return h.hub }

// Routes returns the chi router with all BFF routes mounted.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(LoggingMiddleware(h.log))
	r.Use(MetricsMiddleware(h.reg))

	// Infrastructure (no auth)
	r.Get("/health", h.health)
	r.Get("/ws", h.ServeWS)

	// API v1 — JWT required
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(h.validator))

		// Manufacturing orders
		r.Post("/orders", h.createOrder)
		r.Get("/orders", h.listOrders)
		r.Get("/dispatch", h.getDispatchList)
		r.Post("/orders/from-routing", h.createFromRouting)

		r.Route("/orders/{id}", func(r chi.Router) {
			r.Get("/", h.getOrder)
			r.Post("/suspend", h.suspendOrder)
			r.Post("/resume", h.resumeOrder)
			r.Post("/cancel", h.cancelOrder)
			r.Post("/approve-fai", h.approveFAI)
			r.Patch("/planning", h.setPlanning)

			// Operations
			r.Get("/operations", h.listOperations)
			r.Route("/operations/{op_id}", func(r chi.Router) {
				r.Get("/", h.getOperation)
				r.Post("/start", h.startOperation)
				r.Post("/complete", h.completeOperation)
				r.Post("/skip", h.skipOperation)
				r.Post("/sign-off", h.signOffOperation)
				r.Post("/nc", h.declareNC)
				r.Post("/instructions", h.attachInstructions)
			})
		})

		// Routings
		r.Post("/routings", h.createRouting)
		r.Get("/routings", h.listRoutings)
		r.Get("/routings/{id}", h.getRouting)

		// Lots
		r.Post("/lots", h.createLot)
		r.Get("/lots/{id}", h.getLot)

		// Serial numbers
		r.Post("/serial-numbers", h.registerSN)
		r.Route("/serial-numbers/{sn}", func(r chi.Router) {
			r.Get("/", h.getSN)
			r.Post("/release", h.releaseSN)
			r.Post("/scrap", h.scrapSN)
			r.Post("/genealogy", h.addGenealogyEntry)
			r.Get("/genealogy", h.getGenealogy)
		})

		// Qualifications (AS9100D §7.2)
		r.Post("/qualifications", h.createQualification)
		r.Get("/qualifications", h.listQualifications)
		r.Get("/qualifications/expiring", h.listExpiringQualifications)
		r.Route("/qualifications/{id}", func(r chi.Router) {
			r.Get("/", h.getQualification)
			r.Post("/renew", h.renewQualification)
			r.Post("/revoke", h.revokeQualification)
		})

		// Workstations (BLOC 6)
		r.Post("/workstations", h.createWorkstation)
		r.Get("/workstations", h.listWorkstations)
		r.Route("/workstations/{id}", func(r chi.Router) {
			r.Get("/", h.getWorkstation)
			r.Patch("/status", h.updateWorkstationStatus)
			r.Get("/oee", h.getWorkstationOEE)
		})

		// Time Tracking & Downtimes (BLOC 5)
		r.Post("/time-logs", h.recordTimeLog)
		r.Post("/downtimes/start", h.startDowntime)
		r.Post("/downtimes/{id}/end", h.endDowntime)

		// QMS
		r.Route("/qms", func(r chi.Router) {
			r.Get("/nc", h.listNCs)
			r.Route("/nc/{id}", func(r chi.Router) {
				r.Get("/", h.getNC)
				r.Post("/analyse", h.startAnalysis)
				r.Post("/disposition", h.proposeDisposition)
				r.Post("/close", h.closeNC)
			})

			r.Post("/capa", h.createCAPA)
			r.Get("/capa", h.listCAPAs)
			r.Route("/capa/{id}", func(r chi.Router) {
				r.Get("/", h.getCAPA)
				r.Post("/start", h.startCAPA)
				r.Post("/complete", h.completeCAPA)
			})
		})
	})

	return r
}

// health returns 200 OK with basic service info.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","service":"kors-bff"}`))
}

// natsReq marshals req, sends a NATS request to subject, and unmarshals the
// response into resp. Returns an error if the service returned an error prefix.
func (h *Handler) natsReq(ctx context.Context, subject string, req, resp proto.Message) error {
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}

	msg, err := h.nc.RequestWithContext(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("NATS %s: %w", subject, err)
	}

	if len(msg.Data) > 7 && strings.HasPrefix(string(msg.Data[:7]), "error: ") {
		return fmt.Errorf("%s", msg.Data[7:])
	}

	return proto.Unmarshal(msg.Data, resp)
}
