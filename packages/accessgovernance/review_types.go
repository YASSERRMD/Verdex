package accessgovernance

import (
	"time"

	"github.com/google/uuid"
)

// GrantKind classifies which underlying grant type a Review covers,
// since both CaseGrant and Grant (JIT elevation) go through the same
// review/attest workflow (task 4) rather than each inventing its own.
type GrantKind string

const (
	// GrantKindCase means Review.SubjectID refers to a CaseGrant.
	GrantKindCase GrantKind = "case_grant"

	// GrantKindElevation means Review.SubjectID refers to a Grant
	// (JIT elevation).
	GrantKindElevation GrantKind = "elevation"
)

// IsValid reports whether k is a recognized GrantKind.
func (k GrantKind) IsValid() bool {
	return k == GrantKindCase || k == GrantKindElevation
}

// AttestationDecision is the closed set of outcomes Attest can record.
type AttestationDecision string

const (
	// AttestationApprove confirms the grant remains appropriate; it
	// stays active until its own natural expiry.
	AttestationApprove AttestationDecision = "approve"

	// AttestationRevoke immediately revokes the underlying grant,
	// regardless of its remaining time-bound window.
	AttestationRevoke AttestationDecision = "revoke"
)

// IsValid reports whether d is a recognized AttestationDecision.
func (d AttestationDecision) IsValid() bool {
	return d == AttestationApprove || d == AttestationRevoke
}

// String satisfies fmt.Stringer.
func (d AttestationDecision) String() string { return string(d) }

// Review is one periodic/manual access-review workflow entry (task 4):
// a grant flagged as due for review, awaiting an Attest call from
// someone other than the original requester (task 5 enforces the
// self-approval restriction).
type Review struct {
	// ID uniquely identifies this review.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this review belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// SubjectKind identifies which grant type SubjectID refers to.
	SubjectKind GrantKind `json:"subject_kind"`

	// SubjectID is the CaseGrant.ID or Grant.ID under review.
	SubjectID uuid.UUID `json:"subject_id"`

	// RequestedBy is the user the underlying grant was issued to or
	// requested by -- copied at review-creation time so Attest can
	// enforce segregation of duties without a second lookup.
	RequestedBy uuid.UUID `json:"requested_by"`

	// DueAt is when this review is due to be attested.
	DueAt time.Time `json:"due_at"`

	// Decision is the recorded outcome, or empty while still pending.
	Decision AttestationDecision `json:"decision,omitempty"`

	// AttestedBy is the identity.User who called Attest, or uuid.Nil
	// while still pending.
	AttestedBy uuid.UUID `json:"attested_by,omitempty"`

	// AttestedAt is when Attest was called, or nil while still
	// pending.
	AttestedAt *time.Time `json:"attested_at,omitempty"`

	// Notes is a free-text explanation attached by the attester.
	Notes string `json:"notes,omitempty"`

	// CreatedAt is when this review entry was created.
	CreatedAt time.Time `json:"created_at"`
}

// IsPending reports whether r has not yet been attested.
func (r *Review) IsPending() bool {
	return r != nil && r.Decision == ""
}

// Validate checks r for structural well-formedness.
func (r *Review) Validate() error {
	if r == nil {
		return wrapf("Review.Validate", ErrReviewNotFound)
	}
	if r.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if !r.SubjectKind.IsValid() {
		return wrapf("Review.Validate", ErrInvalidGrant)
	}
	if r.SubjectID == uuid.Nil {
		return wrapf("Review.Validate", ErrInvalidGrant)
	}
	return nil
}

// ConflictRule is a single segregation-of-duties rule (task 5): a
// named check that rejects an action when a "self-dealing" condition
// holds. Rather than hardcoding each rule, ConflictRule captures the
// two roles being compared (e.g. "requester" vs. "approver") so
// CheckConflict can evaluate an open set of named rules uniformly.
type ConflictRule struct {
	// Name identifies this rule (e.g. "requester_cannot_approve",
	// "approver_not_sole_author").
	Name string `json:"name"`

	// Description is a short human-readable explanation of the rule's
	// intent.
	Description string `json:"description,omitempty"`
}

// Well-known ConflictRule names this package enforces out of the box.
const (
	// RuleRequesterCannotApprove rejects an attestation/approval where
	// the attester is the same user who requested (or was granted)
	// the access under review.
	RuleRequesterCannotApprove = "requester_cannot_approve"

	// RuleApproverNotSoleAuthor rejects a sign-off-style approval where
	// the approver is the case's only author/creator -- mirroring the
	// example in packages/signoff's domain: a case's sole author
	// cannot also be its approving reviewer.
	RuleApproverNotSoleAuthor = "approver_not_sole_author"
)

