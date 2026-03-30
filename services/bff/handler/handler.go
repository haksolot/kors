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
	webhooks  *WebhookRegistry
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
		webhooks:  &WebhookRegistry{webhooks: make(map[string]*Webhook)},
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
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/orders", h.createOrder)
		r.Get("/orders", h.listOrders)
		r.Get("/dispatch", h.getDispatchList)
		r.With(RequireRole(core.RoleAdmin)).Post("/orders/from-routing", h.createFromRouting)

		r.Route("/orders/{id}", func(r chi.Router) {
			r.Get("/", h.getOrder)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleProductionManager, core.RoleAdmin)).Post("/suspend", h.suspendOrder)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleProductionManager, core.RoleAdmin)).Post("/resume", h.resumeOrder)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleProductionManager, core.RoleAdmin)).Post("/cancel", h.cancelOrder)
			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/approve-fai", h.approveFAI)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleAdmin)).Patch("/planning", h.setPlanning)

			// Operations
			r.Get("/operations", h.listOperations)
			r.Route("/operations/{op_id}", func(r chi.Router) {
				r.Get("/", h.getOperation)
				r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/start", h.startOperation)
				r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/complete", h.completeOperation)
				r.With(RequireAnyRole(core.RoleSupervisor, core.RoleAdmin)).Post("/skip", h.skipOperation)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/sign-off", h.signOffOperation)
				r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleQualityManager, core.RoleAdmin)).Post("/nc", h.declareNC)
				r.With(RequireAnyRole(core.RoleSupervisor, core.RoleAdmin)).Post("/instructions", h.attachInstructions)
			})
		})

		// Routings
		r.With(RequireRole(core.RoleAdmin)).Post("/routings", h.createRouting)
		r.Get("/routings", h.listRoutings)
		r.Get("/routings/{id}", h.getRouting)

		// Lots
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/lots", h.createLot)
		r.Get("/lots/{id}", h.getLot)

		// Serial numbers
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/serial-numbers", h.registerSN)
		r.Route("/serial-numbers/{sn}", func(r chi.Router) {
			r.Get("/", h.getSN)
			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/release", h.releaseSN)
			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/scrap", h.scrapSN)
			r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/genealogy", h.addGenealogyEntry)
			r.Get("/genealogy", h.getGenealogy)
		})

		// Qualifications (AS9100D §7.2)
		r.With(RequireRole(core.RoleAdmin)).Post("/qualifications", h.createQualification)
		r.Get("/qualifications", h.listQualifications)
		r.Get("/qualifications/expiring", h.listExpiringQualifications)
		r.Route("/qualifications/{id}", func(r chi.Router) {
			r.Get("/", h.getQualification)
			r.With(RequireRole(core.RoleAdmin)).Post("/renew", h.renewQualification)
			r.With(RequireRole(core.RoleAdmin)).Post("/revoke", h.revokeQualification)
		})

		// Workstations (BLOC 6)
		r.With(RequireRole(core.RoleAdmin)).Post("/workstations", h.createWorkstation)
		r.Get("/workstations", h.listWorkstations)
		r.Route("/workstations/{id}", func(r chi.Router) {
			r.Get("/", h.getWorkstation)
			r.With(RequireAnyRole(core.RoleAdmin, core.RoleSupervisor)).Patch("/status", h.updateWorkstationStatus)
			r.Get("/oee", h.getWorkstationOEE)
		})

		// Time Tracking & Downtimes (BLOC 5)
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/time-logs", h.recordTimeLog)
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/downtimes/start", h.startDowntime)
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/downtimes/{id}/end", h.endDowntime)

		// Tools & Gauges (BLOC 8)
		r.With(RequireRole(core.RoleAdmin)).Post("/tools", h.createTool)
		r.Get("/tools", h.listTools)
		r.Route("/tools/{id}", func(r chi.Router) {
			r.Get("/", h.getTool)
			r.With(RequireAnyRole(core.RoleAdmin, core.RoleQualityManager)).Post("/calibrate", h.calibrateTool)
		})
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/operations/{op_id}/tools", h.assignToolToOperation)
		r.Get("/operations/{op_id}/tools", h.listOperationTools)

		// Materials & WIP (BLOC 9)
		r.Route("/materials", func(r chi.Router) {
			r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/consume", h.consumeMaterial)
			r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/toe/start", h.startTOEExposure)
			r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/toe/stop", h.stopTOEExposure)
		})
		r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/transfers", h.transferEntity)

		// Quality Inline (BLOC 10)
		r.Route("/quality", func(r chi.Router) {
			r.With(RequireAnyRole(core.RoleOperator, core.RoleSupervisor, core.RoleAdmin)).Post("/measurements", h.recordMeasurement)
			r.Get("/operations/{id}/characteristics", h.getOperationCharacteristics)
		})

		// Alerts (BLOC 11)
		r.Route("/alerts", func(r chi.Router) {
			r.Get("/active", h.listActiveAlerts)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleAdmin)).Post("/{id}/acknowledge", h.acknowledgeAlert)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleAdmin)).Post("/{id}/resolve", h.resolveAlert)
		})

		// Compliance & Audit Trail (§13 — EN9100)
		r.Route("/compliance", func(r chi.Router) {
			// As-Built dossier: GET /compliance/orders/{id}/as-built
			// Accessible to quality managers, production managers, and admins.
			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleProductionManager, core.RoleAdmin)).
				Get("/orders/{id}/as-built", h.getAsBuilt)
			// Audit trail query: GET /compliance/audit?actor_id=&entity_id=&entity_type=&action=&from=&to=
			// Restricted to admins and quality managers (contains PII — operator IDs).
			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).
				Get("/audit", h.queryAuditTrail)
		})

		// QMS
		r.Route("/qms", func(r chi.Router) {
			r.Get("/nc", h.listNCs)
			r.Route("/nc/{id}", func(r chi.Router) {
				r.Get("/", h.getNC)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/analyse", h.startAnalysis)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/disposition", h.proposeDisposition)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/close", h.closeNC)
			})

			r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin)).Post("/capa", h.createCAPA)
			r.Get("/capa", h.listCAPAs)
			r.Route("/capa/{id}", func(r chi.Router) {
				r.Get("/", h.getCAPA)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin, core.RoleSupervisor)).Post("/start", h.startCAPA)
				r.With(RequireAnyRole(core.RoleQualityManager, core.RoleAdmin, core.RoleSupervisor)).Post("/complete", h.completeCAPA)
			})
		})

		// Dashboards & Supervision (§16)
		r.Route("/dashboard", func(r chi.Router) {
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleProductionManager, core.RoleAdmin)).
				Get("/supervisor", h.getSupervisorDashboard)
			r.With(RequireAnyRole(core.RoleProductionManager, core.RoleQualityManager, core.RoleAdmin)).
				Get("/trs", h.getTRSByPeriod)
			r.With(RequireAnyRole(core.RoleProductionManager, core.RoleQualityManager, core.RoleAdmin)).
				Get("/downtime-causes", h.getDowntimeCauses)
			r.With(RequireAnyRole(core.RoleSupervisor, core.RoleProductionManager, core.RoleAdmin)).
				Get("/production-progress", h.getProductionProgress)
		})

		// Integration & Data Opening (§14)
		r.Route("/integration", func(r chi.Router) {
			r.With(RequireRole(core.RoleAdmin)).Route("/csv", func(r chi.Router) {
				r.Get("/workstations", h.exportWorkstationsCSV)
				r.Post("/workstations", h.importWorkstationsCSV)
				r.Get("/tools", h.exportToolsCSV)
			})

			r.With(RequireRole(core.RoleAdmin)).Route("/webhooks", func(r chi.Router) {
				r.Get("/", h.listWebhooks)
				r.Post("/", h.createWebhook)
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
