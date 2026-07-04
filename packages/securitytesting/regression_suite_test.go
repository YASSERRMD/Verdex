package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewRegressionSuite_EveryScenarioPasses(t *testing.T) {
	t.Parallel()

	scenarios := securitytesting.NewRegressionSuite()
	if len(scenarios) == 0 {
		t.Fatal("NewRegressionSuite() returned zero scenarios")
	}

	h := securitytesting.NewHarness(scenarios...)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	for _, r := range records {
		if r.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("scenario %s outcome = %v, detail = %q, want OutcomePassed -- guardrail should hold against every adversarial attempt in this suite's default composition", r.ScenarioName, r.Result.Outcome, r.Result.Detail)
		}
	}
}

// TestScenarioGuardrailWhitespaceObfuscationKnownGap_CurrentlyFails
// documents, via a passing test of its own, the CURRENT expected state
// of the known gap this phase's adversarial testing found: as of this
// phase, packages/irac.ContainsVerdictLanguage has not yet been
// hardened against whitespace-obfuscated wordlist phrases, so this
// Scenario is expected to report OutcomeFailed. If a future change
// closes that gap (see doc.go's "Known gaps this suite found"
// section), this test should be updated to assert OutcomePassed
// instead -- at which point Engine.VerifyRemediation is the intended
// mechanism for confirming a tracked Finding against this Scenario.
func TestScenarioGuardrailWhitespaceObfuscationKnownGap_CurrentlyFails(t *testing.T) {
	t.Parallel()
	result, err := securitytesting.ScenarioGuardrailWhitespaceObfuscationKnownGap().Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Outcome != securitytesting.OutcomeFailed {
		t.Errorf("Outcome = %v, want OutcomeFailed (this is a documented, currently-open gap -- if this now passes, packages/irac has been hardened and this test should be updated to expect OutcomePassed)", result.Outcome)
	}
}

func TestScenarioGuardrailWhitespaceObfuscationKnownGap_ExcludedFromDefaultSuite(t *testing.T) {
	t.Parallel()
	for _, s := range securitytesting.NewRegressionSuite() {
		if s.Name() == "regression/guardrail-whitespace-obfuscation-known-gap" {
			t.Error("NewRegressionSuite() includes the known-gap scenario -- it must stay out of the default 'everything must pass' composition until the gap is closed")
		}
	}
}
