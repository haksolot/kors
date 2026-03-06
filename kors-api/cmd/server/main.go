package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/safran-ls/kors/kors-api/internal/adapter/postgres"
	korsnats "github.com/safran-ls/kors/kors-api/internal/adapter/nats"
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
	if port == "" {
		port = defaultPort
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	// 1. PostgreSQL Connection
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	// 2. NATS Connection
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Unable to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Unable to initialize JetStream: %v", err)
	}

	// Ensure KORS stream exists
	_, _ = js.AddStream(&nats.StreamConfig{
		Name:     "KORS",
		Subjects: []string{"kors.>"},
		Storage:  nats.FileStorage,
	})

	// 3. Adapters & Repositories
	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	idRepo := &postgres.IdentityRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	ePub := &korsnats.NatsPublisher{JS: js}

	// 4. Default System Identity & Permissions
	ctx := context.Background()
	sysID, _ := idRepo.GetByExternalID(ctx, "system")
	if sysID == nil {
		sysID = &identity.Identity{
			ID:         uuid.MustParse(systemIdentityID),
			ExternalID: "system",
			Name:       "KORS System",
			Type:       "system",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		_ = idRepo.Create(ctx, sysID)
	}

	// Ensure system has core permissions
	actions := []string{"write", "transition"}
	for _, action := range actions {
		allowed, _ := pRepo.Check(ctx, sysID.ID, action, nil, nil)
		if !allowed {
			_ = pRepo.Create(ctx, &permission.Permission{
				ID:         uuid.New(),
				IdentityID: sysID.ID,
				Action:     action,
				CreatedAt:  time.Now(),
			})
		}
	}

	// 5. UseCases
	registerRTUseCase := &usecase.RegisterResourceTypeUseCase{Repo: rtRepo}
	createRUseCase := &usecase.CreateResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
		PermissionRepo:   pRepo,
		EventPublisher:   ePub,
	}
	transitionRUseCase := &usecase.TransitionResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
		PermissionRepo:   pRepo,
		EventPublisher:   ePub,
	}
	grantPUseCase := &usecase.GrantPermissionUseCase{Repo: pRepo}

	// 6. GraphQL Resolver & Server
	resolver := &resolvers.Resolver{
		RegisterResourceTypeUseCase: registerRTUseCase,
		CreateResourceUseCase:       createRUseCase,
		TransitionResourceUseCase:   transitionRUseCase,
		GrantPermissionUseCase:      grantPUseCase,
	}
	
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
