package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	korsnats "github.com/safran-ls/kors/kors-events/internal/adapter/nats"
	"github.com/safran-ls/kors/kors-events/internal/adapter/postgres"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
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

	// 3. Initialize Consumer
	repo := &postgres.EventRepository{Pool: pool}
	consumer := &korsnats.EventConsumer{
		JS:        js,
		EventRepo: repo,
	}

	// 4. Run consumer in context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Consumer failed: %v", err)
	}
}
