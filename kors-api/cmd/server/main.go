package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	korsauth "github.com/haksolot/kors/kors-api/internal/middleware"
	"github.com/haksolot/kors/kors-api/internal/graph/generated"
	"github.com/haksolot/kors/kors-api/internal/graph/resolvers"
	_ "github.com/haksolot/kors/kors-api/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title KORS API
// @version 1.0
// @description KORS (Knowledge-Oriented Resource System) Core API.
// @description This API manages resources, transitions, and module governance.
// @contact.name KORS Support
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization

type HealthStatus struct {
    Status   string            `json:"status"`
    Checks   map[string]string `json:"checks"`
    Hostname string            `json:"hostname"`
}

func makeHealthHandler(pool *pgxpool.Pool, nc *nats.Conn) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        checks := make(map[string]string)
        overall := "ok"

        // DB check
        if err := pool.Ping(r.Context()); err != nil {
            checks["database"] = "error: " + err.Error()
            overall = "degraded"
        } else {
            checks["database"] = "ok"
        }

        // NATS check
        if nc == nil || !nc.IsConnected() {
            checks["nats"] = "disconnected"
            overall = "degraded"
        } else {
            checks["nats"] = "ok"
        }

        hostname, _ := os.Hostname()
        status := HealthStatus{Status: overall, Checks: checks, Hostname: hostname}

        w.Header().Set("Content-Type", "application/json")
        if overall != "ok" {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(status)
    }
}

func main() {
	_ = godotenv.Load()
	cfg := loadConfig()
	
	// Identify current instance (useful for Load Balancing logs)
	hostname, _ := os.Hostname()
	log.Printf("[Instance: %s] Starting KORS API...", hostname)

	// 1. Database Connection with Pool limits
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Critical: failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	log.Println("Running database migrations...")
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	if err := goose.Up(db, "/migrations"); err != nil {
		log.Printf("Warning: migrations failed: %v", err)
	}
	db.Close()

	// 2. NATS Connection
	nc, jsCtx, err := connectNATS(cfg.NatsURL)
	if err == nil {
		defer nc.Close()
	}

	// 3. MinIO Client
	mClient, err := connectMinio(cfg.MinioURL, cfg.MinioAccessKey, cfg.MinioSecretKey, os.Getenv("MINIO_USE_SSL") == "true")
	if err == nil {
		exists, _ := mClient.BucketExists(ctx, cfg.MinioBucket)
		if !exists {
			_ = mClient.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
		}
	}

	// 4. Repositories
	idRepo := &postgres.IdentityRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}

	// 5. Bootstrap System Identity
	_ = bootstrapSystemIdentity(ctx, idRepo, pRepo)

	// 6. GraphQL Setup
	resolver := resolvers.NewResolver(pool, nc, jsCtx, mClient)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	})
	srv.Use(extension.FixedComplexityLimit(cfg.ComplexityLimit))
	if cfg.GraphQLIntrospection {
		srv.Use(extension.Introspection{})
	}

	// 7. HTTP Routing
	mux := chi.NewRouter()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)

	// Public routes (No Auth)
	mux.Get("/healthz", makeHealthHandler(pool, nc))
	mux.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	mux.Get("/swagger/*", httpSwagger.WrapHandler)

	// Protected routes
	jwksCache := korsauth.NewJWKSCache(cfg.JWKSEndpoint, time.Hour)
	
	identityCacheTTLStr := getEnv("IDENTITY_CACHE_TTL", "5m")
	identityCacheTTL, _ := time.ParseDuration(identityCacheTTLStr)
	identityCache := korsauth.NewIdentityCache(identityCacheTTL)

	authMiddleware := &korsauth.AuthMiddleware{IdentityRepo: idRepo, JWKSCache: jwksCache, IdentityCache: identityCache}

	mux.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)
		r.Handle("/query", srv)
		r.Handle("/", playground.Handler("KORS", "/query"))
	})

	// 8. GRACEFUL SHUTDOWN Logic
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("[Instance: %s] API running on port %s", hostname, cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Listen error: %v", err)
		}
	}()

	<-ctx.Done() // Wait for SIGINT or SIGTERM

	log.Printf("[Instance: %s] Shutting down gracefully...", hostname)
	
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Printf("[Instance: %s] KORS API stopped.", hostname)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
