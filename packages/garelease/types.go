package garelease

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DimensionName identifies one named dimension of release readiness
// this package evaluates. A closed enum: the set of dimensions this
// phase's Engine.CheckReadiness aggregates is a fixed, deliberately
// curated list (tasks 1-3, 5-6 of Phase 100's brief), not an open
// taxonomy the way packages/compliance.Framework is -- adding a
// genuinely new readiness dimension is a real engineering change to
// this package, not a per-deployment configuration choice.
type DimensionName string

const (
	// DimensionCriticalFindings covers task 1: every critical/high
	// packages/vulnmanagement.Finding and packages/securitytesting.Finding
	// must be resolved (Status terminal and not itself a still-open
	// critical/high severity) before this dimension passes.
	DimensionCriticalFindings DimensionName = "critical_findings"

	// DimensionComplianceGaps covers task 2: the final
	// packages/compliance.GapAnalysisReport must report zero
	// StatusGap results across every applicable control.
	DimensionComplianceGaps DimensionName = "compliance_gaps"

	// DimensionPerfBudget covers task 3: every packages/perf.Verdict
	// supplied on ReadinessInput must report Passed for every budgeted
	// operation.
	DimensionPerfBudget DimensionName = "perf_budget"

	// DimensionE2ERegression covers task 5: the platform's full-journey
	// E2E suite (packages/e2e, run as a blocking CI gate -- see
	// ReadinessInput.E2EResult's doc comment for why this package does
	// not import packages/e2e directly) must report every scenario
	// passed.
	DimensionE2ERegression DimensionName = "e2e_regression"

	// DimensionGuardrailIntegrity covers task 6's guardrail half: a real
	// call into packages/guardrail's own verdict-language/label checks
	// against a deliberately-bad fixture (must fail) and a
	// properly-labeled fixture (must pass) -- see
	// Engine.VerifyGuardrails.
	DimensionGuardrailIntegrity DimensionName = "guardrail_integrity"

	// DimensionAuditCompleteness covers task 6's audit half: a real
	// structural check that the audit store is queryable and its hash
	// chain is intact -- see Engine.VerifyAuditTrail.
	DimensionAuditCompleteness DimensionName = "audit_completeness"
)

