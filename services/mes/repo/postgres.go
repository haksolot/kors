package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haksolot/kors/services/mes/domain"
)

// PostgresRepo implements domain.OrderRepository, domain.OperationRepository,
// domain.Transactor, and the outbox worker interface using pgx/v5.
type PostgresRepo struct {
	db *pgxpool.Pool
}

// New returns a PostgresRepo backed by the given connection pool.
func New(db *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{db: db}
}

// ── Orders ────────────────────────────────────────────────────────────────────

// Save persists a new Order. Returns domain.ErrOrderAlreadyExists on duplicate reference.
func (r *PostgresRepo) Save(ctx context.Context, o *domain.Order) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO manufacturing_orders
			(id, reference, product_id, quantity, status, created_at, updated_at, started_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		o.ID, o.Reference, o.ProductID, o.Quantity, string(o.Status),
		o.CreatedAt, o.UpdatedAt, o.StartedAt, o.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("Save order %s: %w", o.Reference, domain.ErrOrderAlreadyExists)
		}
		return fmt.Errorf("Save order %s: %w", o.ID, err)
	}
	return nil
}

// FindByID retrieves an Order by its UUID. Returns domain.ErrOrderNotFound if absent.
func (r *PostgresRepo) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, reference, product_id, quantity, status,
		        created_at, updated_at, started_at, completed_at
		 FROM manufacturing_orders WHERE id = $1`, id,
	)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindByID %s: %w", id, domain.ErrOrderNotFound)
		}
		return nil, fmt.Errorf("FindByID %s: %w", id, err)
	}
	return o, nil
}

// FindByReference retrieves an Order by its human-readable reference.
func (r *PostgresRepo) FindByReference(ctx context.Context, reference string) (*domain.Order, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, reference, product_id, quantity, status,
		        created_at, updated_at, started_at, completed_at
		 FROM manufacturing_orders WHERE reference = $1`, reference,
	)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindByReference %s: %w", reference, domain.ErrOrderNotFound)
		}
		return nil, fmt.Errorf("FindByReference %s: %w", reference, err)
	}
	return o, nil
}

// Update persists state changes on an existing Order.
func (r *PostgresRepo) Update(ctx context.Context, o *domain.Order) error {
	_, err := r.db.Exec(ctx,
		`UPDATE manufacturing_orders
		 SET status=$1, updated_at=$2, started_at=$3, completed_at=$4
		 WHERE id=$5`,
		string(o.Status), o.UpdatedAt, o.StartedAt, o.CompletedAt, o.ID,
	)
	if err != nil {
		return fmt.Errorf("Update order %s: %w", o.ID, err)
	}
	return nil
}

