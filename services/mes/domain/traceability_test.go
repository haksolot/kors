package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

// ── Lot ───────────────────────────────────────────────────────────────────────

func TestNewLot_Valid(t *testing.T) {
	lot, err := domain.NewLot("LOT-001", "00000000-0000-0000-0000-000000000001", 100)
	require.NoError(t, err)
	assert.NotEmpty(t, lot.ID)
	assert.Equal(t, "LOT-001", lot.Reference)
	assert.Equal(t, 100, lot.Quantity)
	assert.Empty(t, lot.MaterialCertURL)
	assert.False(t, lot.ReceivedAt.IsZero())
}

func TestNewLot_Validations(t *testing.T) {
	_, err := domain.NewLot("", "prod-id", 10)
	require.ErrorIs(t, err, domain.ErrInvalidLotReference)

	_, err = domain.NewLot("LOT-001", "", 10)
	require.ErrorIs(t, err, domain.ErrInvalidProductID)

	_, err = domain.NewLot("LOT-001", "prod-id", 0)
	require.ErrorIs(t, err, domain.ErrInvalidLotQuantity)

	_, err = domain.NewLot("LOT-001", "prod-id", -5)
	require.ErrorIs(t, err, domain.ErrInvalidLotQuantity)
}

func TestLot_AttachCertificate(t *testing.T) {
	lot, _ := domain.NewLot("LOT-001", "prod-id", 10)
	lot.AttachCertificate("s3://certs/lot-001.pdf")
	assert.Equal(t, "s3://certs/lot-001.pdf", lot.MaterialCertURL)
}

// ── SerialNumber ──────────────────────────────────────────────────────────────

func TestNewSerialNumber_Valid(t *testing.T) {
	sn, err := domain.NewSerialNumber(
		"SN-0042",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000003",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, sn.ID)
	assert.Equal(t, "SN-0042", sn.SN)
	assert.Equal(t, domain.SNStatusProduced, sn.Status)
}

func TestNewSerialNumber_Validations(t *testing.T) {
	_, err := domain.NewSerialNumber("", "lot", "prod", "of")
	require.ErrorIs(t, err, domain.ErrInvalidSerialNumber)

	_, err = domain.NewSerialNumber("SN-1", "lot", "", "of")
	require.ErrorIs(t, err, domain.ErrInvalidProductID)

	_, err = domain.NewSerialNumber("SN-1", "lot", "prod", "")
	require.ErrorIs(t, err, domain.ErrInvalidProductID)
}

func TestSerialNumber_Release(t *testing.T) {
	sn, _ := domain.NewSerialNumber("SN-1", "lot", "prod", "of")
	require.NoError(t, sn.Release())
	assert.Equal(t, domain.SNStatusReleased, sn.Status)

	// idempotency guard
	err := sn.Release()
	require.ErrorIs(t, err, domain.ErrSNAlreadyReleased)
}

func TestSerialNumber_Release_FromScrapped(t *testing.T) {
	sn, _ := domain.NewSerialNumber("SN-1", "lot", "prod", "of")
	require.NoError(t, sn.Scrap())
	err := sn.Release()
	require.ErrorIs(t, err, domain.ErrSNInvalidTransition)
}

func TestSerialNumber_Scrap_FromProduced(t *testing.T) {
	sn, _ := domain.NewSerialNumber("SN-1", "lot", "prod", "of")
	require.NoError(t, sn.Scrap())
	assert.Equal(t, domain.SNStatusScrapped, sn.Status)
}

func TestSerialNumber_Scrap_FromReleased(t *testing.T) {
	sn, _ := domain.NewSerialNumber("SN-1", "lot", "prod", "of")
	require.NoError(t, sn.Release())
	require.NoError(t, sn.Scrap())
	assert.Equal(t, domain.SNStatusScrapped, sn.Status)
}

func TestSerialNumber_Scrap_AlreadyScrapped(t *testing.T) {
	sn, _ := domain.NewSerialNumber("SN-1", "lot", "prod", "of")
	require.NoError(t, sn.Scrap())
	err := sn.Scrap()
	require.ErrorIs(t, err, domain.ErrSNAlreadyScrapped)
}

// ── GenealogyEntry ────────────────────────────────────────────────────────────

func TestNewGenealogyEntry_Valid(t *testing.T) {
	entry, err := domain.NewGenealogyEntry("parent-id", "child-id", "of-id", "op-id")
	require.NoError(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "parent-id", entry.ParentSNID)
	assert.Equal(t, "child-id", entry.ChildSNID)
	assert.False(t, entry.RecordedAt.IsZero())
}

func TestNewGenealogyEntry_Validations(t *testing.T) {
	_, err := domain.NewGenealogyEntry("", "child", "of", "op")
	require.ErrorIs(t, err, domain.ErrInvalidProductID)

	_, err = domain.NewGenealogyEntry("parent", "", "of", "op")
	require.ErrorIs(t, err, domain.ErrInvalidProductID)

	_, err = domain.NewGenealogyEntry("same", "same", "of", "op")
	require.ErrorIs(t, err, domain.ErrSNInvalidTransition)

	_, err = domain.NewGenealogyEntry("parent", "child", "", "op")
	require.ErrorIs(t, err, domain.ErrInvalidProductID)
}

func TestNewGenealogyEntry_OperationIDOptional(t *testing.T) {
	// operationID is optional (some assembly steps don't map to a specific operation)
	entry, err := domain.NewGenealogyEntry("parent-id", "child-id", "of-id", "")
	require.NoError(t, err)
	assert.Empty(t, entry.OperationID)
}
