package reasoningorchestration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
)

// newSeededFixture builds a fixture with one issue, one governing rule,
// and one supporting fact — enough for every stage in the pipeline to
// produce a non-trivial result.
func newSeededFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.seedIssue(t, "issue-1", "Was the contract validly formed?", 0.9)
	f.seedRule(t, "rule-1", "Statute of Frauds requires written evidence for contracts over $500.", "US-CA", "common_law", 0.8)
	f.seedGoverns(t, "rule-1", "issue-1")
	f.seedFact(t, "fact-1", "The parties signed a written memorandum.", 0.75)
	return f
}

func TestRun_EndToEnd_FullPipelineCompletes(t *testing.T) {
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
	if result.State.Termination != reasoningorchestration.TerminationComplete {
		t.Fatalf("Termination = %q, want complete", result.State.Termination)
	}
	if result.State.CurrentStage != reasoningorchestration.StageComplete {
		t.Fatalf("CurrentStage = %q, want complete", result.State.CurrentStage)
	}
	if len(result.State.CompletedStages) != 8 {
		t.Fatalf("len(CompletedStages) = %d, want 8: %v", len(result.State.CompletedStages), result.State.CompletedStages)
	}

	// Every stage's typed artifact must be retrievable from the
	// CheckpointStore after the run — this is both the resume mechanism
	// and the audit trail the plan requires.
	ctx := context.Background()
	issueCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageIssueFraming)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageIssueFraming): %v", err)
	}
	if len(issueCP.IssueAnalysis.Issues) != 1 {
		t.Fatalf("IssueAnalysis.Issues = %d, want 1", len(issueCP.IssueAnalysis.Issues))
	}

	fpCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageFirstPartyArguments)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageFirstPartyArguments): %v", err)
	}
	if len(fpCP.FirstPartyArguments.Arguments) == 0 {
		t.Fatalf("FirstPartyArguments.Arguments is empty")
	}

	spCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageSecondPartyArguments)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageSecondPartyArguments): %v", err)
	}
	if len(spCP.SecondPartyArguments.Arguments) == 0 {
		t.Fatalf("SecondPartyArguments.Arguments is empty")
	}

	evCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageEvidenceWeighing)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageEvidenceWeighing): %v", err)
	}
	if len(evCP.Evidence.FactWeights) == 0 {
		t.Fatalf("Evidence.FactWeights is empty")
	}

	lawCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageLawApplication)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageLawApplication): %v", err)
	}
	if len(lawCP.Law.IssueApplications) != 1 {
		t.Fatalf("Law.IssueApplications = %d, want 1", len(lawCP.Law.IssueApplications))
	}

	synthCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageSynthesis)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageSynthesis): %v", err)
	}
	if len(synthCP.Opinion.Conclusions) != 1 {
		t.Fatalf("Opinion.Conclusions = %d, want 1", len(synthCP.Opinion.Conclusions))
	}

	uncCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageUncertaintySurfacing)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageUncertaintySurfacing): %v", err)
	}
	_ = uncCP.Uncertainty // Report may legitimately have zero Uncertainties; presence of the checkpoint is what matters.

	guardCP, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageGuardrailCheck)
	if err != nil {
		t.Fatalf("GetCheckpoint(StageGuardrailCheck): %v", err)
	}
	if guardCP.GuardrailApproved {
		t.Fatalf("GuardrailApproved = true, want false (no sign-off gate configured, fail-closed default)")
	}

	savedState, err := store.GetRunState(ctx, f.caseID)
	if err != nil {
		t.Fatalf("GetRunState: %v", err)
	}
	if savedState.Termination != reasoningorchestration.TerminationComplete {
		t.Fatalf("saved RunState.Termination = %q, want complete", savedState.Termination)
	}
}

