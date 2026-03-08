package testhelper

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type plainLogger struct {
	t *testing.T
}

func (l plainLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

// SetupTestDB demarre un conteneur PostgreSQL, applique les migrations,
// et retourne un pool pgx pret a l'emploi.
// Le conteneur est automatiquement termine en fin de test via t.Cleanup.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("kors_test"),
		postgres.WithUsername("kors"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
		testcontainers.WithLogger(plainLogger{t: t}),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Appliquer les migrations
	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	defer db.Close()

	// Chemin absolu vers les migrations depuis ce fichier
	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
	require.NoError(t, goose.Up(db, migrationsDir))

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	return pool
}
