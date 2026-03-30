package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── NADCAP Special Process interlock (§13 — EN9100) ──────────────────────────

func TestOperation_Start_NADCAPSpecialProcess_Passes(t *testing.T) {
	tests := []struct {
		name          string
		processCode   string
		nadcapSkills  []string
	}{
		{
			name:         "operator holds exact NADCAP code",
			processCode:  "NADCAP-WELD",
			nadcapSkills: []string{"NADCAP-WELD", "NADCAP-NDT"},
		},
		{
			name:         "operator holds only matching code",
			processCode:  "NADCAP-NDT",
			nadcapSkills: []string{"NADCAP-NDT"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Special weld")
			require.NoError(t, err)
			op.IsSpecialProcess = true
			op.NADCAPProcessCode = tc.processCode

			err = op.Start("00000000-0000-0000-0000-000000000010", nil, tc.nadcapSkills)
			require.NoError(t, err)
			assert.Equal(t, domain.OperationStatusInProgress, op.Status)
		})
	}
}

func TestOperation_Start_NADCAPSpecialProcess_Blocked(t *testing.T) {
	tests := []struct {
		name         string
		processCode  string
		nadcapSkills []string
	}{
		{
			name:         "operator has no NADCAP skills",
			processCode:  "NADCAP-WELD",
			nadcapSkills: nil,
		},
		{
			name:         "operator holds different NADCAP code",
			processCode:  "NADCAP-WELD",
			nadcapSkills: []string{"NADCAP-NDT", "NADCAP-COAT"},
		},
		{
			name:         "empty skills list",
			processCode:  "NADCAP-WELD",
			nadcapSkills: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Special weld")
			require.NoError(t, err)
			op.IsSpecialProcess = true
			op.NADCAPProcessCode = tc.processCode

			err = op.Start("00000000-0000-0000-0000-000000000010", nil, tc.nadcapSkills)
			require.ErrorIs(t, err, domain.ErrNADCAPQualificationRequired)
		})
	}
}

func TestOperation_Start_NotSpecialProcess_IgnoresNADCAPSkills(t *testing.T) {
	// A non-special-process operation must not be blocked regardless of nadcapSkills.
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Standard deburr")
	require.NoError(t, err)
	op.IsSpecialProcess = false // default

	err = op.Start("00000000-0000-0000-0000-000000000010", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, domain.OperationStatusInProgress, op.Status)
}

func TestOperation_Start_BothSkillAndNADCAP_BothRequired(t *testing.T) {
	// An operation requiring both a JWT role AND NADCAP must pass both checks.
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Certified NDT inspection")
	require.NoError(t, err)
	op.RequiredSkill = "ndt_inspector"
	op.IsSpecialProcess = true
	op.NADCAPProcessCode = "NADCAP-NDT"

	t.Run("passes when both skill and NADCAP code present", func(t *testing.T) {
		err := op.Start("op-1", []string{"ndt_inspector"}, []string{"NADCAP-NDT"})
		require.NoError(t, err)
	})
}

func TestOperation_Start_BothSkillAndNADCAP_SkillMissing(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Certified NDT inspection")
	require.NoError(t, err)
	op.RequiredSkill = "ndt_inspector"
	op.IsSpecialProcess = true
	op.NADCAPProcessCode = "NADCAP-NDT"

	// Has NADCAP code but not the JWT role.
	err = op.Start("op-1", []string{"assembler"}, []string{"NADCAP-NDT"})
	require.ErrorIs(t, err, domain.ErrOperatorNotQualified)
}

func TestOperation_Start_BothSkillAndNADCAP_NADCAPMissing(t *testing.T) {
	op, err := domain.NewOperation("00000000-0000-0000-0000-000000000001", 1, "Certified NDT inspection")
	require.NoError(t, err)
	op.RequiredSkill = "ndt_inspector"
	op.IsSpecialProcess = true
	op.NADCAPProcessCode = "NADCAP-NDT"

	// Has JWT role but not the NADCAP qualification.
	err = op.Start("op-1", []string{"ndt_inspector"}, []string{"NADCAP-WELD"})
	require.ErrorIs(t, err, domain.ErrNADCAPQualificationRequired)
}

// ── AuditEntry construction ───────────────────────────────────────────────────

func TestNewAuditEntry_Valid(t *testing.T) {
	e, err := domain.NewAuditEntry(
		"00000000-0000-0000-0000-000000000001",
		"kors-admin",
		domain.AuditActionOFCreated,
		domain.AuditEntityOrder,
		"00000000-0000-0000-0000-000000000002",
	)
	require.NoError(t, err)
	require.NotNil(t, e)
	assert.NotEmpty(t, e.ID)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", e.ActorID)
	assert.Equal(t, "kors-admin", e.ActorRole)
	assert.Equal(t, domain.AuditActionOFCreated, e.Action)
	assert.Equal(t, domain.AuditEntityOrder, e.EntityType)
	assert.False(t, e.CreatedAt.IsZero())
}

func TestNewAuditEntry_MissingFields(t *testing.T) {
	tests := []struct {
		name       string
		actorID    string
		actorRole  string
		action     domain.AuditAction
		entityType domain.AuditEntityType
		entityID   string
		wantErr    error
	}{
		{
			name:       "empty actor ID",
			actorID:    "",
			actorRole:  "admin",
			action:     domain.AuditActionOFCreated,
			entityType: domain.AuditEntityOrder,
			entityID:   "id-1",
			wantErr:    domain.ErrInvalidAuditActor,
		},
		{
			name:       "empty action",
			actorID:    "actor-1",
			actorRole:  "admin",
			action:     "",
			entityType: domain.AuditEntityOrder,
			entityID:   "id-1",
			wantErr:    domain.ErrInvalidAuditAction,
		},
		{
			name:       "empty entity type",
			actorID:    "actor-1",
			actorRole:  "admin",
			action:     domain.AuditActionOFCreated,
			entityType: "",
			entityID:   "id-1",
			wantErr:    domain.ErrInvalidAuditEntityType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewAuditEntry(tc.actorID, tc.actorRole, tc.action, tc.entityType, tc.entityID)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// ── Routing.InstantiateOperations propagates NADCAP fields ───────────────────

func TestRouting_InstantiateOperations_PropagatesNADCAP(t *testing.T) {
	rt, err := domain.NewRouting("00000000-0000-0000-0000-000000000001", "Weld Routing", 1)
	require.NoError(t, err)

	step, err := rt.AddStep(1, "NDT Inspection", 600)
	require.NoError(t, err)
	step.IsSpecialProcess = true
	step.NADCAPProcessCode = "NADCAP-NDT"

	_, err = rt.AddStep(2, "Standard deburr", 120)
	require.NoError(t, err)

	require.NoError(t, rt.Activate())

	ops, err := rt.InstantiateOperations("of-uuid-1")
	require.NoError(t, err)
	require.Len(t, ops, 2)

	// First operation should have NADCAP fields propagated.
	assert.True(t, ops[0].IsSpecialProcess)
	assert.Equal(t, "NADCAP-NDT", ops[0].NADCAPProcessCode)

	// Second operation is not a special process.
	assert.False(t, ops[1].IsSpecialProcess)
	assert.Empty(t, ops[1].NADCAPProcessCode)
}
