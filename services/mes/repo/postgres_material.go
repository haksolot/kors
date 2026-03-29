package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Material Read Operations ─────────────────────────────────────────────────

func (r *PostgresRepo) FindOngoingTOEExposure(ctx context.Context, lotID string) (*domain.TOEExposureLog, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, lot_id, start_time, end_time, operator_id
		 FROM toe_exposure_logs WHERE lot_id = $1 AND end_time IS NULL LIMIT 1`, lotID,
	)
	return scanTOEExposureLog(row)
}

func (r *PostgresRepo) ListConsumptionsByOperation(ctx context.Context, operationID string) ([]*domain.ConsumptionRecord, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, lot_id, operation_id, quantity, operator_id, consumed_at
		 FROM material_consumptions WHERE operation_id = $1`, operationID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListConsumptionsByOperation query: %w", err)
	}
	defer rows.Close()

	var records []*domain.ConsumptionRecord
	for rows.Next() {
		rec, err := scanConsumptionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("ListConsumptionsByOperation scan: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *PostgresRepo) ListTransfersByEntity(ctx context.Context, entityID string) ([]*domain.LocationTransfer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, entity_id, entity_type, from_workstation_id, to_workstation_id, transferred_by, transferred_at
		 FROM location_transfers WHERE entity_id = $1 ORDER BY transferred_at ASC`, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListTransfersByEntity query: %w", err)
	}
	defer rows.Close()

	var transfers []*domain.LocationTransfer
	for rows.Next() {
		t, err := scanLocationTransfer(rows)
		if err != nil {
			return nil, fmt.Errorf("ListTransfersByEntity scan: %w", err)
		}
		transfers = append(transfers, t)
	}
	return transfers, rows.Err()
}

// ── Material Write Operations (TxOps) ────────────────────────────────────────

func (t *txOps) SaveConsumptionRecord(ctx context.Context, r *domain.ConsumptionRecord) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO material_consumptions (id, lot_id, operation_id, quantity, operator_id, consumed_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ID, r.LotID, r.OperationID, r.Quantity, r.OperatorID, r.ConsumedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveConsumptionRecord: %w", err)
	}
	return nil
}

func (t *txOps) SaveTOEExposureLog(ctx context.Context, l *domain.TOEExposureLog) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO toe_exposure_logs (id, lot_id, start_time, end_time, operator_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		l.ID, l.LotID, l.StartTime, l.EndTime, l.OperatorID,
	)
	if err != nil {
		return fmt.Errorf("SaveTOEExposureLog: %w", err)
	}
	return nil
}

func (t *txOps) UpdateTOEExposureLog(ctx context.Context, l *domain.TOEExposureLog) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE toe_exposure_logs SET end_time = $1 WHERE id = $2`,
		l.EndTime, l.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateTOEExposureLog: %w", err)
	}
	return nil
}

func (t *txOps) SaveLocationTransfer(ctx context.Context, tr *domain.LocationTransfer) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO location_transfers (id, entity_id, entity_type, from_workstation_id, to_workstation_id, transferred_by, transferred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tr.ID, tr.EntityID, string(tr.EntityType), tr.FromWorkstationID, tr.ToWorkstationID, tr.TransferredBy, tr.TransferredAt,
	)
	if err != nil {
		return fmt.Errorf("SaveLocationTransfer: %w", err)
	}
	return nil
}

// ── Scanners ─────────────────────────────────────────────────────────────────

func scanTOEExposureLog(row pgx.Row) (*domain.TOEExposureLog, error) {
	var l domain.TOEExposureLog
	err := row.Scan(&l.ID, &l.LotID, &l.StartTime, &l.EndTime, &l.OperatorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func scanConsumptionRecord(row pgx.Row) (*domain.ConsumptionRecord, error) {
	var r domain.ConsumptionRecord
	err := row.Scan(&r.ID, &r.LotID, &r.OperationID, &r.Quantity, &r.OperatorID, &r.ConsumedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanLocationTransfer(row pgx.Row) (*domain.LocationTransfer, error) {
	var t domain.LocationTransfer
	var eType string
	err := row.Scan(&t.ID, &t.EntityID, &eType, &t.FromWorkstationID, &t.ToWorkstationID, &t.TransferredBy, &t.TransferredAt)
	if err != nil {
		return nil, err
	}
	t.EntityType = domain.EntityType(eType)
	return &t, nil
}
