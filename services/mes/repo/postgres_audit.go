package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Audit Trail Write (TxOps) ─────────────────────────────────────────────────

// AppendAuditEntry inserts a new audit entry within the current transaction.
// This method is append-only — there is no UPDATE or DELETE counterpart.
// The audit_trail table has no UPDATE/DELETE grants in production (§13 — EN9100).
func (t *txOps) AppendAuditEntry(ctx context.Context, e *domain.AuditEntry) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO audit_trail
		   (id, actor_id, actor_role, action, entity_type, entity_id, workstation_id, notes, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9)`,
		e.ID, e.ActorID, e.ActorRole, string(e.Action),
		string(e.EntityType), e.EntityID,
		e.WorkstationID, e.Notes, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("AppendAuditEntry: exec: %w", err)
	}
	return nil
}

// ── Audit Trail Read ──────────────────────────────────────────────────────────

// QueryAuditTrail returns audit entries matching the given filter, ordered by created_at DESC.
// PageSize defaults to 50; max 200. PageToken is an opaque cursor (created_at + id).
func (r *PostgresRepo) QueryAuditTrail(ctx context.Context, f domain.AuditFilter) ([]*domain.AuditEntry, error) {
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	args := []any{}
	where := "WHERE 1=1"
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.ActorID != "" {
		where += " AND actor_id = " + arg(f.ActorID)
	}
	if f.EntityType != "" {
		where += " AND entity_type = " + arg(string(f.EntityType))
	}
	if f.EntityID != "" {
		where += " AND entity_id = " + arg(f.EntityID)
	}
	if f.Action != "" {
		where += " AND action = " + arg(string(f.Action))
	}
	if f.From != nil {
		where += " AND created_at >= " + arg(*f.From)
	}
	if f.To != nil {
		where += " AND created_at <= " + arg(*f.To)
	}

	args = append(args, pageSize+1)
	query := fmt.Sprintf(
		`SELECT id, actor_id, actor_role, action, entity_type, entity_id,
		        COALESCE(workstation_id::text,''), COALESCE(notes,''), created_at
		 FROM audit_trail %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		where, len(args),
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("QueryAuditTrail: query: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		e, err := scanAuditEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("QueryAuditTrail: scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func scanAuditEntry(row pgx.Row) (*domain.AuditEntry, error) {
	var e domain.AuditEntry
	var action, entityType string
	err := row.Scan(
		&e.ID, &e.ActorID, &e.ActorRole, &action, &entityType,
		&e.EntityID, &e.WorkstationID, &e.Notes, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAsBuiltNotFound
		}
		return nil, err
	}
	e.Action = domain.AuditAction(action)
	e.EntityType = domain.AuditEntityType(entityType)
	return &e, nil
}
