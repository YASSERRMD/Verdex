package reasoningtrace

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
)

// llmStages lists every reasoningorchestration.Stage whose Checkpoint may
// carry a populated agentframework.Result, in pipeline order. Every other
// stage (evidence weighing, law application, uncertainty surfacing,
// guardrail check) is a deterministic function with no model calls or
// tool use, so it has no Scratchpad to trace.
var llmStages = []reasoningorchestration.Stage{
	reasoningorchestration.StageIssueFraming,
	reasoningorchestration.StageFirstPartyArguments,
	reasoningorchestration.StageSecondPartyArguments,
	reasoningorchestration.StageSynthesis,
}

// runForStage returns the agentframework.Result captured on checkpoint
// for stage, or the zero Result for a stage this package does not trace.
func runForStage(stage reasoningorchestration.Stage, checkpoint reasoningorchestration.Checkpoint) agentframework.Result {
	switch stage {
	case reasoningorchestration.StageIssueFraming:
		return checkpoint.IssueFramingRun
	case reasoningorchestration.StageFirstPartyArguments:
		return checkpoint.FirstPartyRun
	case reasoningorchestration.StageSecondPartyArguments:
		return checkpoint.SecondPartyRun
	case reasoningorchestration.StageSynthesis:
		return checkpoint.SynthesisRun
	default:
		return agentframework.Result{}
	}
}

// Build assembles a Trace for caseID by reading back every Checkpoint
// reasoningorchestration saved during that case's run from store. It
// requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase (see RequireViewPermission) — checked first, so
// an unauthorized caller never triggers a single store read.
//
// Build reads reasoningorchestration.RunState to discover which stages
// actually completed for caseID (ErrIncompleteRun if none did), then
// reads back that stage's Checkpoint for each completed stage. For the
// four LLM-backed stages (see llmStages), it flattens that stage's
// captured agentframework.Result.Scratchpad into StageSteps and
// RetrievalEvents. It then builds the narrative (see narrative.go) and
// every conclusion's AuthorityTrail (see authority.go).
func Build(ctx context.Context, caseID string, store reasoningorchestration.CheckpointStore) (Trace, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return Trace{}, err
	}
	if caseID == "" {
		return Trace{}, ErrEmptyCaseID
	}
	if store == nil {
		return Trace{}, ErrNilCheckpointStore
	}

	state, err := store.GetRunState(ctx, caseID)
	if err != nil {
		return Trace{}, fmt.Errorf("%w: %v", ErrIncompleteRun, err)
	}
	if len(state.CompletedStages) == 0 {
		return Trace{}, ErrIncompleteRun
	}

	checkpoints := make(map[reasoningorchestration.Stage]reasoningorchestration.Checkpoint, len(state.CompletedStages))
	for _, stage := range state.CompletedStages {
		cp, err := store.GetCheckpoint(ctx, caseID, stage)
		if err != nil {
			return Trace{}, fmt.Errorf("reasoningtrace: load checkpoint for stage %q: %w", stage, err)
		}
		checkpoints[stage] = cp
	}

	trace := Trace{CaseID: caseID, GeneratedAt: nowFunc()}

	for _, stage := range state.CompletedStages {
		if !isLLMStage(stage) {
			continue
		}
		cp, ok := checkpoints[stage]
		if !ok {
			continue
		}
		run := runForStage(stage, cp)
		if run.Scratchpad == nil {
			continue
		}
		for _, step := range run.Scratchpad.Steps() {
			trace.Steps = append(trace.Steps, StageStep{
				Stage:         stage,
				Index:         step.Index,
				ModelCalled:   step.Response != nil,
				Concluded:     step.Decision.Conclude,
				ToolCallCount: len(step.Observations),
				Err:           step.Err,
				Duration:      step.Duration(),
			})
			for _, obs := range step.Observations {
				if !isRetrievalTool(obs.Call.Name) {
					continue
				}
				trace.Retrievals = append(trace.Retrievals, RetrievalEvent{
					Stage:         stage,
					StepIndex:     step.Index,
					ToolName:      obs.Call.Name,
					Args:          obs.Call.Args,
					ResultSummary: retrievalSummary(obs),
					Err:           obs.Err,
				})
			}
		}
	}

	if opinionCP, ok := checkpoints[reasoningorchestration.StageSynthesis]; ok {
		trace.AuthorityTrails = buildAuthorityTrails(opinionCP.Opinion, checkpoints[reasoningorchestration.StageLawApplication].Law)
	}

	trace.Segments = buildNarrativeSegments(state, checkpoints)
	trace.Narrative = renderNarrative(trace.Segments)

	return trace, nil
}

// isLLMStage reports whether stage is one of llmStages.
func isLLMStage(stage reasoningorchestration.Stage) bool {
	for _, s := range llmStages {
		if s == stage {
			return true
		}
	}
	return false
}

// isRetrievalTool reports whether toolName is one of the knowledgeapi
// read tools agentframework exposes to agents (search, node lookup, path
// lookup, citation resolution, validation status) — i.e. a call worth
// surfacing as a RetrievalEvent.
func isRetrievalTool(toolName string) bool {
	switch toolName {
	case agentframework.ToolSearchCaseKnowledge,
		agentframework.ToolGetNode,
		agentframework.ToolLookupPaths,
		agentframework.ToolResolveCitation,
		agentframework.ToolValidationStatus:
		return true
	default:
		return false
	}
}

// retrievalSummary renders obs's outcome as a short string: the tool
// result's Content on success, or the error text on failure.
func retrievalSummary(obs agentframework.Observation) string {
	if obs.Err != nil {
		return obs.Err.Error()
	}
	return obs.Result.Content
}
