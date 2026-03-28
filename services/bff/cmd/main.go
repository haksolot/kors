package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/haksolot/kors/libs/core"
	"github.com/haksolot/kors/services/bff/handler"
)

func main() {
	log := core.NewLogger(getEnv("SERVICE_NAME", "kors-bff"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Observability ──────────────────────────────────────────────────────────
	if otlp := getEnv("OTLP_ENDPOINT", ""); otlp != "" {
		shutdown, err := core.InitTracer(ctx, getEnv("SERVICE_NAME", "kors-bff"), otlp)
		if err != nil {
			log.Fatal().Err(err).Msg("init tracer")
		}
		defer func() { _ = shutdown(context.Background()) }()
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// ── NATS ───────────────────────────────────────────────────────────────────
	natsOpts := []nats.Option{
		nats.Name("kors-bff"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Warn().Err(err).Msg("NATS disconnected")
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Info().Msg("NATS reconnected")
		}),
	}
	if credsPath := getEnv("NATS_CREDS_PATH", ""); credsPath != "" {
		natsOpts = append(natsOpts, nats.UserCredentials(credsPath))
	}

	nc, err := nats.Connect(getEnv("NATS_URL", nats.DefaultURL), natsOpts...)
	if err != nil {
		log.Fatal().Err(err).Msg("connect NATS")
	}
	defer nc.Drain() //nolint:errcheck

	// ── JWT validator ──────────────────────────────────────────────────────────
	var validator *core.JWTValidator
	if jwksURL := getEnv("JWKS_ENDPOINT", ""); jwksURL != "" {
		validator, err = core.NewJWTValidator(ctx, jwksURL)
		if err != nil {
			log.Fatal().Err(err).Msg("init JWT validator")
		}
	} else {
		log.Warn().Msg("JWKS_ENDPOINT not set — JWT validation DISABLED (dev mode)")
		validator = core.NewNoopJWTValidator()
	}

	// ── Handler + WebSocket hub ────────────────────────────────────────────────
	h := handler.New(ctx, nc, validator, reg, log)

	evtSubs, err := h.Hub().SubscribeEvents(nc)
	if err != nil {
		log.Fatal().Err(err).Msg("subscribe WebSocket event subjects")
	}
	defer func() {
		for _, s := range evtSubs {
			_ = s.Drain()
		}
	}()

	// ── HTTP server ────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         getEnv("LISTEN_ADDR", ":8080"),
		Handler:      h.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Metrics server (separate port) ────────────────────────────────────────
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	metricsSrv := &http.Server{
		Addr:    getEnv("METRICS_ADDR", ":9092"),
		Handler: metricsMux,
	}

	go func() {
		log.Info().Str("addr", metricsSrv.Addr).Msg("metrics server listening")
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("metrics server error")
		}
	}()

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("BFF HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown")
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("metrics server shutdown")
	}

	log.Info().Msg("stopped")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
