package securitytesting_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

// TestSelfTest_HarnessCorrectlyDistinguishesVulnerableFromFixed is this
// package's single most important test: it proves the harness
// mechanics themselves are not vacuous. VulnerableFixtureScenario and
// FixedFixtureScenario run the IDENTICAL assertion
// (probeAttackerCannotReadTenantASecret in selftest.go) against two
// fixtures that differ only in whether the isolation guard is actually
// wired in. A harness that always reported OutcomePassed regardless of
// input, or always OutcomeFailed, or that silently swallowed a
// scenario's result, would pass neither half of this test correctly --
// so this test is what proves the harness itself can fail, which is
// the load-bearing property this whole package depends on ("a harness
// that can't fail is worthless").
func TestSelfTest_HarnessCorrectlyDistinguishesVulnerableFromFixed(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		securitytesting.VulnerableFixtureScenario(),
		securitytesting.FixedFixtureScenario(),
	)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	if len(records) != 2 {
		t.Fatalf("RunAll() returned %d records, want exactly 2", len(records))
	}

	byName := make(map[string]securitytesting.RunRecord, len(records))
	for _, r := range records {
		byName[r.ScenarioName] = r
	}

	vuln, ok := byName["selftest/vulnerable-fixture-correctly-flagged"]
	if !ok {
		t.Fatal("no RunRecord for the vulnerable-fixture scenario")
	}
	if vuln.Result.Outcome != securitytesting.OutcomeFailed {
		t.Errorf("VulnerableFixtureScenario outcome = %v, want OutcomeFailed -- the harness must correctly flag a fixture with no isolation guard wired in", vuln.Result.Outcome)
	}

	fixed, ok := byName["selftest/fixed-fixture-correctly-passed"]
	if !ok {
		t.Fatal("no RunRecord for the fixed-fixture scenario")
	}
	if fixed.Result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("FixedFixtureScenario outcome = %v, want OutcomePassed -- the harness must correctly pass a fixture with the isolation guard correctly wired in", fixed.Result.Outcome)
	}
}

// TestSelfTest_ExcludedFromDefaultSeedHarness confirms the self-test
// pair does not leak into the default production composition
// (SeedHarness) -- a real CI gate running SeedHarness should never see
// a permanently-failing scenario that isn't a genuine, tracked defense
// gap.
func TestSelfTest_ExcludedFromDefaultSeedHarness(t *testing.T) {
	t.Parallel()

	h := securitytesting.SeedHarness()
	for _, s := range h.Scenarios() {
		if s.Name() == "selftest/vulnerable-fixture-correctly-flagged" {
			t.Error("SeedHarness() includes the self-test's deliberately-vulnerable scenario -- it must stay out of the default production composition")
		}
	}
}

// TestSelfTest_EndToEnd_FindingOpensAndVerifiesThroughEngine drives a
// togglable stand-in for "the vulnerable fixture, then the fixed
// fixture, under the identical Scenario Name" through the FULL Engine
// lifecycle -- RunSuite finds it failing, OpenFinding tracks it, and
// VerifyRemediation correctly refuses to mark it fixed while it is
// still vulnerable, then correctly marks it fixed once the underlying
// probe (delegating to VulnerableFixtureScenario/FixedFixtureScenario's
// own Run method, so this is still exercising the real self-test
// fixtures, not a separate hand-rolled pass/fail toggle) starts
// passing. This is the harness's self-test extended all the way
// through tasks 7/8's Finding/remediation-verification machinery, not
// just the bare Scenario.Run call.
func TestSelfTest_EndToEnd_FindingOpensAndVerifiesThroughEngine(t *testing.T) {
	t.Parallel()

	fixed := false // flips to true once the "fix" ships, below
	togglable := securitytesting.NewScenarioFunc(
		"selftest/end-to-end-togglable",
		securitytesting.CategoryRegression,
		func(ctx context.Context) (securitytesting.Result, error) {
			if fixed {
				return securitytesting.FixedFixtureScenario().Run(ctx)
			}
			return securitytesting.VulnerableFixtureScenario().Run(ctx)
		},
	)
	h := securitytesting.NewHarness(togglable)
	engine, _, tenantID := newTestEngineWithHarness(t, h)
	ctx := ctxWithUser(adminUser(tenantID))

	records, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	failed := securitytesting.FailedRecords(records)
	if len(failed) != 1 {
		t.Fatalf("RunSuite produced %d failed records, want exactly 1 (the still-vulnerable fixture)", len(failed))
	}

	finding, err := engine.OpenFinding(ctx, tenantID, failed[0], securitytesting.SeverityCritical)
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	finding, err = engine.TransitionFinding(ctx, tenantID, finding.ID, securitytesting.FindingTriaged, "")
	if err != nil {
		t.Fatalf("TransitionFinding(->triaged): %v", err)
	}
	finding, err = engine.TransitionFinding(ctx, tenantID, finding.ID, securitytesting.FindingRemediationPending, "")
	if err != nil {
		t.Fatalf("TransitionFinding(->remediation_pending): %v", err)
	}

	// Re-run while still vulnerable: must not verify.
	if _, _, err := engine.VerifyRemediation(ctx, tenantID, finding.ID); err == nil {
		t.Error("VerifyRemediation while still vulnerable returned nil error, want ErrRemediationNotVerified")
	}

	// "Ship the fix".
	fixed = true

	got, rerun, err := engine.VerifyRemediation(ctx, tenantID, finding.ID)
	if err != nil {
		t.Fatalf("VerifyRemediation after fix: %v", err)
	}
	if got.Status != securitytesting.FindingVerifiedFixed {
		t.Errorf("Finding.Status after fixed re-run = %v, want FindingVerifiedFixed", got.Status)
	}
	if rerun.Result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("re-run outcome = %v, want OutcomePassed", rerun.Result.Outcome)
	}
}
