package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/haksolot/kors/services/mes/domain"
)

// FindWorkstationByID retrieves a Workstation by its UUID.
func (r *PostgresRepo) FindWorkstationByID(ctx context.Context, id string) (*domain.Workstation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, description, capacity, nominal_rate, status, created_at, updated_at
		 FROM workstations WHERE id = $1`, id,
	)
	w, err := scanWorkstation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("FindWorkstationByID %s: workstation not found", id)
		}
		return nil, fmt.Errorf("FindWorkstationByID %s: %w", id, err)
	}
	return w, nil
}

// ListWorkstations returns a paginated list of workstations.
func (r *PostgresRepo) ListWorkstations(ctx context.Context, limit, offset int) ([]*domain.Workstation, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, capacity, nominal_rate, status, created_at, updated_at
		 FROM workstations ORDER BY name ASC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ListWorkstations query: %w", err)
	}
	defer rows.Close()

	var workstations []*domain.Workstation
	for rows.Next() {
		w, err := scanWorkstation(rows)
		if err != nil {
			return nil, fmt.Errorf("ListWorkstations scan: %w", err)
		}
		workstations = append(workstations, w)
	}
	return workstations, rows.Err()
}

// SaveWorkstation persists a new Workstation.
func (t *txOps) SaveWorkstation(ctx context.Context, w *domain.Workstation) error {
	_, err := t.tx.Exec(ctx,
		`INSERT INTO workstations
			(id, name, description, capacity, nominal_rate, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		w.ID, w.Name, w.Description, w.Capacity, w.NominalRate, string(w.Status),
		w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("SaveWorkstation %s: %w", w.ID, err)
	}
	return nil
}

// UpdateWorkstation persists state changes on an existing Workstation.
func (t *txOps) UpdateWorkstation(ctx context.Context, w *domain.Workstation) error {
	_, err := t.tx.Exec(ctx,
		`UPDATE workstations
		 SET name=$1, description=$2, capacity=$3, nominal_rate=$4, status=$5, updated_at=$6
		 WHERE id=$7`,
		w.Name, w.Description, w.Capacity, w.NominalRate, string(w.Status), w.UpdatedAt, w.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateWorkstation %s: %w", w.ID, err)
	}
	return nil
}

func scanWorkstation(row pgx.Row) (*domain.Workstation, error) {
	var w domain.Workstation
	var status string
	err := row.Scan(
		&w.ID, &w.Name, &w.Description, &w.Capacity, &w.NominalRate,
		&status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	w.Status = domain.WorkstationStatus(status)
	return &w, nil
}
