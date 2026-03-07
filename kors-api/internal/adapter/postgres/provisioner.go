package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safran-ls/kors/kors-api/internal/domain/provisioning"
)

type PostgresProvisioner struct {
	Pool *pgxpool.Pool
}

func (p *PostgresProvisioner) ProvisionModule(ctx context.Context, moduleName string) (*provisioning.ModuleCredentials, error) {
	username := fmt.Sprintf("user_%s", moduleName)
	schema := moduleName
	password := generateRandomPassword(16)

	// Execute sequence of DDL commands
	// 1. Create Schema
	// 2. Create Role
	// 3. Grant usage on KORS schema (Read-Only)
	// 4. Set owner of new schema to the new role
	queries := []string{
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema),
		fmt.Sprintf("DROP ROLE IF EXISTS %s", username), // Cleanup if exists
		fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD '%s'", username, password),
		fmt.Sprintf("GRANT USAGE ON SCHEMA kors TO %s", username),
		fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA kors TO %s", username),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA kors GRANT SELECT ON TABLES TO %s", username),
		fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", schema, username),
	}

	for _, q := range queries {
		if _, err := p.Pool.Exec(ctx, q); err != nil {
			return nil, fmt.Errorf("failed to execute provisioning query '%s': %w", q, err)
		}
	}

	return &provisioning.ModuleCredentials{
		ModuleName: moduleName,
		Schema:     schema,
		Username:   username,
		Password:   password,
	}, nil
}

func generateRandomPassword(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
