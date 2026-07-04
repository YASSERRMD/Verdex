package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
)

// NewConsumerSetupToOpinionScenario builds the consumer-case
// "setup to draft opinion" scenario: the same genuine, in-process
// journey, run for the consumer case category under a civil-law
// jurisdiction (Egypt), completing this package's coverage of every
// case category the brief's design guidance names by example
// (civil, criminal, domestic-violence, consumer) with at least one
// real, executing journey each.
func NewConsumerSetupToOpinionScenario() (Scenario, error) {
	return NewScenarioFunc("consumer/setup-to-opinion", category.CodeConsumer, runConsumerSetupToOpinion)
}

func runConsumerSetupToOpinion(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	outcome, err := runFullJourney(ctx, "consumer-setup-to-opinion", journeyOptions{
		JurisdictionCode: "EG",
		JurisdictionName: "Supreme Constitutional Court of Egypt",
		LegalFamily:      "civil_law",
	})
	if err != nil {
		return ScenarioResult{}, err
	}

	result := outcome.toScenarioResult(ctx, startedAt, "")
	if result.Outcome != OutcomePassed {
		return result, nil
	}

	if result.CompletedReasoningStages != 8 {
		result.Outcome = OutcomeFailed
		result.Detail = fmt.Sprintf("expected 8 completed reasoning stages, got %d", result.CompletedReasoningStages)
		return result, nil
	}

	result.Detail = "consumer setup-to-opinion journey completed for a civil-law jurisdiction"
	return result, nil
}
