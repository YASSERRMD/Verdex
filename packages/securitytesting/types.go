// Package securitytesting is Phase 086: an adversarial validation
// harness that proves the platform's defenses actually hold, rather
// than merely asserting that they exist. See doc.go for the full
// composition write-up.
package securitytesting

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Severity ranks how serious a Finding is. A closed enum -- an
// internal risk-rating scale this package defines and owns, mirroring
// packages/threatmodel.Severity's shape exactly (this package does not
// import threatmodel.Severity: the two scales serve different
// document types and evolve independently).
type Severity string

const (
	// SeverityLow means limited impact and/or very low exploitability.
	SeverityLow Severity = "low"

	// SeverityMedium means moderate impact or exploitability.
	SeverityMedium Severity = "medium"

	// SeverityHigh means significant impact, plausible exploitability.
	SeverityHigh Severity = "high"

	// SeverityCritical means severe impact (e.g. cross-tenant data
	// exposure, authz bypass, full compromise).
	SeverityCritical Severity = "critical"
)

// IsValid reports whether s is one of the named Severity constants.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Severity) String() string { return string(s) }

// rank returns s's position in the low-to-critical ordering, used by
// sorting/reporting helpers.
func (s Severity) rank() int {
	switch s {
	case SeverityLow:
		return 0
	case SeverityMedium:
		return 1
	case SeverityHigh:
		return 2
	case SeverityCritical:
		return 3
	}
	return -1
}

// Category classifies which adversarial suite (tasks 3-6) produced a
// Finding, or the broader regression-test suite (task 1) that did.
type Category string

const (
	// CategoryRegression covers findings from the general automated
	// security regression suite (task 1) -- previously-fixed
	// vulnerabilities re-checked on every run so a regression is
	// caught immediately.
	CategoryRegression Category = "regression"

	// CategoryPromptInjection covers findings from the prompt-injection
	// adversarial suite (task 3).
	CategoryPromptInjection Category = "prompt_injection"

	// CategoryDataLeakage covers findings from the cross-tenant/
	// cross-case data-leakage suite (task 4).
	CategoryDataLeakage Category = "data_leakage"

	// CategoryAuthzBypass covers findings from the authorization-bypass
	// suite (task 5).
	CategoryAuthzBypass Category = "authz_bypass" // #nosec G101 -- a Category tag, not a credential

	// CategoryAbuseCase covers findings from the abuse-case suite
	// (task 6): quota abuse, oversized payloads, replay.
	CategoryAbuseCase Category = "abuse_case"
)

