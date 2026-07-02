package reasoningorchestration

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a caller supplies an empty case ID
	// to Run, Resume, or a CheckpointStore method.
	ErrEmptyCaseID = errors.New("reasoningorchestration: case id is required")

	// ErrNilRunConfig is returned when Run is called with a nil dependency
	// that RunConfig requires (e.g. a nil KnowledgeAPI or Router).
	ErrNilRunConfig = errors.New("reasoningorchestration: run config is missing a required dependency")

	// ErrNoFramedIssues is returned when the issue-framing stage concludes
	// with zero issues, leaving no work for every later stage.
	ErrNoFramedIssues = errors.New("reasoningorchestration: issue framing produced no issues")

	// ErrCheckpointNotFound is returned by CheckpointStore.Get when no
	// checkpoint has been saved for the requested (CaseID, Stage) pair.
	ErrCheckpointNotFound = errors.New("reasoningorchestration: checkpoint not found")

	// ErrUnknownStage is returned when a Stage value outside the
	// recognized enum is supplied to a function that switches on Stage.
	ErrUnknownStage = errors.New("reasoningorchestration: unrecognized stage")

	// ErrBudgetExhausted is returned when the pipeline's overall wall-clock
	// budget (PipelineBudget.MaxTotalWallClock) is exhausted before the
	// next stage could start. This is a graceful halt, not a stage
	// failure: RunState.Termination is set to TerminationBudgetExhausted
	// rather than TerminationFailed.
	ErrBudgetExhausted = errors.New("reasoningorchestration: pipeline wall-clock budget exhausted")

	// ErrRunNotFound is returned by Resume when no RunState has ever been
	// checkpointed for the requested case ID.
	ErrRunNotFound = errors.New("reasoningorchestration: no run found to resume for case")
)
