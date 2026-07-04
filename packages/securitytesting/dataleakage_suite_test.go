package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewDataLeakageSuite_EveryScenarioPasses(t *testing.T) {
	t.Parallel()

	scenarios := securitytesting.NewDataLeakageSuite()
	if len(scenarios) == 0 {
		t.Fatal("NewDataLeakageSuite() returned zero scenarios")
	}

	h := securitytesting.NewHarness(scenarios...)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	for _, r := range records {
		if r.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("scenario %s outcome = %v, detail = %q, want OutcomePassed -- cross-tenant/cross-case isolation should hold", r.ScenarioName, r.Result.Outcome, r.Result.Detail)
		}
		if r.ScenarioCategory != securitytesting.CategoryDataLeakage {
			t.Errorf("scenario %s category = %v, want CategoryDataLeakage", r.ScenarioName, r.ScenarioCategory)
		}
	}
}
