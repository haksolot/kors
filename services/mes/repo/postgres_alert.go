package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Alert Read Operations ───────────────────────────────────────────────────

func (r *PostgresRepo) FindAlertByID(ctx context.Context, id string) (*domain.Alert, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, category, level, status, workstation_id, operation_id, message, escalation_count,
		        acknowledged_by, acknowledged_at, resolved_by, resolved_at, resolution_notes,
		        created_at, updated_at
		 FROM alerts WHERE id = $1`, id,
	)
	return scanAlert(row)
}

func (r *PostgresRepo) ListActiveAlerts(ctx context.Context) ([]*domain.Alert, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, category, level, status, workstation_id, operation_id, message, escalation_count,
		        acknowledged_by, acknowledged_at, resolved_by, resolved_at, resolution_notes,
		        created_at, updated_at
		 FROM alerts WHERE status != 'RESOLVED' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// ── Alert Write Operations (TxOps) ──────────────────────────────────────────

func (t *txOps) SaveAlert(ctx context.Context, a *domain.Alert) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO alerts (id, category, level, status, workstation_id, operation_id, message, escalation_count, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, string(a.Category), string(a.Level), string(a.Status), a.WorkstationID, a.OperationID, a.Message, a.EscalationCount, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (t *txOps) UpdateAlert(ctx context.Context, a *domain.Alert) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE alerts
		 SET category=$1, level=$2, status=$3, workstation_id=$4, operation_id=$5, message=$6, escalation_count=$7,
		     acknowledged_by=$8, acknowledged_at=$9, resolved_by=$10, resolved_at=$11, resolution_notes=$12, updated_at=$13
		 WHERE id=$14`,
		string(a.Category), string(a.Level), string(a.Status), a.WorkstationID, a.OperationID, a.Message, a.EscalationCount,
		a.AcknowledgedBy, a.AcknowledgedAt, a.ResolvedBy, a.ResolvedAt, a.ResolutionNotes, a.UpdatedAt, a.ID,
	)
	return err
}

// ── Scanner ──────────────────────────────────────────────────────────────────

func scanAlert(row pgx.Row) (*domain.Alert, error) {
	var a domain.Alert
	var cat, level, status string
	var notes *string
	err := row.Scan(
		&a.ID, &cat, &level, &status, &a.WorkstationID, &a.OperationID, &a.Message, &a.EscalationCount,
		&a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedBy, &a.ResolvedAt, &notes,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("alert not found")
		}
		return nil, err
	}
	if notes != nil {
		a.ResolutionNotes = *notes
	}
	a.Category = domain.AlertCategory(cat)
	a.Level = domain.AlertLevel(level)
	a.Status = domain.AlertStatus(status)
	return &a, nil
}
