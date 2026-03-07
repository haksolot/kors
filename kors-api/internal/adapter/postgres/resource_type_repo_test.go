package postgres

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/kors-project/kors/kors-api/internal/domain/resourcetype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestResourceTypeRepository(t *testing.T) {
	ctx := context.Background()

	// 1. Démarrer le conteneur PostgreSQL
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("kors_test"),
		postgres.WithUsername("kors"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(15*time.Second)),
	)
	require.NoError(t, err)
	defer func() {
		err := pgContainer.Terminate(ctx)
		assert.NoError(t, err)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// 2. Appliquer les migrations
	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	defer db.Close()

	// Chemin vers les migrations (relatif à ce fichier de test)
	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	err = goose.Up(db, migrationsDir)
	require.NoError(t, err)

	// 3. Initialiser le pool pgx et le repository
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	repo := &ResourceTypeRepository{Pool: pool}

	t.Run("Create and Get ResourceType", func(t *testing.T) {
		rt := &resourcetype.ResourceType{
			ID:          uuid.New(),
			Name:        "test-tool",
			Description: "A tool for testing",
			JSONSchema:  map[string]interface{}{"type": "object"},
			Transitions: map[string]interface{}{"states": []string{"idle", "busy"}},
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		// Test Create
		err := repo.Create(ctx, rt)
		assert.NoError(t, err)

		// Test GetByName
		found, err := repo.GetByName(ctx, "test-tool")
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, rt.ID, found.ID)
		assert.Equal(t, rt.Name, found.Name)
		assert.Equal(t, rt.Description, found.Description)
	})

	t.Run("List ResourceTypes", func(t *testing.T) {
		results, err := repo.List(ctx)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-tool", results[0].Name)
	})
}