// IsValid reports whether d is one of the named DimensionName
// constants.
func (d DimensionName) IsValid() bool {
	switch d {
	case DimensionCriticalFindings, DimensionComplianceGaps, DimensionPerfBudget,
		DimensionE2ERegression, DimensionGuardrailIntegrity, DimensionAuditCompleteness:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (d DimensionName) String() string { return string(d) }

// CheckStatus reports pass/fail for a single ReadinessCheck.
type CheckStatus string

const (
	// CheckPassed means the dimension's evaluation found nothing
	// blocking release.
	CheckPassed CheckStatus = "passed"

	// CheckFailed means the dimension's evaluation found at least one
	// blocking condition -- this ReadinessCheck alone is enough to make
	// the overall ReleaseReadiness not Ready (fail closed).
	CheckFailed CheckStatus = "failed"
)

// IsValid reports whether s is one of the named CheckStatus constants.
func (s CheckStatus) IsValid() bool {
	switch s {
	case CheckPassed, CheckFailed:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s CheckStatus) String() string { return string(s) }

// ReadinessCheck is one named dimension of release readiness (e.g.
// "critical_findings", "compliance_gaps", "perf_budget",
// "e2e_regression", "guardrail_integrity", "audit_completeness") with a
// pass/fail Status and supporting Detail explaining exactly what was
// evaluated and why it passed or failed -- never a bare boolean with no
// explanation.
type ReadinessCheck struct {
	// Dimension names which readiness dimension this check evaluates.
	Dimension DimensionName `json:"dimension"`

	// Status is this dimension's pass/fail outcome.
	Status CheckStatus `json:"status"`

	// Detail explains what was evaluated and, when Status is
	// CheckFailed, exactly what blocking condition was found (e.g. "2
	// critical findings still open: FIND-001, FIND-014"). Required
	// whenever Status is CheckFailed; encouraged (a short "all N
	// controls satisfied" summary) even when passed.
	Detail string `json:"detail"`

	// EvaluatedAt is when this specific dimension was evaluated.
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// Validate checks c for structural well-formedness.
func (c *ReadinessCheck) Validate() error {
	if c == nil {
		return ErrInvalidReadinessCheck
	}
	if !c.Dimension.IsValid() {
		return wrapf("ReadinessCheck.Validate", ErrInvalidReadinessCheck)
	}
	if !c.Status.IsValid() {
		return wrapf("ReadinessCheck.Validate", ErrInvalidReadinessCheck)
	}
	if c.Status == CheckFailed && strings.TrimSpace(c.Detail) == "" {
		return wrapf("ReadinessCheck.Validate", ErrInvalidReadinessCheck)
	}
	if c.EvaluatedAt.IsZero() {
		return wrapf("ReadinessCheck.Validate", ErrInvalidReadinessCheck)
	}
	return nil
}

// ReleaseReadiness is the aggregation of every ReadinessCheck
// Engine.CheckReadiness evaluated: the full platform's go/no-go
// snapshot for cutting a release. Ready is true only if every
// dimension passes -- fail closed, mirroring
// packages/guardrail.CanFinalize's and packages/dataresidency/
// packages/airgapped's Passed()-only-if-every-check-passed convention.
type ReleaseReadiness struct {
	// Checks is one ReadinessCheck per evaluated DimensionName. Never
	// empty for a readiness snapshot produced by CheckReadiness: every
	// named dimension is always evaluated and reported, never silently
	// skipped.
	Checks []ReadinessCheck `json:"checks"`

	// Ready is true only when every Checks entry has Status ==
	// CheckPassed. A ReleaseReadiness with zero Checks is never Ready --
	// see computeReady's doc comment.
	Ready bool `json:"ready"`

	// EvaluatedAt is when this snapshot was aggregated.
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// computeReady reports whether every check in checks passed. An empty
// checks slice is never considered ready -- a readiness snapshot with
// no dimensions evaluated is a configuration bug, not a vacuously green
// release, mirroring packages/iac.DeploymentVerificationReport.Passed's
// identical "no results is not passed" rule.
func computeReady(checks []ReadinessCheck) bool {
	if len(checks) == 0 {
		return false
	}
	for _, c := range checks {
		if c.Status != CheckPassed {
			return false
		}
	}
	return true
}

// FailedChecks returns the subset of r.Checks whose Status is
// CheckFailed, convenience for a caller that only wants the list of
// blocking dimensions.
func (r ReleaseReadiness) FailedChecks() []ReadinessCheck {
	out := make([]ReadinessCheck, 0)
	for _, c := range r.Checks {
		if c.Status == CheckFailed {
			out = append(out, c)
		}
	}
	return out
}

// CheckFor returns the ReadinessCheck for dimension and true, or a zero
// ReadinessCheck and false if no such dimension was evaluated in r.
func (r ReleaseReadiness) CheckFor(dimension DimensionName) (ReadinessCheck, bool) {
	for _, c := range r.Checks {
		if c.Dimension == dimension {
			return c, true
		}
	}
	return ReadinessCheck{}, false
}

// semverPattern is a pragmatic (not the full SemVer 2.0.0 BNF)
// validator for a release version string: MAJOR.MINOR.PATCH, with an
// optional -prerelease and/or +build metadata suffix, e.g. "1.4.0",
// "2.0.0-rc.1", "1.2.3+build.5". Good enough to reject "v1", "latest",
// or a blank string while accepting every version this package's own
// tests and doc/ga-release.md examples use.
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// IsValidSemver reports whether v is a well-formed MAJOR.MINOR.PATCH
// semantic version, optionally carrying a prerelease and/or build
// metadata suffix. Leading "v" (e.g. "v1.2.3") is not accepted --
// callers that carry a "v"-prefixed tag convention should strip it
// before validating, since ReleaseCandidate.Version is the bare
// semantic version, not a git tag name.
func IsValidSemver(v string) bool {
	return semverPattern.MatchString(v)
}

// isLowerHex reports whether s is non-empty and contains only
// lowercase hexadecimal digit characters, mirroring
// packages/cicdgate's identical helper of the same name (this package
// does not import that unexported helper, so it is duplicated exactly
// rather than exported from cicdgate just for this one use).
func isLowerHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// IsValidCommitSHA reports whether sha is a well-formed 40- or
// 64-character lowercase-hex git object ID (SHA-1 or SHA-256),
// mirroring packages/cicdgate.BuildAttestation.Validate's identical
// check on SourceCommit.
func IsValidCommitSHA(sha string) bool {
	return isLowerHex(sha) && (len(sha) == 40 || len(sha) == 64)
}

// ReleaseCandidate is a frozen snapshot of a specific commit, proposed
// as a release: its semver version, the exact commit SHA it was cut
// from, when it was frozen, and the ReleaseReadiness snapshot it was
// frozen against (task 4: freeze and tag release candidate -- see
// Engine.FreezeReleaseCandidate's doc comment for why this package
// itself never shells out to `git tag`).
//
// ReleaseCandidate is platform-global, not tenant-scoped: a software
// release is a single artifact shared by every tenant of this
// deployment, not a per-tenant record -- see doc/ga-release.md's
// "Persistence" section and this package's doc.go for the full
// rationale, mirroring packages/compliance.Control's identical
// shared-catalogue reasoning.
type ReleaseCandidate struct {
	// ID uniquely identifies this release candidate.
	ID uuid.UUID `json:"id"`

	// Version is the candidate's semantic version (e.g. "1.100.0-rc.1"),
	// validated by IsValidSemver.
	Version string `json:"version"`

	// CommitSHA is the exact git commit this candidate was cut from.
	CommitSHA string `json:"commit_sha"`

	// Readiness is the ReleaseReadiness snapshot this candidate was
	// frozen against -- FreezeReleaseCandidate refuses to freeze unless
	// Readiness.Ready is true at freeze time (task 4).
	Readiness ReleaseReadiness `json:"readiness"`

	// FrozenBy is the identity.User who froze this candidate.
	FrozenBy uuid.UUID `json:"frozen_by"`

	// FrozenAt is when this candidate was cut/frozen.
	FrozenAt time.Time `json:"frozen_at"`
}

// Validate checks c for structural well-formedness.
func (c *ReleaseCandidate) Validate() error {
	if c == nil {
		return ErrInvalidCandidate
	}
	if !IsValidSemver(c.Version) {
		return wrapf("ReleaseCandidate.Validate", ErrInvalidVersion)
	}
	if !IsValidCommitSHA(c.CommitSHA) {
		return wrapf("ReleaseCandidate.Validate", ErrInvalidCommitSHA)
	}
	if len(c.Readiness.Checks) == 0 {
		return wrapf("ReleaseCandidate.Validate", ErrInvalidCandidate)
	}
	if c.FrozenAt.IsZero() {
		return wrapf("ReleaseCandidate.Validate", ErrInvalidCandidate)
	}
	return nil
}

// Release is a frozen ReleaseCandidate promoted to general
// availability (task 8: cut GA release tag). Release models the
// software-side "cut" record only -- it does NOT and must not shell
// out to `git tag`; the actual git tagging is a separate, deliberate
// step the orchestrator performs after this record (and the PR that
// introduces it) has merged. See Engine.CutRelease's doc comment and
// doc/ga-release.md for the full boundary explanation.
//
// Like ReleaseCandidate, Release is platform-global, not
// tenant-scoped.
type Release struct {
	// ID uniquely identifies this release.
	ID uuid.UUID `json:"id"`

	// CandidateID references the ReleaseCandidate this release was cut
	// from.
	CandidateID uuid.UUID `json:"candidate_id"`

	// Version mirrors the candidate's Version at cut time.
	Version string `json:"version"`

	// CommitSHA mirrors the candidate's CommitSHA at cut time.
	CommitSHA string `json:"commit_sha"`

	// CutBy is the identity.User who cut this release.
	CutBy uuid.UUID `json:"cut_by"`

	// CutAt is when this release was cut.
	CutAt time.Time `json:"cut_at"`
}

// Validate checks r for structural well-formedness.
func (r *Release) Validate() error {
	if r == nil {
		return ErrInvalidRelease
	}
	if r.CandidateID == uuid.Nil {
		return wrapf("Release.Validate", ErrInvalidRelease)
	}
	if !IsValidSemver(r.Version) {
		return wrapf("Release.Validate", ErrInvalidVersion)
	}
	if !IsValidCommitSHA(r.CommitSHA) {
		return wrapf("Release.Validate", ErrInvalidCommitSHA)
	}
	if r.CutAt.IsZero() {
		return wrapf("Release.Validate", ErrInvalidRelease)
	}
	return nil
}
