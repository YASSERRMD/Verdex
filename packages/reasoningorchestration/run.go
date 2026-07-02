package reasoningorchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

// nowFunc is overridden in tests for deterministic timestamps.
var nowFunc = time.Now

// Run drives caseID through every Stage in stageOrder, from scratch:
// issue framing, first- and second-party arguments, evidence weighing,
// law application, synthesis, uncertainty surfacing, and the guardrail
// check. Each stage's output is checkpointed via cfg.Checkpoints as soon
// as it completes (see doc/reasoning-orchestration.md for exactly what
// is persisted and when).
//
// Run stops cleanly — without attempting any later stage — the moment a
// stage fails or the pipeline's overall wall-clock budget
// (cfg.Budget.MaxTotalWallClock) would be exceeded by starting the next
// stage. Both cases are reported via the returned RunResult rather than a
// panic: RunResult.State.Termination distinguishes "failed at stage X"
// from "budget exhausted before stage X" from "complete", and
// RunResult.Err is non-nil in both non-complete cases.
//
// Run always starts from StageIssueFraming. A caller resuming a
// previously interrupted run should call Resume instead, which skips
// every already-completed stage found in cfg.Checkpoints.
func Run(ctx context.Context, caseID string, cfg RunConfig) RunResult {
	if caseID == "" {
		return RunResult{Err: ErrEmptyCaseID}
	}
	if err := cfg.validate(); err != nil {
		return RunResult{Err: err}
	}

	state := RunState{
		CaseID:       caseID,
		CurrentStage: StageIssueFraming,
		Termination:  TerminationRunning,
		StartedAt:    nowFunc(),
		UpdatedAt:    nowFunc(),
	}
	return drive(ctx, caseID, cfg, state)
}

// Resume continues a previously started run for caseID: it loads the
// last-persisted RunState from cfg.Checkpoints, skips every stage already
// present in RunState.CompletedStages, and drives the pipeline forward
// from the first incomplete stage. Returns ErrRunNotFound if no RunState
// was ever checkpointed for caseID.
//
// Resume is the crash/restart recovery path this package exists to
// provide: every LLM-agent stage (issue framing, both argument stages,
// synthesis) is an expensive, billed call, so re-running a stage whose
// checkpoint already exists would silently double that cost. Resume
// instead reads that stage's Checkpoint back out and moves straight to
// the next incomplete stage.
func Resume(ctx context.Context, caseID string, cfg RunConfig) RunResult {
	if caseID == "" {
		return RunResult{Err: ErrEmptyCaseID}
	}
	if err := cfg.validate(); err != nil {
		return RunResult{Err: err}
	}

	state, err := cfg.Checkpoints.GetRunState(ctx, caseID)
	if err != nil {
		return RunResult{Err: err}
	}

	if state.Termination == TerminationComplete {
		return RunResult{State: state}
	}

	// A resumed run continues accruing wall-clock budget from its
	// original StartedAt, not from now: MaxTotalWallClock bounds the
	// case's total processing time across every attempt, not any single
	// process's uptime.
	state.Termination = TerminationRunning
	state.UpdatedAt = nowFunc()
	return drive(ctx, caseID, cfg, state)
}

// pipelineContext accumulates every stage's typed output as the pipeline
// advances, so each stage's entrypoint call can be built from the prior
// stages' real results (freshly computed this run, or loaded back from a
// checkpoint on Resume — see loadCompletedStageOutputs).
type pipelineContext struct {
	issues      issueagent.IssueAnalysisResult
	firstParty  firstpartyagent.ArgumentSet
	secondParty secondpartyagent.ArgumentSet
	evidence    evidenceweighing.Result
	law         lawapplication.Result
	opinion     synthesisagent.Opinion
	uncertainty uncertainty.Report
}