// IsValid reports whether c is one of the named Category constants.
func (c Category) IsValid() bool {
	switch c {
	case CategoryRegression, CategoryPromptInjection, CategoryDataLeakage,
		CategoryAuthzBypass, CategoryAbuseCase:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c Category) String() string { return string(c) }

// FindingStatus tracks a Finding's triage/remediation lifecycle,
// mirroring the state-machine shape packages/threatmodel.MitigationStatus
// and packages/privacy.SARStatus both establish (an allowed-transitions
// map plus a CanTransition guard), and conceptually mirroring the
// Finding/triage shape a sibling phase's packages/vulnmanagement is
// expected to define -- this package does not import
// packages/vulnmanagement; the shape is deliberately parallel, not
// shared code.
type FindingStatus string

const (
	// FindingOpen is the initial state: a Scenario run reproduced the
	// vulnerability and no remediation has been attempted yet.
	FindingOpen FindingStatus = "open"

	// FindingTriaged means a human has reviewed the Finding and
	// recorded a severity/owner decision, but no fix has shipped yet.
	FindingTriaged FindingStatus = "triaged"

	// FindingRemediationPending means a fix is believed to have
	// shipped but has not yet been independently re-verified by
	// re-running the originating Scenario.
	FindingRemediationPending FindingStatus = "remediation_pending"

	// FindingVerifiedFixed is a terminal state: Engine.VerifyRemediation
	// re-ran the originating Scenario and it no longer reproduces the
	// vulnerability. This is the only path into this state -- it is
	// never set directly, exactly mirroring
	// packages/threatmodel.MitigationVerified's "someone actually
	// checked" guarantee.
	FindingVerifiedFixed FindingStatus = "verified_fixed"

	// FindingRiskAccepted is a terminal state: a human with
	// managePermission has explicitly recorded that this Finding will
	// not be fixed (e.g. accepted residual risk, compensating control
	// exists elsewhere), with a required justification.
	FindingRiskAccepted FindingStatus = "risk_accepted"
)

// IsValid reports whether s is one of the named FindingStatus
// constants.
func (s FindingStatus) IsValid() bool {
	switch s {
	case FindingOpen, FindingTriaged, FindingRemediationPending,
		FindingVerifiedFixed, FindingRiskAccepted:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s FindingStatus) String() string { return string(s) }

// IsTerminal reports whether s is a terminal FindingStatus that
// CanTransitionFinding never permits leaving.
func (s FindingStatus) IsTerminal() bool {
	return s == FindingVerifiedFixed || s == FindingRiskAccepted
}

// allowedFindingTransitions maps each FindingStatus to the set of
// statuses a transition may move to. FindingVerifiedFixed is reachable
// only from FindingRemediationPending, and only via
// Engine.VerifyRemediation actually re-running the Scenario -- never a
// direct SetStatus call -- so "verified fixed" always means a passing
// re-run happened, not merely that someone asserted it.
var allowedFindingTransitions = map[FindingStatus][]FindingStatus{
	FindingOpen:               {FindingTriaged, FindingRiskAccepted},
	FindingTriaged:            {FindingRemediationPending, FindingRiskAccepted},
	FindingRemediationPending: {FindingVerifiedFixed, FindingOpen, FindingRiskAccepted},
	FindingVerifiedFixed:      {},
	FindingRiskAccepted:       {},
}

// CanTransitionFinding reports whether from -> to is a permitted
// Finding status transition.
func CanTransitionFinding(from, to FindingStatus) bool {
	for _, allowed := range allowedFindingTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Outcome is the pass/fail result of running a single Scenario.
type Outcome string

const (
	// OutcomePassed means the Scenario ran and did NOT reproduce the
	// vulnerability/violation it probes for -- the defense held.
	OutcomePassed Outcome = "passed"

	// OutcomeFailed means the Scenario ran and DID reproduce the
	// vulnerability/violation -- the defense did not hold, and a
	// Finding should be recorded.
	OutcomeFailed Outcome = "failed"

	// OutcomeError means the Scenario could not be evaluated at all
	// (e.g. a dependency was unavailable) -- distinct from
	// OutcomePassed, since an inconclusive run must never be silently
	// treated as a passing one.
	OutcomeError Outcome = "error"
)

// IsValid reports whether o is one of the named Outcome constants.
func (o Outcome) IsValid() bool {
	switch o {
	case OutcomePassed, OutcomeFailed, OutcomeError:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (o Outcome) String() string { return string(o) }

// Result is what a Scenario.Run call returns: whether the defense held,
// plus enough human-readable detail to act on a failure without
// re-running the scenario under a debugger.
type Result struct {
	// Outcome is the pass/fail/error verdict.
	Outcome Outcome `json:"outcome"`

	// Detail is a short, human-readable explanation of what was probed
	// and what was observed (e.g. "cross-tenant Get returned tenant B's
	// record" or "all 6 injection payloads were flagged").
	Detail string `json:"detail"`

	// Evidence carries scenario-specific supporting data (e.g. the
	// exact payload that got through, the record ID that leaked) --
	// deliberately a string map, not a scenario-specific struct, so
	// every Scenario implementation can return Result without this
	// package needing a type per scenario kind.
	Evidence map[string]string `json:"evidence,omitempty"`
}

// Validate checks r for structural well-formedness.
func (r Result) Validate() error {
	if !r.Outcome.IsValid() {
		return wrapf("Result.Validate", ErrInvalidRunRecord)
	}
	if strings.TrimSpace(r.Detail) == "" {
		return wrapf("Result.Validate", ErrInvalidRunRecord)
	}
	return nil
}

// RunRecord is a durable record of one Scenario execution: which
// scenario ran, when, against which tenant (if tenant-scoped), and
// what Result it produced. RunRecord is what Finding.SourceRunID and
// the remediation-verification history (remediation.go) both point
// back to, so "when was this last checked, and what happened" is
// always answerable without re-running anything.
type RunRecord struct {
	// ID uniquely identifies this run.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this run, or uuid.Nil for a platform-wide
	// scenario with no single-tenant target (e.g. a prompt-injection
	// corpus run against a detector, not against tenant data).
	TenantID uuid.UUID `json:"tenant_id,omitempty"`

	// ScenarioName is the Scenario.Name() that ran, and ScenarioCategory
	// is its Category() -- both denormalized onto the record so a
	// RunRecord remains fully readable even if the originating Scenario
	// implementation is later removed from the harness.
	ScenarioName     string   `json:"scenario_name"`
	ScenarioCategory Category `json:"scenario_category"`

	// Result is what the Scenario returned.
	Result Result `json:"result"`

	// RunBy is the identity.User who triggered this run, or uuid.Nil for
	// a run triggered by an unattended/scheduled harness invocation.
	RunBy uuid.UUID `json:"run_by,omitempty"`

	// RanAt is when this run executed.
	RanAt time.Time `json:"ran_at"`
}

// Validate checks rr for structural well-formedness.
func (rr RunRecord) Validate() error {
	if strings.TrimSpace(rr.ScenarioName) == "" {
		return wrapf("RunRecord.Validate", ErrInvalidRunRecord)
	}
	if !rr.ScenarioCategory.IsValid() {
		return wrapf("RunRecord.Validate", ErrInvalidRunRecord)
	}
	if err := rr.Result.Validate(); err != nil {
		return wrapf("RunRecord.Validate", err)
	}
	return nil
}
