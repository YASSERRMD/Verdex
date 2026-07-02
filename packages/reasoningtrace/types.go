package reasoningtrace

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
)

// StageStep is one agentframework.Step, flattened out of a stage's
// Scratchpad and tagged with the reasoningorchestration.Stage it came
// from, so a caller walking a Trace never has to cross-reference back
// into a raw Checkpoint to know which stage produced a given step.
type StageStep struct {
	// Stage identifies which pipeline stage produced this step.
	Stage reasoningorchestration.Stage

	// Index is the step's 0-based position within its own stage's run
	// (agentframework.Step.Index), not a position within the whole
	// Trace.
	Index int

	// ModelCalled is true if a model request was actually sent for this
	// step (Step.Response was non-nil).
	ModelCalled bool

	// Concluded is true if this step's Decision.Conclude was true.
	Concluded bool

	// ToolCallCount is the number of tool calls dispatched during this
	// step (len(Step.Observations)).
	ToolCallCount int

	// Err is this step's error, if any (agentframework.Step.Err).
	Err error

	// Duration is this step's wall-clock duration
	// (agentframework.Step.Duration()).
	Duration time.Duration
}

// RetrievalEvent records one tool call an agent made that reads case
// knowledge — a retrieved node, a resolved citation, a validation-status
// check, or a search/path lookup — extracted from a stage's Scratchpad
// Observations. This is the trace's answer to "what evidence did the
// pipeline actually look at."
type RetrievalEvent struct {
	// Stage identifies which pipeline stage made this call.
	Stage reasoningorchestration.Stage

	// StepIndex is the Step.Index the call occurred within.
	StepIndex int

	// ToolName is the agentframework tool constant invoked (e.g.
	// agentframework.ToolGetNode, agentframework.ToolResolveCitation).
	ToolName string

	// Args are the tool call's arguments, verbatim.
	Args map[string]any

	// ResultSummary is the tool result's rendered Content, or the error
	// text if the call failed.
	ResultSummary string

	// Err is set when the tool invocation itself failed.
	Err error
}

// NarrativeSegment is one paragraph of the human-readable narrative,
// paired with the structured node/rule/issue identifiers it discusses so
// a UI can render "explain this sentence" links back into the IRAC tree
// without parsing prose.
type NarrativeSegment struct {
	// Stage identifies which pipeline stage this segment narrates.
	Stage reasoningorchestration.Stage

	// Text is this segment's plain-English prose.
	Text string

	// RelatedNodeIDs are every tree node ID (issue, rule, or fact) this
	// segment discusses — the union of an IssueNodeID plus any
	// SupportingFactIDs/SupportingRuleIDs relevant to the segment.
	RelatedNodeIDs []string
}

// CitationTrail is one resolved authority (rule) supporting a
// conclusion, plus its verification status — the leaf of an
// AuthorityTrail's expandable tree.
type CitationTrail struct {
	// RuleID is the irac.RuleNode.ID this citation supports.
	RuleID string

	// Citation is the resolved citation text, if any.
	Citation string

	// Verified is true if the underlying node was independently
	// confirmed to exist in the case's tree (mirrors
	// lawapplication.AppliedCitation.Verified).
	Verified bool

	// Resolved is true if a citation lookup was attempted without
	// error (mirrors lawapplication.AppliedCitation.Resolved).
	Resolved bool
}

// AuthorityTrail is the expandable evidence/authority trail for one
// conclusion: which issue it resolves, which rules control that issue,
// and each controlling rule's citation and verification status. A
// future UI renders this as a collapsible IssueNodeID -> rules ->
// citations tree.
type AuthorityTrail struct {
	// IssueNodeID is the TentativeConclusion.IssueNodeID this trail
	// belongs to.
	IssueNodeID string

	// SupportingFactIDs are the conclusion's verified supporting facts
	// (synthesisagent.TentativeConclusion.SupportingFactIDs).
	SupportingFactIDs []string

	// Citations is one CitationTrail per controlling rule found for
	// IssueNodeID in the law-application stage's output.
	Citations []CitationTrail
}

// Trace is the fully assembled, auditable record of one case's run
// through reasoningorchestration: every step and tool call taken by the
// four LLM-backed stages, every retrieved node/citation, a
// stage-by-stage narrative linked back to tree nodes, and an
// AuthorityTrail per conclusion. Build assembles a Trace by reading back
// every Checkpoint reasoningorchestration saved for a case; Trace itself
// holds no behavior beyond what Narrative/Export/IntegrityHash compute
// over it.
type Trace struct {
	// CaseID is the case this trace was assembled for.
	CaseID string

	// Steps is every StageStep from every LLM-backed stage's Scratchpad,
	// in stage-then-step order.
	Steps []StageStep

	// Retrievals is every RetrievalEvent extracted from those same
	// Scratchpads, in stage-then-step order.
	Retrievals []RetrievalEvent

	// Narrative is the flat, human-readable prose walking through the
	// pipeline stage by stage.
	Narrative string

	// Segments is Narrative broken into per-stage paragraphs, each
	// carrying the tree node IDs it discusses.
	Segments []NarrativeSegment

	// AuthorityTrails is one AuthorityTrail per conclusion in the
	// synthesized Opinion.
	AuthorityTrails []AuthorityTrail

	// GeneratedAt records when this Trace was assembled.
	GeneratedAt time.Time
}
