package privacy

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SARStatus is the closed set of states a SubjectAccessRequest moves
// through (task 4).
type SARStatus string

const (
	// SARStatusReceived is the initial state: the request has been
	// logged but processing has not yet begun.
	SARStatusReceived SARStatus = "received"

	// SARStatusInProgress means the request is actively being worked.
	SARStatusInProgress SARStatus = "in_progress"

	// SARStatusFulfilled is a terminal state: the requested data was
	// compiled and delivered to the subject.
	SARStatusFulfilled SARStatus = "fulfilled"

	// SARStatusRejected is a terminal state: the request was declined
	// (e.g. identity could not be verified, or an exemption applies).
	SARStatusRejected SARStatus = "rejected"
)

// IsValid reports whether s is one of the named SARStatus constants.
func (s SARStatus) IsValid() bool {
	switch s {
	case SARStatusReceived, SARStatusInProgress, SARStatusFulfilled, SARStatusRejected:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s SARStatus) String() string { return string(s) }

// IsTerminal reports whether s is a terminal SARStatus that
// CanTransitionSAR never permits leaving.
func (s SARStatus) IsTerminal() bool {
	return s == SARStatusFulfilled || s == SARStatusRejected
}

// allowedSARTransitions is the single authoritative source of truth
// for which SARStatus-to-SARStatus moves TransitionSAR permits,
// mirroring packages/caselifecycle's allowedTransitions +
// CanTransition guard-map style by reference (Phase 063): a map of
// permitted destination states per origin state, consulted by a guard
// function before any status is mutated.
var allowedSARTransitions = map[SARStatus][]SARStatus{
	SARStatusReceived:   {SARStatusInProgress, SARStatusRejected},
	SARStatusInProgress: {SARStatusFulfilled, SARStatusRejected},
	SARStatusFulfilled:  {},
	SARStatusRejected:   {},
}

// CanTransitionSAR reports whether moving directly from `from` to `to`
// is permitted by allowedSARTransitions.
func CanTransitionSAR(from, to SARStatus) bool {
	for _, s := range allowedSARTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// SubjectAccessRequest is a tenant-scoped data-subject access request
// (SAR, task 4): a data subject's ask to know what personal data the
// tenant holds about them, tracked through a guarded status state
// machine (received -> in_progress -> fulfilled/rejected).
type SubjectAccessRequest struct {
	// ID uniquely identifies this request.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this request belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// SubjectID identifies the data subject making the request (see
	// ConsentRecord.SubjectID's doc comment for why this is a plain
	// string rather than a uuid.UUID tied to identity.User).
	SubjectID string `json:"subject_id"`

	// CaseRefs optionally scopes the request to specific cases the
	// subject believes their data appears in. Referenced by ID only;
	// no packages/caselifecycle import.
	CaseRefs []uuid.UUID `json:"case_refs,omitempty"`

	// Status is this request's current position in the state machine.
	Status SARStatus `json:"status"`

	// ReceivedAt is when the request was logged.
	ReceivedAt time.Time `json:"received_at"`

	// DueAt is the regulatory/policy deadline by which this request
	// should be fulfilled or rejected (commonly 30 days from
	// ReceivedAt, though this package leaves the exact window to the
	// caller rather than hardcoding one jurisdiction's rule).
	DueAt time.Time `json:"due_at"`

	// ResolvedAt, if non-nil, is when Status last moved to a terminal
	// state (SARStatusFulfilled or SARStatusRejected).
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// ResolutionNotes is a free-text explanation attached when the
	// request reaches a terminal state (e.g. rejection rationale, or a
	// pointer to where the fulfilled export was delivered).
	ResolutionNotes string `json:"resolution_notes,omitempty"`

	// HandledBy is the identity.User currently or most recently
	// assigned to work this request.
	HandledBy uuid.UUID `json:"handled_by,omitempty"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks r for structural well-formedness.
func (r *SubjectAccessRequest) Validate() error {
	if r == nil {
		return ErrInvalidSAR
	}
	if r.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(r.SubjectID) == "" {
		return wrapf("SubjectAccessRequest.Validate", ErrInvalidSAR)
	}
	if !r.Status.IsValid() {
		return wrapf("SubjectAccessRequest.Validate", ErrInvalidSAR)
	}
	if r.ReceivedAt.IsZero() {
		return wrapf("SubjectAccessRequest.Validate", ErrInvalidSAR)
	}
	return nil
}

// TransitionSAR moves r.Status to to, provided CanTransitionSAR
// permits that move from r's current Status, mirroring
// packages/caselifecycle.Transition's guard discipline. Returns
// ErrIllegalSARTransition without mutating r if the move is not
// permitted. On a move into a terminal status, ResolvedAt is stamped
// with now and notes (if non-blank) is recorded as ResolutionNotes.
func TransitionSAR(r *SubjectAccessRequest, to SARStatus, now time.Time, notes string) error {
	if r == nil {
		return ErrInvalidSAR
	}
	if !to.IsValid() {
		return wrapf("TransitionSAR", ErrInvalidSAR)
	}
	if !CanTransitionSAR(r.Status, to) {
		return wrapf("TransitionSAR", ErrIllegalSARTransition)
	}

	r.Status = to
	r.UpdatedAt = now
	if to.IsTerminal() {
		t := now
		r.ResolvedAt = &t
		if strings.TrimSpace(notes) != "" {
			r.ResolutionNotes = notes
		}
	}
	return nil
}
