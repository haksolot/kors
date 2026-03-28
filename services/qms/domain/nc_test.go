package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/qms/domain"
)

// ── NewNC ─────────────────────────────────────────────────────────────────────

func TestNewNC_Success(t *testing.T) {
	nc, err := domain.NewNC(
		"op-001", "of-001", "SCRATCH", "Surface scratch on panel", 1,
		[]string{"SN-001"}, "operator-001",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, nc.ID)
	assert.Equal(t, domain.NCStatusOpen, nc.Status)
	assert.Equal(t, "SCRATCH", nc.DefectCode)
	assert.Equal(t, 1, nc.AffectedQuantity)
	assert.Len(t, nc.SerialNumbers, 1)
}

func TestNewNC_Validations(t *testing.T) {
	_, err := domain.NewNC("", "of-001", "code", "desc", 1, nil, "op")
	assert.ErrorIs(t, err, domain.ErrInvalidOperationID)

	_, err = domain.NewNC("op-001", "", "code", "desc", 1, nil, "op")
	assert.ErrorIs(t, err, domain.ErrInvalidOFID)

	_, err = domain.NewNC("op-001", "of-001", "", "desc", 1, nil, "op")
	assert.ErrorIs(t, err, domain.ErrInvalidDefectCode)

	_, err = domain.NewNC("op-001", "of-001", "code", "desc", 0, nil, "op")
	assert.ErrorIs(t, err, domain.ErrInvalidAffectedQuantity)

	_, err = domain.NewNC("op-001", "of-001", "code", "desc", 1, nil, "")
	assert.ErrorIs(t, err, domain.ErrInvalidDeclaredBy)
}

// ── StartAnalysis ─────────────────────────────────────────────────────────────

func TestNC_StartAnalysis_Success(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	err := nc.StartAnalysis("analyst-001")
	require.NoError(t, err)
	assert.Equal(t, domain.NCStatusUnderAnalysis, nc.Status)
}

func TestNC_StartAnalysis_NotOpen_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	_ = nc.StartAnalysis("analyst-001")
	err := nc.StartAnalysis("analyst-001")
	assert.ErrorIs(t, err, domain.ErrNCInvalidTransition)
}

func TestNC_StartAnalysis_EmptyAnalyst_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	err := nc.StartAnalysis("")
	assert.ErrorIs(t, err, domain.ErrUnauthorizedActor)
}

// ── ProposeDisposition ────────────────────────────────────────────────────────

func TestNC_ProposeDisposition_Success(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	_ = nc.StartAnalysis("analyst-001")
	err := nc.ProposeDisposition(domain.NCDispositionRework, "analyst-001")
	require.NoError(t, err)
	assert.Equal(t, domain.NCStatusPendingDisposition, nc.Status)
	assert.Equal(t, domain.NCDispositionRework, nc.Disposition)
}

func TestNC_ProposeDisposition_WrongState_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	// Still OPEN, not UNDER_ANALYSIS
	err := nc.ProposeDisposition(domain.NCDispositionScrap, "analyst-001")
	assert.ErrorIs(t, err, domain.ErrNCInvalidTransition)
}

func TestNC_ProposeDisposition_UnspecifiedDisposition_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	_ = nc.StartAnalysis("analyst-001")
	err := nc.ProposeDisposition(domain.NCDispositionUnspecified, "analyst-001")
	assert.ErrorIs(t, err, domain.ErrInvalidDisposition)
}

// ── Close ─────────────────────────────────────────────────────────────────────

func TestNC_Close_Success(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	_ = nc.StartAnalysis("analyst-001")
	_ = nc.ProposeDisposition(domain.NCDispositionUseAsIs, "analyst-001")
	err := nc.Close("manager-001")
	require.NoError(t, err)
	assert.Equal(t, domain.NCStatusClosed, nc.Status)
	assert.NotNil(t, nc.ClosedAt)
	assert.Equal(t, "manager-001", nc.ClosedBy)
}

func TestNC_Close_NotPendingDisposition_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	err := nc.Close("manager-001")
	assert.ErrorIs(t, err, domain.ErrNCInvalidTransition)
}

func TestNC_Close_EmptyClosedBy_Error(t *testing.T) {
	nc, _ := domain.NewNC("op-001", "of-001", "SCRATCH", "desc", 1, nil, "op")
	_ = nc.StartAnalysis("analyst-001")
	_ = nc.ProposeDisposition(domain.NCDispositionRework, "analyst-001")
	err := nc.Close("")
	assert.ErrorIs(t, err, domain.ErrUnauthorizedActor)
}
