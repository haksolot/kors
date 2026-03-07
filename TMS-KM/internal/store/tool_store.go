package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safran-ls/tms-km/internal/model"
)

type ToolStore struct {
	Pool *pgxpool.Pool
}

func (s *ToolStore) Save(ctx context.Context, tool *model.Tool) error {
	query := `
		INSERT INTO tms.tools (id, serial_number, model, diameter, length)
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
		return fmt.Errorf("failed to save tool in TMS schema: %w", err)
	}
	return nil
}
