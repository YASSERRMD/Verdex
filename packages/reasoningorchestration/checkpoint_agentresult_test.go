package reasoningorchestration_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
)

// TestRun_CheckpointsCaptureAgentframeworkResult verifies that the four
// LLM-backed stages (issue framing, first-party arguments, second-party
// arguments, synthesis) persist their full agentframework.Result —
// including the Scratchpad recording every model call, tool call, and
// observation — on the Checkpoint saved for that stage, not just the
// stage's typed domain result. This is the data source
// packages/reasoningtrace depends on to assemble a complete audit trail.
func TestRun_CheckpointsCaptureAgentframeworkResult(t *testing.T) {
	f := newSeededFixture(t)
	p := &sequencedProvider{responses: []string{
		fakeIssueFramingJSON,
		fakeFirstPartyArgumentJSON,
		fakeSecondPartyArgumentJSON,
		fakeSynthesisJSON,
	}}
	r := newTestRouter(t, p)
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff", Label: "the plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant", Label: "the defendant"},
		LegalFamily: "common_law",
		Checkpoints: store,
	}

	result := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if result.Err != nil {
		t.Fatalf("Run: %v", result.Err)
	}

	ctx := context.Background()

	assertPopulatedRun := func(stage reasoningorchestration.Stage, get func(reasoningorchestration.Checkpoint) agentframework.Result) {
		t.Helper()
		cp, err := store.GetCheckpoint(ctx, f.caseID, stage)
		if err != nil {
			t.Fatalf("GetCheckpoint(%q): %v", stage, err)
		}
		run := get(cp)
		if run.CaseID != f.caseID {
			t.Fatalf("%s: Run.CaseID = %q, want %q", stage, run.CaseID, f.caseID)
		}
		if run.Termination != agentframework.TerminationConcluded {
			t.Fatalf("%s: Run.Termination = %q, want concluded", stage, run.Termination)
		}
		if run.Scratchpad == nil {
			t.Fatalf("%s: Run.Scratchpad is nil, want populated", stage)
		}
		if run.Scratchpad.StepCount() == 0 {
			t.Fatalf("%s: Run.Scratchpad.StepCount() = 0, want at least one step", stage)
		}
	}

	assertPopulatedRun(reasoningorchestration.StageIssueFraming, func(cp reasoningorchestration.Checkpoint) agentframework.Result {
		return cp.IssueFramingRun
	})
	assertPopulatedRun(reasoningorchestration.StageFirstPartyArguments, func(cp reasoningorchestration.Checkpoint) agentframework.Result {
		return cp.FirstPartyRun
	})
	assertPopulatedRun(reasoningorchestration.StageSecondPartyArguments, func(cp reasoningorchestration.Checkpoint) agentframework.Result {
		return cp.SecondPartyRun
	})
	assertPopulatedRun(reasoningorchestration.StageSynthesis, func(cp reasoningorchestration.Checkpoint) agentframework.Result {
		return cp.SynthesisRun
	})

	// Non-LLM stages must not carry a spuriously populated run: their
	// Checkpoint fields for these new fields should remain the zero
	// value since no agentframework.Result was ever produced for them.
	evCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageEvidenceWeighing)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageEvidenceWeighing): %v", err)
	}
	if evCP.IssueFramingRun.Scratchpad != nil {
		t.Fatalf("StageEvidenceWeighing checkpoint unexpectedly carries a non-nil IssueFramingRun.Scratchpad")
	}
}
