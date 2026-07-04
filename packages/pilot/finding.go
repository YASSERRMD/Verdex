package pilot

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Priority ranks how urgently a PilotFinding should be addressed. A
// closed enum, mirroring packages/vulnmanagement.Severity's ranked,
// closed-taxonomy shape by reference -- this package does not import
// vulnmanagement. Named Priority rather than Severity because a
// pilot-feedback-driven issue is triaged for how soon it should be
// fixed, not rated for exploitability/impact the way a security
// finding is.
type Priority string

const (
	// PriorityLow means a minor issue with limited impact on the
	// pilot's outcomes.
	PriorityLow Priority = "low"

	// PriorityMedium means a moderate issue worth addressing during the
	// pilot but not blocking its continuation.
	PriorityMedium Priority = "medium"

	// PriorityHigh means a significant issue that should be refined
	// before the pilot concludes.
	PriorityHigh Priority = "high"

	// PriorityCritical means an issue serious enough to require
	// immediate attention (e.g. a non-binding-compliance failure).
	PriorityCritical Priority = "critical"
)

// IsValid reports whether p is one of the named Priority constants.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (p Priority) String() string { return string(p) }

// rank returns p's position in the low-to-critical ordering, used by
// report.go to sort findings by descending priority, mirroring
// packages/vulnmanagement.Severity.rank exactly.
func (p Priority) rank() int {
	switch p {
	case PriorityLow:
		return 0
	case PriorityMedium:
		return 1
	case PriorityHigh:
		return 2
	case PriorityCritical:
		return 3
	}
	return -1
}

// FindingStatus tracks a PilotFinding's real-world triage state
// machine: Open -> Triaged -> InProgress -> Resolved/WontFix. Mirrors
// packages/vulnmanagement.Status's guarded-transition shape by
// reference -- this package imports neither vulnmanagement nor
// packages/caselifecycle.State, both of which this shape is modeled
// after.
type FindingStatus string

const (
	// FindingStatusOpen is a PilotFinding's initial state: recorded from
	// one or more FeedbackEntry records, not yet reviewed by a human
	// (task 6).
	FindingStatusOpen FindingStatus = "open"

	// FindingStatusTriaged means a human has reviewed the finding,
	// assigned it a Priority, and recorded why (task 6). A
	// RefinementRecord may only reference a finding once it has reached
	// at least this state -- see Engine.RecordRefinement.
	FindingStatusTriaged FindingStatus = "triaged"

	// FindingStatusInProgress means a refinement addressing this
	// finding is actively being applied (task 7).
	FindingStatusInProgress FindingStatus = "in_progress"

	// FindingStatusResolved is a terminal state: the finding has been
	// addressed and verified fixed.
	FindingStatusResolved FindingStatus = "resolved"

	// FindingStatusWontFix is a terminal state: a human with
	// managePermission has explicitly decided not to address the
	// finding within this pilot (e.g. out of scope, deferred to a later
	// phase).
	FindingStatusWontFix FindingStatus = "wont_fix"
)

