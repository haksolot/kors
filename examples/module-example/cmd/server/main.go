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
	"github.com/kors-project/kors/examples/module-example/internal/graph/generated"
	"github.com/kors-project/kors/examples/module-example/internal/graph/resolvers"
	"github.com/kors-project/kors/examples/module-example/internal/store"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to TMS DB: %v", err)
	}
	defer pool.Close()

	// Ensure business table exists
	_, _ = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS tms.tools (
			id UUID PRIMARY KEY,
			serial_number TEXT NOT NULL,
			model TEXT NOT NULL,
			diameter DOUBLE PRECISION,
			length DOUBLE PRECISION
		)
	`)

	// GraphQL Setup
	resolver := &resolvers.Resolver{
		Store: &store.ToolStore{Pool: pool},
	}
	
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	http.Handle("/", playground.Handler("TMS Subgraph", "/query"))
	http.Handle("/query", srv)

	port := "8081"
	log.Printf("TMS Subgraph running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
