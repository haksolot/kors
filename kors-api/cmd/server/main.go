package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
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
	"github.com/safran-ls/kors/kors-api/internal/adapter/postgres"
	korsauth "github.com/safran-ls/kors/kors-api/internal/middleware"
	"github.com/safran-ls/kors/kors-api/internal/domain/identity"
	"github.com/safran-ls/kors/kors-api/internal/domain/permission"
	"github.com/safran-ls/kors/kors-api/internal/graph/generated"
	"github.com/safran-ls/kors/kors-api/internal/graph/resolvers"
)

func main() {
	_ = godotenv.Load()
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	// 1. Core Services Connections
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatalf("DB: %v", err) }
	
	nc, _ := nats.Connect(os.Getenv("NATS_URL"))
	var js nats.JetStreamContext
	if nc != nil { js, _ = nc.JetStream() }

	mClient, _ := minio.New(os.Getenv("MINIO_URL"), &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
		Secure: false,
	})

	// 2. Identity Bootstrap
	idRepo := &postgres.IdentityRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	ctx := context.Background()
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

	// 3. Assemble Resolver
	r := resolvers.NewResolver(pool, nc, js, mClient)

	// 4. GraphQL Server
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	})

	mux := chi.NewRouter()
	mux.Use(middleware.Recoverer)
	mux.Use((&korsauth.AuthMiddleware{IdentityRepo: idRepo}).Handler)
	
	mux.Handle("/query", srv)
	mux.Handle("/", playground.Handler("KORS", "/query"))

	log.Printf("KORS API fully stabilized on %s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
