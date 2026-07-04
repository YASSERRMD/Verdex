package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
)

// NewCivilSetupToOpinionScenario builds the civil-case
// "setup to draft opinion" scenario (task 2's centerpiece): a genuine,
// in-process journey composing packages/ingestion's orchestrator,
// packages/reasoningorchestration's full pipeline, and (implicitly,
// via the fail-closed default) packages/signoff's SignoffGate
// extension point, for a civil case category, common-law jurisdiction.
//
// This scenario asserts real properties of the journey it just ran:
// ingestion reached StageComplete, the reasoning pipeline reached
// TerminationComplete with all eight stages recorded, and -- since no
// sign-off has been recorded for this fresh case -- the guardrail
// check correctly reports GuardrailApproved == false (the fail-closed
// default). This is not a description of what the pipeline should do;
// it is the real pipeline actually doing it.
func NewCivilSetupToOpinionScenario() (Scenario, error) {
	return NewScenarioFunc("civil/setup-to-opinion", category.CodeCivil, runCivilSetupToOpinion)
}

func runCivilSetupToOpinion(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	outcome, err := runFullJourney(ctx, "civil-setup-to-opinion", journeyOptions{
		JurisdictionCode: "US-CA",
		JurisdictionName: "California Superior Court",
		LegalFamily:      "common_law",
	})
	if err != nil {
		return ScenarioResult{}, err
	}

	result := outcome.toScenarioResult(ctx, startedAt)
	if result.Outcome != OutcomePassed {
		return result, nil
	}

	if result.CompletedReasoningStages != 8 {
		result.Outcome = OutcomeFailed
		result.Detail = fmt.Sprintf("expected 8 completed reasoning stages, got %d", result.CompletedReasoningStages)
		return result, nil
	}
	if result.GuardrailApproved {
		result.Outcome = OutcomeFailed
		result.Detail = "expected GuardrailApproved=false for a case with no recorded sign-off (fail-closed default), got true"
		return result, nil
	}

	result.Detail = "civil setup-to-opinion journey completed: ingestion, reasoning pipeline, and fail-closed guardrail check all verified"
	return result, nil
}
