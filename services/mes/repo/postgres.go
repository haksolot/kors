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
			(id, reference, product_id, quantity, status, created_at, updated_at, started_at, completed_at,
			 is_fai, fai_approved_by, fai_approved_at, due_date, priority)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		o.ID, o.Reference, o.ProductID, o.Quantity, string(o.Status),
		o.CreatedAt, o.UpdatedAt, o.StartedAt, o.CompletedAt,
		o.IsFAI, nullableString(o.FAIApprovedBy), o.FAIApprovedAt,
		o.DueDate, o.Priority,
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
		        created_at, updated_at, started_at, completed_at,
		        is_fai, fai_approved_by, fai_approved_at,
		        due_date, priority
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
		        created_at, updated_at, started_at, completed_at,
		        is_fai, fai_approved_by, fai_approved_at,
		        due_date, priority
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
		 SET status=$1, updated_at=$2, started_at=$3, completed_at=$4,
		     is_fai=$5, fai_approved_by=$6, fai_approved_at=$7,
		     due_date=$8, priority=$9
		 WHERE id=$10`,
		string(o.Status), o.UpdatedAt, o.StartedAt, o.CompletedAt,
		o.IsFAI, nullableString(o.FAIApprovedBy), o.FAIApprovedAt,
		o.DueDate, o.Priority, o.ID,
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
			        created_at, updated_at, started_at, completed_at,
			        is_fai, fai_approved_by, fai_approved_at,
			        due_date, priority
			 FROM manufacturing_orders
			 WHERE status = $1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			string(*filter.Status), pageSize,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, reference, product_id, quantity, status,
			        created_at, updated_at, started_at, completed_at,
			        is_fai, fai_approved_by, fai_approved_at,
			        due_date, priority
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

// DispatchList returns PLANNED and IN_PROGRESS orders sorted by priority DESC, due_date ASC.
func (r *PostgresRepo) DispatchList(ctx context.Context, limit int) ([]*domain.Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, reference, product_id, quantity, status,
		        created_at, updated_at, started_at, completed_at,
		        is_fai, fai_approved_by, fai_approved_at,
		        due_date, priority
		 FROM manufacturing_orders
		 WHERE status IN ('planned', 'in_progress')
		 ORDER BY priority DESC, due_date ASC NULLS LAST
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("DispatchList: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("DispatchList scan: %w", err)
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
			(id, of_id, step_number, name, operator_id, status, skip_reason, created_at, started_at, completed_at,
			 requires_sign_off, signed_off_by, signed_off_at, instructions_url,
			 planned_duration_seconds, actual_duration_seconds, required_skill)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		op.ID, op.OFID, op.StepNumber, op.Name,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.CreatedAt, op.StartedAt, op.CompletedAt,
		op.RequiresSignOff, nullableString(op.SignedOffBy), op.SignedOffAt, nullableString(op.InstructionsURL),
		op.PlannedDurationSeconds, op.ActualDurationSeconds, nullableString(op.RequiredSkill),
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
		        created_at, started_at, completed_at,
		        requires_sign_off, signed_off_by, signed_off_at, instructions_url,
		        planned_duration_seconds, actual_duration_seconds, required_skill
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
		        created_at, started_at, completed_at,
		        requires_sign_off, signed_off_by, signed_off_at, instructions_url,
		        planned_duration_seconds, actual_duration_seconds, required_skill
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
		 SET operator_id=$1, status=$2, skip_reason=$3, started_at=$4, completed_at=$5,
		     requires_sign_off=$6, signed_off_by=$7, signed_off_at=$8, instructions_url=$9,
		     planned_duration_seconds=$10, actual_duration_seconds=$11, required_skill=$12
		 WHERE id=$13`,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.StartedAt, op.CompletedAt,
		op.RequiresSignOff, nullableString(op.SignedOffBy), op.SignedOffAt, nullableString(op.InstructionsURL),
		op.PlannedDurationSeconds, op.ActualDurationSeconds, nullableString(op.RequiredSkill),
		op.ID,
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
			(id, reference, product_id, quantity, status, created_at, updated_at, started_at, completed_at,
			 is_fai, fai_approved_by, fai_approved_at, due_date, priority)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		o.ID, o.Reference, o.ProductID, o.Quantity, string(o.Status),
		o.CreatedAt, o.UpdatedAt, o.StartedAt, o.CompletedAt,
		o.IsFAI, nullableString(o.FAIApprovedBy), o.FAIApprovedAt,
		o.DueDate, o.Priority,
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
		 SET status=$1, updated_at=$2, started_at=$3, completed_at=$4,
		     is_fai=$5, fai_approved_by=$6, fai_approved_at=$7,
		     due_date=$8, priority=$9
		 WHERE id=$10`,
		string(o.Status), o.UpdatedAt, o.StartedAt, o.CompletedAt,
		o.IsFAI, nullableString(o.FAIApprovedBy), o.FAIApprovedAt,
		o.DueDate, o.Priority, o.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOrder %s: %w", o.ID, err)
	}
	return nil
}

func (t *txOps) SaveOperation(ctx context.Context, op *domain.Operation) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO operations
			(id, of_id, step_number, name, operator_id, status, skip_reason, created_at, started_at, completed_at,
			 requires_sign_off, signed_off_by, signed_off_at, instructions_url,
			 planned_duration_seconds, actual_duration_seconds, required_skill)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		op.ID, op.OFID, op.StepNumber, op.Name,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.CreatedAt, op.StartedAt, op.CompletedAt,
		op.RequiresSignOff, nullableString(op.SignedOffBy), op.SignedOffAt, nullableString(op.InstructionsURL),
		op.PlannedDurationSeconds, op.ActualDurationSeconds, nullableString(op.RequiredSkill),
	)
	if err != nil {
		return fmt.Errorf("SaveOperation %s: %w", op.ID, err)
	}
	return nil
}

func (t *txOps) UpdateOperation(ctx context.Context, op *domain.Operation) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE operations
		 SET operator_id=$1, status=$2, skip_reason=$3, started_at=$4, completed_at=$5,
		     requires_sign_off=$6, signed_off_by=$7, signed_off_at=$8, instructions_url=$9,
		     planned_duration_seconds=$10, actual_duration_seconds=$11, required_skill=$12
		 WHERE id=$13`,
		nullableString(op.OperatorID), string(op.Status), nullableString(op.SkipReason),
		op.StartedAt, op.CompletedAt,
		op.RequiresSignOff, nullableString(op.SignedOffBy), op.SignedOffAt, nullableString(op.InstructionsURL),
		op.PlannedDurationSeconds, op.ActualDurationSeconds, nullableString(op.RequiredSkill),
		op.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOperation %s: %w", op.ID, err)
	}
	return nil
}

