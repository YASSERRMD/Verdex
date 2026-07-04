package securitytesting

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Finding is a single tracked security-testing result (task 7): a
// Scenario ran, the defense it probed did not hold, and this record
// tracks that vulnerability from discovery through triage to a
// remediation independently re-verified as effective. Finding mirrors
// the shape a sibling phase's packages/vulnmanagement is expected to
// define for its own (broader, dependency-scan-sourced) findings --
// this package does not import packages/vulnmanagement; Finding is a
// parallel, independently-owned type scoped to results this harness's
// own Scenarios produce, not a shared type or a wrapper around
// vulnmanagement's.
type Finding struct {
	// ID uniquely identifies this finding.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this finding, or uuid.Nil for a platform-wide
	// finding with no single-tenant target (e.g. a prompt-injection
	// detection gap that affects every tenant identically).
	TenantID uuid.UUID `json:"tenant_id,omitempty"`

	// Title is a short human-readable name for this finding.
	Title string `json:"title"`

	// Category classifies which adversarial suite (or the general
	// regression suite) produced this finding.
	Category Category `json:"category"`

	// Severity ranks how serious this finding is.
	Severity Severity `json:"severity"`

	// SourceScenario is the Scenario.Name() whose Run call produced the
	// RunRecord this finding was opened from -- what
	// Engine.VerifyRemediation re-runs to check whether a fix actually
	// landed.
	SourceScenario string `json:"source_scenario"`

	// SourceRunID is the RunRecord.ID this finding was opened from,
	// preserved for audit/traceability even though RunRecord itself may
	// have aged out of any bounded-retention query.
	SourceRunID uuid.UUID `json:"source_run_id"`

	// Detail explains what the scenario observed (typically copied from
	// the originating RunRecord.Result.Detail at open time, but may be
	// hand-edited during triage to add analyst context).
	Detail string `json:"detail"`

	// Status tracks this finding's triage/remediation lifecycle.
	Status FindingStatus `json:"status"`

	// RiskAcceptedJustification is required (see Validate) whenever
	// Status == FindingRiskAccepted, recording why the finding will not
	// be fixed. Blank for every other status.
	RiskAcceptedJustification string `json:"risk_accepted_justification,omitempty"`

	// OpenedBy is the identity.User (or uuid.Nil for an unattended
	// harness run) who triggered the Scenario run this finding was
	// opened from.
	OpenedBy uuid.UUID `json:"opened_by,omitempty"`

	// OpenedAt is when this finding was first recorded.
	OpenedAt time.Time `json:"opened_at"`

	// UpdatedAt is bumped on every status transition.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks f for structural well-formedness.
func (f *Finding) Validate() error {
	if f == nil {
		return ErrInvalidFinding
	}
	if strings.TrimSpace(f.Title) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if !f.Category.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if !f.Severity.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if strings.TrimSpace(f.SourceScenario) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if !f.Status.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if f.Status == FindingRiskAccepted && strings.TrimSpace(f.RiskAcceptedJustification) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	return nil
}

// IsOpenLike reports whether f is still an active, unresolved finding
// (FindingOpen, FindingTriaged, or FindingRemediationPending) --
// convenience for reporting code that wants an "outstanding work"
// count without listing every non-terminal status inline.
func (f Finding) IsOpenLike() bool {
	switch f.Status {
	case FindingOpen, FindingTriaged, FindingRemediationPending:
		return true
	default:
		return false
	}
}

// FindingsBySeverityDesc returns a copy of findings sorted by
// descending Severity (SeverityCritical first, SeverityLow last),
// stable on ties -- mirroring
// packages/threatmodel.ThreatModel.ThreatsBySeverityDesc's identical
// shape by reference (this package does not import
// packages/threatmodel). The ordering a human-facing findings
// dashboard wants: the most serious outstanding finding surfaced
// first, rather than storage/discovery order.
func FindingsBySeverityDesc(findings []Finding) []Finding {
	out := make([]Finding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Severity.rank() > out[j].Severity.rank()
	})
	return out
}
