package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/haksolot/kors/kors-api/internal/domain/provisioning"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var moduleNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{1,30}$`)

func validateModuleName(name string) error {
	if !moduleNameRegex.MatchString(name) {
		return fmt.Errorf(
			"invalid module name %q: must match ^[a-z][a-z0-9_]{1,30}$ (lowercase, start with letter, max 30 chars)",
			name,
		)
	}
	return nil
}

func buildConnectionString(username, password, schema string) string {
	base := os.Getenv("DATABASE_URL")
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	u.User = url.UserPassword(username, password)
	q := u.Query()
	q.Set("options", fmt.Sprintf("-csearch_path=%s,kors", schema))
	u.RawQuery = q.Encode()
	return u.String()
}

type PostgresProvisioner struct {
	Pool *pgxpool.Pool
}

func (p *PostgresProvisioner) ProvisionModule(ctx context.Context, moduleName string) (*provisioning.ModuleCredentials, error) {
	if err := validateModuleName(moduleName); err != nil {
		return nil, err
	}

	username := fmt.Sprintf("user_%s", moduleName)
	schema := moduleName
	password := generateRandomPassword(16)

	// Execute sequence of DDL commands
	// 1. Create Schema
	// 2. Create Role
	// 3. Grant usage on KORS schema (Read-Only)
	// 4. Set search_path for the new role
	// 5. Set owner of new schema to the new role
	queries := []string{
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema),
		fmt.Sprintf("DROP ROLE IF EXISTS %s", username), // Cleanup if exists
		fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD '%s'", username, password),
		fmt.Sprintf("GRANT USAGE ON SCHEMA kors TO %s", username),
		fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA kors TO %s", username),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA kors GRANT SELECT ON TABLES TO %s", username),
		fmt.Sprintf("ALTER ROLE %s SET search_path TO %s, kors", username, schema),
		fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", schema, username),
	}

	for _, q := range queries {
		if _, err := p.Pool.Exec(ctx, q); err != nil {
			return nil, fmt.Errorf("failed to execute provisioning query '%s': %w", q, err)
		}
	}

	bucketName := fmt.Sprintf("module-%s", strings.ReplaceAll(strings.ToLower(moduleName), "_", "-"))
	return &provisioning.ModuleCredentials{
		ModuleName:       moduleName,
		Schema:           schema,
		Username:         username,
		Password:         password,
		ConnectionString: buildConnectionString(username, password, schema),
		BucketName:       bucketName,
	}, nil
}

func (p *PostgresProvisioner) DeprovisionModule(ctx context.Context, moduleName string) error {
	if err := validateModuleName(moduleName); err != nil {
		return err
	}

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

func (p *PostgresProvisioner) RegisterModule(ctx context.Context, r *provisioning.ModuleRecord) error {
	_, err := p.Pool.Exec(ctx, `
        INSERT INTO kors.modules
            (name, schema_name, pg_username, minio_bucket, identity_id, provisioned_at, provisioned_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (name) DO UPDATE SET
            schema_name    = EXCLUDED.schema_name,
            pg_username    = EXCLUDED.pg_username,
            minio_bucket   = EXCLUDED.minio_bucket,
            identity_id    = EXCLUDED.identity_id,
            provisioned_at = EXCLUDED.provisioned_at,
            provisioned_by = EXCLUDED.provisioned_by
    `, r.Name, r.SchemaName, r.PgUsername, r.MinioBucket,
		r.IdentityID, r.ProvisionedAt, r.ProvisionedBy)
	return err
}

func (p *PostgresProvisioner) UnregisterModule(ctx context.Context, moduleName string) error {
	_, err := p.Pool.Exec(ctx, "DELETE FROM kors.modules WHERE name = $1", moduleName)
	return err
}

func (p *PostgresProvisioner) GetModule(ctx context.Context, moduleName string) (*provisioning.ModuleRecord, error) {
	var r provisioning.ModuleRecord
	err := p.Pool.QueryRow(ctx, `
        SELECT name, schema_name, pg_username, minio_bucket, identity_id, provisioned_at, provisioned_by
        FROM kors.modules WHERE name = $1
    `, moduleName).Scan(&r.Name, &r.SchemaName, &r.PgUsername, &r.MinioBucket,
		&r.IdentityID, &r.ProvisionedAt, &r.ProvisionedBy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (p *PostgresProvisioner) ListModules(ctx context.Context) ([]string, error) {
	rows, err := p.Pool.Query(ctx, "SELECT name FROM kors.modules ORDER BY provisioned_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (p *PostgresProvisioner) RotatePassword(ctx context.Context, moduleName string) (*provisioning.ModuleCredentials, error) {
	if err := validateModuleName(moduleName); err != nil {
		return nil, err
	}
	username := fmt.Sprintf("user_%s", moduleName)
	newPassword := generateRandomPassword(16)
	_, err := p.Pool.Exec(ctx, fmt.Sprintf("ALTER ROLE %s WITH PASSWORD '%s'", username, newPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to rotate password: %w", err)
	}
	bucketName := fmt.Sprintf("module-%s", strings.ReplaceAll(strings.ToLower(moduleName), "_", "-"))
	return &provisioning.ModuleCredentials{
		ModuleName:       moduleName,
		Schema:           moduleName,
		Username:         username,
		Password:         newPassword,
		ConnectionString: buildConnectionString(username, newPassword, moduleName),
		BucketName:       bucketName,
	}, nil
}

func generateRandomPassword(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
