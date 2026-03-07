package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/safran-ls/tms-km/internal/kors"
	"github.com/safran-ls/tms-km/internal/model"
	"github.com/safran-ls/tms-km/internal/store"
)

func main() {
	_ = godotenv.Load()

	// 1. Connection using the PROVISIONED credentials
	// In a real scenario, these would come from env vars
	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to TMS DB: %v", err)
	}
	defer pool.Close()

	// 2. Ensure business table exists in TMS schema
	_, _ = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS tms.tools (
			id UUID PRIMARY KEY,
			serial_number TEXT NOT NULL,
			model TEXT NOT NULL,
			diameter DOUBLE PRECISION,
			length DOUBLE PRECISION
		)
	`)

	// 3. Setup KORS Client
	korsClient := kors.NewClient("http://localhost:8080/query", "system")

	// 4. BUSINESS ACTION: Create a new tool
	newTool := &model.Tool{
		SerialNumber: "TMS-2026-X1",
		Model:        "SuperDrill 3000",
		Diameter:     12.5,
		Length:       150.0,
	}

	log.Println("Step 1: Registering tool in KORS...")
	korsID, err := korsClient.CreateResource(context.Background(), "tool", "idle", map[string]interface{}{
		"serial": newTool.SerialNumber,
	})
	if err != nil {
		log.Fatalf("KORS Registration failed: %v", err)
	}
	newTool.ID = korsID
	log.Printf("KORS assigned UUID: %s", korsID)

	log.Println("Step 2: Saving local business data...")
	toolStore := &store.ToolStore{Pool: pool}
	if err := toolStore.Save(context.Background(), newTool); err != nil {
		log.Fatalf("Business data save failed: %v", err)
	}

	log.Println("SUCCESS: Tool fully synchronized between KORS and TMS module.")
}