// List returns a page of Orders matching the filter.
func (r *PostgresRepo) List(ctx context.Context, filter domain.ListOrdersFilter) ([]*domain.Order, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	var (
		rows pgx.Rows
		err  error
	)
	if filter.Status != nil {
		rows, err = r.db.Query(ctx,
			`SELECT id, reference, product_id, quantity, status,
			        created_at, updated_at, started_at, completed_at
			 FROM manufacturing_orders
			 WHERE status = $1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			string(*filter.Status), pageSize,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, reference, product_id, quantity, status,
			        created_at, updated_at, started_at, completed_at
			 FROM manufacturing_orders
			 ORDER BY created_at DESC
			 LIMIT $1`,
			pageSize,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("List orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("List orders scan: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// ── Operations ────────────────────────────────────────────────────────────────

// SaveOperation persists a new Operation (used by integration tests).
func (r *PostgresRepo) SaveOperation(ctx context.Context, op *domain.Operation) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO operations
			(id, of_id, step_number, name, operator_id, status, skip_reason, created_at, started_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		op.ID, op.OFID, op.StepNumber, op.Name,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.CreatedAt, op.StartedAt, op.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveOperation %s: %w", op.ID, err)
	}
	return nil
}

// FindOperationByID retrieves an Operation by its UUID.
func (r *PostgresRepo) FindOperationByID(ctx context.Context, id string) (*domain.Operation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, of_id, step_number, name, operator_id, status, skip_reason,
		        created_at, started_at, completed_at
		 FROM operations WHERE id = $1`, id,
	)
	op, err := scanOperation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindOperationByID %s: %w", id, domain.ErrOperationNotFound)
		}
		return nil, fmt.Errorf("FindOperationByID %s: %w", id, err)
	}
	return op, nil
}

// FindOperationsByOFID returns all operations for a given manufacturing order.
func (r *PostgresRepo) FindOperationsByOFID(ctx context.Context, ofID string) ([]*domain.Operation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, of_id, step_number, name, operator_id, status, skip_reason,
		        created_at, started_at, completed_at
		 FROM operations WHERE of_id = $1 ORDER BY step_number`, ofID,
	)
	if err != nil {
		return nil, fmt.Errorf("FindOperationsByOFID %s: %w", ofID, err)
	}
	defer rows.Close()

	var ops []*domain.Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("FindOperationsByOFID scan: %w", err)
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// UpdateOperation persists state changes on an existing Operation.
func (r *PostgresRepo) UpdateOperation(ctx context.Context, op *domain.Operation) error {
	_, err := r.db.Exec(ctx,
		`UPDATE operations
		 SET operator_id=$1, status=$2, skip_reason=$3, started_at=$4, completed_at=$5
		 WHERE id=$6`,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.StartedAt, op.CompletedAt, op.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOperation %s: %w", op.ID, err)
	}
	return nil
}

// ── Transactor ────────────────────────────────────────────────────────────────

// WithTx implements domain.Transactor. It begins a transaction, calls fn with a
// txOps bound to that transaction, and commits on success or rolls back on error.
func (r *PostgresRepo) WithTx(ctx context.Context, fn func(domain.TxOps) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("WithTx begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(&txOps{tx: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// txOps wraps a pgx.Tx and implements domain.TxOps.
// All methods route SQL through the active transaction.
type txOps struct{ tx pgx.Tx }

func (t *txOps) SaveOrder(ctx context.Context, o *domain.Order) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO manufacturing_orders
			(id, reference, product_id, quantity, status, created_at, updated_at, started_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		o.ID, o.Reference, o.ProductID, o.Quantity, string(o.Status),
		o.CreatedAt, o.UpdatedAt, o.StartedAt, o.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveOrder %s: %w", o.Reference, domain.ErrOrderAlreadyExists)
		}
		return fmt.Errorf("SaveOrder %s: %w", o.ID, err)
	}
	return nil
}

func (t *txOps) UpdateOrder(ctx context.Context, o *domain.Order) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE manufacturing_orders
		 SET status=$1, updated_at=$2, started_at=$3, completed_at=$4
		 WHERE id=$5`,
		string(o.Status), o.UpdatedAt, o.StartedAt, o.CompletedAt, o.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOrder %s: %w", o.ID, err)
	}
	return nil
}

func (t *txOps) SaveOperation(ctx context.Context, op *domain.Operation) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO operations
			(id, of_id, step_number, name, operator_id, status, skip_reason, created_at, started_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		op.ID, op.OFID, op.StepNumber, op.Name,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.CreatedAt, op.StartedAt, op.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveOperation %s: %w", op.ID, err)
	}
	return nil
}

func (t *txOps) UpdateOperation(ctx context.Context, op *domain.Operation) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE operations
		 SET operator_id=$1, status=$2, skip_reason=$3, started_at=$4, completed_at=$5
		 WHERE id=$6`,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.StartedAt, op.CompletedAt, op.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOperation %s: %w", op.ID, err)
	}
	return nil
}

func (t *txOps) InsertOutbox(ctx context.Context, entry domain.OutboxEntry) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO outbox (event_type, subject, payload) VALUES ($1, $2, $3)`,
		entry.EventType, entry.Subject, entry.Payload,
	)
	if err != nil {
		return fmt.Errorf("InsertOutbox %s: %w", entry.EventType, err)
	}
	return nil
}

// ── Outbox ────────────────────────────────────────────────────────────────────

// InsertOutboxTx writes a single outbox entry within the provided pgx.Tx.
// The transaction must be open; this method does not commit.
func (r *PostgresRepo) InsertOutboxTx(ctx context.Context, tx pgx.Tx, entry domain.OutboxEntry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO outbox (event_type, subject, payload) VALUES ($1, $2, $3)`,
		entry.EventType, entry.Subject, entry.Payload,
	)
	if err != nil {
		return fmt.Errorf("InsertOutboxTx %s: %w", entry.EventType, err)
	}
	return nil
}

// ListUnpublishedOutbox returns up to limit unpublished outbox entries ordered by id.
func (r *PostgresRepo) ListUnpublishedOutbox(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, event_type, subject, payload
		 FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ListUnpublishedOutbox: %w", err)
	}
	defer rows.Close()

	var entries []domain.OutboxEntry
	for rows.Next() {
		var e domain.OutboxEntry
		if err := rows.Scan(&e.ID, &e.EventType, &e.Subject, &e.Payload); err != nil {
			return nil, fmt.Errorf("ListUnpublishedOutbox scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MarkOutboxPublished sets published_at = NOW() for the given entry IDs.
func (r *PostgresRepo) MarkOutboxPublished(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE outbox SET published_at = $1 WHERE id = ANY($2)`,
		time.Now().UTC(), ids,
	)
	if err != nil {
		return fmt.Errorf("MarkOutboxPublished: %w", err)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(s scanner) (*domain.Order, error) {
	var o domain.Order
	var status string
	if err := s.Scan(
		&o.ID, &o.Reference, &o.ProductID, &o.Quantity, &status,
		&o.CreatedAt, &o.UpdatedAt, &o.StartedAt, &o.CompletedAt,
	); err != nil {
		return nil, err
	}
	o.Status = domain.OrderStatus(status)
	return &o, nil
}

func scanOperation(s scanner) (*domain.Operation, error) {
	var op domain.Operation
	var status, operatorID, skipReason *string
	if err := s.Scan(
		&op.ID, &op.OFID, &op.StepNumber, &op.Name,
		&operatorID, &status, &skipReason,
		&op.CreatedAt, &op.StartedAt, &op.CompletedAt,
	); err != nil {
		return nil, err
	}
	if status != nil {
		op.Status = domain.OperationStatus(*status)
	}
	if operatorID != nil {
		op.OperatorID = *operatorID
	}
	if skipReason != nil {
		op.SkipReason = *skipReason
	}
	return &op, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && (containsCode(err, "23505"))
}

func containsCode(err error, code string) bool {
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == code
	}
	return false
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
