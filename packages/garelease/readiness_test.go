package garelease_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
	"github.com/YASSERRMD/verdex/packages/garelease"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/perf"
	"github.com/YASSERRMD/verdex/packages/securitytesting"
	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

// greenInput builds a ReadinessInput with every dimension in a passing
// state, for tests that need a known-green starting point before
// mutating exactly one dimension to prove it alone flips Ready to
// false.
func greenInput() garelease.ReadinessInput {
	return garelease.ReadinessInput{
		VulnFindings:            nil,
		SecurityTestingFindings: nil,
		ComplianceGapReport: compliance.GapAnalysisReport{
			TenantID:    uuid.New(),
			GeneratedAt: time.Now().UTC(),
			Results: []compliance.ControlGapResult{
				{
					Control: compliance.Control{ID: uuid.New(), Code: "TEST-01", Title: "t", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis},
					Status:  compliance.StatusSatisfied,
				},
			},
		},
		PerfVerdicts: []perf.Verdict{
			{Operation: perf.OpHybridRetrieval, Passed: true},
			{Operation: perf.OpGraphTraversal, Passed: true},
		},
		E2EResult: garelease.E2EResult{
			TotalScenarios:  10,
			PassedScenarios: 10,
		},
	}
}

func TestCheckReadiness_RequiresPermission(t *testing.T) {
	engine := newTestEngine(t)
	_, err := engine.CheckReadiness(context.Background(), greenInput())
	if err == nil {
		t.Fatalf("CheckReadiness with no actor = nil error, want ErrUnauthenticated")
	}

	ctx := ctxWithUser(noPermUser())
	_, err = engine.CheckReadiness(ctx, greenInput())
	if err == nil {
		t.Fatalf("CheckReadiness with no-permission user = nil error, want ErrForbidden")
	}
}

func TestCheckReadiness_AllGreen(t *testing.T) {
	engine, ctx := newGreenEngine(t)

	readiness, err := engine.CheckReadiness(ctx, greenInput())
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if !readiness.Ready {
		t.Fatalf("CheckReadiness with all-green input: Ready = false, want true. Failed checks: %+v", readiness.FailedChecks())
	}
	if len(readiness.Checks) != 6 {
		t.Fatalf("len(Checks) = %d, want 6 (one per named dimension)", len(readiness.Checks))
	}
	for _, dim := range []garelease.DimensionName{
		garelease.DimensionCriticalFindings,
		garelease.DimensionComplianceGaps,
		garelease.DimensionPerfBudget,
		garelease.DimensionE2ERegression,
		garelease.DimensionGuardrailIntegrity,
		garelease.DimensionAuditCompleteness,
	} {
		check, ok := readiness.CheckFor(dim)
		if !ok {
			t.Errorf("dimension %s not evaluated", dim)
			continue
		}
		if check.Status != garelease.CheckPassed {
			t.Errorf("dimension %s: Status = %v, want CheckPassed (detail: %s)", dim, check.Status, check.Detail)
		}
	}
}

func TestCheckReadiness_ViewerCanCallWithoutManage(t *testing.T) {
	// RoleAuditor holds PermViewRelease (not PermManageRelease) and
	// PermAuditRead, so CheckReadiness should succeed and every
	// dimension -- including audit completeness, which itself requires
	// PermAuditRead under the hood -- should evaluate normally.
	engine, greenCtx := newGreenEngine(t)
	greenUser, ok := identity.UserFromContext(greenCtx)
	if !ok {
		t.Fatalf("newGreenEngine's context carries no user")
	}
	ctx := ctxWithUser(newTestUserForTenant(greenUser.TenantID, identity.RoleAuditor))

	readiness, err := engine.CheckReadiness(ctx, greenInput())
	if err != nil {
		t.Fatalf("CheckReadiness as auditor: %v", err)
	}
	if !readiness.Ready {
		t.Fatalf("CheckReadiness as auditor: Ready = false, want true. Failed: %+v", readiness.FailedChecks())
	}
}

func TestCheckReadiness_OpenCriticalFindingBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.VulnFindings = []vulnmanagement.Finding{
		{
			ID:           uuid.New(),
			TenantID:     uuid.New(),
			Source:       vulnmanagement.ScannerSourceSCA,
			Package:      "example.com/vulnerable/pkg",
			Severity:     vulnmanagement.SeverityCritical,
			AdvisoryID:   "CVE-2099-00001",
			Title:        "critical RCE",
			Status:       vulnmanagement.StatusOpen,
			DiscoveredAt: time.Now().UTC(),
		},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with an open critical finding: Ready = true, want false")
	}
	check, ok := readiness.CheckFor(garelease.DimensionCriticalFindings)
	if !ok || check.Status != garelease.CheckFailed {
		t.Fatalf("DimensionCriticalFindings = %+v, want CheckFailed", check)
	}
}

