package vulnmanagement

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ScannerSource identifies which category of scanner produced a
// Finding. A closed enum: the set of scanner categories this platform
// integrates with is a fixed, deliberately curated list (task 1's SCA,
// task 2's SAST, task 3's container-image scanning, task 7's license
// compliance), not an open taxonomy the way
// packages/compliance.Framework is -- adding a genuinely new scanner
// category is a real engineering change, not a per-deployment
// configuration choice.
type ScannerSource string

const (
	// ScannerSourceSCA identifies a Software Composition Analysis
	// finding: a known vulnerability in a third-party dependency,
	// detected by tools such as govulncheck (see
	// .github/workflows/ci.yml's sca-scan job).
	ScannerSourceSCA ScannerSource = "sca"

	// ScannerSourceSAST identifies a Static Application Security
	// Testing finding: a vulnerability pattern detected in this
	// platform's own source code, as opposed to a dependency.
	ScannerSourceSAST ScannerSource = "sast"

	// ScannerSourceContainer identifies a container-image scanning
	// finding: a known vulnerability in an OS package or layer within
	// a built container image.
	ScannerSourceContainer ScannerSource = "container"

	// ScannerSourceLicense identifies a license-compliance finding: a
	// dependency whose license falls outside this platform's allow
	// list (see LicenseCheck, license.go).
	ScannerSourceLicense ScannerSource = "license"
)

// IsValid reports whether s is one of the named ScannerSource
// constants.
func (s ScannerSource) IsValid() bool {
	switch s {
	case ScannerSourceSCA, ScannerSourceSAST, ScannerSourceContainer, ScannerSourceLicense:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s ScannerSource) String() string { return string(s) }

// Severity ranks how serious a Finding is. A closed enum, mirroring
// packages/threatmodel.Severity: severity is an internal risk-rating
// scale this package defines and owns, not an open/extensible
// taxonomy.
type Severity string

const (
	// SeverityLow means limited impact and/or very low exploitability.
	SeverityLow Severity = "low"

	// SeverityMedium means moderate impact or exploitability.
	SeverityMedium Severity = "medium"

	// SeverityHigh means significant impact, plausible exploitability.
	SeverityHigh Severity = "high"

	// SeverityCritical means severe impact (e.g. remote code execution,
	// full compromise) with a short remediation SLA -- see
	// RemediationDeadlineFor (sla.go).
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
// reporting code (report.go) to sort findings by descending severity.
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

// Status tracks a Finding's real-world remediation state machine:
// Open -> Triaged -> Remediating -> Resolved/AcceptedRisk/FalsePositive.
// Mirrors packages/caselifecycle.State's and
// packages/threatmodel.MitigationStatus's guarded-transition shape by
// reference -- this package imports neither.
type Status string

const (
	// StatusOpen is a Finding's initial state: detected by a scanner,
	// not yet reviewed by a human.
	StatusOpen Status = "open"

	// StatusTriaged means a human has reviewed the finding and recorded
	// a TriageDecision, but remediation work has not yet started.
	StatusTriaged Status = "triaged"

	// StatusRemediating means remediation work (a dependency bump, a
	// code fix, a rebuilt image) is actively in progress.
	StatusRemediating Status = "remediating"

	// StatusResolved is a terminal state: the underlying vulnerability
	// has been fixed and verified gone.
	StatusResolved Status = "resolved"

	// StatusAcceptedRisk is a terminal state: a human with
	// managePermission has explicitly decided not to remediate (e.g.
	// the affected code path is unreachable), recorded via a
	// TriageDecision naming the reason.
	StatusAcceptedRisk Status = "accepted_risk"

	// StatusFalsePositive is a terminal state: the finding does not
	// actually apply (e.g. the vulnerable function is never called),
	// recorded via a TriageDecision naming the reason.
	StatusFalsePositive Status = "false_positive"
)

// IsValid reports whether s is one of the named Status constants.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusTriaged, StatusRemediating, StatusResolved, StatusAcceptedRisk, StatusFalsePositive:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Status) String() string { return string(s) }

// IsTerminal reports whether s is one of the state machine's terminal
// states (Resolved, AcceptedRisk, FalsePositive) -- no further
// transition is permitted out of a terminal state, mirroring
// packages/threatmodel.MitigationVerified's terminal-via-CanTransition
// shape.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusResolved, StatusAcceptedRisk, StatusFalsePositive:
		return true
	default:
		return false
	}
}

// allowedTransitions is the single authoritative source of truth for
// which Status-to-Status moves CanTransition permits, mirroring
// packages/caselifecycle.allowedTransitions's map-of-slices shape by
// reference (this package does not import packages/caselifecycle).
// Every terminal state maps to an empty slice: regression out of a
// terminal state is not an ordinary transition (there is no
// sanctioned "reopen a resolved finding" path in this phase -- a
// rediscovered vulnerability is a new scanner Finding, not a reopened
// old one).
var allowedTransitions = map[Status][]Status{
	StatusOpen:          {StatusTriaged, StatusFalsePositive, StatusAcceptedRisk},
	StatusTriaged:       {StatusRemediating, StatusAcceptedRisk, StatusFalsePositive},
	StatusRemediating:   {StatusResolved, StatusTriaged},
	StatusResolved:      {},
	StatusAcceptedRisk:  {},
	StatusFalsePositive: {},
}

