package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewAuthzBypassSuite_EveryScenarioPasses(t *testing.T) {
	t.Parallel()

	scenarios := securitytesting.NewAuthzBypassSuite()
	if len(scenarios) == 0 {
		t.Fatal("NewAuthzBypassSuite() returned zero scenarios")
	}

	h := securitytesting.NewHarness(scenarios...)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	for _, r := range records {
		if r.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("scenario %s outcome = %v, detail = %q, want OutcomePassed -- every constructed bypass attempt should be correctly rejected", r.ScenarioName, r.Result.Outcome, r.Result.Detail)
		}
		if r.ScenarioCategory != securitytesting.CategoryAuthzBypass {
			t.Errorf("scenario %s category = %v, want CategoryAuthzBypass", r.ScenarioName, r.ScenarioCategory)
		}
	}
}

func TestScenarioExpiredGrantStillDenied_IndividuallyPasses(t *testing.T) {
	t.Parallel()
	result, err := securitytesting.ScenarioExpiredGrantStillDenied().Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("Outcome = %v, detail = %q, want OutcomePassed", result.Outcome, result.Detail)
	}
}

func TestScenarioWrongTenantEvaluateRejected_IndividuallyPasses(t *testing.T) {
	t.Parallel()
	result, err := securitytesting.ScenarioWrongTenantEvaluateRejected().Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("Outcome = %v, detail = %q, want OutcomePassed", result.Outcome, result.Detail)
	}
}
