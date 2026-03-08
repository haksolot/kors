package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	korsauth "github.com/haksolot/kors/kors-api/internal/middleware"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
	"github.com/haksolot/kors/kors-api/internal/graph/generated"
	"github.com/haksolot/kors/kors-api/internal/graph/resolvers"
)

func main() {
	_ = godotenv.Load()
	
	// --- Configuration (Environment only) ---
	port := getEnv("PORT", "8080")
	dbURL := os.Getenv("DATABASE_URL")
	natsURL := getEnv("NATS_URL", nats.DefaultURL)
	minioURL := os.Getenv("MINIO_URL")
	minioAK := os.Getenv("MINIO_ACCESS_KEY")
	minioSK := os.Getenv("MINIO_SECRET_KEY")
	minioBucket := getEnv("MINIO_BUCKET", "kors-files")
	complexityLimit, _ := strconv.Atoi(getEnv("GRAPHQL_COMPLEXITY_LIMIT", "1000"))
	
	// Identify current instance (useful for Load Balancing logs)
	hostname, _ := os.Hostname()
	log.Printf("[Instance: %s] Starting KORS API...", hostname)

	// 1. Database Connection with Pool limits
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Critical: failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 2. NATS Connection
	nc, err := nats.Connect(natsURL)
	if err == nil {
		defer nc.Close()
		js, _ := nc.JetStream()
		if js != nil {
			_, _ = js.AddStream(&nats.StreamConfig{Name: "KORS", Subjects: []string{"kors.>"}})
		}
	}

	// 3. MinIO Client
	mClient, err := minio.New(minioURL, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAK, minioSK, ""),
		Secure: os.Getenv("MINIO_USE_SSL") == "true",
	})
	if err == nil {
		exists, _ := mClient.BucketExists(ctx, minioBucket)
		if !exists {
			_ = mClient.MakeBucket(ctx, minioBucket, minio.MakeBucketOptions{})
		}
	}

	// 4. Repositories
	idRepo := &postgres.IdentityRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}

	// 5. Bootstrap System Identity
	sysID, _ := idRepo.GetByExternalID(ctx, "system")
	if sysID == nil {
		_ = idRepo.Create(ctx, &identity.Identity{ID: uuid.Nil, ExternalID: "system", Name: "System", Type: "system", CreatedAt: time.Now()})
	}
	for _, a := range []string{"write", "transition", "admin"} {
		allowed, _ := pRepo.Check(ctx, uuid.Nil, a, nil, nil)
		if !allowed {
			_ = pRepo.Create(ctx, &permission.Permission{ID: uuid.New(), IdentityID: uuid.Nil, Action: a, CreatedAt: time.Now()})
		}
	}

	// 6. GraphQL Setup
	var jsCtx nats.JetStreamContext
	if nc != nil { jsCtx, _ = nc.JetStream() }
	
	resolver := resolvers.NewResolver(pool, nc, jsCtx, mClient)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	})
	srv.Use(extension.FixedComplexityLimit(complexityLimit))
	if os.Getenv("GRAPHQL_INTROSPECTION") == "true" {
		srv.Use(extension.Introspection{})
	}

	// 7. HTTP Routing
	mux := chi.NewRouter()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	mux.Use((&korsauth.AuthMiddleware{IdentityRepo: idRepo}).Handler)
	
	mux.Handle("/query", srv)
	mux.Handle("/", playground.Handler("KORS", "/query"))
	mux.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 8. GRACEFUL SHUTDOWN Logic
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[Instance: %s] API running on port %s", hostname, port)
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