func (t *txOps) SaveLot(ctx context.Context, l *domain.Lot) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO lots (id, reference, product_id, quantity, material_cert_url, received_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		l.ID, l.Reference, l.ProductID, l.Quantity,
		nullableString(l.MaterialCertURL), l.ReceivedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveLot %s: %w", l.Reference, domain.ErrLotAlreadyExists)
		}
		return fmt.Errorf("SaveLot %s: %w", l.ID, err)
	}
	return nil
}

func (t *txOps) UpdateLot(ctx context.Context, l *domain.Lot) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE lots SET material_cert_url=$1 WHERE id=$2`,
		nullableString(l.MaterialCertURL), l.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateLot %s: %w", l.ID, err)
	}
	return nil
}

func (t *txOps) SaveSerialNumber(ctx context.Context, sn *domain.SerialNumber) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO serial_numbers (id, sn, lot_id, product_id, of_id, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sn.ID, sn.SN, nullableString(sn.LotID), sn.ProductID, sn.OFID,
		string(sn.Status), sn.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveSerialNumber %s: %w", sn.SN, domain.ErrSerialNumberAlreadyExists)
		}
		return fmt.Errorf("SaveSerialNumber %s: %w", sn.ID, err)
	}
	return nil
}

func (t *txOps) UpdateSerialNumber(ctx context.Context, sn *domain.SerialNumber) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE serial_numbers SET status=$1 WHERE id=$2`,
		string(sn.Status), sn.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateSerialNumber %s: %w", sn.ID, err)
	}
	return nil
}

func (t *txOps) SaveGenealogyEntry(ctx context.Context, e *domain.GenealogyEntry) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO genealogy (id, parent_sn_id, child_sn_id, of_id, operation_id, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		e.ID, e.ParentSNID, e.ChildSNID, e.OFID,
		nullableString(e.OperationID), e.RecordedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveGenealogyEntry: parent-child pair already exists: %w", domain.ErrSNInvalidTransition)
		}
		return fmt.Errorf("SaveGenealogyEntry %s: %w", e.ID, err)
	}
	return nil
}

func (t *txOps) SaveRouting(ctx context.Context, rt *domain.Routing) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO routings (id, product_id, version, name, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		rt.ID, rt.ProductID, rt.Version, rt.Name, rt.IsActive, rt.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveRouting %s v%d: %w", rt.ProductID, rt.Version, domain.ErrRoutingNotFound)
		}
		return fmt.Errorf("SaveRouting %s: %w", rt.ID, err)
	}
	return nil
}

