package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haksolot/kors/services/qms/domain"
	"github.com/haksolot/kors/services/qms/handler"
	"github.com/haksolot/kors/services/qms/outbox"
	"github.com/haksolot/kors/services/qms/repo"
)

// compile-time interface compliance checks.
var (
	_ handler.NCRepository   = (*repo.PostgresRepo)(nil)
	_ handler.CAPARepository = (*repo.PostgresRepo)(nil)
	_ domain.Transactor      = (*repo.PostgresRepo)(nil)
	_ outbox.Repository      = (*repo.PostgresRepo)(nil)
)

// setupDB starts a PostgreSQL container, runs migrations, and returns a pool.
func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("kors_test"),
		tcpostgres.WithUsername("kors"),
		tcpostgres.WithPassword("kors_test"),
		tcpostgres.WithSQLDriver("pgx"),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return pool.Ping(ctx) == nil
	}, 30*time.Second, 500*time.Millisecond, "postgres not ready")
	t.Cleanup(pool.Close)

	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, "../migrations"), "run migrations")

	return pool
}

// ── NonConformity tests ───────────────────────────────────────────────────────

func TestPostgresRepo_SaveAndFindNC(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, err := domain.NewNC("op-uuid-001", "of-uuid-001", "DEF-001", "surface scratch", 3, []string{"SN-001", "SN-002"}, "operator-uuid")
	require.NoError(t, err)

	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	got, err := r.FindNCByID(ctx, nc.ID)
	require.NoError(t, err)
	assert.Equal(t, nc.ID, got.ID)
	assert.Equal(t, nc.OperationID, got.OperationID)
	assert.Equal(t, nc.OFID, got.OFID)
	assert.Equal(t, nc.DefectCode, got.DefectCode)
	assert.Equal(t, nc.Description, got.Description)
	assert.Equal(t, nc.AffectedQuantity, got.AffectedQuantity)
	assert.Equal(t, nc.SerialNumbers, got.SerialNumbers)
	assert.Equal(t, nc.DeclaredBy, got.DeclaredBy)
	assert.Equal(t, domain.NCStatusOpen, got.Status)
}

func TestPostgresRepo_FindNCByOperationID(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, _ := domain.NewNC("op-uuid-002", "of-uuid-001", "DEF-002", "", 1, nil, "operator-uuid")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	got, err := r.FindNCByOperationID(ctx, "op-uuid-002")
	require.NoError(t, err)
	assert.Equal(t, nc.ID, got.ID)
}

func TestPostgresRepo_FindNCByID_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.FindNCByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNCNotFound))
}

func TestPostgresRepo_NCAlreadyExists(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, _ := domain.NewNC("op-uuid-003", "of-uuid-001", "DEF-003", "", 1, nil, "operator-uuid")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	nc2, _ := domain.NewNC("op-uuid-003", "of-uuid-001", "DEF-003", "", 1, nil, "operator-uuid")
	err := r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc2)
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNCAlreadyExists))
}

func TestPostgresRepo_UpdateNC(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, _ := domain.NewNC("op-uuid-004", "of-uuid-001", "DEF-004", "", 1, nil, "operator-uuid")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	require.NoError(t, nc.StartAnalysis("analyst-uuid"))
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateNC(ctx, nc)
	}))

	got, err := r.FindNCByID(ctx, nc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.NCStatusUnderAnalysis, got.Status)
}

func TestPostgresRepo_ListNCs(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	for i, opID := range []string{"op-list-1", "op-list-2", "op-list-3"} {
		nc, _ := domain.NewNC(opID, "of-uuid-001", "DEF-LIST", "", i+1, nil, "operator-uuid")
		require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
			return tx.SaveNC(ctx, nc)
		}))
	}

	all, err := r.ListNCs(ctx, domain.ListNCsFilter{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 3)

	open := domain.NCStatusOpen
	filtered, err := r.ListNCs(ctx, domain.ListNCsFilter{Status: &open})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(filtered), 3)
}

// ── CAPA tests ────────────────────────────────────────────────────────────────

func TestPostgresRepo_SaveAndFindCAPA(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, _ := domain.NewNC("op-capa-001", "of-uuid-001", "DEF-CAPA", "", 1, nil, "operator-uuid")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	capa, err := domain.NewCAPA(nc.ID, domain.CAPAActionCorrective, "Fix the defect thoroughly", "owner-uuid", nil)
	require.NoError(t, err)

	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveCAPA(ctx, capa)
	}))

	got, err := r.FindCAPAByID(ctx, capa.ID)
	require.NoError(t, err)
	assert.Equal(t, capa.ID, got.ID)
	assert.Equal(t, nc.ID, got.NCID)
	assert.Equal(t, domain.CAPAActionCorrective, got.ActionType)
	assert.Equal(t, domain.CAPAStatusOpen, got.Status)
}

func TestPostgresRepo_FindCAPAByID_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.FindCAPAByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrCAPANotFound))
}

func TestPostgresRepo_UpdateCAPA(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	nc, _ := domain.NewNC("op-capa-002", "of-uuid-001", "DEF-CAPA2", "", 1, nil, "operator-uuid")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveNC(ctx, nc)
	}))

	capa, _ := domain.NewCAPA(nc.ID, domain.CAPAActionPreventive, "Prevent recurrence", "owner-uuid", nil)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveCAPA(ctx, capa)
	}))

	require.NoError(t, capa.Start())
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.UpdateCAPA(ctx, capa)
	}))

	got, err := r.FindCAPAByID(ctx, capa.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CAPAStatusInProgress, got.Status)
}

func TestPostgresRepo_Outbox(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "TestEvent",
			Subject:   "kors.test.event",
			Payload:   []byte(`{"test":true}`),
		})
	}))

	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "TestEvent", entries[0].EventType)

	require.NoError(t, r.MarkOutboxPublished(ctx, []int64{entries[0].ID}))

	after, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, after)
}
