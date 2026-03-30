package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// GetAsBuiltByOFID builds the complete As-Built dossier (§13 — Dossier Industriel Numérique)
// for the given manufacturing order. It joins orders, operations, measurements,
// consumed lots, tools and serial numbers in a single read-side query set.
// The result is a denormalized view suited for audit export (EN9100 §8.5.2, §8.6).
func (r *PostgresRepo) GetAsBuiltByOFID(ctx context.Context, ofID string) (*domain.AsBuiltReport, error) {
	// ── 1. Load the order ────────────────────────────────────────────────────
	row := r.db.QueryRow(ctx,
		`SELECT id, reference, product_id, quantity, status,
		        is_fai, fai_approved_by, fai_approved_at,
		        started_at, completed_at
		 FROM manufacturing_orders WHERE id = $1`, ofID,
	)
	report := &domain.AsBuiltReport{GeneratedAt: time.Now().UTC()}
	var faiApprovedBy *string
	var statusStr string
	err := row.Scan(
		&report.OrderID, &report.Reference, &report.ProductID, &report.Quantity, &statusStr,
		&report.IsFAI, &faiApprovedBy, &report.FAIApprovedAt,
		&report.StartedAt, &report.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAsBuiltNotFound
		}
		return nil, fmt.Errorf("GetAsBuiltByOFID: load order: %w", err)
	}
	report.Status = domain.OrderStatus(statusStr)
	if faiApprovedBy != nil {
		report.FAIApprovedBy = *faiApprovedBy
	}

	// ── 2. Load operations ───────────────────────────────────────────────────
	opRows, err := r.db.Query(ctx,
		`SELECT id, step_number, name, status, operator_id,
		        workstation_id, requires_sign_off, signed_off_by, signed_off_at,
		        is_special_process, nadcap_process_code,
		        planned_duration_seconds, actual_duration_seconds,
		        started_at, completed_at
		 FROM operations WHERE of_id = $1 ORDER BY step_number`, ofID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAsBuiltByOFID: load operations: %w", err)
	}
	defer opRows.Close()

	ops := map[string]*domain.AsBuiltOperation{}
	for opRows.Next() {
		o := &domain.AsBuiltOperation{}
		var opStatus, workstationID, operatorID, signedOffBy *string
		var nadcapCode *string
		err := opRows.Scan(
			&o.OperationID, &o.StepNumber, &o.Name, &opStatus, &operatorID,
			&workstationID, &o.RequiresSignOff, &signedOffBy, &o.SignedOffAt,
			&o.IsSpecialProcess, &nadcapCode,
			&o.PlannedDurationSeconds, &o.ActualDurationSeconds,
			&o.StartedAt, &o.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAsBuiltByOFID: scan operation: %w", err)
		}
		if opStatus != nil {
			o.Status = domain.OperationStatus(*opStatus)
		}
		if workstationID != nil {
			o.WorkstationID = *workstationID
		}
		if operatorID != nil {
			o.OperatorID = *operatorID
		}
		if signedOffBy != nil {
			o.SignedOffBy = *signedOffBy
		}
		if nadcapCode != nil {
			o.NADCAPProcessCode = *nadcapCode
		}
		ops[o.OperationID] = o
		report.Operations = append(report.Operations, o)
	}
	opRows.Close()

	// ── 3. Load measurements per operation ───────────────────────────────────
	if len(ops) > 0 {
		measRows, err := r.db.Query(ctx,
			`SELECT m.id, m.operation_id, m.characteristic_id, m.value, m.status,
			        m.operator_id, m.recorded_at
			 FROM measurements m
			 JOIN operations op ON op.id = m.operation_id
			 WHERE op.of_id = $1`, ofID,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAsBuiltByOFID: load measurements: %w", err)
		}
		defer measRows.Close()
		for measRows.Next() {
			m := &domain.Measurement{}
			var statusStr string
			if err := measRows.Scan(&m.ID, &m.OperationID, &m.CharacteristicID, &m.Value, &statusStr, &m.OperatorID, &m.RecordedAt); err != nil {
				return nil, fmt.Errorf("GetAsBuiltByOFID: scan measurement: %w", err)
			}
			m.Status = domain.MeasurementStatus(statusStr)
			if o, ok := ops[m.OperationID]; ok {
				o.Measurements = append(o.Measurements, m)
			}
		}
	}

	// ── 4. Load consumed lots per operation ──────────────────────────────────
	if len(ops) > 0 {
		consRows, err := r.db.Query(ctx,
			`SELECT cr.id, cr.operation_id, cr.lot_id, cr.quantity_consumed, cr.operator_id, cr.consumed_at
			 FROM consumption_records cr
			 JOIN operations op ON op.id = cr.operation_id
			 WHERE op.of_id = $1`, ofID,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAsBuiltByOFID: load consumptions: %w", err)
		}
		defer consRows.Close()
		for consRows.Next() {
			c := &domain.ConsumptionRecord{}
			if err := consRows.Scan(&c.ID, &c.OperationID, &c.LotID, &c.Quantity, &c.OperatorID, &c.ConsumedAt); err != nil {
				return nil, fmt.Errorf("GetAsBuiltByOFID: scan consumption: %w", err)
			}
			if o, ok := ops[c.OperationID]; ok {
				o.ConsumedLots = append(o.ConsumedLots, c)
			}
		}
	}

	// ── 5. Load tools per operation ──────────────────────────────────────────
	if len(ops) > 0 {
		toolRows, err := r.db.Query(ctx,
			`SELECT ot.operation_id, t.id, t.serial_number, t.name, t.calibration_expires_at
			 FROM operation_tools ot
			 JOIN tools t ON t.id = ot.tool_id
			 JOIN operations op ON op.id = ot.operation_id
			 WHERE op.of_id = $1`, ofID,
		)
		if err != nil {
			return nil, fmt.Errorf("GetAsBuiltByOFID: load tools: %w", err)
		}
		defer toolRows.Close()
		for toolRows.Next() {
			var operationID string
			tool := domain.AsBuiltTool{}
			if err := toolRows.Scan(&operationID, &tool.ToolID, &tool.SerialNumber, &tool.Name, &tool.CalibrationExpiry); err != nil {
				return nil, fmt.Errorf("GetAsBuiltByOFID: scan tool: %w", err)
			}
			if o, ok := ops[operationID]; ok {
				o.Tools = append(o.Tools, tool)
			}
		}
	}

	// ── 6. Load serial numbers produced by this OF ───────────────────────────
	snRows, err := r.db.Query(ctx,
		`SELECT id, sn, lot_id, product_id, of_id, status, created_at
		 FROM serial_numbers WHERE of_id = $1`, ofID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetAsBuiltByOFID: load SNs: %w", err)
	}
	defer snRows.Close()
	for snRows.Next() {
		sn := &domain.SerialNumber{}
		var statusStr string
		if err := snRows.Scan(&sn.ID, &sn.SN, &sn.LotID, &sn.ProductID, &sn.OFID, &statusStr, &sn.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetAsBuiltByOFID: scan SN: %w", err)
		}
		sn.Status = domain.SerialNumberStatus(statusStr)
		report.SerialNumbers = append(report.SerialNumbers, sn)
	}

	return report, nil
}
