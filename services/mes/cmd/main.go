package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/haksolot/kors/libs/core"
	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
	"github.com/haksolot/kors/services/mes/outbox"
	"github.com/haksolot/kors/services/mes/qualification"
	"github.com/haksolot/kors/services/mes/repo"
)

// Config holds all configuration read from environment variables at startup.
type Config struct {
	DatabaseURL  string `env:"DATABASE_URL,required"`
	NATSUrl      string `env:"NATS_URL,required"`
	NATSCreds    string `env:"NATS_CREDS_PATH"`
	JWKSEndpoint         string `env:"JWKS_ENDPOINT"`        // unused by MES — JWT validated in BFF
	OTLPEndpoint         string `env:"OTLP_ENDPOINT"`
	ServiceName          string `env:"SERVICE_NAME,required"`
	QualWarningDays      int    `env:"QUAL_WARNING_DAYS"`    // days before expiry to emit alerts (default 30)
	QualScanIntervalSecs int    `env:"QUAL_SCAN_INTERVAL_SECS"` // scanner interval in seconds (default 3600)
}

func main() {
	log := core.NewLogger("mes")

	var cfg Config
	if err := core.LoadEnv(&cfg); err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Tracing ───────────────────────────────────────────────────────────────
	if cfg.OTLPEndpoint != "" {
		shutdown, err := core.InitTracer(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to init tracer")
		}
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				log.Error().Err(err).Msg("tracer shutdown error")
			}
		}()
	}

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("database ping failed")
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal().Err(err).Msg("goose set dialect")
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}
	log.Info().Msg("migrations applied")

	// ── NATS ──────────────────────────────────────────────────────────────────
	nc, err := core.NewNATSConn(cfg.NATSUrl, cfg.NATSCreds)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer func() { _ = nc.Drain() }()
	log.Info().Str("nats_url", cfg.NATSUrl).Msg("connected to NATS")

	// ── Metrics ───────────────────────────────────────────────────────────────
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		srv := &http.Server{Addr: ":9090", Handler: mux}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("metrics server error")
		}
	}()

	// ── Wiring ────────────────────────────────────────────────────────────────
	r := repo.New(pool)
	h := handler.New(r, r, r, r, r, r, r, reg, &log)
	worker := outbox.New(r, nc, log, reg)

	// ── Qualification expiry scanner ──────────────────────────────────────────
	warningDays := cfg.QualWarningDays
	if warningDays <= 0 {
		warningDays = domain.DefaultExpiryWarningDays
	}
	scanInterval := time.Duration(cfg.QualScanIntervalSecs) * time.Second
	qualScanner := qualification.New(r, warningDays, scanInterval, log, reg)

	// ── Subscriptions ─────────────────────────────────────────────────────────
	subs := subscribeAll(ctx, h, nc, log)
	defer drainAll(subs)

	// ── Background workers ────────────────────────────────────────────────────
	go worker.Run(ctx)
	go qualScanner.Run(ctx)

	log.Info().Str("service", cfg.ServiceName).Msg("MES service started")
	<-ctx.Done()
	log.Info().Msg("shutting down MES service")
}

// subscribeAll registers all NATS request-reply handlers using queue groups.
func subscribeAll(ctx context.Context, h *handler.Handler, nc *nats.Conn, log zerolog.Logger) []*nats.Subscription {
	type entry struct {
		subject string
		fn      func(context.Context, []byte) ([]byte, error)
	}

	routes := []entry{
		// Orders
		{domain.SubjectOFCreate, h.CreateOrder},
		{domain.SubjectOFGet, h.GetOrder},
		{domain.SubjectOFList, h.ListOrders},
		{domain.SubjectOFSuspend, h.SuspendOrder},
		{domain.SubjectOFResume, h.ResumeOrder},
		{domain.SubjectOFCancel, h.CancelOrder},
		// Operations
		{domain.SubjectOperationCreate, h.CreateOperation},
		{domain.SubjectOperationGet, h.GetOperation},
		{domain.SubjectOperationList, h.ListOperations},
		{domain.SubjectOperationStart, h.StartOperation},
		{domain.SubjectOperationComplete, h.CompleteOperation},
		{domain.SubjectOperationSkip, h.SkipOperation},
		// Traceability — lots
		{domain.SubjectLotCreate, h.CreateLot},
		{domain.SubjectLotGet, h.GetLot},
		// Traceability — serial numbers
		{domain.SubjectSNRegister, h.RegisterSN},
		{domain.SubjectSNGet, h.GetSN},
		{domain.SubjectSNRelease, h.ReleaseSN},
		{domain.SubjectSNScrap, h.ScrapSN},
		// Traceability — genealogy
		{domain.SubjectGenealogyAdd, h.AddGenealogyEntry},
		{domain.SubjectGenealogyGet, h.GetGenealogy},
		// Quality — hold points, NC, FAI, work instructions (BLOC 4)
		{domain.SubjectOperationSignOff, h.SignOffOperation},
		{domain.SubjectOperationDeclareNC, h.DeclareNC},
		{domain.SubjectOFFAIApprove, h.ApproveFAI},
		{domain.SubjectOperationAttachInstructions, h.AttachInstructions},
		// Routings & planning (BLOC 5)
		{domain.SubjectRoutingCreate, h.CreateRouting},
		{domain.SubjectRoutingGet, h.GetRouting},
		{domain.SubjectRoutingList, h.ListRoutings},
		{domain.SubjectOFCreateFromRouting, h.CreateFromRouting},
		{domain.SubjectOFDispatchList, h.GetDispatchList},
		{domain.SubjectOFSetPlanning, h.SetPlanning},
		// Workstations (BLOC 6)
		{domain.SubjectWorkstationCreate, h.CreateWorkstation},
		{domain.SubjectWorkstationGet, h.GetWorkstation},
		{domain.SubjectWorkstationList, h.ListWorkstations},
		{domain.SubjectWorkstationUpdateStatus, h.UpdateWorkstationStatus},
	}

	subs := make([]*nats.Subscription, 0, len(routes))
	for _, r := range routes {
		sub, err := nc.QueueSubscribe(r.subject, domain.QueueGroupMES, func(msg *nats.Msg) {
			resp, err := r.fn(ctx, msg.Data)
			if err != nil {
				log.Error().Err(err).Str("subject", r.subject).Msg("handler error")
				if msg.Reply != "" {
					_ = msg.Respond([]byte("error: " + err.Error()))
				}
				return
			}
			if msg.Reply != "" {
				_ = msg.Respond(resp)
			}
		})
		if err != nil {
			log.Fatal().Err(err).Str("subject", r.subject).Msg("subscribe failed")
		}
		subs = append(subs, sub)
		log.Info().Str("subject", r.subject).Msg("subscribed")
	}
	return subs
}

func drainAll(subs []*nats.Subscription) {
	for _, sub := range subs {
		_ = sub.Drain()
	}
}

// compile-time interface compliance checks.
var (
	_ handler.DispatchRepository      = (*repo.PostgresRepo)(nil)
	_ handler.OperationRepository     = (*repo.PostgresRepo)(nil)
	_ handler.TraceabilityRepository  = (*repo.PostgresRepo)(nil)
	_ handler.RoutingRepository       = (*repo.PostgresRepo)(nil)
	_ handler.QualificationRepository = (*repo.PostgresRepo)(nil)
	_ handler.WorkstationRepository   = (*repo.PostgresRepo)(nil)
	_ domain.Transactor               = (*repo.PostgresRepo)(nil)
	_ outbox.Repository               = (*repo.PostgresRepo)(nil)
	_ qualification.Repository        = (*repo.PostgresRepo)(nil)
)
