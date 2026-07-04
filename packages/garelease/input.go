package garelease

import (
	"github.com/YASSERRMD/verdex/packages/compliance"
	"github.com/YASSERRMD/verdex/packages/perf"
	"github.com/YASSERRMD/verdex/packages/securitytesting"
	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

// E2EResult is a plain-data mirror of packages/e2e's
// SuiteRecord/AllPassed shape, copied field-by-field rather than
// aliased or imported. packages/e2e's own go.mod pulls in
// packages/ingestion, packages/reasoningorchestration,
// packages/signoff, packages/category, and the rest of that suite's
// transitive dependency tree -- everything a full-journey test needs,
// none of which this package's readiness-aggregation surface needs.
// Importing packages/e2e here purely to reuse a
// "scenario name + pass/fail + detail" shape would drag that entire
// tree into packages/garelease's own module graph for a single
// read-only report type.
//
// Task 5 ("run full regression and E2E") is satisfied by the
// already-existing CI gate: packages/e2e's suite already runs in CI as
// a blocking check (see .github/workflows/ci.yml) before this
// package's readiness check is ever consulted for a real release. This
// package does not re-invoke that suite -- it accepts the suite's
// outcome as an explicit, typed input, sourced from CI, exactly as
// this file's doc comment (and doc/ga-release.md) state.
//
// A caller holding real packages/e2e.SuiteRecord values adapts them
// with E2EResultFromCounts or by constructing E2EResult directly from
// e2e.AllPassed(records) and e2e.FailedRecords(records)/
// e2e.ErroredRecords(records) at the CI-orchestration layer that has
// packages/e2e in its own dependency graph already (a CI job script or
// a thin composition binary), not from within this package.
type E2EResult struct {
	// TotalScenarios is how many packages/e2e.Scenario entries the CI
	// suite run.
	TotalScenarios int `json:"total_scenarios"`

	// PassedScenarios is how many of those scenarios reported
	// e2e.OutcomePassed.
	PassedScenarios int `json:"passed_scenarios"`

	// FailedScenarioNames lists the e2e.Scenario.Name() of every
	// scenario that reported e2e.OutcomeFailed or e2e.OutcomeErrored --
	// both count as a blocking regression for release-readiness
	// purposes, mirroring packages/e2e.AllPassed's deliberately strict
	// "an errored run is not green" rule.
	FailedScenarioNames []string `json:"failed_scenario_names,omitempty"`

	// SourceCIRunURL is a reference to the CI run this result was
	// sourced from (e.g. a GitHub Actions run URL), so a reviewer of a
	// ReadinessCheck's Detail can trace back to the actual suite
	// execution rather than trusting an unverifiable claim.
	SourceCIRunURL string `json:"source_ci_run_url,omitempty"`
}

// AllPassed reports whether every scenario the CI suite ran passed,
// mirroring packages/e2e.AllPassed's exact semantics: zero total
// scenarios is never considered passed (an empty/misconfigured suite
// run is not evidence of a green regression pass).
func (r E2EResult) AllPassed() bool {
	return r.TotalScenarios > 0 && r.PassedScenarios == r.TotalScenarios && len(r.FailedScenarioNames) == 0
}

// ReadinessInput bundles every typed input Engine.CheckReadiness
// aggregates into one ReleaseReadiness snapshot. Findings/gap-analysis/
// budget-results are taken as small, already-computed report types
// from packages/vulnmanagement, packages/securitytesting,
// packages/compliance, and packages/perf directly (all four are light
// enough -- and already composed with by sibling phases such as
// packages/pilot -- that importing them does not create a
// dependency-tree explosion the way packages/e2e would; see
// E2EResult's doc comment for that contrasting case).
type ReadinessInput struct {
	// VulnFindings is every packages/vulnmanagement.Finding on file for
	// the platform as of this readiness evaluation (across every
	// tenant, since a release-readiness gate cares about the whole
	// deployment's outstanding risk, not one tenant's). Task 1's
	// "resolve all critical and high findings" reads from this slice.
	VulnFindings []vulnmanagement.Finding

	// SecurityTestingFindings is every packages/securitytesting.Finding
	// on file, feeding task 1 alongside VulnFindings -- two
	// independently-owned Finding types (see
	// securitytesting.Finding's doc comment), both checked.
	SecurityTestingFindings []securitytesting.Finding

	// ComplianceGapReport is the final packages/compliance.GapAnalysisReport
	// (task 2's "final compliance review"), typically produced by
	// compliance.Engine.RunGapAnalysis shortly before this readiness
	// check runs.
	ComplianceGapReport compliance.GapAnalysisReport

	// PerfVerdicts is every packages/perf.Verdict produced by evaluating
	// this release's benchmark measurements against their
	// packages/perf.Budget (task 3's "final performance and scale
	// validation"), typically produced by perf.Evaluate for each
	// perf.OperationName this platform budgets.
	PerfVerdicts []perf.Verdict

	// E2EResult is the outcome of the full-journey E2E suite's CI run
	// (task 5). See E2EResult's doc comment for why this is a plain
	// typed input rather than a live packages/e2e invocation.
	E2EResult E2EResult
}

// criticalOrHighVulnFindingsOpen returns every VulnFindings entry whose
// Severity is critical or high and whose Status is not yet terminal --
// task 1's precise definition of "still needs resolving" for
// packages/vulnmanagement.Finding. A finding in a terminal Status
// (Resolved, AcceptedRisk, or FalsePositive -- see
// vulnmanagement.Status.IsTerminal) is not a blocking finding even at
// critical/high severity: the remediation workflow has already run its
// course for it, whether by fixing it, formally accepting the risk, or
// determining it does not apply.
func criticalOrHighVulnFindingsOpen(findings []vulnmanagement.Finding) []vulnmanagement.Finding {
	out := make([]vulnmanagement.Finding, 0)
	for _, f := range findings {
		if f.Severity != vulnmanagement.SeverityCritical && f.Severity != vulnmanagement.SeverityHigh {
			continue
		}
		if f.Status.IsTerminal() {
			continue
		}
		out = append(out, f)
	}
	return out
}

// criticalOrHighSecurityFindingsOpen mirrors
// criticalOrHighVulnFindingsOpen for packages/securitytesting.Finding,
// using Finding.IsOpenLike (Open/Triaged/RemediationPending all count
// as still-outstanding work) rather than a terminal-status check, since
// securitytesting.FindingStatus's terminal set includes
// FindingVerifiedFixed and FindingRiskAccepted -- IsOpenLike already
// names exactly the "not yet resolved" predicate this dimension needs.
func criticalOrHighSecurityFindingsOpen(findings []securitytesting.Finding) []securitytesting.Finding {
	out := make([]securitytesting.Finding, 0)
	for _, f := range findings {
		if f.Severity != securitytesting.SeverityCritical && f.Severity != securitytesting.SeverityHigh {
			continue
		}
		if !f.IsOpenLike() {
			continue
		}
		out = append(out, f)
	}
	return out
}
