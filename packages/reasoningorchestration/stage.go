package reasoningorchestration

import "time"

// Stage identifies one step of the reasoning pipeline this package
// coordinates. Stages are strictly ordered: each stage's inputs are the
// prior stages' outputs (see doc/reasoning-orchestration.md's dependency
// table for exactly which prior stage feeds which), so a case's Stage
// value always advances monotonically through this list until
// StageComplete or a failure/budget halt.
type Stage string

// Stage constants, in pipeline order.
const (
	// StageIssueFraming runs packages/issueagent.Analyze: frames and
	// ranks every issue in the case's tree. The pipeline's first stage —
	// every later stage consumes its IssueAnalysisResult.
	StageIssueFraming Stage = "issue_framing"

	// StageFirstPartyArguments runs packages/firstpartyagent.Argue against
	// StageIssueFraming's output, constructing the first party's
	// affirmative case.
	StageFirstPartyArguments Stage = "first_party_arguments"

	// StageSecondPartyArguments runs packages/secondpartyagent.Argue
	// against StageIssueFraming's output and
	// StageFirstPartyArguments's ArgumentSet (for rebuttal targeting).
	StageSecondPartyArguments Stage = "second_party_arguments"

	// StageEvidenceWeighing runs packages/evidenceweighing.Weigh against
	// both parties' ArgumentSets, deterministically scoring and
	// cross-checking the evidentiary record.
	StageEvidenceWeighing Stage = "evidence_weighing"

	// StageLawApplication runs packages/lawapplication.Apply against the
	// framed issues, both ArgumentSets, and the evidence-weighing result,
	// deterministically mapping controlling authority to each issue.
	StageLawApplication Stage = "law_application"

	// StageSynthesis runs packages/synthesisagent.Synthesize against
	// every prior stage's output, producing the case's draft Opinion.
	StageSynthesis Stage = "synthesis"

	// StageUncertaintySurfacing runs packages/uncertainty.Surface against
	// the synthesized Opinion (plus the upstream Results it was derived
	// from), ranking every reason to doubt part of the draft analysis.
	StageUncertaintySurfacing Stage = "uncertainty_surfacing"

	// StageGuardrailCheck runs the project-wide non-binding guardrail
	// checks (packages/guardrail.CheckText over the synthesized opinion's
	// text, plus a CanFinalize sign-off-gate lookup) as the pipeline's
	// final stage before StageComplete.
	StageGuardrailCheck Stage = "guardrail_check"

	// StageComplete is a terminal marker, not a stage with its own work:
	// RunState.CurrentStage is set to StageComplete once
	// StageGuardrailCheck finishes successfully.
	StageComplete Stage = "complete"
)

// stageOrder is the exhaustive, ordered list of stages Run executes,
// excluding the terminal StageComplete marker. Every function in this
// package that needs "the next stage" or "is stage A before stage B"
// answers it by indexing into this slice rather than hardcoding
// stage-to-stage transitions in multiple places.
var stageOrder = []Stage{
	StageIssueFraming,
	StageFirstPartyArguments,
	StageSecondPartyArguments,
	StageEvidenceWeighing,
	StageLawApplication,
	StageSynthesis,
	StageUncertaintySurfacing,
	StageGuardrailCheck,
}

