package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib" // also registers the "pgx" database/sql driver via init()
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
		tcpostgres.WithSQLDriver("pgx"), // uses registered pgx driver for SQL-level health check
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return pool.Ping(ctx) == nil
	}, 30*time.Second, 500*time.Millisecond, "postgres not ready to accept connections")
	t.Cleanup(pool.Close)

	// Run goose migrations using the same stdlib adapter as cmd/main.go.
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, "../migrations"), "run migrations")

	return pool
}

// ── Order tests ───────────────────────────────────────────────────────────────

func TestPostgresRepo_SaveAndFindOrder(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, err := domain.NewOrder("OF-TEST-001", "00000000-0000-0000-0000-000000000001", 50)
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

	_, err := r.FindByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
}

func TestPostgresRepo_Save_DuplicateReference(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	o1, _ := domain.NewOrder("OF-DUP-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.Save(ctx, o1))

	o2, _ := domain.NewOrder("OF-DUP-001", "00000000-0000-0000-0000-000000000002", 20)
	err := r.Save(ctx, o2)
	require.ErrorIs(t, err, domain.ErrOrderAlreadyExists)
}

func TestPostgresRepo_UpdateOrder(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-UPDATE-001", "00000000-0000-0000-0000-000000000001", 10)
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

	o1, _ := domain.NewOrder("OF-LIST-001", "00000000-0000-0000-0000-000000000001", 10)
	o2, _ := domain.NewOrder("OF-LIST-002", "00000000-0000-0000-0000-000000000001", 20)
	o3, _ := domain.NewOrder("OF-LIST-003", "00000000-0000-0000-0000-000000000001", 30)
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

	order, _ := domain.NewOrder("OF-OP-001", "00000000-0000-0000-0000-000000000001", 10)
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

	_, err := r.FindOperationByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, domain.ErrOperationNotFound)
}

func TestPostgresRepo_UpdateOperation(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-UOP-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.Save(ctx, order))
	op, _ := domain.NewOperation(order.ID, 1, "Soudure")
	require.NoError(t, r.SaveOperation(ctx, op))

	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010"))
	require.NoError(t, r.UpdateOperation(ctx, op))

	got, err := r.FindOperationByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusInProgress, got.Status)
	assert.Equal(t, "00000000-0000-0000-0000-000000000010", got.OperatorID)
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

// ── WithTx (Transactor) tests ─────────────────────────────────────────────────

func TestPostgresRepo_WithTx_CommitsOrderAndOutbox(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-TX-001", "00000000-0000-0000-0000-000000000001", 10)

	err := r.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveOrder(ctx, order); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OFCreated",
			Subject:   "kors.mes.of.created",
			Payload:   []byte("payload"),
		})
	})
	require.NoError(t, err)

	// Order must be persisted.
	got, err := r.FindByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, "OF-TX-001", got.Reference)

	// Outbox entry must be persisted and unpublished.
	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "OFCreated", entries[0].EventType)
}

func TestPostgresRepo_WithTx_RollbackOnError(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-TX-ROLLBACK-001", "00000000-0000-0000-0000-000000000001", 10)
	wantErr := errors.New("forced error")

	err := r.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveOrder(ctx, order); err != nil {
			return err
		}
		return wantErr // trigger rollback
	})
	require.ErrorIs(t, err, wantErr)

	// Order must NOT have been persisted.
	_, err = r.FindByID(ctx, order.ID)
	require.ErrorIs(t, err, domain.ErrOrderNotFound)

	// Outbox must be empty.
	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestPostgresRepo_WithTx_UpdateOperationAndOutbox(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	// Setup: persist order and operation outside of the tx being tested.
	order, _ := domain.NewOrder("OF-TX-OP-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.Save(ctx, order))
	op, _ := domain.NewOperation(order.ID, 1, "Découpe")
	require.NoError(t, r.SaveOperation(ctx, op))

	// Start the operation inside a transaction.
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010"))
	err := r.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateOperation(ctx, op); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "OperationStarted",
			Subject:   "kors.mes.operation.started",
			Payload:   []byte("payload"),
		})
	})
	require.NoError(t, err)

	got, err := r.FindOperationByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusInProgress, got.Status)
	assert.Equal(t, "00000000-0000-0000-0000-000000000010", got.OperatorID)

	entries, err := r.ListUnpublishedOutbox(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "OperationStarted", entries[0].EventType)
}

// ── Traceability tests ────────────────────────────────────────────────────────

