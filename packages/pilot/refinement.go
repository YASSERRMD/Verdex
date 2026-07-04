package pilot

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// RefinementRecord is a tenant-scoped record of a concrete change made
// in response to a PilotFinding (task 7): what was changed, which
// finding it addresses, and whether it has been verified to actually
// fix the underlying issue. A RefinementRecord may only reference a
// PilotFinding that has already reached FindingStatus.IsAtLeastTriaged
// -- see Engine.RecordRefinement, which enforces this before the
// record is ever persisted.
type RefinementRecord struct {
	// ID uniquely identifies this refinement record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this refinement record belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// FindingID references the PilotFinding this refinement addresses.
	FindingID uuid.UUID `json:"finding_id"`

	// Description explains what was changed (e.g. "tightened the
	// grounding threshold for commercial-contract issues", "added a
	// jurisdiction-specific citation-format rule").
	Description string `json:"description"`

	// VerifiedFixed reports whether the change has been confirmed to
	// resolve the underlying PilotFinding. False until
	// Engine.VerifyRefinement is called.
	VerifiedFixed bool `json:"verified_fixed"`

	// VerificationNote explains the basis for VerifiedFixed (e.g. "the
	// same case type re-run against three fresh pilot cases with no
	// recurrence"). Required once VerifiedFixed is true.
	VerificationNote string `json:"verification_note,omitempty"`

	// AppliedBy is the identity.User who applied this refinement.
	AppliedBy uuid.UUID `json:"applied_by"`

	// AppliedAt is when this refinement was applied.
	AppliedAt time.Time `json:"applied_at"`

	// VerifiedBy is the identity.User who verified this refinement. Nil
	// until verification occurs.
	VerifiedBy *uuid.UUID `json:"verified_by,omitempty"`

	// VerifiedAt is when VerifiedFixed was set true. Nil until then.
	VerifiedAt *time.Time `json:"verified_at,omitempty"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks r for structural well-formedness. Validate alone
// cannot enforce the "referenced finding must already be triaged"
// rule (that requires a repository lookup) -- see
// Engine.RecordRefinement for the full precondition check.
func (r *RefinementRecord) Validate() error {
	if r == nil {
		return ErrInvalidRefinement
	}
	if r.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if r.FindingID == uuid.Nil {
		return wrapf("RefinementRecord.Validate", ErrInvalidRefinement)
	}
	if strings.TrimSpace(r.Description) == "" {
		return wrapf("RefinementRecord.Validate", ErrInvalidRefinement)
	}
	if r.AppliedBy == uuid.Nil {
		return wrapf("RefinementRecord.Validate", ErrInvalidRefinement)
	}
	if r.AppliedAt.IsZero() {
		return wrapf("RefinementRecord.Validate", ErrInvalidRefinement)
	}
	if r.VerifiedFixed && strings.TrimSpace(r.VerificationNote) == "" {
		return wrapf("RefinementRecord.Validate", ErrInvalidRefinement)
	}
	return nil
}
