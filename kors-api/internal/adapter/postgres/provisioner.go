package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
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

func (p *PostgresProvisioner) DeprovisionModule(ctx context.Context, moduleName string) error {
	username := fmt.Sprintf("user_%s", moduleName)
	schema := moduleName

	// Séquence de nettoyage chirurgicale
	cleanupQueries := []string{
		// 1. Révoquer les droits sur le cœur
		fmt.Sprintf("REVOKE ALL PRIVILEGES ON SCHEMA kors FROM %s", username),
		fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA kors FROM %s", username),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA kors REVOKE ALL ON TABLES FROM %s", username),
		
		// 2. Supprimer les objets appartenant au module
		fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema),
		
		// 3. Désengager le rôle des objets restants
		fmt.Sprintf("REASSIGN OWNED BY %s TO kors", username),
		fmt.Sprintf("DROP OWNED BY %s", username),
	}

	for _, q := range cleanupQueries {
		_, _ = p.Pool.Exec(ctx, q)
	}

	// 4. Suppression finale du rôle
	_, err := p.Pool.Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", username))
	if err != nil {
		return fmt.Errorf("failed to drop role: %w", err)
	}

	return nil
}

func (p *PostgresProvisioner) ListModules(ctx context.Context) ([]string, error) {
	// We search for roles that follow our naming convention "user_*"
	query := `
		SELECT rolname 
		FROM pg_roles 
		WHERE rolname LIKE 'user_%'
	`
	rows, err := p.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []string
	for rows.Next() {
		var rolname string
		if err := rows.Scan(&rolname); err != nil {
			return nil, err
		}
		// Remove "user_" prefix to get original module name
		modules = append(modules, rolname[5:])
	}
	return modules, nil
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
