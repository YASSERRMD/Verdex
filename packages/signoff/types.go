package signoff

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// DecisionSource is a stable verb-phrase identifying why a
// SignoffRecord entry exists, distinguishing an explicit human
// decision from an automatic system-triggered re-review.
type DecisionSource string

// DecisionSource values.
const (
	// DecisionSourceReviewer means a human reviewer explicitly called
	// Approve or Reject.
	DecisionSourceReviewer DecisionSource = "reviewer"

	// DecisionSourceReReview means the system automatically reverted
	// an Approved record to Pending because the case's content changed
	// after approval (see ReReviewOnCaseUpdate in rereview.go).
	DecisionSourceReReview DecisionSource = "re_review"

	// DecisionSourceInitial means the record was created for a case
	// that has never been reviewed (the implicit Pending state a case
	// starts in once it enters the sign-off workflow).
	DecisionSourceInitial DecisionSource = "initial"
)

// String satisfies fmt.Stringer.
func (d DecisionSource) String() string { return string(d) }

// SignoffRecord is the current, mutable sign-off state for a single
// case, plus (via History, see repository.go) the immutable audit
// trail of every decision and re-review trigger that ever changed it.
//
// SignoffRecord.Status uses guardrail.SignoffStatus directly rather
// than duplicating the enum, since this package's entire purpose is
// to be the real backing store behind that status.
type SignoffRecord struct {
	// ID uniquely identifies this sign-off record.
	ID uuid.UUID `json:"id"`

	// CaseID identifies the packages/caselifecycle.Case this record
	// applies to. Exactly one current SignoffRecord exists per case.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is the tenant this record belongs to, copied from the
	// case at creation time.
	TenantID uuid.UUID `json:"tenant_id"`

	// Status is the current sign-off status: Pending, Approved, or
	// Rejected.
	Status guardrail.SignoffStatus `json:"status"`

	// ReviewerID is the identity.User who made the most recent
	// decision. Nil (uuid.Nil) if Status is still Pending and no
	// reviewer has ever acted (DecisionSourceInitial).
	ReviewerID uuid.UUID `json:"reviewer_id,omitempty"`

	// Notes is the reviewer's free-text explanation. Required
	// (non-blank) whenever Status is Rejected; optional when Approved.
	Notes string `json:"notes,omitempty"`

	// CaseVersion is the packages/caselifecycle Case.MetadataVersion
	// that was current at the time of this decision. Approve/Reject
	// require the caller to supply the version they reviewed; a
	// mismatch against the case's live version is rejected
	// (ErrCaseVersionMismatch) so a reviewer can never approve content
	// they did not actually see.
	CaseVersion int `json:"case_version"`

	// Source records why this record's current state exists: an
	// explicit reviewer decision, an automatic re-review reversion, or
	// the initial un-reviewed state.
	Source DecisionSource `json:"source"`

	// DecidedAt is when this decision (or re-review trigger) was
	// recorded.
	DecidedAt time.Time `json:"decided_at"`

	// CreatedAt is when the sign-off record was first created for this
	// case (i.e. when the case first entered the sign-off workflow).
	CreatedAt time.Time `json:"created_at"`
}

// Clone returns a deep copy of r. A nil receiver returns nil.
func (r *SignoffRecord) Clone() *SignoffRecord {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

// AuditEntry is one immutable entry in a case's sign-off audit trail:
// every decision and every automatic re-review trigger, in order.
// Unlike SignoffRecord (which reflects only the current state),
// AuditEntry rows are append-only and are never updated or deleted —
// mirroring packages/caselifecycle.TransitionRecord's role for case
// state transitions.
type AuditEntry struct {
	// ID uniquely identifies this audit entry.
	ID uuid.UUID `json:"id"`

	// CaseID identifies the case this entry applies to.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is the tenant this entry belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// FromStatus is the sign-off status immediately before this entry.
	FromStatus guardrail.SignoffStatus `json:"from_status"`

	// ToStatus is the sign-off status immediately after this entry.
	ToStatus guardrail.SignoffStatus `json:"to_status"`

	// Actor is the identity.User who performed the action. Nil
	// (uuid.Nil) for system-triggered re-review entries.
	Actor uuid.UUID `json:"actor,omitempty"`

	// Source records why this entry exists (see DecisionSource).
	Source DecisionSource `json:"source"`

	// Notes carries the reviewer's notes for a reviewer decision, or a
	// system-generated explanation for a re-review entry (e.g. "case
	// metadata version changed from 3 to 4").
	Notes string `json:"notes,omitempty"`

	// CaseVersion is the case's MetadataVersion at the time of this
	// entry.
	CaseVersion int `json:"case_version"`

	// OccurredAt is when this entry was recorded.
	OccurredAt time.Time `json:"occurred_at"`
}