// CanTransition reports whether moving directly from `from` to `to` is
// permitted by allowedTransitions.
func CanTransition(from, to Status) bool {
	for _, s := range allowedTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Finding is a single tenant-scoped vulnerability/dependency finding
// (the "Vulnerability" of this phase's brief): what scanner detected
// it, what package+version is affected, how severe it is, its
// CVE/advisory identifier, when it was discovered, and its current
// Status within the remediation state machine.
type Finding struct {
	// ID uniquely identifies this finding.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this finding was recorded for.
	TenantID uuid.UUID `json:"tenant_id"`

	// Source identifies which scanner category produced this finding.
	Source ScannerSource `json:"source"`

	// Package is the affected dependency/component name (e.g.
	// "golang.org/x/crypto", "openssl", or a source file path for a
	// SAST finding).
	Package string `json:"package"`

	// Version is the affected version string (e.g. "v0.17.0"). May be
	// blank for a SAST finding, which has no package version.
	Version string `json:"version,omitempty"`

	// Severity ranks how serious this finding is.
	Severity Severity `json:"severity"`

	// AdvisoryID is the CVE, GHSA, or other advisory identifier naming
	// this vulnerability (e.g. "CVE-2024-12345"). May be a
	// scanner-internal rule ID for a SAST finding with no assigned CVE.
	AdvisoryID string `json:"advisory_id"`

	// Title is a short human-readable summary of the finding.
	Title string `json:"title"`

	// Description explains the finding in plain language: what the
	// vulnerability is and why it matters.
	Description string `json:"description,omitempty"`

	// Status is this finding's current position in the remediation
	// state machine.
	Status Status `json:"status"`

	// DiscoveredAt is when the scanner first reported this finding.
	DiscoveredAt time.Time `json:"discovered_at"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks f for structural well-formedness.
func (f *Finding) Validate() error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if !f.Source.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if strings.TrimSpace(f.Package) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if !f.Severity.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if strings.TrimSpace(f.AdvisoryID) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if strings.TrimSpace(f.Title) == "" {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if !f.Status.IsValid() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	if f.DiscoveredAt.IsZero() {
		return wrapf("Finding.Validate", ErrInvalidFinding)
	}
	return nil
}

// TriageDecision is a tenant-scoped record of a human decision made
// about a Finding during triage (task 5): who decided, what they
// decided (the new Status), and why (Notes). Mirrors
// packages/accessgovernance's Attest and packages/signoff's
// explicit-acknowledgement pattern by reference: a decision always
// records an actor and a reason, never a bare status flip with no
// accountability trail.
type TriageDecision struct {
	// ID uniquely identifies this triage decision.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this decision was recorded for.
	TenantID uuid.UUID `json:"tenant_id"`

	// FindingID references the Finding this decision applies to.
	FindingID uuid.UUID `json:"finding_id"`

	// FromStatus is the Finding's Status immediately before this
	// decision was applied.
	FromStatus Status `json:"from_status"`

	// ToStatus is the Finding's Status this decision moved it to. Must
	// be a legal CanTransition(FromStatus, ToStatus) move.
	ToStatus Status `json:"to_status"`

	// Notes explains why this decision was made. Required
	// (non-blank, after trimming): a triage decision that does not
	// explain itself is not a real decision, mirroring
	// packages/signoff.Reject's non-blank-Notes requirement.
	Notes string `json:"notes"`

	// Actor is the identity.User who made this decision.
	Actor uuid.UUID `json:"actor"`

	// DecidedAt is when this decision was recorded.
	DecidedAt time.Time `json:"decided_at"`
}

// Validate checks d for structural well-formedness.
func (d *TriageDecision) Validate() error {
	if d == nil {
		return ErrInvalidTriageDecision
	}
	if d.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if d.FindingID == uuid.Nil {
		return wrapf("TriageDecision.Validate", ErrInvalidTriageDecision)
	}
	if !d.FromStatus.IsValid() || !d.ToStatus.IsValid() {
		return wrapf("TriageDecision.Validate", ErrInvalidTriageDecision)
	}
	if strings.TrimSpace(d.Notes) == "" {
		return wrapf("TriageDecision.Validate", ErrNotesRequired)
	}
	if d.Actor == uuid.Nil {
		return wrapf("TriageDecision.Validate", ErrInvalidTriageDecision)
	}
	if d.DecidedAt.IsZero() {
		return wrapf("TriageDecision.Validate", ErrInvalidTriageDecision)
	}
	return nil
}
