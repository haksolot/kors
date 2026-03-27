package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

func TestNewOperation(t *testing.T) {
	tests := []struct {
		name       string
		ofID       string
		stepNumber int
		opName     string
		wantErr    error
	}{
		{
			name:       "valid operation",
			ofID:       "of-uuid-1",
			stepNumber: 1,
			opName:     "Découpe laser",
		},
		{
			name:       "empty of_id returns error",
			ofID:       "",
			stepNumber: 1,
			opName:     "Step 1",
			wantErr:    domain.ErrInvalidProductID, // reusing for missing FK — see errors.go note
		},
		{
			name:       "zero step number returns error",
			ofID:       "of-uuid-1",
			stepNumber: 0,
			opName:     "Step 1",
			wantErr:    domain.ErrInvalidStepNumber,
		},
		{
			name:       "empty name returns error",
			ofID:       "of-uuid-1",
			stepNumber: 1,
			opName:     "",
			wantErr:    domain.ErrInvalidOperationName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := domain.NewOperation(tc.ofID, tc.stepNumber, tc.opName)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, op)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, op)
			assert.NotEmpty(t, op.ID)
			assert.Equal(t, tc.ofID, op.OFID)
			assert.Equal(t, tc.stepNumber, op.StepNumber)
			assert.Equal(t, tc.opName, op.Name)
			assert.Equal(t, domain.OperationStatusPending, op.Status)
			assert.Empty(t, op.OperatorID)
			assert.Nil(t, op.StartedAt)
			assert.Nil(t, op.CompletedAt)
		})
	}
}

func TestOperation_Start(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() *domain.Operation
		operatorID string
		wantErr    error
	}{
		{
			name: "pending operation can be started",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				return op
			},
			operatorID: "operator-uuid-1",
		},
		{
			name: "in-progress operation cannot be started again",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				_ = op.Start("op-1")
				return op
			},
			operatorID: "operator-uuid-1",
			wantErr:    domain.ErrOperationAlreadyStarted,
		},
		{
			name: "completed operation cannot be started",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				_ = op.Start("op-1")
				_ = op.Complete("op-1")
				return op
			},
			operatorID: "op-1",
			wantErr:    domain.ErrInvalidTransition,
		},
		{
			name: "empty operator ID returns error",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				return op
			},
			operatorID: "",
			wantErr:    domain.ErrInvalidProductID, // missing operator — see errors.go
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := tc.setup()
			err := op.Start(tc.operatorID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OperationStatusInProgress, op.Status)
			assert.Equal(t, tc.operatorID, op.OperatorID)
			require.NotNil(t, op.StartedAt)
		})
	}
}

func TestOperation_Complete(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() *domain.Operation
		operatorID string
		wantErr    error
	}{
		{
			name: "in-progress operation can be completed",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				_ = op.Start("op-1")
				return op
			},
			operatorID: "op-1",
		},
		{
			name: "pending operation cannot be completed",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				return op
			},
			operatorID: "op-1",
			wantErr:    domain.ErrOperationNotStarted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := tc.setup()
			err := op.Complete(tc.operatorID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OperationStatusCompleted, op.Status)
			require.NotNil(t, op.CompletedAt)
		})
	}
}

func TestOperation_Skip(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *domain.Operation
		reason  string
		wantErr error
	}{
		{
			name: "pending operation can be skipped with reason",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 2, "Optional check")
				return op
			},
			reason: "not applicable for this product variant",
		},
		{
			name: "skip without reason returns error",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 2, "Optional check")
				return op
			},
			reason:  "",
			wantErr: domain.ErrSkipReasonRequired,
		},
		{
			name: "completed operation cannot be skipped",
			setup: func() *domain.Operation {
				op, _ := domain.NewOperation("of-1", 1, "Step 1")
				_ = op.Start("op-1")
				_ = op.Complete("op-1")
				return op
			},
			reason:  "late skip",
			wantErr: domain.ErrInvalidTransition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := tc.setup()
			err := op.Skip(tc.reason)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OperationStatusSkipped, op.Status)
			assert.Equal(t, tc.reason, op.SkipReason)
		})
	}
}
