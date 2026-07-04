package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
)

// NewDomesticViolenceSetupToOpinionScenario builds the
// domestic-violence-case "setup to draft opinion" scenario: the same
// genuine, in-process journey, run for the domestic-violence case
// category. Domestic-violence matters are named explicitly in the
// brief's scenario list (task 1) and are frequently the category with
// the tightest sensitivity/urgency requirements in a real deployment,
// so this scenario exists to prove the identical journey machinery
// handles this category correctly, not just civil/criminal filings.
func NewDomesticViolenceSetupToOpinionScenario() (Scenario, error) {
	return NewScenarioFunc("domestic-violence/setup-to-opinion", category.CodeDomesticViolence, runDomesticViolenceSetupToOpinion)
}

func runDomesticViolenceSetupToOpinion(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	outcome, err := runFullJourney(ctx, "domestic-violence-setup-to-opinion", journeyOptions{
		JurisdictionCode: "GB",
		JurisdictionName: "UK Family Court",
		LegalFamily:      "common_law",
	})
	if err != nil {
		return ScenarioResult{}, err
	}

	result := outcome.toScenarioResult(ctx, startedAt, "")
	if result.Outcome != OutcomePassed {
		return result, nil
	}

	if result.ReasoningTermination != "complete" {
		result.Outcome = OutcomeFailed
		result.Detail = fmt.Sprintf("expected reasoning termination complete, got %q", result.ReasoningTermination)
		return result, nil
	}

	result.Detail = "domestic-violence setup-to-opinion journey completed"
	return result, nil
}
