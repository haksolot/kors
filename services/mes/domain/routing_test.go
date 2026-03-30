package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Routing creation ──────────────────────────────────────────────────────────

func TestNewRouting_Success(t *testing.T) {
	r, err := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Airframe Assembly v1", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, r.ID)
	assert.Equal(t, "Airframe Assembly v1", r.Name)
	assert.Equal(t, 1, r.Version)
	assert.False(t, r.IsActive)
}

func TestNewRouting_InvalidArgs(t *testing.T) {
	_, err := domain.NewRouting("", "name", 1)
	assert.ErrorIs(t, err, domain.ErrInvalidProductID)

	_, err = domain.NewRouting("prod-id", "", 1)
	assert.ErrorIs(t, err, domain.ErrInvalidRoutingName)

	_, err = domain.NewRouting("prod-id", "name", 0)
	assert.ErrorIs(t, err, domain.ErrInvalidRoutingVersion)
}

// ── AddStep ───────────────────────────────────────────────────────────────────

func TestRouting_AddStep_Success(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Test routing", 1)
	step, err := r.AddStep(1, "Découpe laser", 300)
	require.NoError(t, err)
	assert.Equal(t, 1, step.StepNumber)
	assert.Equal(t, 300, step.PlannedDurationSeconds)
	assert.Len(t, r.Steps, 1)
}

func TestRouting_AddStep_InvalidArgs(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Test", 1)

	_, err := r.AddStep(0, "step", 0)
	assert.ErrorIs(t, err, domain.ErrInvalidStepNumber)

	_, err = r.AddStep(1, "", 0)
	assert.ErrorIs(t, err, domain.ErrInvalidOperationName)

	_, err = r.AddStep(1, "step", -1)
	assert.ErrorIs(t, err, domain.ErrInvalidPlannedDuration)
}

// ── Activate ─────────────────────────────────────────────────────────────────

func TestRouting_Activate_Success(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Test", 1)
	_, _ = r.AddStep(1, "Step A", 60)
	err := r.Activate()
	require.NoError(t, err)
	assert.True(t, r.IsActive)
}

func TestRouting_Activate_NoSteps_Error(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Test", 1)
	err := r.Activate()
	assert.ErrorIs(t, err, domain.ErrRoutingHasNoSteps)
}

// ── InstantiateOperations ─────────────────────────────────────────────────────

func TestRouting_InstantiateOperations_CreatesOperations(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Frame assembly", 1)
	step1, _ := r.AddStep(1, "Cut", 120)
	step1.RequiredSkill = "laser_operator"
	step2, _ := r.AddStep(2, "Weld", 300)
	step2.RequiresSignOff = true
	_ = r.Activate()

	ofID := "00000000-0000-0000-0000-000000000002"
	ops, err := r.InstantiateOperations(ofID)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, ofID, ops[0].OFID)
	assert.Equal(t, 1, ops[0].StepNumber)
	assert.Equal(t, "Cut", ops[0].Name)
	assert.Equal(t, 120, ops[0].PlannedDurationSeconds)
	assert.Equal(t, "laser_operator", ops[0].RequiredSkill)
	assert.False(t, ops[0].RequiresSignOff)

	assert.Equal(t, 2, ops[1].StepNumber)
	assert.True(t, ops[1].RequiresSignOff)
	assert.Equal(t, domain.OperationStatusPending, ops[1].Status)
}

func TestRouting_InstantiateOperations_NotActive_Error(t *testing.T) {
	r, _ := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Test", 1)
	_, _ = r.AddStep(1, "Step", 0)
	// do NOT activate

	_, err := r.InstantiateOperations("00000000-0000-0000-0000-000000000002")
	assert.ErrorIs(t, err, domain.ErrRoutingNotActive)
}

// ── Skill qualification ───────────────────────────────────────────────────────

func TestOperation_Start_WithRequiredSkill_Passes(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Weld")
	require.NoError(t, err)
	op.RequiredSkill = "welder_certified"

	err = op.Start("00000000-0000-0000-0000-000000000010", []string{"assembler", "welder_certified"}, nil)
	require.NoError(t, err)
}

func TestOperation_Start_WithRequiredSkill_Missing_Error(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Weld")
	require.NoError(t, err)
	op.RequiredSkill = "welder_certified"

	err = op.Start("00000000-0000-0000-0000-000000000010", []string{"assembler"}, nil)
	assert.ErrorIs(t, err, domain.ErrOperatorNotQualified)
}

func TestOperation_Start_NoRequiredSkill_Passes(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Inspect")
	require.NoError(t, err)
	// no RequiredSkill set

	err = op.Start("00000000-0000-0000-0000-000000000010", nil, nil)
	require.NoError(t, err)
}

// ── Cycle time ────────────────────────────────────────────────────────────────

func TestOperation_Complete_ComputesActualDuration(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Deburr")
	require.NoError(t, err)
	op.PlannedDurationSeconds = 60
	require.NoError(t, op.Start("00000000-0000-0000-0000-000000000010", nil, nil))

	require.NoError(t, op.Complete("00000000-0000-0000-0000-000000000010"))
	assert.GreaterOrEqual(t, op.ActualDurationSeconds, 0)
	assert.NotNil(t, op.CompletedAt)
}

// ── Order planning ────────────────────────────────────────────────────────────

func TestOrder_SetPlanning_Success(t *testing.T) {
	o, err := domain.NewOrder("OF-P-001", "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, err)

	err = o.SetPlanning(nil, 80)
	require.NoError(t, err)
	assert.Equal(t, 80, o.Priority)
	assert.Nil(t, o.DueDate)
}

func TestOrder_SetPlanning_InvalidPriority(t *testing.T) {
	o, _ := domain.NewOrder("OF-P-002", "00000000-0000-0000-0000-000000000001", 1)
	assert.ErrorIs(t, o.SetPlanning(nil, 0), domain.ErrInvalidPriority)
	assert.ErrorIs(t, o.SetPlanning(nil, 101), domain.ErrInvalidPriority)
}

func TestNewOrder_DefaultPriority(t *testing.T) {
	o, err := domain.NewOrder("OF-P-003", "00000000-0000-0000-0000-000000000001", 1)
	require.NoError(t, err)
	assert.Equal(t, 50, o.Priority)
}
