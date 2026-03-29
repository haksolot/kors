package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

// base is a fixed point in time used across all qualification tests.
var base = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// validQual returns a freshly created, valid qualification expiring in one year.
func validQual(t *testing.T) *domain.Qualification {
	t.Helper()
	q, err := domain.NewQualification("op-1", "soudure_tig", "Soudure TIG", base, base.AddDate(1, 0, 0), "mgr-1")
	require.NoError(t, err)
	return q
}

// ── NewQualification ──────────────────────────────────────────────────────────

func TestNewQualification(t *testing.T) {
	tests := []struct {
		name       string
		operatorID string
		skill      string
		label      string
		issuedAt   time.Time
		expiresAt  time.Time
		grantedBy  string
		wantErr    error
	}{
		{
			name:       "valid qualification",
			operatorID: "op-1",
			skill:      "soudure_tig",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base.AddDate(1, 0, 0),
			grantedBy:  "mgr-1",
		},
		{
			name:       "empty operatorID returns error",
			operatorID: "",
			skill:      "soudure_tig",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base.AddDate(1, 0, 0),
			grantedBy:  "mgr-1",
			wantErr:    domain.ErrInvalidProductID,
		},
		{
			name:       "empty skill returns error",
			operatorID: "op-1",
			skill:      "",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base.AddDate(1, 0, 0),
			grantedBy:  "mgr-1",
			wantErr:    domain.ErrInvalidQualificationSkill,
		},
		{
			name:       "empty label returns error",
			operatorID: "op-1",
			skill:      "soudure_tig",
			label:      "",
			issuedAt:   base,
			expiresAt:  base.AddDate(1, 0, 0),
			grantedBy:  "mgr-1",
			wantErr:    domain.ErrInvalidQualificationLabel,
		},
		{
			name:       "expiresAt before issuedAt returns error",
			operatorID: "op-1",
			skill:      "soudure_tig",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base.AddDate(-1, 0, 0),
			grantedBy:  "mgr-1",
			wantErr:    domain.ErrInvalidQualificationExpiry,
		},
		{
			name:       "expiresAt equal to issuedAt returns error",
			operatorID: "op-1",
			skill:      "soudure_tig",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base,
			grantedBy:  "mgr-1",
			wantErr:    domain.ErrInvalidQualificationExpiry,
		},
		{
			name:       "empty grantedBy returns unauthorized error",
			operatorID: "op-1",
			skill:      "soudure_tig",
			label:      "Soudure TIG",
			issuedAt:   base,
			expiresAt:  base.AddDate(1, 0, 0),
			grantedBy:  "",
			wantErr:    domain.ErrUnauthorizedRole,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := domain.NewQualification(tc.operatorID, tc.skill, tc.label, tc.issuedAt, tc.expiresAt, tc.grantedBy)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, q)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, q)
			assert.NotEmpty(t, q.ID)
			assert.Equal(t, tc.operatorID, q.OperatorID)
			assert.Equal(t, tc.skill, q.Skill)
			assert.Equal(t, tc.label, q.Label)
			assert.False(t, q.IsRevoked)
			assert.Nil(t, q.RevokedAt)
		})
	}
}

// ── IsValid ───────────────────────────────────────────────────────────────────

func TestQualification_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) *domain.Qualification
		now       time.Time
		wantValid bool
	}{
		{
			name:      "active qualification is valid",
			setup:     validQual,
			now:       base,
			wantValid: true,
		},
		{
			name: "qualification expired one second ago is not valid",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.Add(time.Hour), "mgr-1")
				return q
			},
			now:       base.Add(time.Hour + time.Second), // one second after expiry
			wantValid: false,
		},
		{
			name: "qualification expiring exactly at now is not valid (After is strict)",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.Add(time.Hour), "mgr-1")
				return q
			},
			now:       base.Add(time.Hour),
			wantValid: false,
		},
		{
			name: "revoked qualification is not valid",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q := validQual(t)
				require.NoError(t, q.Revoke("mgr-1", "reason"))
				return q
			},
			now:       base,
			wantValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.setup(t)
			assert.Equal(t, tc.wantValid, q.IsValid(tc.now))
		})
	}
}

// ── IsExpiringSoon ────────────────────────────────────────────────────────────

func TestQualification_IsExpiringSoon(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *domain.Qualification
		now         time.Time
		warningDays int
		want        bool
	}{
		{
			name: "expires in 29 days with 30-day window — is expiring soon",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 29), "mgr-1")
				return q
			},
			now:         base,
			warningDays: 30,
			want:        true,
		},
		{
			name: "expires in exactly 30 days with 30-day window — is expiring soon (boundary)",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 30), "mgr-1")
				return q
			},
			now:         base,
			warningDays: 30,
			want:        true,
		},
		{
			name: "expires in 31 days with 30-day window — not expiring soon",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 31), "mgr-1")
				return q
			},
			now:         base,
			warningDays: 30,
			want:        false,
		},
		{
			name: "already expired — not expiring soon",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 1), "mgr-1")
				return q
			},
			now:         base.AddDate(0, 0, 2), // past expiry
			warningDays: 30,
			want:        false,
		},
		{
			name: "revoked — not expiring soon",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q := validQual(t)
				require.NoError(t, q.Revoke("mgr-1", "reason"))
				return q
			},
			now:         base,
			warningDays: 30,
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.setup(t)
			assert.Equal(t, tc.want, q.IsExpiringSoon(tc.now, tc.warningDays))
		})
	}
}

