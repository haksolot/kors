package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/safran-ls/kors/kors-api/internal/adapter/postgres"
	"github.com/safran-ls/kors/kors-api/internal/graph/generated"
	"github.com/safran-ls/kors/kors-api/internal/graph/resolvers"
	"github.com/safran-ls/kors/kors-api/internal/usecase"
)

const defaultPort = "8080"

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

	// 3. Initialiser les usecases
	registerRTUseCase := &usecase.RegisterResourceTypeUseCase{Repo: rtRepo}
	createRUseCase := &usecase.CreateResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
	}
	transitionRUseCase := &usecase.TransitionResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
	}

	// 4. Initialiser le serveur GraphQL avec les resolvers et les usecases injectés
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