// IsValid reports whether s is one of the named FindingStatus
// constants.
func (s FindingStatus) IsValid() bool {
	switch s {
	case FindingStatusOpen, FindingStatusTriaged, FindingStatusInProgress, FindingStatusResolved, FindingStatusWontFix:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s FindingStatus) String() string { return string(s) }

// IsTerminal reports whether s is one of the state machine's terminal
// states (Resolved, WontFix).
func (s FindingStatus) IsTerminal() bool {
	switch s {
	case FindingStatusResolved, FindingStatusWontFix:
		return true
	default:
		return false
	}
}

// IsAtLeastTriaged reports whether s is FindingStatusTriaged or any
// status reachable only after triage (InProgress, Resolved, WontFix)
// -- used by Engine.RecordRefinement to enforce that a refinement can
// only reference a finding that has actually been triaged.
func (s FindingStatus) IsAtLeastTriaged() bool {
	switch s {
	case FindingStatusTriaged, FindingStatusInProgress, FindingStatusResolved, FindingStatusWontFix:
		return true
	default:
		return false
	}
}

// allowedFindingTransitions is the single authoritative source of
// truth for which FindingStatus-to-FindingStatus moves
// CanTransitionFinding permits, mirroring
// packages/vulnmanagement.allowedTransitions's map-of-slices shape by
// reference. Every terminal state maps to an empty slice.
var allowedFindingTransitions = map[FindingStatus][]FindingStatus{
	FindingStatusOpen:       {FindingStatusTriaged, FindingStatusWontFix},
	FindingStatusTriaged:    {FindingStatusInProgress, FindingStatusWontFix},
	FindingStatusInProgress: {FindingStatusResolved, FindingStatusTriaged, FindingStatusWontFix},
	FindingStatusResolved:   {},
	FindingStatusWontFix:    {},
}

// CanTransitionFinding reports whether moving directly from `from` to
// `to` is permitted by allowedFindingTransitions.
func CanTransitionFinding(from, to FindingStatus) bool {
	for _, s := range allowedFindingTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// PilotFinding is a tenant-scoped issue surfaced during the pilot
// (task 6): sourced from one or more FeedbackEntry records, assigned a
// Priority, and tracked through a Status triage state machine.
// Mirrors packages/vulnmanagement.Finding's shape, applied to
// pilot-feedback-driven issues instead of security findings.
//
//nolint:revive // "PilotFinding" is the exact type name this phase's design brief specifies; a bare "Finding" would collide with packages/vulnmanagement.Finding and packages/grounding.Finding, both of which name a different kind of finding entirely.
type PilotFinding struct {
	// ID uniquely identifies this finding.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this finding was recorded for.
	TenantID uuid.UUID `json:"tenant_id"`

	// DeploymentID references the PilotDeployment this finding was
	// surfaced under.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// SourceFeedbackIDs lists the FeedbackEntry IDs this finding was
	// sourced from -- one or more, since a recurring issue is often
	// raised independently by several reviewers before being
	// consolidated into a single finding.
	SourceFeedbackIDs []uuid.UUID `json:"source_feedback_ids"`

	// Title is a short human-readable summary of the finding.
	Title string `json:"title"`

	// Description explains the finding in plain language.
	Description string `json:"description,omitempty"`

	// Priority ranks how urgently this finding should be addressed.
	Priority Priority `json:"priority"`

	// Status is this finding's current position in the triage state
	// machine.
	Status FindingStatus `json:"status"`

	// TriageNotes explains the Priority/Status decision recorded at
	// triage time. Required once Status has moved past Open, mirroring
	// packages/vulnmanagement.TriageDecision.Notes's non-blank
	// accountability requirement.
	TriageNotes string `json:"triage_notes,omitempty"`

	// TriagedBy is the identity.User who triaged this finding. Nil
	// until triage occurs.
	TriagedBy *uuid.UUID `json:"triaged_by,omitempty"`

	// TriagedAt is when this finding was triaged. Nil until then.
	TriagedAt *time.Time `json:"triaged_at,omitempty"`

	// DiscoveredAt is when this finding was first recorded.
	DiscoveredAt time.Time `json:"discovered_at"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks f for structural well-formedness.
func (f *PilotFinding) Validate() error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if f.DeploymentID == uuid.Nil {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	if len(f.SourceFeedbackIDs) == 0 {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	if strings.TrimSpace(f.Title) == "" {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	if !f.Priority.IsValid() {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	if !f.Status.IsValid() {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	if f.DiscoveredAt.IsZero() {
		return wrapf("PilotFinding.Validate", ErrInvalidFinding)
	}
	return nil
}

// SortFindingsByPriorityDesc returns a copy of findings sorted by
// descending Priority (PriorityCritical first, PriorityLow last),
// stable on ties -- mirroring
// packages/vulnmanagement.Report.SLABreachesBySeverityDesc's identical
// ordering convention: the most urgent finding surfaced first, rather
// than insertion order, for a caller rendering
// Engine.ListFindingsForDeployment's result.
func SortFindingsByPriorityDesc(findings []PilotFinding) []PilotFinding {
	out := make([]PilotFinding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority.rank() > out[j].Priority.rank()
	})
	return out
}