func TestPostgresRepo_Lot_SaveAndFind(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	lot, err := domain.NewLot("LOT-2026-001", "00000000-0000-0000-0000-000000000001", 100)
	require.NoError(t, err)

	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveLot(ctx, lot)
	}))

	got, err := r.FindLotByID(ctx, lot.ID)
	require.NoError(t, err)
	assert.Equal(t, lot.ID, got.ID)
	assert.Equal(t, "LOT-2026-001", got.Reference)
	assert.Equal(t, 100, got.Quantity)
	assert.Empty(t, got.MaterialCertURL)
}

func TestPostgresRepo_Lot_AttachCertificate(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	lot, _ := domain.NewLot("LOT-CERT-001", "00000000-0000-0000-0000-000000000001", 50)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveLot(ctx, lot) }))

	lot.AttachCertificate("s3://certs/lot-cert-001.pdf")
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.UpdateLot(ctx, lot) }))

	got, err := r.FindLotByID(ctx, lot.ID)
	require.NoError(t, err)
	assert.Equal(t, "s3://certs/lot-cert-001.pdf", got.MaterialCertURL)
}

func TestPostgresRepo_Lot_DuplicateReference(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	lot1, _ := domain.NewLot("LOT-DUP-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveLot(ctx, lot1) }))

	lot2, _ := domain.NewLot("LOT-DUP-001", "00000000-0000-0000-0000-000000000001", 20)
	err := r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveLot(ctx, lot2) })
	require.ErrorIs(t, err, domain.ErrLotAlreadyExists)
}

func TestPostgresRepo_Lot_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	_, err := r.FindLotByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, domain.ErrLotNotFound)
}

func TestPostgresRepo_SerialNumber_SaveAndFind(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-SN-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveOrder(ctx, order) }))

	sn, err := domain.NewSerialNumber("SN-0001", "", "00000000-0000-0000-0000-000000000001", order.ID)
	require.NoError(t, err)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveSerialNumber(ctx, sn) }))

	got, err := r.FindSNBySN(ctx, "SN-0001")
	require.NoError(t, err)
	assert.Equal(t, sn.ID, got.ID)
	assert.Equal(t, domain.SNStatusProduced, got.Status)

	gotByID, err := r.FindSNByID(ctx, sn.ID)
	require.NoError(t, err)
	assert.Equal(t, sn.SN, gotByID.SN)
}

func TestPostgresRepo_SerialNumber_Release(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-SN-REL-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveOrder(ctx, order) }))

	sn, _ := domain.NewSerialNumber("SN-REL-001", "", "00000000-0000-0000-0000-000000000001", order.ID)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveSerialNumber(ctx, sn) }))

	require.NoError(t, sn.Release())
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.UpdateSerialNumber(ctx, sn) }))

	got, err := r.FindSNBySN(ctx, sn.SN)
	require.NoError(t, err)
	assert.Equal(t, domain.SNStatusReleased, got.Status)
}

func TestPostgresRepo_SerialNumber_NotFound(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	_, err := r.FindSNBySN(context.Background(), "SN-NONEXISTENT")
	require.ErrorIs(t, err, domain.ErrSerialNumberNotFound)
}

func TestPostgresRepo_GenealogyEntry_SaveAndQuery(t *testing.T) {
	pool := setupDB(t)
	r := repo.New(pool)
	ctx := context.Background()

	order, _ := domain.NewOrder("OF-GEN-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error { return tx.SaveOrder(ctx, order) }))

	parent, _ := domain.NewSerialNumber("SN-PARENT", "", "00000000-0000-0000-0000-000000000001", order.ID)
	child, _ := domain.NewSerialNumber("SN-CHILD", "", "00000000-0000-0000-0000-000000000001", order.ID)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveSerialNumber(ctx, parent); err != nil {
			return err
		}
		return tx.SaveSerialNumber(ctx, child)
	}))

	entry, err := domain.NewGenealogyEntry(parent.ID, child.ID, order.ID, "")
	require.NoError(t, err)
	require.NoError(t, r.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.SaveGenealogyEntry(ctx, entry)
	}))

	byParent, err := r.GetGenealogyByParentSN(ctx, parent.ID)
	require.NoError(t, err)
	require.Len(t, byParent, 1)
	assert.Equal(t, child.ID, byParent[0].ChildSNID)

	byChild, err := r.GetGenealogyByChildSN(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, byChild, 1)
	assert.Equal(t, parent.ID, byChild[0].ParentSNID)
}

// compile-time checks: PostgresRepo satisfies the required domain interfaces.
var (
	_ domain.OrderRepository        = (*repo.PostgresRepo)(nil)
	_ domain.OperationRepository    = (*repo.PostgresRepo)(nil)
	_ domain.LotRepository          = (*repo.PostgresRepo)(nil)
	_ domain.TraceabilityRepository = (*repo.PostgresRepo)(nil)
	_ domain.Transactor             = (*repo.PostgresRepo)(nil)
	_ = errors.New                  // keep import
)
