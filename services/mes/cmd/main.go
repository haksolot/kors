package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"

	"github.com/haksolot/kors/libs/core"
	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
	"github.com/haksolot/kors/services/mes/outbox"
	"github.com/haksolot/kors/services/mes/repo"
)

// Config holds all configuration read from environment variables at startup.
type Config struct {
	DatabaseURL  string `env:"DATABASE_URL,required"`
	NATSUrl      string `env:"NATS_URL,required"`
	NATSCreds    string `env:"NATS_CREDS_PATH"`
	JWKSEndpoint string `env:"JWKS_ENDPOINT,required"`
	OTLPEndpoint string `env:"OTLP_ENDPOINT"`
	ServiceName  string `env:"SERVICE_NAME,required"`
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

	// ── Wiring ────────────────────────────────────────────────────────────────
	r := repo.New(pool)
	h := handler.New(r, r, &log)
	worker := outbox.New(r, nc, log)

	// ── Subscriptions ─────────────────────────────────────────────────────────
	subs := subscribeAll(ctx, h, nc, log)
	defer drainAll(subs)

	// ── Outbox worker ─────────────────────────────────────────────────────────
	go worker.Run(ctx)

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
		{domain.SubjectOFCreate, h.CreateOrder},
		{domain.SubjectOFGet, h.GetOrder},
		{domain.SubjectOFList, h.ListOrders},
		{domain.SubjectOperationStart, h.StartOperation},
		{domain.SubjectOperationComplete, h.CompleteOperation},
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
	_ handler.OrderRepository     = (*repo.PostgresRepo)(nil)
	_ handler.OperationRepository = (*repo.PostgresRepo)(nil)
	_ outbox.Repository           = (*repo.PostgresRepo)(nil)
)