// stageIndex returns stage's position in stageOrder, or -1 if stage is
// not a recognized, runnable stage (including StageComplete, which has no
// index of its own).
func stageIndex(stage Stage) int {
	for i, s := range stageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// nextStage returns the Stage that logically follows stage in
// stageOrder, or StageComplete if stage is the last runnable stage.
// Returns ErrUnknownStage if stage is not recognized.
func nextStage(stage Stage) (Stage, error) {
	idx := stageIndex(stage)
	if idx == -1 {
		return "", ErrUnknownStage
	}
	if idx == len(stageOrder)-1 {
		return StageComplete, nil
	}
	return stageOrder[idx+1], nil
}

// isStageComplete reports whether every stage in stageOrder is present in
// completed.
func isStageComplete(completed []Stage, stage Stage) bool {
	for _, s := range completed {
		if s == stage {
			return true
		}
	}
	return false
}

// TerminationReason classifies why a Run/Resume call stopped, mirroring
// agentframework.TerminationReason's role for a single agent run but
// scoped to the whole multi-stage pipeline.
type TerminationReason string

// TerminationReason constants.
const (
	// TerminationRunning means the run has not yet reached a terminal
	// state — only ever observed on a RunState snapshot taken mid-run
	// (e.g. via a checkpoint read while another goroutine drives Run);
	// Run and Resume never return with this value as their final
	// RunState.Termination.
	TerminationRunning TerminationReason = "running"

	// TerminationComplete means every stage finished successfully through
	// StageGuardrailCheck.
	TerminationComplete TerminationReason = "complete"

	// TerminationFailed means a stage returned an error and the pipeline
	// stopped cleanly without attempting any later stage.
	// RunState.FailedStage and RunState.FailureReason describe which
	// stage failed and why.
	TerminationFailed TerminationReason = "failed"

	// TerminationBudgetExhausted means the pipeline's overall wall-clock
	// budget was exhausted before the next stage could start. This is a
	// graceful halt performed between stages, never mid-stage — see
	// PipelineBudget's field documentation.
	TerminationBudgetExhausted TerminationReason = "budget_exhausted"
)

// RunState is the full, persistable state of one case's pipeline run: how
// far it has progressed, which stages are done, and — if it stopped
// short of StageComplete — why.
//
// RunState is deliberately plain data (no behavior), mirroring
// agentframework.Result's own "plain data" convention, so it can be
// checkpointed, serialized, and inspected by a future case-workspace UI
// (Part 6) without that caller depending on this package's internal
// stage-execution logic.
type RunState struct {
	// CaseID is the case this run is scoped to.
	CaseID string

	// CurrentStage is the stage the run is at: the next stage to execute
	// if Termination is TerminationRunning or TerminationBudgetExhausted,
	// or StageComplete if Termination is TerminationComplete.
	CurrentStage Stage

	// CompletedStages lists every stage that finished successfully, in
	// execution order. A stage's presence here is what Resume uses to
	// decide it may be skipped.
	CompletedStages []Stage

	// Termination classifies why the run is not actively progressing:
	// still running (a mid-run snapshot only), complete, failed, or
	// halted on budget exhaustion.
	Termination TerminationReason

	// FailedStage names the stage that returned an error, populated only
	// when Termination is TerminationFailed.
	FailedStage Stage

	// FailureReason is a human-readable description of why FailedStage
	// failed, populated only when Termination is TerminationFailed.
	FailureReason string

	// StartedAt records when this run (or, after a Resume, the original
	// run) began.
	StartedAt time.Time

	// UpdatedAt records the last time this RunState changed.
	UpdatedAt time.Time
}

// Clone returns a deep-enough copy of s safe for a caller to mutate
// without affecting the original: CompletedStages is copied into a new
// backing array so appending to one copy never aliases the other.
func (s RunState) Clone() RunState {
	out := s
	out.CompletedStages = append([]Stage(nil), s.CompletedStages...)
	return out
}

// RunResult is Run/Resume's return value: the final RunState plus a Go
// error a caller can check with a plain `if err != nil` in addition to
// inspecting RunState.Termination directly. Mirrors
// agentframework.Runner.Run's "Result plus error" convention: Err is
// non-nil whenever Termination is not TerminationComplete, so callers
// that only check the error still get correct success/failure behavior.
type RunResult struct {
	// State is the run's final RunState.
	State RunState

	// Err is non-nil whenever State.Termination is not
	// TerminationComplete. It wraps ErrBudgetExhausted for a budget halt,
	// or carries the underlying stage error (unwrapped) for a failure.
	Err error
}
