package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Tool Read Operations ──────────────────────────────────────────────────────

func (r *PostgresRepo) FindToolByID(ctx context.Context, id string) (*domain.Tool, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, serial_number, name, description, category, status,
		        last_calibration_at, next_calibration_at, current_cycles, max_cycles,
		        created_at, updated_at
		 FROM tools WHERE id = $1`, id,
	)
	return scanTool(row)
}

func (r *PostgresRepo) FindToolBySerialNumber(ctx context.Context, sn string) (*domain.Tool, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, serial_number, name, description, category, status,
		        last_calibration_at, next_calibration_at, current_cycles, max_cycles,
		        created_at, updated_at
		 FROM tools WHERE serial_number = $1`, sn,
	)
	return scanTool(row)
}

func (r *PostgresRepo) ListTools(ctx context.Context, limit, offset int) ([]*domain.Tool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, serial_number, name, description, category, status,
		        last_calibration_at, next_calibration_at, current_cycles, max_cycles,
		        created_at, updated_at
		 FROM tools ORDER BY name ASC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ListTools query: %w", err)
	}
	defer rows.Close()

	var tools []*domain.Tool
	for rows.Next() {
		t, err := scanTool(rows)
		if err != nil {
			return nil, fmt.Errorf("ListTools scan: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *PostgresRepo) ListToolsByOperation(ctx context.Context, operationID string) ([]*domain.Tool, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id, t.serial_number, t.name, t.description, t.category, t.status,
		        t.last_calibration_at, t.next_calibration_at, t.current_cycles, t.max_cycles,
		        t.created_at, t.updated_at
		 FROM tools t
		 JOIN operation_tools ot ON t.id = ot.tool_id
		 WHERE ot.operation_id = $1`, operationID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListToolsByOperation query: %w", err)
	}
	defer rows.Close()

	var tools []*domain.Tool
	for rows.Next() {
		t, err := scanTool(rows)
		if err != nil {
			return nil, fmt.Errorf("ListToolsByOperation scan: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

// ── Tool Write Operations (TxOps) ─────────────────────────────────────────────

func (t *txOps) SaveTool(ctx context.Context, tool *domain.Tool) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO tools
			(id, serial_number, name, description, category, status,
			 last_calibration_at, next_calibration_at, current_cycles, max_cycles,
			 created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		tool.ID, tool.SerialNumber, tool.Name, tool.Description, tool.Category, string(tool.Status),
		tool.LastCalibrationAt, tool.NextCalibrationAt, tool.CurrentCycles, tool.MaxCycles,
		tool.CreatedAt, tool.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveTool %s: %w", tool.SerialNumber, err)
	}
	return nil
}

func (t *txOps) UpdateTool(ctx context.Context, tool *domain.Tool) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE tools
		 SET name=$1, description=$2, category=$3, status=$4,
		     last_calibration_at=$5, next_calibration_at=$6,
		     current_cycles=$7, max_cycles=$8, updated_at=$9
		 WHERE id=$10`,
		tool.Name, tool.Description, tool.Category, string(tool.Status),
		tool.LastCalibrationAt, tool.NextCalibrationAt,
		tool.CurrentCycles, tool.MaxCycles, tool.UpdatedAt, tool.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateTool %s: %w", tool.ID, err)
	}
	return nil
}

func (t *txOps) LinkToolToOperation(ctx context.Context, operationID, toolID string) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO operation_tools (operation_id, tool_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		operationID, toolID,
	)
	if err != nil {
		return fmt.Errorf("LinkToolToOperation: %w", err)
	}
	return nil
}

// ── Scanner ───────────────────────────────────────────────────────────────────

func scanTool(row pgx.Row) (*domain.Tool, error) {
	var t domain.Tool
	var status string
	err := row.Scan(
		&t.ID, &t.SerialNumber, &t.Name, &t.Description, &t.Category, &status,
		&t.LastCalibrationAt, &t.NextCalibrationAt, &t.CurrentCycles, &t.MaxCycles,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrToolNotFound
		}
		return nil, err
	}
	t.Status = domain.ToolStatus(status)
	return &t, nil
}