// drive runs every stage from state.CurrentStage through StageComplete
// (or until failure/budget exhaustion), checkpointing as it goes. It is
// shared by Run (starting state) and Resume (a loaded, partially
// completed state).
func drive(ctx context.Context, caseID string, cfg RunConfig, state RunState) RunResult {
	budget := cfg.Budget.withDefaults()
	telemetry := newTelemetryRecorder()

	pc, err := loadCompletedStageOutputs(ctx, cfg, caseID, state.CompletedStages)
	if err != nil {
		return failRun(ctx, cfg, state, StageIssueFraming, err)
	}

	for _, stage := range stageOrder {
		if isStageComplete(state.CompletedStages, stage) {
			continue
		}

		elapsed := nowFunc().Sub(state.StartedAt)
		if elapsed >= budget.MaxTotalWallClock {
			state.CurrentStage = stage
			state.Termination = TerminationBudgetExhausted
			state.UpdatedAt = nowFunc()
			persistRunState(ctx, cfg, state)
			return RunResult{State: state, Err: fmt.Errorf("%w: stage %q not started after %s", ErrBudgetExhausted, stage, elapsed)}
		}

		stageStart := nowFunc()
		checkpoint, stageErr := runStage(ctx, cfg, caseID, stage, &pc)
		duration := nowFunc().Sub(stageStart)

		telemetry.record(StageTelemetry{Stage: stage, Duration: duration, Err: stageErr})

		if stageErr != nil {
			return failRun(ctx, cfg, state, stage, stageErr)
		}

		persistCheckpointAsync(cfg, caseID, checkpoint)

		state.CompletedStages = append(state.CompletedStages, stage)
		next, nextErr := nextStage(stage)
		if nextErr != nil {
			return failRun(ctx, cfg, state, stage, nextErr)
		}
		state.CurrentStage = next
		state.UpdatedAt = nowFunc()
		persistRunState(ctx, cfg, state)
	}

	state.Termination = TerminationComplete
	state.CurrentStage = StageComplete
	state.UpdatedAt = nowFunc()
	persistRunState(ctx, cfg, state)

	return RunResult{State: state}
}

// failRun finalizes state as TerminationFailed at failedStage, persists
// it, and returns the corresponding RunResult.
func failRun(ctx context.Context, cfg RunConfig, state RunState, failedStage Stage, err error) RunResult {
	state.Termination = TerminationFailed
	state.FailedStage = failedStage
	state.FailureReason = err.Error()
	state.UpdatedAt = nowFunc()
	persistRunState(ctx, cfg, state)
	return RunResult{State: state, Err: err}
}

// persistRunState saves state to cfg.Checkpoints, swallowing (not
// panicking on) a persistence error: checkpoint/telemetry persistence is
// documented as fire-and-forget best effort (see doc/
// reasoning-orchestration.md's concurrency section) so a transient store
// failure never masks the pipeline's own, more important stage error.
func persistRunState(ctx context.Context, cfg RunConfig, state RunState) {
	_ = cfg.Checkpoints.SaveRunState(ctx, state.Clone())
}

// persistCheckpointAsync saves checkpoint for caseID in a separate
// goroutine: checkpoint persistence has no ordering dependency on the
// next stage starting (the next stage reads from pipelineContext, an
// in-process value, never from cfg.Checkpoints), so making it
// fire-and-forget keeps a slow or momentarily-unavailable
// CheckpointStore off the pipeline's critical path. See doc/
// reasoning-orchestration.md for why this is safe: each goroutine only
// ever touches its own Checkpoint value (no shared mutable state) and
// InMemoryCheckpointStore (and any real implementation) is expected to
// serialize its own internal state.
func persistCheckpointAsync(cfg RunConfig, caseID string, checkpoint Checkpoint) {
	go func() {
		_ = cfg.Checkpoints.SaveCheckpoint(context.Background(), caseID, checkpoint)
	}()
}

// runStage dispatches to the concrete stage implementation for stage,
// mutating pc with that stage's output on success and returning the
// Checkpoint to persist.
func runStage(ctx context.Context, cfg RunConfig, caseID string, stage Stage, pc *pipelineContext) (Checkpoint, error) {
	switch stage {
	case StageIssueFraming:
		return runIssueFraming(ctx, cfg, caseID, pc)
	case StageFirstPartyArguments:
		return runFirstPartyArguments(ctx, cfg, caseID, pc)
	case StageSecondPartyArguments:
		return runSecondPartyArguments(ctx, cfg, caseID, pc)
	case StageEvidenceWeighing:
		return runEvidenceWeighing(ctx, cfg, caseID, pc)
	case StageLawApplication:
		return runLawApplication(ctx, cfg, caseID, pc)
	case StageSynthesis:
		return runSynthesis(ctx, cfg, caseID, pc)
	case StageUncertaintySurfacing:
		return runUncertaintySurfacing(caseID, pc)
	case StageGuardrailCheck:
		return runGuardrailCheck(ctx, cfg, caseID, pc)
	default:
		return Checkpoint{}, ErrUnknownStage
	}
}

