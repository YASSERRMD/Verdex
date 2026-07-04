package garelease

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CheckReadiness aggregates every named DimensionName into one
// ReleaseReadiness snapshot (Engine.CheckReadiness), requiring
// viewPermission: computing a readiness snapshot is a read-oriented
// aggregation over already-collected inputs, not a mutation, mirroring
// packages/pilot.ValidateNonBindingCompliance's identical
// view-permission-only rationale. Ready is true only if every
// dimension passes -- fail closed, per doc.go and types.go's
// ReleaseReadiness doc comment.
//
// CheckReadiness itself never returns a non-nil error for a merely
// unready platform: a Ready == false ReleaseReadiness IS the correct,
// successful result of evaluating a platform that is not yet ready to
// release. An error return here is reserved for genuine operational
// failures (missing permission, a nil Engine dependency an internal
// check needs).
func (e *Engine) CheckReadiness(ctx context.Context, input ReadinessInput) (ReleaseReadiness, error) {
	if err := authorizeView(ctx); err != nil {
		return ReleaseReadiness{}, err
	}

	now := e.now()
	checks := []ReadinessCheck{
		checkCriticalFindings(input, now),
		checkComplianceGaps(input, now),
		checkPerfBudget(input, now),
		checkE2ERegression(input, now),
	}

	guardrailCheck, err := e.checkGuardrailIntegrity(now)
	if err != nil {
		return ReleaseReadiness{}, wrapf("CheckReadiness", err)
	}
	checks = append(checks, guardrailCheck)

	if e.auditStore != nil {
		auditCheck, err := e.checkAuditCompleteness(ctx, now)
		if err != nil {
			return ReleaseReadiness{}, wrapf("CheckReadiness", err)
		}
		checks = append(checks, auditCheck)
	} else {
		checks = append(checks, ReadinessCheck{
			Dimension:   DimensionAuditCompleteness,
			Status:      CheckFailed,
			Detail:      "no audit store configured on this Engine -- audit completeness cannot be verified",
			EvaluatedAt: now,
		})
	}

	readiness := ReleaseReadiness{
		Checks:      checks,
		Ready:       computeReady(checks),
		EvaluatedAt: now,
	}

	if e.audit != nil {
		_, _ = e.audit.RecordReadinessCheck(ctx, actorFromCtx(ctx), readiness, nil)
	}

	return readiness, nil
}

// checkCriticalFindings implements DimensionCriticalFindings (task 1):
// fails if any packages/vulnmanagement.Finding or
// packages/securitytesting.Finding at critical/high severity is still
// outstanding.
func checkCriticalFindings(input ReadinessInput, now time.Time) ReadinessCheck {
	openVuln := criticalOrHighVulnFindingsOpen(input.VulnFindings)
	openSecurity := criticalOrHighSecurityFindingsOpen(input.SecurityTestingFindings)

	if len(openVuln) == 0 && len(openSecurity) == 0 {
		return ReadinessCheck{
			Dimension:   DimensionCriticalFindings,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("no open critical/high findings (%d vulnerability findings, %d security-testing findings evaluated)", len(input.VulnFindings), len(input.SecurityTestingFindings)),
			EvaluatedAt: now,
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d open critical/high finding(s) block release:", len(openVuln)+len(openSecurity))
	for _, f := range openVuln {
		fmt.Fprintf(&b, " vulnmanagement:%s(%s/%s)", f.ID, f.Severity, f.Status)
	}
	for _, f := range openSecurity {
		fmt.Fprintf(&b, " securitytesting:%s(%s/%s)", f.ID, f.Severity, f.Status)
	}

	return ReadinessCheck{
		Dimension:   DimensionCriticalFindings,
		Status:      CheckFailed,
		Detail:      b.String(),
		EvaluatedAt: now,
	}
}

// checkComplianceGaps implements DimensionComplianceGaps (task 2):
// fails if the supplied packages/compliance.GapAnalysisReport reports
// any StatusGap control.
func checkComplianceGaps(input ReadinessInput, now time.Time) ReadinessCheck {
	gaps := input.ComplianceGapReport.Gaps()
	if len(gaps) == 0 {
		return ReadinessCheck{
			Dimension:   DimensionComplianceGaps,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("compliance gap analysis reports zero gaps across %d evaluated control(s)", len(input.ComplianceGapReport.Results)),
			EvaluatedAt: now,
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d compliance control(s) still show a gap:", len(gaps))
	for _, g := range gaps {
		fmt.Fprintf(&b, " %s", g.Control.Code)
	}

	return ReadinessCheck{
		Dimension:   DimensionComplianceGaps,
		Status:      CheckFailed,
		Detail:      b.String(),
		EvaluatedAt: now,
	}
}

// checkPerfBudget implements DimensionPerfBudget (task 3): fails if any
// supplied packages/perf.Verdict did not pass, or if no verdicts were
// supplied at all (an unevaluated performance budget is not evidence of
// a passing one).
func checkPerfBudget(input ReadinessInput, now time.Time) ReadinessCheck {
	if len(input.PerfVerdicts) == 0 {
		return ReadinessCheck{
			Dimension:   DimensionPerfBudget,
			Status:      CheckFailed,
			Detail:      "no performance verdicts supplied -- perf budgets have not been evaluated for this release",
			EvaluatedAt: now,
		}
	}

	var failing []string
	for _, v := range input.PerfVerdicts {
		if !v.Passed {
			failing = append(failing, string(v.Operation))
		}
	}
	if len(failing) == 0 {
		return ReadinessCheck{
			Dimension:   DimensionPerfBudget,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("all %d budgeted operation(s) met their performance budget", len(input.PerfVerdicts)),
			EvaluatedAt: now,
		}
	}

	return ReadinessCheck{
		Dimension:   DimensionPerfBudget,
		Status:      CheckFailed,
		Detail:      fmt.Sprintf("%d of %d budgeted operation(s) missed their performance budget: %s", len(failing), len(input.PerfVerdicts), strings.Join(failing, ", ")),
		EvaluatedAt: now,
	}
}

// checkE2ERegression implements DimensionE2ERegression (task 5): fails
// unless the supplied E2EResult reports every scenario passed. See
// E2EResult's doc comment for why this is a typed input rather than a
// live packages/e2e invocation.
func checkE2ERegression(input ReadinessInput, now time.Time) ReadinessCheck {
	if input.E2EResult.AllPassed() {
		return ReadinessCheck{
			Dimension:   DimensionE2ERegression,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("full-journey E2E suite: %d/%d scenarios passed", input.E2EResult.PassedScenarios, input.E2EResult.TotalScenarios),
			EvaluatedAt: now,
		}
	}

	detail := fmt.Sprintf("full-journey E2E suite: %d/%d scenarios passed", input.E2EResult.PassedScenarios, input.E2EResult.TotalScenarios)
	if len(input.E2EResult.FailedScenarioNames) > 0 {
		detail = fmt.Sprintf("%s; failed/errored: %s", detail, strings.Join(input.E2EResult.FailedScenarioNames, ", "))
	}
	return ReadinessCheck{
		Dimension:   DimensionE2ERegression,
		Status:      CheckFailed,
		Detail:      detail,
		EvaluatedAt: now,
	}
}
