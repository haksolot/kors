package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Time Tracking Read Operations ─────────────────────────────────────────────

func (r *PostgresRepo) FindDowntimeByID(ctx context.Context, id string) (*domain.DowntimeEvent, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, workstation_id, operation_id, category, description, start_time, end_time, reported_by, created_at
		 FROM downtime_events WHERE id = $1`, id,
	)
	return scanDowntime(row)
}

func (r *PostgresRepo) FindOngoingDowntime(ctx context.Context, workstationID string) (*domain.DowntimeEvent, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, workstation_id, operation_id, category, description, start_time, end_time, reported_by, created_at
		 FROM downtime_events WHERE workstation_id = $1 AND end_time IS NULL LIMIT 1`, workstationID,
	)
	dt, err := scanDowntime(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No ongoing downtime
		}
		return nil, fmt.Errorf("FindOngoingDowntime: %w", err)
	}
	return dt, nil
}

func (r *PostgresRepo) ListTimeLogsByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.TimeLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, operation_id, workstation_id, operator_id, log_type, start_time, end_time, good_qty, scrap_qty, created_at
		 FROM time_logs
		 WHERE workstation_id = $1 AND start_time >= $2 AND end_time <= $3
		 ORDER BY start_time ASC`, workstationID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("ListTimeLogs query: %w", err)
	}
	defer rows.Close()

	var logs []*domain.TimeLog
	for rows.Next() {
		l, err := scanTimeLog(rows)
		if err != nil {
			return nil, fmt.Errorf("ListTimeLogs scan: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (r *PostgresRepo) ListDowntimesByWorkstation(ctx context.Context, workstationID string, from, to time.Time) ([]*domain.DowntimeEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workstation_id, operation_id, category, description, start_time, end_time, reported_by, created_at
		 FROM downtime_events
		 WHERE workstation_id = $1 AND start_time >= $2 AND (end_time IS NULL OR end_time <= $3)
		 ORDER BY start_time ASC`, workstationID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("ListDowntimes query: %w", err)
	}
	defer rows.Close()

	var events []*domain.DowntimeEvent
	for rows.Next() {
		dt, err := scanDowntime(rows)
		if err != nil {
			return nil, fmt.Errorf("ListDowntimes scan: %w", err)
		}
		events = append(events, dt)
	}
	return events, rows.Err()
}

// ── Time Tracking Write Operations (TxOps) ────────────────────────────────────

func (t *txOps) SaveTimeLog(ctx context.Context, l *domain.TimeLog) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO time_logs
			(id, operation_id, workstation_id, operator_id, log_type, start_time, end_time, good_qty, scrap_qty, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		l.ID, l.OperationID, l.WorkstationID, l.OperatorID, string(l.LogType),
		l.StartTime, l.EndTime, l.GoodQuantity, l.ScrapQuantity, l.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveTimeLog %s: %w", l.ID, err)
	}
	return nil
}

func (t *txOps) SaveDowntimeEvent(ctx context.Context, d *domain.DowntimeEvent) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO downtime_events
			(id, workstation_id, operation_id, category, description, start_time, end_time, reported_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		d.ID, d.WorkstationID, d.OperationID, string(d.Category), d.Description,
		d.StartTime, d.EndTime, d.ReportedBy, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveDowntimeEvent %s: %w", d.ID, err)
	}
	return nil
}

func (t *txOps) UpdateDowntimeEvent(ctx context.Context, d *domain.DowntimeEvent) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE downtime_events
		 SET category=$1, description=$2, end_time=$3
		 WHERE id=$4`,
		string(d.Category), d.Description, d.EndTime, d.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateDowntimeEvent %s: %w", d.ID, err)
	}
	return nil
}

// ── Scanners ──────────────────────────────────────────────────────────────────

func scanTimeLog(row pgx.Row) (*domain.TimeLog, error) {
	var l domain.TimeLog
	var logType string
	err := row.Scan(
		&l.ID, &l.OperationID, &l.WorkstationID, &l.OperatorID, &logType,
		&l.StartTime, &l.EndTime, &l.GoodQuantity, &l.ScrapQuantity, &l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("TimeLog not found")
		}
		return nil, err
	}
	l.LogType = domain.TimeLogType(logType)
	return &l, nil
}

func scanDowntime(row pgx.Row) (*domain.DowntimeEvent, error) {
	var dt domain.DowntimeEvent
	var category string
	err := row.Scan(
		&dt.ID, &dt.WorkstationID, &dt.OperationID, &category, &dt.Description,
		&dt.StartTime, &dt.EndTime, &dt.ReportedBy, &dt.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDowntimeNotFound
		}
		return nil, err
	}
	dt.Category = domain.DowntimeCategory(category)
	return &dt, nil
}
