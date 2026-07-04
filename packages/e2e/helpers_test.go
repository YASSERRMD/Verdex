package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/e2e"
)

// requireScenario constructs scenario via build, failing the test
// immediately if construction itself errors -- every scenario
// constructor in this package should succeed with the parameters this
// suite's own tests supply.
func requireScenario(t *testing.T, build func() (e2e.Scenario, error)) e2e.Scenario {
	t.Helper()
	sc, err := build()
	if err != nil {
		t.Fatalf("scenario construction failed: %v", err)
	}
	return sc
}

// runAndRequirePassed runs sc against a background context with a
// generous timeout (the discard-guarantee scenario alone waits several
// hundred milliseconds for a real TTL to elapse) and fails the test
// with the full ScenarioResult detail if the scenario did not pass.
func runAndRequirePassed(t *testing.T, sc e2e.Scenario) e2e.ScenarioResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sc.Run(ctx)
	if err != nil {
		t.Fatalf("%s: Run returned error: %v", sc.Name(), err)
	}
	if result.Outcome != e2e.OutcomePassed {
		t.Fatalf("%s: Outcome = %q, want %q. Detail: %s", sc.Name(), result.Outcome, e2e.OutcomePassed, result.Detail)
	}
	return result
}
