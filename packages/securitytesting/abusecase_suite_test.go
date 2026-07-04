package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewAbuseCaseSuite_EveryScenarioPasses(t *testing.T) {
	t.Parallel()

	scenarios := securitytesting.NewAbuseCaseSuite()
	if len(scenarios) == 0 {
		t.Fatal("NewAbuseCaseSuite() returned zero scenarios")
	}

	h := securitytesting.NewHarness(scenarios...)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	for _, r := range records {
		if r.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("scenario %s outcome = %v, detail = %q, want OutcomePassed", r.ScenarioName, r.Result.Outcome, r.Result.Detail)
		}
		if r.ScenarioCategory != securitytesting.CategoryAbuseCase {
			t.Errorf("scenario %s category = %v, want CategoryAbuseCase", r.ScenarioName, r.ScenarioCategory)
		}
	}
}

func TestScenarioIntakeDailyQuotaAbuse_IndividuallyPasses(t *testing.T) {
	t.Parallel()
	result, err := securitytesting.ScenarioIntakeDailyQuotaAbuse().Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("Outcome = %v, detail = %q, want OutcomePassed", result.Outcome, result.Detail)
	}
}

func TestScenarioAuditReplayRejected_IndividuallyPasses(t *testing.T) {
	t.Parallel()
	result, err := securitytesting.ScenarioAuditReplayRejected().Run(t.Context())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("Outcome = %v, detail = %q, want OutcomePassed", result.Outcome, result.Detail)
	}
}
