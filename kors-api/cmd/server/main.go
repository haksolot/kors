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
	"github.com/safran-ls/kors/kors-api/internal/adapter/postgres"
	"github.com/safran-ls/kors/kors-api/internal/domain/identity"
	"github.com/safran-ls/kors/kors-api/internal/graph/generated"
	"github.com/safran-ls/kors/kors-api/internal/graph/resolvers"
	"github.com/safran-ls/kors/kors-api/internal/usecase"
)

const (
	defaultPort      = "8080"
	systemIdentityID = "00000000-0000-0000-0000-000000000000"
)

func main() {
	// Charger .env si présent
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// 1. Initialiser le pool de connexion PostgreSQL
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Initialiser les adapters/repositories
	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	idRepo := &postgres.IdentityRepository{Pool: pool}

	// 3. Initialiser l'identité système par défaut (pour les tests)
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
		if err := idRepo.Create(ctx, sysID); err != nil {
			log.Printf("Warning: failed to create system identity: %v", err)
		}
	}

	// 4. Initialiser les usecases
	registerRTUseCase := &usecase.RegisterResourceTypeUseCase{Repo: rtRepo}
	createRUseCase := &usecase.CreateResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
	}
	transitionRUseCase := &usecase.TransitionResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
	}

	// 5. Initialiser le serveur GraphQL avec les resolvers et les usecases injectés
	resolver := &resolvers.Resolver{
		RegisterResourceTypeUseCase: registerRTUseCase,
		CreateResourceUseCase:       createRUseCase,
		TransitionResourceUseCase:   transitionRUseCase,
	}
	
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	// Routes
	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
