package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/repo"
)

// setupDB starts a PostgreSQL container, runs migrations, and returns a pool.
// It is registered for cleanup via t.Cleanup.
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
	t.Cleanup(pool.Close)

	// Run goose migrations from the migrations directory.
	db, err := goose.OpenDBWithDriver("pgx", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, goose.Up(db, "../migrations"), "run migrations")

	return pool
}

// ── Order tests ───────────────────────────────────────────────────────────────

func TestPostgresRepo_SaveAndFindOrder(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, err := domain.NewOrder("OF-TEST-001", "prod-uuid-1", 50)
	require.NoError(t, err)

	require.NoError(t, r.Save(ctx, order))

	got, err := r.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
	assert.Equal(t, order.Reference, got.Reference)
	assert.Equal(t, order.ProductID, got.ProductID)
	assert.Equal(t, order.Quantity, got.Quantity)
	assert.Equal(t, domain.OrderStatusPlanned, got.Status)
}

func TestPostgresRepo_FindByID_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.FindByID(context.Background(), "non-existent-id")
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
}

func TestPostgresRepo_Save_DuplicateReference(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	o1, _ := domain.NewOrder("OF-DUP-001", "prod-1", 10)
	require.NoError(t, r.Save(ctx, o1))

	o2, _ := domain.NewOrder("OF-DUP-001", "prod-2", 20)
	err := r.Save(ctx, o2)
	require.ErrorIs(t, err, domain.ErrOrderAlreadyExists)
}

func TestPostgresRepo_UpdateOrder(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-UPDATE-001", "prod-1", 10)
	require.NoError(t, r.Save(ctx, order))

	require.NoError(t, order.Start())
	require.NoError(t, r.Update(ctx, order))

	got, err := r.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusInProgress, got.Status)
	assert.NotNil(t, got.StartedAt)
}

func TestPostgresRepo_ListOrders(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	o1, _ := domain.NewOrder("OF-LIST-001", "prod-1", 10)
	o2, _ := domain.NewOrder("OF-LIST-002", "prod-1", 20)
	o3, _ := domain.NewOrder("OF-LIST-003", "prod-1", 30)
	require.NoError(t, r.Save(ctx, o1))
	require.NoError(t, r.Save(ctx, o2))
	require.NoError(t, r.Save(ctx, o3))
	require.NoError(t, o2.Start())
	require.NoError(t, r.Update(ctx, o2))

	t.Run("list all", func(t *testing.T) {
		orders, err := r.List(ctx, domain.ListOrdersFilter{PageSize: 10})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(orders), 3)
	})

	t.Run("filter by status planned", func(t *testing.T) {
		planned := domain.OrderStatusPlanned
		orders, err := r.List(ctx, domain.ListOrdersFilter{Status: &planned, PageSize: 10})
		require.NoError(t, err)
		for _, o := range orders {
			assert.Equal(t, domain.OrderStatusPlanned, o.Status)
		}
	})
}

// ── Operation tests ───────────────────────────────────────────────────────────

func TestPostgresRepo_SaveAndFindOperation(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-OP-001", "prod-1", 10)
	require.NoError(t, r.Save(ctx, order))

	op, err := domain.NewOperation(order.ID, 1, "Découpe laser")
	require.NoError(t, err)
	require.NoError(t, r.SaveOperation(ctx, op))

	got, err := r.FindOperationByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, op.ID, got.ID)
	assert.Equal(t, order.ID, got.OFID)
	assert.Equal(t, 1, got.StepNumber)
	assert.Equal(t, domain.OperationStatusPending, got.Status)
}

func TestPostgresRepo_FindOperationByID_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)

	_, err := r.FindOperationByID(context.Background(), "non-existent")
	require.ErrorIs(t, err, domain.ErrOperationNotFound)
}

func TestPostgresRepo_UpdateOperation(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-UOP-001", "prod-1", 10)
	require.NoError(t, r.Save(ctx, order))
	op, _ := domain.NewOperation(order.ID, 1, "Soudure")
	require.NoError(t, r.SaveOperation(ctx, op))

	require.NoError(t, op.Start("operator-uuid-1"))
	require.NoError(t, r.UpdateOperation(ctx, op))

	got, err := r.FindOperationByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusInProgress, got.Status)
	assert.Equal(t, "operator-uuid-1", got.OperatorID)
}

// ── Outbox tests ──────────────────────────────────────────────────────────────

func TestPostgresRepo_OutboxRoundTrip(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	// Insert an outbox entry inside a real transaction.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	entry := domain.OutboxEntry{
		EventType: "of.created",
		Subject:   "kors.mes.of.created",
		Payload:   []byte("fake-proto-payload"),
	}
	require.NoError(t, r.InsertOutboxTx(ctx, tx, entry))
	require.NoError(t, tx.Commit(ctx))

	// The entry should now appear as unpublished.
	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "of.created", entries[0].EventType)
	assert.Equal(t, []byte("fake-proto-payload"), entries[0].Payload)

	// Mark as published.
	require.NoError(t, r.MarkOutboxPublished(ctx, []int64{entries[0].ID}))

	// Should no longer appear.
	remaining, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestPostgresRepo_OutboxRollback(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	// Insert inside a transaction that gets rolled back.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, r.InsertOutboxTx(ctx, tx, domain.OutboxEntry{
		EventType: "of.created",
		Subject:   "kors.mes.of.created",
		Payload:   []byte("will be lost"),
	}))
	require.NoError(t, tx.Rollback(ctx))

	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, entries, "rolled-back entry must not appear")
}

// compile-time check: PostgresRepo satisfies the domain interfaces.
var (
	_ interface {
		Save(context.Context, *domain.Order) error
		FindByID(context.Context, string) (*domain.Order, error)
		FindByReference(context.Context, string) (*domain.Order, error)
		Update(context.Context, *domain.Order) error
		List(context.Context, domain.ListOrdersFilter) ([]*domain.Order, error)
	} = (*repo.PostgresRepo)(nil)
	_ = errors.New // suppress unused import if needed
)
