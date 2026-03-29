package domain_test

import (
	"testing"
	"time"

	"github.com/haksolot/kors/services/mes/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTimeLog(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		operationID   string
		workstationID string
		operatorID    string
		logType       domain.TimeLogType
		start         time.Time
		end           time.Time
		good          int
		scrap         int
		wantErr       error
	}{
		{
			name:          "valid run log",
			operationID:   "op-1",
			workstationID: "ws-1",
			operatorID:    "oper-1",
			logType:       domain.TimeLogTypeRun,
			start:         now,
			end:           now.Add(1 * time.Hour),
			good:          100,
			scrap:         2,
			wantErr:       nil,
		},
		{
			name:          "missing operation id",
			operationID:   "",
			workstationID: "ws-1",
			operatorID:    "oper-1",
			logType:       domain.TimeLogTypeRun,
			start:         now,
			end:           now.Add(1 * time.Hour),
			good:          100,
			scrap:         2,
			wantErr:       domain.ErrInvalidTimeLogInput,
		},
		{
			name:          "end before start",
			operationID:   "op-1",
			workstationID: "ws-1",
			operatorID:    "oper-1",
			logType:       domain.TimeLogTypeSetup,
			start:         now.Add(1 * time.Hour),
			end:           now,
			good:          0,
			scrap:         0,
			wantErr:       domain.ErrInvalidTimeLogDates,
		},
		{
			name:          "negative quantities",
			operationID:   "op-1",
			workstationID: "ws-1",
			operatorID:    "oper-1",
			logType:       domain.TimeLogTypeRun,
			start:         now,
			end:           now.Add(1 * time.Hour),
			good:          -1,
			scrap:         0,
			wantErr:       domain.ErrInvalidTimeLogQuantities,
		},
		{
			name:          "invalid type",
			operationID:   "op-1",
			workstationID: "ws-1",
			operatorID:    "oper-1",
			logType:       "INVALID",
			start:         now,
			end:           now.Add(1 * time.Hour),
			good:          0,
			scrap:         0,
			wantErr:       domain.ErrInvalidTimeLogType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log, err := domain.NewTimeLog(tc.operationID, tc.workstationID, tc.operatorID, tc.logType, tc.start, tc.end, tc.good, tc.scrap)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, log.ID)
			assert.Equal(t, tc.operationID, log.OperationID)
			assert.Equal(t, 1*time.Hour, log.Duration())
		})
	}
}

func TestNewDowntimeEvent(t *testing.T) {
	opID := "op-1"
	tests := []struct {
		name          string
		workstationID string
		operationID   *string
		category      domain.DowntimeCategory
		description   string
		reportedBy    string
		wantErr       error
	}{
		{
			name:          "valid downtime with operation",
			workstationID: "ws-1",
			operationID:   &opID,
			category:      domain.DowntimeCategoryMachineFailure,
			description:   "Jammed",
			reportedBy:    "oper-1",
			wantErr:       nil,
		},
		{
			name:          "valid downtime without operation",
			workstationID: "ws-1",
			operationID:   nil,
			category:      domain.DowntimeCategoryPreventiveMaintenance,
			description:   "Monthly oil change",
			reportedBy:    "maint-1",
			wantErr:       nil,
		},
		{
			name:          "missing workstation",
			workstationID: "",
			operationID:   nil,
			category:      domain.DowntimeCategoryMachineFailure,
			description:   "",
			reportedBy:    "oper-1",
			wantErr:       domain.ErrInvalidDowntimeInput,
		},
		{
			name:          "invalid category",
			workstationID: "ws-1",
			operationID:   nil,
			category:      "UNKNOWN",
			description:   "",
			reportedBy:    "oper-1",
			wantErr:       domain.ErrInvalidDowntimeCategory,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dt, err := domain.NewDowntimeEvent(tc.workstationID, tc.operationID, tc.category, tc.description, tc.reportedBy)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, dt.ID)
			assert.Nil(t, dt.EndTime)
			assert.True(t, dt.Duration() >= 0) // Should be almost 0 immediately after creation

			err = dt.End()
			require.NoError(t, err)
			assert.NotNil(t, dt.EndTime)

			err = dt.End()
			assert.ErrorIs(t, err, domain.ErrDowntimeAlreadyEnded)
		})
	}
}

func TestCalculateOEE(t *testing.T) {
	tests := []struct {
		name            string
		plannedTime     time.Duration
		operatingTime   time.Duration
		downtime        time.Duration
		goodQty         int
		scrapQty        int
		idealCycleTime  float64
		wantAvail       float64
		wantPerf        float64
		wantQual        float64
		wantTRS         float64
	}{
		{
			name:            "perfect shift",
			plannedTime:     8 * time.Hour,
			operatingTime:   8 * time.Hour,
			downtime:        0,
			goodQty:         800,
			scrapQty:        0,
			idealCycleTime:  36.0, // 36s per unit -> 100 per hour -> 800 in 8h
			wantAvail:       1.0,
			wantPerf:        1.0,
			wantQual:        1.0,
			wantTRS:         1.0,
		},
		{
			name:            "with downtime and scrap",
			plannedTime:     10 * time.Hour, // 36000s
			operatingTime:   8 * time.Hour,  // 28800s
			downtime:        2 * time.Hour,  // 7200s
			goodQty:         700,
			scrapQty:        100,            // Total = 800
			idealCycleTime:  30.0,           // 30s per unit -> should do 960 in 8h. Did 800. Perf = 800*30 / 28800 = 24000/28800 = 0.8333
			wantAvail:       0.8,            // (10-2)/10 = 0.8
			wantPerf:        24000.0 / 28800.0, // ~0.833
			wantQual:        700.0 / 800.0,  // 0.875
			wantTRS:         0.8 * (24000.0 / 28800.0) * (700.0 / 800.0), // ~0.583
		},
		{
			name:            "capped performance",
			plannedTime:     1 * time.Hour, // 3600s
			operatingTime:   1 * time.Hour, // 3600s
			downtime:        0,
			goodQty:         120,
			scrapQty:        0,
			idealCycleTime:  36.0, // 36*120 = 4320s theoretical, but only took 3600s -> Perf > 1.0 -> should cap at 1.0
			wantAvail:       1.0,
			wantPerf:        1.0,
			wantQual:        1.0,
			wantTRS:         1.0,
		},
		{
			name:            "zero planned time",
			plannedTime:     0,
			operatingTime:   0,
			downtime:        0,
			goodQty:         0,
			scrapQty:        0,
			idealCycleTime:  10.0,
			wantAvail:       0.0,
			wantPerf:        0.0,
			wantQual:        1.0, // 1.0 when no production
			wantTRS:         0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.CalculateOEE(tc.plannedTime, tc.operatingTime, tc.downtime, tc.goodQty, tc.scrapQty, tc.idealCycleTime)
			assert.InDelta(t, tc.wantAvail, got.Availability, 0.001, "Availability mismatch")
			assert.InDelta(t, tc.wantPerf, got.Performance, 0.001, "Performance mismatch")
			assert.InDelta(t, tc.wantQual, got.Quality, 0.001, "Quality mismatch")
			assert.InDelta(t, tc.wantTRS, got.TRS, 0.001, "TRS mismatch")
		})
	}
}
