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
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/safran-ls/kors/kors-api/internal/adapter/postgres"
	korsnats "github.com/safran-ls/kors/kors-api/internal/adapter/nats"
	korsminio "github.com/safran-ls/kors/kors-api/internal/adapter/minio"
	"github.com/safran-ls/kors/kors-api/internal/domain/identity"
	"github.com/safran-ls/kors/kors-api/internal/domain/permission"
	"github.com/safran-ls/kors/kors-api/internal/graph/generated"
	"github.com/safran-ls/kors/kors-api/internal/graph/resolvers"
	"github.com/safran-ls/kors/kors-api/internal/usecase"
)

const (
	defaultPort      = "8080"
	systemIdentityID = "00000000-0000-0000-0000-000000000000"
)

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" { port = defaultPort }

	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil { log.Fatalf("DB error: %v", err) }
	defer pool.Close()

	natsURL := os.Getenv("NATS_URL")
	nc, err := nats.Connect(natsURL)
	if err != nil { log.Fatalf("NATS error: %v", err) }
	defer nc.Close()
	js, _ := nc.JetStream()
	_, _ = js.AddStream(&nats.StreamConfig{Name: "KORS", Subjects: []string{"kors.>"}})

	minioURL := os.Getenv("MINIO_URL")
	minioAK := os.Getenv("MINIO_ACCESS_KEY")
	minioSK := os.Getenv("MINIO_SECRET_KEY")
	mClient, _ := minio.New(minioURL, &minio.Options{Creds: credentials.NewStaticV4(minioAK, minioSK, ""), Secure: false})

	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	idRepo := &postgres.IdentityRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	revRepo := &postgres.RevisionRepository{Pool: pool}
	fStore := &korsminio.MinioFileStore{Client: mClient, Bucket: "kors-files"}
	ePub := &korsnats.NatsPublisher{JS: js}

	ctx := context.Background()
	sysID, _ := idRepo.GetByExternalID(ctx, "system")
	if sysID == nil {
		sysID = &identity.Identity{ID: uuid.MustParse(systemIdentityID), ExternalID: "system", Name: "KORS System", Type: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		_ = idRepo.Create(ctx, sysID)
	}
	for _, action := range []string{"write", "transition", "admin"} {
		allowed, _ := pRepo.Check(ctx, sysID.ID, action, nil, nil)
		if !allowed {
			_ = pRepo.Create(ctx, &permission.Permission{ID: uuid.New(), IdentityID: sysID.ID, Action: action, CreatedAt: time.Now()})
		}
	}

	rootResolver := &resolvers.Resolver{
		RegisterResourceTypeUseCase: &usecase.RegisterResourceTypeUseCase{Repo: rtRepo, PermissionRepo: pRepo},
		CreateResourceUseCase:       &usecase.CreateResourceUseCase{ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub},
		TransitionResourceUseCase:   &usecase.TransitionResourceUseCase{ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub},
		GrantPermissionUseCase:      &usecase.GrantPermissionUseCase{Repo: pRepo},
		CreateRevisionUseCase:       &usecase.CreateRevisionUseCase{ResourceRepo: rRepo, RevisionRepo: revRepo, FileStore: fStore, EventRepo: eRepo, EventPublisher: ePub},
		ListResourcesUseCase:        &usecase.ListResourcesUseCase{Repo: rRepo},
		NatsConn:                    nc,
	}

	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: rootResolver}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	})

	http.Handle("/", playground.Handler("KORS GraphQL", "/query"))
	http.Handle("/query", srv)
	log.Printf("KORS API running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
