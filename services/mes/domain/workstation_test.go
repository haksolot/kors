package domain_test

import (
	"testing"
	"time"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkstation(t *testing.T) {
	tests := []struct {
		name        string
		wsName      string
		desc        string
		capacity    int
		nominalRate float64
		wantErr     bool
	}{
		{
			name:        "valid workstation",
			wsName:      "WS-001",
			desc:        "Assembly Line 1",
			capacity:    2,
			nominalRate: 100.5,
			wantErr:     false,
		},
		{
			name:        "empty name returns error",
			wsName:      "",
			desc:        "Desc",
			capacity:    1,
			nominalRate: 50.0,
			wantErr:     true,
		},
		{
			name:        "zero capacity returns error",
			wsName:      "WS-002",
			desc:        "Desc",
			capacity:    0,
			nominalRate: 50.0,
			wantErr:     true,
		},
		{
			name:        "negative nominal rate returns error",
			wsName:      "WS-003",
			desc:        "Desc",
			capacity:    1,
			nominalRate: -10.0,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws, err := domain.NewWorkstation(tc.wsName, tc.desc, tc.capacity, tc.nominalRate)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, ws.ID)
			assert.Equal(t, tc.wsName, ws.Name)
			assert.Equal(t, tc.desc, ws.Description)
			assert.Equal(t, tc.capacity, ws.Capacity)
			assert.Equal(t, tc.nominalRate, ws.NominalRate)
			assert.Equal(t, domain.WorkstationStatusAvailable, ws.Status)
			assert.NotZero(t, ws.CreatedAt)
			assert.NotZero(t, ws.UpdatedAt)
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	ws, err := domain.NewWorkstation("WS-001", "", 1, 10.0)
	require.NoError(t, err)

	originalUpdatedAt := ws.UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure time progresses

	tests := []struct {
		name      string
		newStatus domain.WorkstationStatus
		wantErr   bool
	}{
		{
			name:      "update to in production",
			newStatus: domain.WorkstationStatusInProduction,
			wantErr:   false,
		},
		{
			name:      "update to down",
			newStatus: domain.WorkstationStatusDown,
			wantErr:   false,
		},
		{
			name:      "update to maintenance",
			newStatus: domain.WorkstationStatusMaintenance,
			wantErr:   false,
		},
		{
			name:      "update to available",
			newStatus: domain.WorkstationStatusAvailable,
			wantErr:   false,
		},
		{
			name:      "invalid status",
			newStatus: "UNKNOWN_STATUS",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ws.UpdateStatus(tc.newStatus)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.newStatus, ws.Status)
			assert.True(t, ws.UpdatedAt.After(originalUpdatedAt))
		})
	}
}