func TestRun_MidPipelineFailure_StopsCleanlyWithEarlierCheckpointsIntact(t *testing.T) {
	f := newSeededFixture(t)
	// Fail exactly at the synthesis call (the 4th Chat call: issue
	// framing, first-party, second-party, then synthesis).
	base := &sequencedProvider{responses: []string{
		fakeIssueFramingJSON,
		fakeFirstPartyArgumentJSON,
		fakeSecondPartyArgumentJSON,
		fakeSynthesisJSON,
	}}
	injected := errors.New("injected synthesis provider failure")
	p := &failAtCallProvider{sequencedProvider: base, failAt: 3, failErr: injected}
	r := newTestRouter(t, p)
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		LegalFamily: "common_law",
		Checkpoints: store,
	}

	result := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if result.Err == nil {
		t.Fatalf("Run: want error, got nil")
	}
	if result.State.Termination != reasoningorchestration.TerminationFailed {
		t.Fatalf("Termination = %q, want failed", result.State.Termination)
	}
	if result.State.FailedStage != reasoningorchestration.StageSynthesis {
		t.Fatalf("FailedStage = %q, want %q", result.State.FailedStage, reasoningorchestration.StageSynthesis)
	}
	if result.State.FailureReason == "" {
		t.Fatalf("FailureReason is empty")
	}

	// Earlier stages' checkpoints must survive the later failure.
	ctx := context.Background()
	if _, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageIssueFraming); err != nil {
		t.Fatalf("GetCheckpoint(StageIssueFraming) after failure: %v", err)
	}
	if _, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageEvidenceWeighing); err != nil {
		t.Fatalf("GetCheckpoint(StageEvidenceWeighing) after failure: %v", err)
	}
	if _, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageLawApplication); err != nil {
		t.Fatalf("GetCheckpoint(StageLawApplication) after failure: %v", err)
	}

	// Synthesis and every later stage must NOT have run.
	if _, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageSynthesis); !errors.Is(err, reasoningorchestration.ErrCheckpointNotFound) {
		t.Fatalf("GetCheckpoint(StageSynthesis) = %v, want ErrCheckpointNotFound", err)
	}
	if _, err := store.GetCheckpoint(ctx, f.caseID, reasoningorchestration.StageUncertaintySurfacing); !errors.Is(err, reasoningorchestration.ErrCheckpointNotFound) {
		t.Fatalf("GetCheckpoint(StageUncertaintySurfacing) = %v, want ErrCheckpointNotFound", err)
	}

	savedState, err := store.GetRunState(ctx, f.caseID)
	if err != nil {
		t.Fatalf("GetRunState: %v", err)
	}
	if savedState.Termination != reasoningorchestration.TerminationFailed {
		t.Fatalf("saved RunState.Termination = %q, want failed", savedState.Termination)
	}
}

func TestResume_SkipsCompletedStages(t *testing.T) {
	f := newSeededFixture(t)
	base := &sequencedProvider{responses: []string{
		fakeIssueFramingJSON,
		fakeFirstPartyArgumentJSON,
		fakeSecondPartyArgumentJSON,
		fakeSynthesisJSON,
	}}
	injected := errors.New("injected synthesis provider failure")
	p := &failAtCallProvider{sequencedProvider: base, failAt: 3, failErr: injected}
	r := newTestRouter(t, p)
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		LegalFamily: "common_law",
		Checkpoints: store,
	}

	first := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if first.State.Termination != reasoningorchestration.TerminationFailed {
		t.Fatalf("initial Run Termination = %q, want failed", first.State.Termination)
	}

	// "Fix" the outage: the underlying provider's next call (call index 3,
	// synthesis) will now succeed rather than erroring, since
	// failAtCallProvider only fails once per configured index and the
	// sequencedProvider underneath already has the synthesis response
	// queued at that index.
	p.failAt = -1

	resumed := reasoningorchestration.Resume(authedContext(), f.caseID, cfg)
	if resumed.Err != nil {
		t.Fatalf("Resume: %v", resumed.Err)
	}
	if resumed.State.Termination != reasoningorchestration.TerminationComplete {
		t.Fatalf("Resume Termination = %q, want complete", resumed.State.Termination)
	}
	if len(resumed.State.CompletedStages) != 8 {
		t.Fatalf("len(CompletedStages) after resume = %d, want 8", len(resumed.State.CompletedStages))
	}

	// The underlying provider must have been called exactly 4 times total
	// (issue framing, first-party, second-party, synthesis) across BOTH
	// Run and Resume — Resume must not have repeated any already-completed
	// LLM-agent stage's call.
	if base.calls != 4 {
		t.Fatalf("provider calls across Run+Resume = %d, want 4 (no re-run of completed stages)", base.calls)
	}
}

func TestResume_NoPriorRun_ReturnsErrRunNotFound(t *testing.T) {
	f := newSeededFixture(t)
	r := newTestRouter(t, &sequencedProvider{})
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		Checkpoints: store,
	}

	result := reasoningorchestration.Resume(authedContext(), f.caseID, cfg)
	if !errors.Is(result.Err, reasoningorchestration.ErrRunNotFound) {
		t.Fatalf("Resume err = %v, want ErrRunNotFound", result.Err)
	}
}

