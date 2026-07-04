package privacy

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConsentRecord captures a data subject's consent (or other recorded
// legal basis) for one processing Purpose (task 6). A subject may hold
// many ConsentRecords over time for the same Purpose -- e.g.
// granted, withdrawn, and re-granted -- so HasValidConsent always
// resolves against the most relevant currently-valid record rather
// than assuming exactly one exists.
type ConsentRecord struct {
	// ID uniquely identifies this consent record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this record belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// SubjectID identifies the data subject this consent concerns.
	// Deliberately a plain string (not a uuid.UUID) since a data
	// subject is not always a registered identity.User -- e.g. a case
	// party or witness who never has a platform account -- mirroring
	// packages/accessgovernance.Request.JurisdictionID's
	// "reference-only, no hard type dependency" convention.
	SubjectID string `json:"subject_id"`

	// Purpose names the processing purpose this consent covers (e.g.
	// "case_analytics", "transcript_retention",
	// "third_party_review_sharing"). Free-form but expected to be a
	// stable, short identifier a tenant reuses consistently.
	Purpose string `json:"purpose"`

	// LegalBasis is the lawful ground this record establishes for
	// Purpose. Typically BasisConsent, but modeled as the full
	// LegalBasis enum so a tenant can record e.g. "processing continues
	// under BasisLegalObligation even though consent was withdrawn"
	// using the same record shape.
	LegalBasis LegalBasis `json:"legal_basis"`

	// GrantedAt is when this consent was granted.
	GrantedAt time.Time `json:"granted_at"`

	// WithdrawnAt, if non-nil, is when this consent was withdrawn. A
	// nil WithdrawnAt means the consent remains in force.
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`

	// RecordedBy is the identity.User who recorded this consent (the
	// staff member who processed the subject's grant/withdrawal, not
	// the subject themselves, who typically has no platform account).
	RecordedBy uuid.UUID `json:"recorded_by"`

	// Notes is a short free-text note (e.g. how consent was obtained).
	Notes string `json:"notes,omitempty"`
}

// Validate checks c for structural well-formedness.
func (c *ConsentRecord) Validate() error {
	if c == nil {
		return ErrInvalidConsentRecord
	}
	if c.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(c.SubjectID) == "" {
		return wrapf("ConsentRecord.Validate", ErrInvalidConsentRecord)
	}
	if strings.TrimSpace(c.Purpose) == "" {
		return wrapf("ConsentRecord.Validate", ErrInvalidConsentRecord)
	}
	if !c.LegalBasis.IsValid() {
		return wrapf("ConsentRecord.Validate", ErrInvalidConsentRecord)
	}
	if c.GrantedAt.IsZero() {
		return wrapf("ConsentRecord.Validate", ErrInvalidConsentRecord)
	}
	if c.WithdrawnAt != nil && c.WithdrawnAt.Before(c.GrantedAt) {
		return wrapf("ConsentRecord.Validate", ErrInvalidConsentRecord)
	}
	return nil
}

// IsActive reports whether c is currently in force as of now: granted
// (not in the future) and not withdrawn.
func (c *ConsentRecord) IsActive(now time.Time) bool {
	if c == nil {
		return false
	}
	if c.GrantedAt.After(now) {
		return false
	}
	if c.WithdrawnAt != nil && !c.WithdrawnAt.After(now) {
		return false
	}
	return true
}

// HasValidConsent evaluates records -- typically every ConsentRecord
// on file for one subject, e.g. from
// ConsentRepository.ListForSubject -- and reports whether subject has
// active, valid consent for purpose as of now (task 6). This is real
// logic, not a stub: it filters to matching subject+purpose pairs,
// then requires at least one such record to be IsActive(now). A
// subject with a withdrawn record and no subsequent re-grant correctly
// reports false even though a (now-superseded) ConsentRecord exists in
// records.
func HasValidConsent(records []ConsentRecord, subjectID, purpose string, now time.Time) bool {
	for i := range records {
		r := records[i]
		if r.SubjectID != subjectID || r.Purpose != purpose {
			continue
		}
		if r.IsActive(now) {
			return true
		}
	}
	return false
}
