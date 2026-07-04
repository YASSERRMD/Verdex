package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
)

// NewCriminalSetupToOpinionScenario builds the criminal-case
// "setup to draft opinion" scenario: the same genuine, in-process
// journey as NewCivilSetupToOpinionScenario, run for a criminal case
// category, mixed-legal-family jurisdiction (UAE), so this package's
// journey composition is proven against a second, distinct case
// category and legal family, not only the civil/common-law pairing.
func NewCriminalSetupToOpinionScenario() (Scenario, error) {
	return NewScenarioFunc("criminal/setup-to-opinion", category.CodeCriminal, runCriminalSetupToOpinion)
}

func runCriminalSetupToOpinion(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	outcome, err := runFullJourney(ctx, "criminal-setup-to-opinion", journeyOptions{
		JurisdictionCode: "AE",
		JurisdictionName: "Dubai Courts (High Criminal Court)",
		LegalFamily:      "mixed",
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
	if result.IngestionStage != "complete" {
		result.Outcome = OutcomeFailed
		result.Detail = fmt.Sprintf("expected ingestion stage complete, got %q", result.IngestionStage)
		return result, nil
	}

	result.Detail = "criminal setup-to-opinion journey completed for a mixed-legal-family jurisdiction"
	return result, nil
}
