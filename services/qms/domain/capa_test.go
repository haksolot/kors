package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/qms/domain"
)

// ── NewCAPA ───────────────────────────────────────────────────────────────────

func TestNewCAPA_Success(t *testing.T) {
	c, err := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Re-train operators on torque specs", "owner-001", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, domain.CAPAStatusOpen, c.Status)
	assert.Equal(t, domain.CAPAActionCorrective, c.ActionType)
}

func TestNewCAPA_Validations(t *testing.T) {
	_, err := domain.NewCAPA("", domain.CAPAActionCorrective, "desc", "owner", nil)
	assert.ErrorIs(t, err, domain.ErrInvalidNCID)

	_, err = domain.NewCAPA("nc-001", domain.CAPAActionUnspecified, "desc", "owner", nil)
	assert.ErrorIs(t, err, domain.ErrInvalidCAPAActionType)

	_, err = domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "", "owner", nil)
	assert.ErrorIs(t, err, domain.ErrInvalidCAPADescription)

	_, err = domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "desc", "", nil)
	assert.ErrorIs(t, err, domain.ErrInvalidCAPAOwner)
}

// ── Start ─────────────────────────────────────────────────────────────────────

func TestCAPA_Start_Success(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionPreventive, "Inspect all batches", "owner-001", nil)
	err := c.Start()
	require.NoError(t, err)
	assert.Equal(t, domain.CAPAStatusInProgress, c.Status)
}

func TestCAPA_Start_AlreadyInProgress_Error(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionPreventive, "Inspect", "owner-001", nil)
	_ = c.Start()
	err := c.Start()
	assert.ErrorIs(t, err, domain.ErrCAPAInvalidTransition)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func TestCAPA_Complete_Success(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Fix tooling", "owner-001", nil)
	_ = c.Start()
	err := c.Complete()
	require.NoError(t, err)
	assert.Equal(t, domain.CAPAStatusCompleted, c.Status)
	assert.NotNil(t, c.CompletedAt)
}

func TestCAPA_Complete_NotInProgress_Error(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Fix tooling", "owner-001", nil)
	err := c.Complete()
	assert.ErrorIs(t, err, domain.ErrCAPAInvalidTransition)
}

// ── Cancel ────────────────────────────────────────────────────────────────────

func TestCAPA_Cancel_FromOpen_Success(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Fix tooling", "owner-001", nil)
	err := c.Cancel()
	require.NoError(t, err)
	assert.Equal(t, domain.CAPAStatusCancelled, c.Status)
}

func TestCAPA_Cancel_FromInProgress_Success(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Fix tooling", "owner-001", nil)
	_ = c.Start()
	err := c.Cancel()
	require.NoError(t, err)
	assert.Equal(t, domain.CAPAStatusCancelled, c.Status)
}

func TestCAPA_Cancel_Completed_Error(t *testing.T) {
	c, _ := domain.NewCAPA("nc-001", domain.CAPAActionCorrective, "Fix tooling", "owner-001", nil)
	_ = c.Start()
	_ = c.Complete()
	err := c.Cancel()
	assert.ErrorIs(t, err, domain.ErrCAPAInvalidTransition)
}
