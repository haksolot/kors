package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haksolot/kors/services/qms/domain"
)

// PostgresRepo implements domain.NCRepository, domain.CAPARepository,
// domain.Transactor, and the outbox worker interface using pgx/v5.
type PostgresRepo struct {
	db *pgxpool.Pool
}

// New returns a PostgresRepo backed by the given connection pool.
func New(db *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{db: db}
}

// ── NonConformities ───────────────────────────────────────────────────────────

// FindNCByID retrieves a NonConformity by UUID.
func (r *PostgresRepo) FindNCByID(ctx context.Context, id string) (*domain.NonConformity, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, operation_id, of_id, defect_code, description,
		        affected_quantity, serial_numbers, declared_by,
		        status, disposition, closed_by, created_at, updated_at, closed_at
		 FROM non_conformities WHERE id = $1`, id,
	)
	nc, err := scanNC(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindNCByID %s: %w", id, domain.ErrNCNotFound)
		}
		return nil, fmt.Errorf("FindNCByID %s: %w", id, err)
	}
	return nc, nil
}

// FindNCByOperationID retrieves a NonConformity by its MES operation_id (dedup key).
func (r *PostgresRepo) FindNCByOperationID(ctx context.Context, operationID string) (*domain.NonConformity, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, operation_id, of_id, defect_code, description,
		        affected_quantity, serial_numbers, declared_by,
		        status, disposition, closed_by, created_at, updated_at, closed_at
		 FROM non_conformities WHERE operation_id = $1`, operationID,
	)
	nc, err := scanNC(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindNCByOperationID %s: %w", operationID, domain.ErrNCNotFound)
		}
		return nil, fmt.Errorf("FindNCByOperationID %s: %w", operationID, err)
	}
	return nc, nil
}

// ListNCs returns a page of NonConformities matching the filter.
func (r *PostgresRepo) ListNCs(ctx context.Context, filter domain.ListNCsFilter) ([]*domain.NonConformity, error) {
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
			`SELECT id, operation_id, of_id, defect_code, description,
			        affected_quantity, serial_numbers, declared_by,
			        status, disposition, closed_by, created_at, updated_at, closed_at
			 FROM non_conformities WHERE status = $1
			 ORDER BY created_at DESC LIMIT $2`,
			string(*filter.Status), pageSize,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, operation_id, of_id, defect_code, description,
			        affected_quantity, serial_numbers, declared_by,
			        status, disposition, closed_by, created_at, updated_at, closed_at
			 FROM non_conformities
			 ORDER BY created_at DESC LIMIT $1`,
			pageSize,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("ListNCs: %w", err)
	}
	defer rows.Close()

	var ncs []*domain.NonConformity
	for rows.Next() {
		nc, err := scanNC(rows)
		if err != nil {
			return nil, fmt.Errorf("ListNCs scan: %w", err)
		}
		ncs = append(ncs, nc)
	}
	return ncs, rows.Err()
}

// ── CAPAs ─────────────────────────────────────────────────────────────────────

// FindCAPAByID retrieves a CAPA by UUID.
func (r *PostgresRepo) FindCAPAByID(ctx context.Context, id string) (*domain.CAPA, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, nc_id, action_type, description, owner_id, status,
		        due_date, created_at, updated_at, completed_at
		 FROM capas WHERE id = $1`, id,
	)
	c, err := scanCAPA(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindCAPAByID %s: %w", id, domain.ErrCAPANotFound)
		}
		return nil, fmt.Errorf("FindCAPAByID %s: %w", id, err)
	}
	return c, nil
}

// ListCAPAs returns CAPAs matching the filter.
func (r *PostgresRepo) ListCAPAs(ctx context.Context, filter domain.ListCAPAsFilter) ([]*domain.CAPA, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	q := `SELECT id, nc_id, action_type, description, owner_id, status,
	             due_date, created_at, updated_at, completed_at
	      FROM capas WHERE 1=1`
	args := []any{}
	n := 1

	if filter.NCID != "" {
		q += fmt.Sprintf(" AND nc_id = $%d", n)
		args = append(args, filter.NCID)
		n++
	}
	if filter.Status != nil {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, string(*filter.Status))
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", n)
	args = append(args, pageSize)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ListCAPAs: %w", err)
	}
	defer rows.Close()

	var capas []*domain.CAPA
	for rows.Next() {
		c, err := scanCAPA(rows)
		if err != nil {
			return nil, fmt.Errorf("ListCAPAs scan: %w", err)
		}
		capas = append(capas, c)
	}
	return capas, rows.Err()
}