// loadCompletedStageOutputs rebuilds a pipelineContext from every stage
// already present in completed, reading each one's typed result back
// from cfg.Checkpoints. This is Resume's core mechanism: a stage present
// in completed is never re-run, so its output must instead come from
// storage.
func loadCompletedStageOutputs(ctx context.Context, cfg RunConfig, caseID string, completed []Stage) (pipelineContext, error) {
	var pc pipelineContext
	for _, stage := range completed {
		cp, err := cfg.Checkpoints.GetCheckpoint(ctx, caseID, stage)
		if err != nil {
			return pipelineContext{}, fmt.Errorf("reasoningorchestration: load checkpoint for completed stage %q: %w", stage, err)
		}
		applyCheckpoint(&pc, cp)
	}
	return pc, nil
}

// applyCheckpoint copies checkpoint's populated field into pc, based on
// checkpoint.Stage.
func applyCheckpoint(pc *pipelineContext, checkpoint Checkpoint) {
	switch checkpoint.Stage {
	case StageIssueFraming:
		pc.issues = checkpoint.IssueAnalysis
	case StageFirstPartyArguments:
		pc.firstParty = checkpoint.FirstPartyArguments
	case StageSecondPartyArguments:
		pc.secondParty = checkpoint.SecondPartyArguments
	case StageEvidenceWeighing:
		pc.evidence = checkpoint.Evidence
	case StageLawApplication:
		pc.law = checkpoint.Law
	case StageSynthesis:
		pc.opinion = checkpoint.Opinion
	case StageUncertaintySurfacing:
		pc.uncertainty = checkpoint.Uncertainty
	case StageGuardrailCheck:
		// No pipelineContext field: StageGuardrailCheck is terminal and
		// carries no output any later stage consumes.
	}
}

// resolveWeights concurrently resolves this case's reasoningprofile
// Family/Weights alongside StageIssueFraming — see runIssueFraming. It
// has no dependency on issue framing's output (both only need the case's
// jurisdiction/legal-family context) and issue framing has no dependency
// on it either, so running them concurrently is safe: no shared mutable
// state, no ordering requirement between the two.
func resolveWeights(legalFamily string) reasoningprofile.Weights {
	family := reasoningprofile.Family(legalFamily)
	weights, err := reasoningprofile.WeightsForFamily(family)
	if err != nil {
		return reasoningprofile.Weights{}
	}
	return weights
}

// guardrailApproved runs the pipeline's final checks: every
// TentativeConclusion's Text must pass guardrail.CheckText, and the case
// must pass guardrail.CanFinalize against cfg.SignoffGate (defaulting to
// guardrail.NoSignoffRecordedGate, fail-closed, when unset). Unlike
// synthesisagent.Provider's own CheckText enforcement (which silently
// drops a rejected conclusion when assembling the tree), this stage
// surfaces a failure explicitly: it is the pipeline's own last line of
// defense, run over the same Opinion.Conclusions text one more time.
func runGuardrailCheck(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	for _, tc := range pc.opinion.Conclusions {
		if err := guardrail.CheckText(tc.Text); err != nil {
			return Checkpoint{}, fmt.Errorf("reasoningorchestration: guardrail check failed for issue %q: %w", tc.IssueNodeID, err)
		}
	}

	gate := cfg.SignoffGate
	if gate == nil {
		gate = guardrail.NoSignoffRecordedGate{}
	}

	// CanFinalize is expected to report false (SignoffPending) until a
	// future Phase 068 sign-off workflow exists — see
	// guardrail.NoSignoffRecordedGate's own doc comment. This stage
	// therefore records the outcome on the Checkpoint rather than
	// treating a not-yet-approved sign-off as a pipeline failure: the
	// reasoning work through synthesis and uncertainty surfacing is
	// still complete and checkpointed either way, only finalization
	// (a separate, later action) is gated on approval.
	approved, _ := guardrail.CanFinalize(ctx, caseID, gate)

	return Checkpoint{Stage: StageGuardrailCheck, GuardrailApproved: approved}, nil
}
