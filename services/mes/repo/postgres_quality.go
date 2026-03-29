package repo

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Quality Read Operations ──────────────────────────────────────────────────

func (r *PostgresRepo) FindCharacteristicByID(ctx context.Context, id string) (*domain.ControlCharacteristic, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, step_id, name, type, unit, nominal_value, upper_tolerance, lower_tolerance, is_mandatory
		 FROM control_characteristics WHERE id = $1`, id,
	)
	return scanCharacteristic(row)
}

func (r *PostgresRepo) ListCharacteristicsByStep(ctx context.Context, stepID string) ([]*domain.ControlCharacteristic, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, step_id, name, type, unit, nominal_value, upper_tolerance, lower_tolerance, is_mandatory
		 FROM control_characteristics WHERE step_id = $1`, stepID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.ControlCharacteristic
	for rows.Next() {
		c, err := scanCharacteristic(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

func (r *PostgresRepo) ListCharacteristicsByOperation(ctx context.Context, operationID string) ([]*domain.ControlCharacteristic, error) {
	rows, err := r.db.Query(ctx,
		`SELECT cc.id, cc.step_id, cc.name, cc.type, cc.unit, cc.nominal_value, cc.upper_tolerance, cc.lower_tolerance, cc.is_mandatory
		 FROM control_characteristics cc
		 JOIN routing_steps rs ON cc.step_id = rs.id
		 JOIN operations o ON rs.step_number = o.step_number
		 JOIN manufacturing_orders mo ON o.of_id = mo.id
		 JOIN routings rt ON rs.routing_id = rt.id
		 WHERE o.id = $1`, operationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.ControlCharacteristic
	for rows.Next() {
		c, err := scanCharacteristic(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

func (r *PostgresRepo) ListMeasurementsByOperation(ctx context.Context, operationID string) ([]*domain.Measurement, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, operation_id, characteristic_id, value, status, operator_id, recorded_at
		 FROM measurements WHERE operation_id = $1`, operationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.Measurement
	for rows.Next() {
		m, err := scanMeasurement(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

func (r *PostgresRepo) ListMeasurementsByCharacteristic(ctx context.Context, characteristicID string, limit int) ([]*domain.Measurement, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, operation_id, characteristic_id, value, status, operator_id, recorded_at
		 FROM measurements WHERE characteristic_id = $1 ORDER BY recorded_at DESC LIMIT $2`, characteristicID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.Measurement
	for rows.Next() {
		m, err := scanMeasurement(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

// ── Quality Write Operations (TxOps) ─────────────────────────────────────────

func (t *txOps) SaveControlCharacteristic(ctx context.Context, c *domain.ControlCharacteristic) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO control_characteristics (id, step_id, name, type, unit, nominal_value, upper_tolerance, lower_tolerance, is_mandatory)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		c.ID, c.StepID, c.Name, string(c.Type), c.Unit, c.NominalValue, c.UpperTolerance, c.LowerTolerance, c.IsMandatory,
	)
	return err
}

func (t *txOps) SaveMeasurement(ctx context.Context, m *domain.Measurement) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO measurements (id, operation_id, characteristic_id, value, status, operator_id, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		m.ID, m.OperationID, m.CharacteristicID, m.Value, string(m.Status), m.OperatorID, m.RecordedAt,
	)
	return err
}

// ── Scanners ─────────────────────────────────────────────────────────────────

func scanCharacteristic(row pgx.Row) (*domain.ControlCharacteristic, error) {
	var c domain.ControlCharacteristic
	var cType string
	err := row.Scan(&c.ID, &c.StepID, &c.Name, &cType, &c.Unit, &c.NominalValue, &c.UpperTolerance, &c.LowerTolerance, &c.IsMandatory)
	if err != nil {
		return nil, err
	}
	c.Type = domain.CharacteristicType(cType)
	return &c, nil
}

func scanMeasurement(row pgx.Row) (*domain.Measurement, error) {
	var m domain.Measurement
	var status string
	err := row.Scan(&m.ID, &m.OperationID, &m.CharacteristicID, &m.Value, &status, &m.OperatorID, &m.RecordedAt)
	if err != nil {
		return nil, err
	}
	m.Status = domain.MeasurementStatus(status)
	return &m, nil
}
