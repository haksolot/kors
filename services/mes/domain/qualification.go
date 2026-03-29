package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// QualificationStatus is the computed validity state of a Qualification.
// It is derived at runtime from ExpiresAt and IsRevoked — never stored in the database.
type QualificationStatus string

const (
	QualificationStatusActive   QualificationStatus = "active"
	QualificationStatusExpiring QualificationStatus = "expiring" // valid but within the alert window
	QualificationStatusExpired  QualificationStatus = "expired"
	QualificationStatusRevoked  QualificationStatus = "revoked"
)

// DefaultExpiryWarningDays is used by IsExpiringSoon when no explicit window is provided.
const DefaultExpiryWarningDays = 30

// Qualification represents a single habilitation held by one operator for one skill (AS9100D §7.2).
// All state changes go through methods — never mutate fields directly.
type Qualification struct {
	ID             string
	OperatorID     string // UUID — extracted from JWT by the BFF, never from client payload
	Skill          string // must match Operation.RequiredSkill, e.g. "soudure_tig"
	Label          string // human-readable label, e.g. "Soudure TIG niveau 2"
	IssuedAt       time.Time
	ExpiresAt      time.Time
	GrantedBy      string     // UUID of the manager/quality user who granted this qualification
	CertificateURL string     // optional MinIO URL for the certificate document
	IsRevoked      bool
	RevokedBy      string     // UUID of the manager who revoked this qualification
	RevokedAt      *time.Time // nil until Revoke() is called
	RevokeReason   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewQualification creates a new Qualification and validates all required fields.
// All times must be UTC. grantedBy must be the UUID of the approving manager (from JWT).
func NewQualification(operatorID, skill, label string, issuedAt, expiresAt time.Time, grantedBy string) (*Qualification, error) {
	if operatorID == "" {
		return nil, ErrInvalidProductID // operatorID is a required FK
	}
	if skill == "" {
		return nil, ErrInvalidQualificationSkill
	}
	if label == "" {
		return nil, ErrInvalidQualificationLabel
	}
	if !expiresAt.After(issuedAt) {
		return nil, fmt.Errorf("NewQualification: expires_at=%v issued_at=%v: %w", expiresAt, issuedAt, ErrInvalidQualificationExpiry)
	}
	if grantedBy == "" {
		return nil, ErrUnauthorizedRole
	}

	now := time.Now().UTC()
	return &Qualification{
		ID:         uuid.NewString(),
		OperatorID: operatorID,
		Skill:      skill,
		Label:      label,
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
		GrantedBy:  grantedBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// IsValid reports whether this qualification is currently valid at time now.
// A qualification is valid if it is not revoked and has not yet expired.
func (q *Qualification) IsValid(now time.Time) bool {
	return !q.IsRevoked && q.ExpiresAt.After(now)
}

// IsExpiringSoon reports whether the qualification will expire within warningDays of now.
// Returns false if the qualification is already expired or revoked.
func (q *Qualification) IsExpiringSoon(now time.Time, warningDays int) bool {
	if !q.IsValid(now) {
		return false
	}
	deadline := now.AddDate(0, 0, warningDays)
	return !q.ExpiresAt.After(deadline)
}

// Status computes the current qualification status without mutating the struct.
// now is injected to keep this a pure function (testable without real time).
func (q *Qualification) Status(now time.Time) QualificationStatus {
	if q.IsRevoked {
		return QualificationStatusRevoked
	}
	if !q.ExpiresAt.After(now) {
		return QualificationStatusExpired
	}
	if q.IsExpiringSoon(now, DefaultExpiryWarningDays) {
		return QualificationStatusExpiring
	}
	return QualificationStatusActive
}

// Renew extends the expiry date of this qualification.
// newExpiresAt must be strictly after the current ExpiresAt.
// renewedBy must be the UUID of the authorising manager (from JWT).
func (q *Qualification) Renew(newExpiresAt time.Time, renewedBy string) error {
	if renewedBy == "" {
		return ErrUnauthorizedRole
	}
	if q.IsRevoked {
		return ErrQualificationRevoked
	}
	if !newExpiresAt.After(q.ExpiresAt) {
		return fmt.Errorf("Renew: new_expires_at=%v must be after current expires_at=%v: %w",
			newExpiresAt, q.ExpiresAt, ErrInvalidQualificationExpiry)
	}
	q.ExpiresAt = newExpiresAt
	q.UpdatedAt = time.Now().UTC()
	return nil
}

// Revoke marks this qualification as revoked with a mandatory reason.
// revokedBy must be the UUID of the authorising manager (from JWT).
func (q *Qualification) Revoke(revokedBy, reason string) error {
	if revokedBy == "" {
		return ErrUnauthorizedRole
	}
	if q.IsRevoked {
		return ErrQualificationAlreadyRevoked
	}
	now := time.Now().UTC()
	q.IsRevoked = true
	q.RevokedBy = revokedBy
	q.RevokedAt = &now
	q.RevokeReason = reason
	q.UpdatedAt = now
	return nil
}

// AttachCertificate sets the MinIO URL for the qualification certificate document.
func (q *Qualification) AttachCertificate(url string) {
	q.CertificateURL = url
	q.UpdatedAt = time.Now().UTC()
}
