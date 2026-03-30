package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Operation sign-off (hold points) ─────────────────────────────────────────

func TestOperation_Complete_WithSignOff_MovesPendingSignOff(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Weld joint")
	require.NoError(t, err)
	op.RequiresSignOff = true
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))

	err = op.Complete("00000000-0000-0000-0000-000000000010")
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusPendingSignOff, op.Status)
	assert.NotNil(t, op.CompletedAt)
}

func TestOperation_Complete_WithoutSignOff_MovesCompleted(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Deburr")
	require.NoError(t, err)
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))

	err = op.Complete("00000000-0000-0000-0000-000000000010")
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusCompleted, op.Status)
}

func TestOperation_SignOff_Success(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Weld joint")
	require.NoError(t, err)
	op.RequiresSignOff = true
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))
	require.NoError(t, op.Complete("00000000-0000-0000-0000-000000000010"))

	inspectorID := "00000000-0000-0000-0000-000000000020"
	err = op.SignOff(inspectorID)
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusReleased, op.Status)
	assert.Equal(t, inspectorID, op.SignedOffBy)
	assert.NotNil(t, op.SignedOffAt)
}

func TestOperation_SignOff_EmptyInspector_ReturnsUnauthorized(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Weld joint")
	require.NoError(t, err)
	op.RequiresSignOff = true
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))
	require.NoError(t, op.Complete("00000000-0000-0000-0000-000000000010"))

	err = op.SignOff("")
	assert.ErrorIs(t, err, domain.ErrUnauthorizedRole)
}

func TestOperation_SignOff_NotPendingSignOff_ReturnsError(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Deburr")
	require.NoError(t, err)
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))
	require.NoError(t, op.Complete("00000000-0000-0000-0000-000000000010"))
	// op is Completed, not PendingSignOff

	err = op.SignOff("00000000-0000-0000-0000-000000000020")
	assert.ErrorIs(t, err, domain.ErrNotPendingSignOff)
}

// ── AttachInstructions ────────────────────────────────────────────────────────

func TestOperation_AttachInstructions(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Inspect surface")
	require.NoError(t, err)

	op.AttachInstructions("minio://instructions/weld-sop-v3.pdf")
	assert.Equal(t, "minio://instructions/weld-sop-v3.pdf", op.InstructionsURL)
}

// ── FAI (First Article Inspection) ───────────────────────────────────────────

func TestOrder_ApproveFAI_Success(t *testing.T) {
	o, err := domain.NewOrder("OF-001", "00000000-0000-0000-0000-000000000001", 1)
	require.NoError(t, err)
	o.IsFAI = true

	managerID := "00000000-0000-0000-0000-000000000030"
	err = o.ApproveFAI(managerID)
	require.NoError(t, err)
	assert.Equal(t, managerID, o.FAIApprovedBy)
	assert.NotNil(t, o.FAIApprovedAt)
}

func TestOrder_ApproveFAI_NotFAIOrder_ReturnsError(t *testing.T) {
	o, err := domain.NewOrder("OF-002", "00000000-0000-0000-0000-000000000001", 5)
	require.NoError(t, err)
	// IsFAI is false by default

	err = o.ApproveFAI("00000000-0000-0000-0000-000000000030")
	assert.ErrorIs(t, err, domain.ErrNotFAIOrder)
}

func TestOrder_ApproveFAI_AlreadyApproved_ReturnsError(t *testing.T) {
	o, err := domain.NewOrder("OF-003", "00000000-0000-0000-0000-000000000001", 1)
	require.NoError(t, err)
	o.IsFAI = true
	require.NoError(t, o.ApproveFAI("00000000-0000-0000-0000-000000000030"))

	err = o.ApproveFAI("00000000-0000-0000-0000-000000000031")
	assert.ErrorIs(t, err, domain.ErrFAIAlreadyApproved)
}

func TestOrder_ApproveFAI_EmptyApprover_ReturnsUnauthorized(t *testing.T) {
	o, err := domain.NewOrder("OF-004", "00000000-0000-0000-0000-000000000001", 1)
	require.NoError(t, err)
	o.IsFAI = true

	err = o.ApproveFAI("")
	assert.ErrorIs(t, err, domain.ErrUnauthorizedRole)
}