// ── Status ────────────────────────────────────────────────────────────────────

func TestQualification_Status(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) *domain.Qualification
		now        time.Time
		wantStatus domain.QualificationStatus
	}{
		{
			name:       "active, far from expiry → Active",
			setup:      validQual, // expires in 1 year
			now:        base,
			wantStatus: domain.QualificationStatusActive,
		},
		{
			name: "expires in 15 days → Expiring",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 15), "mgr-1")
				return q
			},
			now:        base,
			wantStatus: domain.QualificationStatusExpiring,
		},
		{
			name: "expired yesterday → Expired",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 1), "mgr-1")
				return q
			},
			now:        base.AddDate(0, 0, 2),
			wantStatus: domain.QualificationStatusExpired,
		},
		{
			name: "revoked → Revoked",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q := validQual(t)
				require.NoError(t, q.Revoke("mgr-1", "reason"))
				return q
			},
			now:        base,
			wantStatus: domain.QualificationStatusRevoked,
		},
		{
			name: "revoked AND expired — Revoked takes precedence",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q, _ := domain.NewQualification("op-1", "skill", "Label", base, base.AddDate(0, 0, 1), "mgr-1")
				require.NoError(t, q.Revoke("mgr-1", "reason"))
				return q
			},
			now:        base.AddDate(0, 0, 2), // past expiry
			wantStatus: domain.QualificationStatusRevoked,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.setup(t)
			assert.Equal(t, tc.wantStatus, q.Status(tc.now))
		})
	}
}

// ── Renew ─────────────────────────────────────────────────────────────────────

func TestQualification_Renew(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) *domain.Qualification
		newExpiresAt time.Time
		renewedBy    string
		wantErr      error
	}{
		{
			name:         "valid renewal extends expiry",
			setup:        validQual, // expires at base+1y
			newExpiresAt: base.AddDate(2, 0, 0),
			renewedBy:    "mgr-1",
		},
		{
			name: "cannot renew revoked qualification",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q := validQual(t)
				require.NoError(t, q.Revoke("mgr-1", "reason"))
				return q
			},
			newExpiresAt: base.AddDate(2, 0, 0),
			renewedBy:    "mgr-1",
			wantErr:      domain.ErrQualificationRevoked,
		},
		{
			name:         "new expiry not after current expiry returns error",
			setup:        validQual, // expires at base+1y
			newExpiresAt: base,      // before current expiry
			renewedBy:    "mgr-1",
			wantErr:      domain.ErrInvalidQualificationExpiry,
		},
		{
			name:         "new expiry equal to current expiry returns error",
			setup:        validQual,
			newExpiresAt: base.AddDate(1, 0, 0), // same as current expiry
			renewedBy:    "mgr-1",
			wantErr:      domain.ErrInvalidQualificationExpiry,
		},
		{
			name:         "empty renewedBy returns unauthorized error",
			setup:        validQual,
			newExpiresAt: base.AddDate(2, 0, 0),
			renewedBy:    "",
			wantErr:      domain.ErrUnauthorizedRole,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.setup(t)
			err := q.Renew(tc.newExpiresAt, tc.renewedBy)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, q.ExpiresAt.Equal(tc.newExpiresAt))
		})
	}
}

// ── Revoke ────────────────────────────────────────────────────────────────────

func TestQualification_Revoke(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) *domain.Qualification
		revokedBy string
		reason    string
		wantErr   error
	}{
		{
			name:      "valid revocation",
			setup:     validQual,
			revokedBy: "mgr-1",
			reason:    "non-conformity detected",
		},
		{
			name:      "empty revokedBy returns unauthorized error",
			setup:     validQual,
			revokedBy: "",
			reason:    "reason",
			wantErr:   domain.ErrUnauthorizedRole,
		},
		{
			name: "cannot revoke already revoked qualification",
			setup: func(t *testing.T) *domain.Qualification {
				t.Helper()
				q := validQual(t)
				require.NoError(t, q.Revoke("mgr-1", "first revocation"))
				return q
			},
			revokedBy: "mgr-2",
			reason:    "duplicate",
			wantErr:   domain.ErrQualificationAlreadyRevoked,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.setup(t)
			err := q.Revoke(tc.revokedBy, tc.reason)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, q.IsRevoked)
			assert.Equal(t, tc.revokedBy, q.RevokedBy)
			assert.NotNil(t, q.RevokedAt)
			assert.Equal(t, tc.reason, q.RevokeReason)
		})
	}
}
