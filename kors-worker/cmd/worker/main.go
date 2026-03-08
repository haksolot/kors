package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/haksolot/kors/kors-worker/internal/adapter/postgres"
	"github.com/haksolot/kors/kors-worker/internal/jobs"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// 1. Database Connection
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Initialize Repositories
	pRepo := &postgres.PermissionRepository{Pool: pool}

	// 3. Initialize Jobs
	intervalStr := os.Getenv("PERMISSIONS_CLEANUP_INTERVAL")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		interval = 1 * time.Hour // Default
	}

	cleanupJob := &jobs.PermissionCleanupJob{
		Repo:     pRepo,
		Interval: interval,
	}

	// 4. Run jobs in background
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Println("KORS Worker started...")
	
	go cleanupJob.Run(ctx)

	<-ctx.Done()
	log.Println("KORS Worker shutting down...")
}
