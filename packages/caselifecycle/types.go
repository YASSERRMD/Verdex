package caselifecycle

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// State is the coarse lifecycle state of a Case. See doc.go for the
// full state machine diagram and transition.go for the allowed
// transitions between states.
type State string

const (
	// StateDraft is the initial state of every Case (see NewCase):
	// intake is in progress, evidence/parties/category may still be
	// incomplete, and nothing about the case is yet considered ready
	// for judicial review.
	StateDraft State = "draft"

	// StateActive means the case is fully in progress: ingestion,
	// categorization, and reasoning may all proceed normally.
	StateActive State = "active"

	// StateUnderReview means the case has been submitted for judicial
	// review of its draft reasoning output. Most mutating operations
	// on case-scoped data are frozen in this state (see actions.go).
	StateUnderReview State = "under_review"

	// StateClosed means the case has reached a disposition. Closed
	// cases are read-only except via Reopen, which explicitly
	// transitions back to StateActive with a recorded justification.
	StateClosed State = "closed"

	// StateArchived is a terminal state distinct from StateClosed: an
	// archived case cannot be reopened. Only Archive (see archive.go)
	// moves a case into this state, and only from StateClosed.
	StateArchived State = "archived"
)

// Valid reports whether s is one of the known State constants.
func (s State) Valid() bool {
	switch s {
	case StateDraft, StateActive, StateUnderReview, StateClosed, StateArchived:
		return true
	default:
		return false
	}
}

// String satisfies fmt.Stringer.
func (s State) String() string { return string(s) }

// IsTerminal reports whether s is a state no further Transition can
// leave. StateArchived is the only terminal state — StateClosed is
// deliberately not terminal because Reopen can leave it.
func (s State) IsTerminal() bool {
	return s == StateArchived
}

// Case is the canonical record of a single matter moving through
// Verdex, from intake to closure. This is the entity every other
// package's bare CaseID string ultimately refers to.
type Case struct {
	// ID is the globally unique identifier for this case.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this case belongs to. Every Repository
	// method is scoped to a tenantID and refuses cross-tenant access
	// (see ErrCrossTenantAccess).
	TenantID uuid.UUID `json:"tenant_id"`

	// JurisdictionID links to the packages/jurisdiction.Jurisdiction
	// this case is being heard under. This package does not duplicate
	// jurisdiction data or lookup logic; it stores the reference only.
	JurisdictionID uuid.UUID `json:"jurisdiction_id"`

	// CategoryID links to the packages/category taxonomy entry (a
	// packages/category.CategoryCode value, stored as a plain string
	// here so this package does not take a hard dependency on
	// packages/category's Go types) that classifies this case. Empty
	// until intake assigns a category.
	CategoryID string `json:"category_id"`

	// Title is a short human-readable label for the case (e.g. "Doe v.
	// Acme Corp"). Required.
	Title string `json:"title"`

	// Reference is an optional external/docket reference number or
	// citation string (e.g. a court-assigned case number). Distinct
	// from ID: ID is Verdex's own internal identifier, Reference is
	// whatever label the court or intake process uses.
	Reference string `json:"reference,omitempty"`

	// State is the case's current lifecycle state.
	State State `json:"state"`

	// Metadata is a free-form set of additional fields this package
	// does not model explicitly (e.g. docket numbers, external system
	// references). Mutate only via SetMetadata/MergeMetadata, never by
	// writing this map directly, so MetadataVersion stays accurate.
	Metadata map[string]string `json:"metadata"`

	// MetadataVersion increments on every successful SetMetadata or
	// MergeMetadata call, letting callers detect a lost-update race
	// (see ErrMetadataVersionConflict).
	MetadataVersion int `json:"metadata_version"`

	// CreatedBy is the ID of the identity.User who created this case.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt is when this case was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this case was last modified in any way
	// (state transition, metadata change, or field update).
	UpdatedAt time.Time `json:"updated_at"`

	// ArchivedAt is when Archive moved this case to StateArchived. Nil
	// until then.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// Validate checks that c has every field required to be persisted:
// a non-nil TenantID, JurisdictionID, non-blank Title, and a valid
// State. CategoryID and Reference may be blank (category assignment
// can happen after intake; not every case has an external reference).
// Returns ErrInvalidCase if any check fails.
func (c *Case) Validate() error {
	if c == nil {
		return ErrInvalidCase
	}
	if c.TenantID == uuid.Nil {
		return ErrInvalidCase
	}
	if c.JurisdictionID == uuid.Nil {
		return ErrInvalidCase
	}
	if trimmed := strings.TrimSpace(c.Title); trimmed == "" {
		return ErrInvalidCase
	}
	if !c.State.Valid() {
		return ErrInvalidCase
	}
	return nil
}

// TransitionRecord is one immutable entry in a Case's transition audit
// log: who moved the case from which state to which state, when, and
// why.
type TransitionRecord struct {
	// ID uniquely identifies this transition record.
	ID uuid.UUID `json:"id"`

	// CaseID identifies the case this transition applied to.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is copied from the case at the time of transition, so
	// audit queries can be tenant-scoped without a join.
	TenantID uuid.UUID `json:"tenant_id"`

	// FromState is the case's state immediately before this
	// transition.
	FromState State `json:"from_state"`

	// ToState is the case's state immediately after this transition.
	ToState State `json:"to_state"`

	// Actor is the ID of the identity.User who performed the
	// transition.
	Actor uuid.UUID `json:"actor"`

	// Reason is a free-form explanation for the transition. Required
	// (non-blank) for Reopen; optional for ordinary Transition calls.
	Reason string `json:"reason,omitempty"`

	// OccurredAt is when the transition was recorded.
	OccurredAt time.Time `json:"occurred_at"`
}
