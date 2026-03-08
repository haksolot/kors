package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/haksolot/kors/examples/module-example/internal/model"
)

type ToolStore struct {
	Pool *pgxpool.Pool
}

func (s *ToolStore) Save(ctx context.Context, tool *model.Tool) error {
	query := `
		INSERT INTO tools (id, serial_number, model, diameter, length)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.Pool.Exec(ctx, query,
		tool.ID,
		tool.SerialNumber,
		tool.Model,
		tool.Diameter,
		tool.Length,
	)
	if err != nil {
		return fmt.Errorf("failed to save tool: %w", err)
	}
	return nil
}

func (s *ToolStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Tool, error) {
	query := "SELECT id, serial_number, model, diameter, length FROM tools WHERE id = $1"
	var tool model.Tool
	err := s.Pool.QueryRow(ctx, query, id).Scan(&tool.ID, &tool.SerialNumber, &tool.Model, &tool.Diameter, &tool.Length)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (s *ToolStore) ListAll(ctx context.Context) ([]*model.Tool, error) {
	// Grace au search_path configure (tache 1.5), "tools" est accessible directement
	rows, err := s.Pool.Query(ctx, "SELECT id, serial_number, model, diameter, length FROM tools")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tools []*model.Tool
	for rows.Next() {
		var t model.Tool
		if err := rows.Scan(&t.ID, &t.SerialNumber, &t.Model, &t.Diameter, &t.Length); err != nil {
			return nil, err
		}
		tools = append(tools, &t)
	}
	return tools, nil
}