func (t *txOps) SaveRoutingStep(ctx context.Context, step *domain.RoutingStep) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO routing_steps
			(id, routing_id, step_number, name, planned_duration_seconds,
			 required_skill, instructions_url, requires_sign_off)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		step.ID, step.RoutingID, step.StepNumber, step.Name, step.PlannedDurationSeconds,
		nullableString(step.RequiredSkill), nullableString(step.InstructionsURL), step.RequiresSignOff,
	)
	if err != nil {
		return fmt.Errorf("SaveRoutingStep %s: %w", step.ID, err)
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

// ── Traceability — read-only ──────────────────────────────────────────────────

// FindLotByID retrieves a Lot by UUID.
func (r *PostgresRepo) FindLotByID(ctx context.Context, id string) (*domain.Lot, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, reference, product_id, quantity, material_cert_url, received_at
		 FROM lots WHERE id = $1`, id,
	)
	lot, err := scanLot(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindLotByID %s: %w", id, domain.ErrLotNotFound)
		}
		return nil, fmt.Errorf("FindLotByID %s: %w", id, err)
	}
	return lot, nil
}

// FindSNByID retrieves a SerialNumber by UUID.
func (r *PostgresRepo) FindSNByID(ctx context.Context, id string) (*domain.SerialNumber, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, sn, lot_id, product_id, of_id, status, created_at
		 FROM serial_numbers WHERE id = $1`, id,
	)
	sn, err := scanSN(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindSNByID %s: %w", id, domain.ErrSerialNumberNotFound)
		}
		return nil, fmt.Errorf("FindSNByID %s: %w", id, err)
	}
	return sn, nil
}

