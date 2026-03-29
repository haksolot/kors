package domain_test

import (
	"testing"
	"time"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTool(t *testing.T) {
	now := time.Now().UTC()
	nextCal := now.Add(365 * 24 * time.Hour)

	tests := []struct {
		name      string
		sn        string
		toolName  string
		maxCycles int
		wantErr   error
	}{
		{
			name:      "valid tool",
			sn:        "SN-TOOL-001",
			toolName:  "Caliber 150mm",
			maxCycles: 1000,
			wantErr:   nil,
		},
		{
			name:      "missing sn",
			sn:        "",
			toolName:  "Tool",
			maxCycles: 0,
			wantErr:   domain.ErrInvalidToolInput,
		},
		{
			name:      "negative cycles",
			sn:        "SN-1",
			toolName:  "Tool",
			maxCycles: -1,
			wantErr:   domain.ErrInvalidToolCycles,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := domain.NewTool(tc.sn, tc.toolName, "Desc", "Gauges", &now, &nextCal, tc.maxCycles)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, tool.ID)
			assert.Equal(t, domain.ToolStatusValid, tool.Status)
			assert.True(t, tool.IsCalibrationValid(now))
		})
	}
}

func TestTool_IsCalibrationValid(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	t.Run("valid calibration", func(t *testing.T) {
		tool := &domain.Tool{NextCalibrationAt: &future}
		assert.True(t, tool.IsCalibrationValid(now))
	})

	t.Run("expired calibration", func(t *testing.T) {
		tool := &domain.Tool{NextCalibrationAt: &past}
		assert.False(t, tool.IsCalibrationValid(now))
	})

	t.Run("no calibration required", func(t *testing.T) {
		tool := &domain.Tool{NextCalibrationAt: nil}
		assert.True(t, tool.IsCalibrationValid(now))
	})
}

func TestTool_HasRemainingLife(t *testing.T) {
	t.Run("unlimited cycles", func(t *testing.T) {
		tool := &domain.Tool{MaxCycles: 0, CurrentCycles: 9999}
		assert.True(t, tool.HasRemainingLife())
	})

	t.Run("has remaining", func(t *testing.T) {
		tool := &domain.Tool{MaxCycles: 100, CurrentCycles: 50}
		assert.True(t, tool.HasRemainingLife())
	})

	t.Run("reached limit", func(t *testing.T) {
		tool := &domain.Tool{MaxCycles: 100, CurrentCycles: 100}
		assert.False(t, tool.HasRemainingLife())
	})
}