// DefaultConflictRules returns the standard segregation-of-duties
// rules this package enforces unless a caller supplies a different
// set to CheckConflict.
func DefaultConflictRules() []ConflictRule {
	return []ConflictRule{
		{Name: RuleRequesterCannotApprove, Description: "the actor who requested a grant cannot also approve or attest it"},
		{Name: RuleApproverNotSoleAuthor, Description: "a case's sole author cannot be the approving reviewer"},
	}
}

// ConflictCheck is the input CheckConflict evaluates: who requested
// (or authored) the resource, who is attempting to approve/attest it
// now, and whether the approver is the case's sole author.
type ConflictCheck struct {
	// RequestedBy is the user who requested the grant, or the case's
	// author, depending on which rule is being checked.
	RequestedBy uuid.UUID

	// ActingUserID is the user attempting to approve/attest.
	ActingUserID uuid.UUID

	// SoleAuthor, when true, indicates ActingUserID (if equal to
	// RequestedBy) would be approving as the case's only author --
	// feeds RuleApproverNotSoleAuthor. When false, only
	// RuleRequesterCannotApprove is evaluated.
	SoleAuthor bool
}

// CheckConflict evaluates check against rules (DefaultConflictRules if
// rules is nil), returning the first violated ConflictRule.Name, or
// "" if no rule is violated.
func CheckConflict(check ConflictCheck, rules []ConflictRule) string {
	if rules == nil {
		rules = DefaultConflictRules()
	}
	if check.ActingUserID == uuid.Nil || check.RequestedBy == uuid.Nil {
		return ""
	}
	if check.ActingUserID != check.RequestedBy {
		return ""
	}
	for _, r := range rules {
		switch r.Name {
		case RuleRequesterCannotApprove:
			return RuleRequesterCannotApprove
		case RuleApproverNotSoleAuthor:
			if check.SoleAuthor {
				return RuleApproverNotSoleAuthor
			}
		}
	}
	return ""
}

// Period is a half-open time range [Start, End) a certification
// report or privileged-activity query covers.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Validate checks that p.End is strictly after p.Start.
func (p Period) Validate() error {
	if !p.End.After(p.Start) {
		return ErrInvalidPeriod
	}
	return nil
}

// Contains reports whether t falls within [Start, End).
func (p Period) Contains(t time.Time) bool {
	return !t.Before(p.Start) && t.Before(p.End)
}

// Report is the structured output of Certify (task 7): every grant,
// elevation, and attestation recorded for tenantID within Period,
// exportable as CSV/JSON.
type Report struct {
	TenantID    uuid.UUID   `json:"tenant_id"`
	Period      Period      `json:"period"`
	GeneratedAt time.Time   `json:"generated_at"`
	CaseGrants  []CaseGrant `json:"case_grants"`
	Elevations  []Grant     `json:"elevations"`
	Reviews     []Review    `json:"reviews"`
}

// TotalEntries returns the combined count of grants, elevations, and
// reviews in r -- a quick aggregate figure for a report summary line.
func (r *Report) TotalEntries() int {
	if r == nil {
		return 0
	}
	return len(r.CaseGrants) + len(r.Elevations) + len(r.Reviews)
}

// ExportFormat selects the rendering Certify's export helper produces,
// mirroring packages/auditlog.ExportFormat / packages/dataresidency's
// export conventions exactly.
type ExportFormat string

const (
	// ExportFormatCSV renders a Report as CSV (one sheet per section,
	// concatenated with header rows -- see export.go).
	ExportFormatCSV ExportFormat = "csv"

	// ExportFormatJSON renders a Report as a single JSON object.
	ExportFormatJSON ExportFormat = "json"
)

// IsValid reports whether f is a recognized ExportFormat.
func (f ExportFormat) IsValid() bool {
	return f == ExportFormatCSV || f == ExportFormatJSON
}

// PrivilegedFilter narrows PrivilegedActivity to a subset of a
// tenant's elevated/break-glass-style access events (task 6).
type PrivilegedFilter struct {
	// Actor, if non-empty, restricts results to this exact actor
	// label.
	Actor string

	// CaseID, if non-nil, restricts results to this case.
	CaseID uuid.UUID

	// Since/Until, if non-zero, bound the query's time range.
	Since time.Time
	Until time.Time

	// Limit caps the number of results returned. Zero uses the
	// underlying auditlog.Filter's own default.
	Limit int
}