// FindSNBySN retrieves a SerialNumber by its human-readable serial string.
func (r *PostgresRepo) FindSNBySN(ctx context.Context, sn string) (*domain.SerialNumber, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, sn, lot_id, product_id, of_id, status, created_at
		 FROM serial_numbers WHERE sn = $1`, sn,
	)
	s, err := scanSN(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindSNBySN %s: %w", sn, domain.ErrSerialNumberNotFound)
		}
		return nil, fmt.Errorf("FindSNBySN %s: %w", sn, err)
	}
	return s, nil
}

// GetGenealogyByParentSN returns all genealogy entries where the given SN is the parent.
func (r *PostgresRepo) GetGenealogyByParentSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error) {
	return r.queryGenealogy(ctx, "parent_sn_id", snID)
}

// GetGenealogyByChildSN returns all genealogy entries where the given SN is the child.
func (r *PostgresRepo) GetGenealogyByChildSN(ctx context.Context, snID string) ([]*domain.GenealogyEntry, error) {
	return r.queryGenealogy(ctx, "child_sn_id", snID)
}

func (r *PostgresRepo) queryGenealogy(ctx context.Context, col, snID string) ([]*domain.GenealogyEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, parent_sn_id, child_sn_id, of_id, operation_id, recorded_at
		 FROM genealogy WHERE `+col+` = $1 ORDER BY recorded_at`, snID,
	)
	if err != nil {
		return nil, fmt.Errorf("queryGenealogy %s=%s: %w", col, snID, err)
	}
	defer rows.Close()

	var entries []*domain.GenealogyEntry
	for rows.Next() {
		e, err := scanGenealogyEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("queryGenealogy scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ── Routings ──────────────────────────────────────────────────────────────────

// FindRoutingByID retrieves a Routing (with its steps) by UUID.
func (r *PostgresRepo) FindRoutingByID(ctx context.Context, id string) (*domain.Routing, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, product_id, version, name, is_active, created_at
		 FROM routings WHERE id = $1`, id,
	)
	routing, err := scanRouting(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindRoutingByID %s: %w", id, domain.ErrRoutingNotFound)
		}
		return nil, fmt.Errorf("FindRoutingByID %s: %w", id, err)
	}
	steps, err := r.findStepsByRoutingID(ctx, id)
	if err != nil {
		return nil, err
	}
	routing.Steps = steps
	return routing, nil
}

// FindRoutingsByProductID retrieves all routings for a product (without steps).
func (r *PostgresRepo) FindRoutingsByProductID(ctx context.Context, productID string) ([]*domain.Routing, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, product_id, version, name, is_active, created_at
		 FROM routings WHERE product_id = $1 ORDER BY version DESC`, productID,
	)
	if err != nil {
		return nil, fmt.Errorf("FindRoutingsByProductID %s: %w", productID, err)
	}
	defer rows.Close()

	var routings []*domain.Routing
	for rows.Next() {
		rt, err := scanRouting(rows)
		if err != nil {
			return nil, fmt.Errorf("FindRoutingsByProductID scan: %w", err)
		}
		routings = append(routings, rt)
	}
	return routings, rows.Err()
}

func (r *PostgresRepo) findStepsByRoutingID(ctx context.Context, routingID string) ([]*domain.RoutingStep, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, routing_id, step_number, name,
		        planned_duration_seconds, required_skill, instructions_url, requires_sign_off
		 FROM routing_steps WHERE routing_id = $1 ORDER BY step_number`, routingID,
	)
	if err != nil {
		return nil, fmt.Errorf("findStepsByRoutingID %s: %w", routingID, err)
	}
	defer rows.Close()

	var steps []*domain.RoutingStep
	for rows.Next() {
		step, err := scanRoutingStep(rows)
		if err != nil {
			return nil, fmt.Errorf("findStepsByRoutingID scan: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func scanRouting(s scanner) (*domain.Routing, error) {
	var rt domain.Routing
	if err := s.Scan(&rt.ID, &rt.ProductID, &rt.Version, &rt.Name, &rt.IsActive, &rt.CreatedAt); err != nil {
		return nil, err
	}
	return &rt, nil
}

func scanRoutingStep(s scanner) (*domain.RoutingStep, error) {
	var step domain.RoutingStep
	var requiredSkill, instructionsURL *string
	if err := s.Scan(
		&step.ID, &step.RoutingID, &step.StepNumber, &step.Name,
		&step.PlannedDurationSeconds, &requiredSkill, &instructionsURL, &step.RequiresSignOff,
	); err != nil {
		return nil, err
	}
	if requiredSkill != nil {
		step.RequiredSkill = *requiredSkill
	}
	if instructionsURL != nil {
		step.InstructionsURL = *instructionsURL
	}
	return &step, nil
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
	var faiApprovedBy *string
	if err := s.Scan(
		&o.ID, &o.Reference, &o.ProductID, &o.Quantity, &status,
		&o.CreatedAt, &o.UpdatedAt, &o.StartedAt, &o.CompletedAt,
		&o.IsFAI, &faiApprovedBy, &o.FAIApprovedAt,
		&o.DueDate, &o.Priority,
	); err != nil {
		return nil, err
	}
	o.Status = domain.OrderStatus(status)
	if faiApprovedBy != nil {
		o.FAIApprovedBy = *faiApprovedBy
	}
	return &o, nil
}

func scanOperation(s scanner) (*domain.Operation, error) {
	var op domain.Operation
	var status, operatorID, skipReason, signedOffBy, instructionsURL, requiredSkill *string
	if err := s.Scan(
		&op.ID, &op.OFID, &op.StepNumber, &op.Name,
		&operatorID, &status, &skipReason,
		&op.CreatedAt, &op.StartedAt, &op.CompletedAt,
		&op.RequiresSignOff, &signedOffBy, &op.SignedOffAt, &instructionsURL,
		&op.PlannedDurationSeconds, &op.ActualDurationSeconds, &requiredSkill,
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
	if signedOffBy != nil {
		op.SignedOffBy = *signedOffBy
	}
	if instructionsURL != nil {
		op.InstructionsURL = *instructionsURL
	}
	if requiredSkill != nil {
		op.RequiredSkill = *requiredSkill
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

func scanLot(s scanner) (*domain.Lot, error) {
	var l domain.Lot
	var certURL *string
	if err := s.Scan(&l.ID, &l.Reference, &l.ProductID, &l.Quantity, &certURL, &l.ReceivedAt); err != nil {
		return nil, err
	}
	if certURL != nil {
		l.MaterialCertURL = *certURL
	}
	return &l, nil
}

func scanSN(s scanner) (*domain.SerialNumber, error) {
	var sn domain.SerialNumber
	var lotID *string
	var status string
	if err := s.Scan(&sn.ID, &sn.SN, &lotID, &sn.ProductID, &sn.OFID, &status, &sn.CreatedAt); err != nil {
		return nil, err
	}
	if lotID != nil {
		sn.LotID = *lotID
	}
	sn.Status = domain.SerialNumberStatus(status)
	return &sn, nil
}

func scanGenealogyEntry(s scanner) (*domain.GenealogyEntry, error) {
	var e domain.GenealogyEntry
	var opID *string
	if err := s.Scan(&e.ID, &e.ParentSNID, &e.ChildSNID, &e.OFID, &opID, &e.RecordedAt); err != nil {
		return nil, err
	}
	if opID != nil {
		e.OperationID = *opID
	}
	return &e, nil
}