func TestCheckReadiness_ResolvedCriticalFindingDoesNotBlock(t *testing.T) {
	engine, ctx := newGreenEngine(t)

	input := greenInput()
	input.VulnFindings = []vulnmanagement.Finding{
		{
			ID:           uuid.New(),
			TenantID:     uuid.New(),
			Source:       vulnmanagement.ScannerSourceSCA,
			Package:      "example.com/vulnerable/pkg",
			Severity:     vulnmanagement.SeverityCritical,
			AdvisoryID:   "CVE-2099-00001",
			Title:        "critical RCE, now fixed",
			Status:       vulnmanagement.StatusResolved,
			DiscoveredAt: time.Now().UTC(),
		},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if !readiness.Ready {
		t.Fatalf("CheckReadiness with a resolved critical finding: Ready = false, want true. Failed: %+v", readiness.FailedChecks())
	}
}

func TestCheckReadiness_OpenSecurityTestingFindingBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.SecurityTestingFindings = []securitytesting.Finding{
		{
			ID:             uuid.New(),
			Title:          "authz bypass still open",
			Category:       securitytesting.CategoryAuthzBypass,
			Severity:       securitytesting.SeverityHigh,
			SourceScenario: "wrong-tenant-bypass",
			Status:         securitytesting.FindingOpen,
			OpenedAt:       time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with an open high security-testing finding: Ready = true, want false")
	}
}

func TestCheckReadiness_ComplianceGapBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.ComplianceGapReport = compliance.GapAnalysisReport{
		TenantID:    uuid.New(),
		GeneratedAt: time.Now().UTC(),
		Results: []compliance.ControlGapResult{
			{
				Control: compliance.Control{ID: uuid.New(), Code: "GAP-01", Title: "t", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis},
				Status:  compliance.StatusGap,
			},
		},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with a compliance gap: Ready = true, want false")
	}
	check, _ := readiness.CheckFor(garelease.DimensionComplianceGaps)
	if check.Status != garelease.CheckFailed {
		t.Fatalf("DimensionComplianceGaps.Status = %v, want CheckFailed", check.Status)
	}
}

func TestCheckReadiness_PerfBudgetMissBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.PerfVerdicts = []perf.Verdict{
		{Operation: perf.OpHybridRetrieval, Passed: false},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with a failing perf verdict: Ready = true, want false")
	}
}

func TestCheckReadiness_NoPerfVerdictsBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.PerfVerdicts = nil

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with zero perf verdicts: Ready = true, want false (an unevaluated budget is not a passing one)")
	}
}

func TestCheckReadiness_E2EFailureBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.E2EResult = garelease.E2EResult{
		TotalScenarios:      5,
		PassedScenarios:     4,
		FailedScenarioNames: []string{"civil/setup-to-opinion"},
	}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with an E2E failure: Ready = true, want false")
	}
	check, _ := readiness.CheckFor(garelease.DimensionE2ERegression)
	if check.Status != garelease.CheckFailed {
		t.Fatalf("DimensionE2ERegression.Status = %v, want CheckFailed", check.Status)
	}
}

func TestCheckReadiness_ZeroTotalScenariosBlocksRelease(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	input := greenInput()
	input.E2EResult = garelease.E2EResult{}

	readiness, err := engine.CheckReadiness(ctx, input)
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	if readiness.Ready {
		t.Fatalf("CheckReadiness with zero total E2E scenarios: Ready = true, want false")
	}
}

func TestCheckReadiness_NoAuditStoreConfiguredFailsClosed(t *testing.T) {
	// An Engine with no WithAuditTrailStore configured should report
	// DimensionAuditCompleteness as failed, not silently omit it or pass
	// it vacuously.
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	readiness, err := engine.CheckReadiness(ctx, greenInput())
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	check, ok := readiness.CheckFor(garelease.DimensionAuditCompleteness)
	if !ok {
		t.Fatalf("DimensionAuditCompleteness not evaluated")
	}
	if check.Status != garelease.CheckFailed {
		t.Fatalf("DimensionAuditCompleteness.Status = %v, want CheckFailed (no audit store configured)", check.Status)
	}
	if readiness.Ready {
		t.Fatalf("Ready = true with no audit store configured, want false")
	}
}

func TestCheckReadiness_WithAuditStoreConfigured(t *testing.T) {
	// newGreenEngine wires a real, seeded *auditlog.Store via
	// WithAuditTrailStore and returns a context scoped to the exact
	// tenant that store was seeded under, so this test exercises the
	// genuinely-passing path end to end.
	engine, ctx := newGreenEngine(t)

	readiness, err := engine.CheckReadiness(ctx, greenInput())
	if err != nil {
		t.Fatalf("CheckReadiness: %v", err)
	}
	check, ok := readiness.CheckFor(garelease.DimensionAuditCompleteness)
	if !ok {
		t.Fatalf("DimensionAuditCompleteness not evaluated")
	}
	if check.Status != garelease.CheckPassed {
		t.Fatalf("DimensionAuditCompleteness.Status = %v, want CheckPassed (detail: %s)", check.Status, check.Detail)
	}
	if !readiness.Ready {
		t.Fatalf("Ready = false with a real, intact audit store configured, want true. Failed: %+v", readiness.FailedChecks())
	}
}