func TestRun_BudgetExhausted_HaltsBeforeNextStage(t *testing.T) {
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
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		LegalFamily: "common_law",
		Checkpoints: store,
		Budget: reasoningorchestration.PipelineBudget{
			// A budget this tight is exceeded well before the 8-stage
			// pipeline can finish, but StageIssueFraming (the first
			// stage) is still allowed to start since elapsed time is
			// ~0 at the very first check.
			MaxTotalWallClock: 1 * time.Nanosecond,
		},
	}

	result := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if result.Err == nil {
		t.Fatalf("Run: want error, got nil")
	}
	if !errors.Is(result.Err, reasoningorchestration.ErrBudgetExhausted) {
		t.Fatalf("Run err = %v, want ErrBudgetExhausted", result.Err)
	}
	if result.State.Termination != reasoningorchestration.TerminationBudgetExhausted {
		t.Fatalf("Termination = %q, want budget_exhausted", result.State.Termination)
	}
	// A budget halt must occur BETWEEN stages, never mid-stage: no stage
	// should have been recorded as completed given a ~0 budget checked
	// before the very first stage even starts.
	if len(result.State.CompletedStages) != 0 {
		t.Fatalf("CompletedStages = %v, want none (budget exhausted before first stage)", result.State.CompletedStages)
	}
}

func TestRun_EmptyCaseID_ReturnsError(t *testing.T) {
	f := newSeededFixture(t)
	r := newTestRouter(t, &sequencedProvider{})
	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		Checkpoints: reasoningorchestration.NewInMemoryCheckpointStore(),
	}

	result := reasoningorchestration.Run(authedContext(), "", cfg)
	if !errors.Is(result.Err, reasoningorchestration.ErrEmptyCaseID) {
		t.Fatalf("Run err = %v, want ErrEmptyCaseID", result.Err)
	}
}

func TestRun_NilDependency_ReturnsErrNilRunConfig(t *testing.T) {
	f := newSeededFixture(t)
	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		Checkpoints: reasoningorchestration.NewInMemoryCheckpointStore(),
		// Router intentionally nil.
	}

	result := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if !errors.Is(result.Err, reasoningorchestration.ErrNilRunConfig) {
		t.Fatalf("Run err = %v, want ErrNilRunConfig", result.Err)
	}
}

// TestRun_Telemetry_RecordsEveryStage is a smoke test that Run's
// per-stage telemetry-recording code path executes without panicking and
// without affecting the run's outcome. The telemetry.Stats value itself
// is currently internal (accessed only through StageTelemetry values
// this package records for its own future Stats()/Telemetry() accessor);
// this test instead exercises Run end-to-end once more to guard against a
// regression in the recorder's bookkeeping causing a panic.
func TestRun_Telemetry_DoesNotAffectOutcome(t *testing.T) {
	f := newSeededFixture(t)
	p := &sequencedProvider{responses: []string{
		fakeIssueFramingJSON,
		fakeFirstPartyArgumentJSON,
		fakeSecondPartyArgumentJSON,
		fakeSynthesisJSON,
	}}
	r := newTestRouter(t, p)
	cfg := reasoningorchestration.RunConfig{
		API:         f.api,
		Router:      r,
		FirstParty:  reasoningorchestration.PartyConfig{ID: "plaintiff"},
		SecondParty: reasoningorchestration.PartyConfig{ID: "defendant"},
		LegalFamily: "common_law",
		Checkpoints: reasoningorchestration.NewInMemoryCheckpointStore(),
	}

	result := reasoningorchestration.Run(authedContext(), f.caseID, cfg)
	if result.Err != nil {
		t.Fatalf("Run: %v", result.Err)
	}
	stats := result.Stats()
	if len(stats.PerStage) != 8 {
		t.Fatalf("len(Stats().PerStage) = %d, want 8", len(stats.PerStage))
	}
	if stats.TotalWallClock <= 0 {
		t.Fatalf("Stats().TotalWallClock = %v, want > 0", stats.TotalWallClock)
	}
	for i, st := range stats.PerStage {
		if st.Err != nil {
			t.Fatalf("PerStage[%d] (%q) unexpectedly errored: %v", i, st.Stage, st.Err)
		}
	}
}

// TestRun_UsesAgentframeworkBudgetType is a compile-time-flavored
// assertion that PipelineBudget.PerStageBudget really is an
// agentframework.Budget, so RunConfig stays wired to the same type every
// LLM-agent stage's own Config accepts.
func TestRun_UsesAgentframeworkBudgetType(t *testing.T) {
	var b reasoningorchestration.PipelineBudget
	b.PerStageBudget = agentframework.Budget{MaxSteps: 5}
	if b.PerStageBudget.MaxSteps != 5 {
		t.Fatalf("PerStageBudget.MaxSteps = %d, want 5", b.PerStageBudget.MaxSteps)
	}
}
