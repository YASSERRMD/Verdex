package caselifecycle

// Action identifies a case-scoped operation another package might
// want to perform against a Case, gated by that case's current State.
// This package does not perform any of these operations itself — it
// only defines which ones are permitted in which State, as a guard
// downstream packages (packages/ingestion, packages/category,
// packages/timeline, packages/synthesisagent, and others) can consult
// before mutating case-scoped data.
type Action string

const (
	// ActionIngestEvidence covers uploading, transcribing, or
	// extracting new evidentiary material for a case (the operations
	// packages/ingestion performs).
	ActionIngestEvidence Action = "ingest_evidence"

	// ActionEditCategory covers changing a case's category/subcategory
	// assignment (packages/category).
	ActionEditCategory Action = "edit_category"

	// ActionEditTimeline covers adding or editing parties and timeline
	// events (packages/timeline).
	ActionEditTimeline Action = "edit_timeline"

	// ActionGenerateReasoning covers running the reasoning pipeline
	// (issue/fact/rule extraction, tree assembly, synthesis) against a
	// case.
	ActionGenerateReasoning Action = "generate_reasoning"

	// ActionReviewOpinion covers a judge or reviewer reading and
	// annotating a synthesized draft opinion.
	ActionReviewOpinion Action = "review_opinion"

	// ActionEditMetadata covers SetMetadata/MergeMetadata calls
	// against the case itself.
	ActionEditMetadata Action = "edit_metadata"
)

// permittedActions maps each State to the set of Actions allowed
// against a case in that state. A case not present in this map (which
// should never happen for a valid State) permits no actions.
//
// Rationale for each state's set:
//   - StateDraft: intake is still assembling the case; evidence,
//     category, timeline, and metadata may all still change, but
//     nothing has been reasoned over yet.
//   - StateActive: the normal working state — everything is
//     permitted, including running the reasoning pipeline.
//   - StateUnderReview: the case has been submitted for judicial
//     review of its draft output; case-scoped data is frozen except
//     for the review action itself and metadata (e.g. adding review
//     notes), so the material the reviewer is looking at cannot shift
//     under them mid-review.
//   - StateClosed: read-only except via the explicit Reopen flow,
//     which transitions back to StateActive before any of these
//     actions become available again.
//   - StateArchived: fully read-only; no action is permitted.
var permittedActions = map[State]map[Action]bool{
	StateDraft: {
		ActionIngestEvidence: true,
		ActionEditCategory:   true,
		ActionEditTimeline:   true,
		ActionEditMetadata:   true,
	},
	StateActive: {
		ActionIngestEvidence:    true,
		ActionEditCategory:      true,
		ActionEditTimeline:      true,
		ActionGenerateReasoning: true,
		ActionReviewOpinion:     true,
		ActionEditMetadata:      true,
	},
	StateUnderReview: {
		ActionReviewOpinion: true,
		ActionEditMetadata:  true,
	},
	StateClosed:   {},
	StateArchived: {},
}

// CanPerform reports whether action is permitted for a case currently
// in state.
func CanPerform(state State, action Action) bool {
	return permittedActions[state][action]
}

// RequireAction returns nil if action is permitted for state, or
// ErrActionNotPermitted otherwise. Intended as a one-line guard for
// downstream packages: `if err := caselifecycle.RequireAction(c.State,
// caselifecycle.ActionIngestEvidence); err != nil { return err }`.
func RequireAction(state State, action Action) error {
	if !CanPerform(state, action) {
		return ErrActionNotPermitted
	}
	return nil
}

// PermittedActions returns the sorted-by-declaration-order set of
// Actions permitted for state, primarily for documentation/introspection
// purposes (e.g. surfacing "what can I do with this case right now" in
// an API response). Returns an empty, non-nil slice for a state with
// no permitted actions or an unknown state.
func PermittedActions(state State) []Action {
	allowed := permittedActions[state]
	out := make([]Action, 0, len(allowed))
	// Iterate in the fixed declaration order below rather than map
	// order, so callers get a stable, deterministic result.
	for _, a := range []Action{
		ActionIngestEvidence,
		ActionEditCategory,
		ActionEditTimeline,
		ActionGenerateReasoning,
		ActionReviewOpinion,
		ActionEditMetadata,
	} {
		if allowed[a] {
			out = append(out, a)
		}
	}
	return out
}