// ── Transactor ────────────────────────────────────────────────────────────────

// WithTx implements domain.Transactor.
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
type txOps struct{ tx pgx.Tx }

func (t *txOps) SaveNC(ctx context.Context, nc *domain.NonConformity) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO non_conformities
			(id, operation_id, of_id, defect_code, description, affected_quantity,
			 serial_numbers, declared_by, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		nc.ID, nc.OperationID, nc.OFID, nc.DefectCode, nc.Description, nc.AffectedQuantity,
		coalesceStrings(nc.SerialNumbers), nc.DeclaredBy, string(nc.Status),
		nc.CreatedAt, nc.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("SaveNC %s: %w", nc.OperationID, domain.ErrNCAlreadyExists)
		}
		return fmt.Errorf("SaveNC %s: %w", nc.ID, err)
	}
	return nil
}

func (t *txOps) UpdateNC(ctx context.Context, nc *domain.NonConformity) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE non_conformities
		 SET status=$1, disposition=$2, closed_by=$3, updated_at=$4, closed_at=$5
		 WHERE id=$6`,
		string(nc.Status), nullableString(string(nc.Disposition)), nullableString(nc.ClosedBy),
		nc.UpdatedAt, nc.ClosedAt, nc.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateNC %s: %w", nc.ID, err)
	}
	return nil
}

func (t *txOps) SaveCAPA(ctx context.Context, c *domain.CAPA) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO capas
			(id, nc_id, action_type, description, owner_id, status, due_date, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		c.ID, c.NCID, string(c.ActionType), c.Description, c.OwnerID, string(c.Status),
		c.DueDate, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveCAPA %s: %w", c.ID, err)
	}
	return nil
}

func (t *txOps) UpdateCAPA(ctx context.Context, c *domain.CAPA) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE capas SET status=$1, updated_at=$2, completed_at=$3 WHERE id=$4`,
		string(c.Status), c.UpdatedAt, c.CompletedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateCAPA %s: %w", c.ID, err)
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

// ── Outbox worker interface ────────────────────────────────────────────────────

// ListUnpublishedOutbox returns up to limit unpublished outbox entries.
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

func scanNC(s scanner) (*domain.NonConformity, error) {
	var nc domain.NonConformity
	var status string
	var disposition, closedBy *string
	var serialNumbers []string
	if err := s.Scan(
		&nc.ID, &nc.OperationID, &nc.OFID, &nc.DefectCode, &nc.Description,
		&nc.AffectedQuantity, &serialNumbers, &nc.DeclaredBy,
		&status, &disposition, &closedBy,
		&nc.CreatedAt, &nc.UpdatedAt, &nc.ClosedAt,
	); err != nil {
		return nil, err
	}
	nc.Status = domain.NCStatus(status)
	nc.SerialNumbers = serialNumbers
	if disposition != nil {
		nc.Disposition = domain.NCDisposition(*disposition)
	}
	if closedBy != nil {
		nc.ClosedBy = *closedBy
	}
	return &nc, nil
}

func scanCAPA(s scanner) (*domain.CAPA, error) {
	var c domain.CAPA
	var actionType, status string
	if err := s.Scan(
		&c.ID, &c.NCID, &actionType, &c.Description, &c.OwnerID, &status,
		&c.DueDate, &c.CreatedAt, &c.UpdatedAt, &c.CompletedAt,
	); err != nil {
		return nil, err
	}
	c.ActionType = domain.CAPAActionType(actionType)
	c.Status = domain.CAPAStatus(status)
	return &c, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && containsCode(err, "23505")
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

func coalesceStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
