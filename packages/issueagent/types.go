package issueagent

import "time"

// FramedIssue is this agent's structured framing of a single, already
// existing irac.IssueNode: how material it is relative to the case's
// other issues, what governing legal question(s) it raises, any
// ambiguities or gaps in its rule linkage, and a confidence score.
//
// FramedIssue deliberately references SourceIssueNodeID rather than
// embedding or duplicating the irac.IssueNode itself — this package reads
// the tree via knowledgeapi, it never constructs new tree nodes (that
// remains packages/issue's and packages/treeassembly's job).
type FramedIssue struct {
	// SourceIssueNodeID is the irac.IssueNode.ID this framing describes.
	SourceIssueNodeID string `json:"source_issue_node_id"`

	// Question is the issue's text as read from the tree, carried forward
	// for a downstream consumer that only wants FramedIssue values
	// without a second knowledgeapi round trip.
	Question string `json:"question"`

	// MaterialityRank is this issue's 1-based rank among every issue in
	// the same IssueAnalysisResult, 1 being the most material (most
	// determinative of the case's outcome). Ties are broken
	// deterministically by SourceIssueNodeID (see rank.go).
	MaterialityRank int `json:"materiality_rank"`

	// MaterialityScore is the raw [0,1] score MaterialityRank was derived
	// from, kept alongside the rank so a downstream consumer can judge how
	// close two issues' rankings were rather than only seeing an ordinal.
	MaterialityScore float64 `json:"materiality_score"`

	// GoverningQuestions are the specific legal question(s) this issue
	// raises, typically one per governing RuleNode linked to the issue
	// (Rule --governs--> Issue, see irac.EdgeGoverns), refined by the
	// model into a precise question rather than the rule's raw text.
	GoverningQuestions []string `json:"governing_questions"`

	// Ambiguities flags reasons this issue is thin, unclear, or
	// contradictory: missing/weak rule linkage, low source confidence, or
	// a model-identified factual conflict. Empty means no ambiguity was
	// surfaced.
	Ambiguities []string `json:"ambiguities,omitempty"`

	// Confidence is this framing's own [0,1] confidence, combining any
	// model-reported confidence with the heuristic RuleLinkage signal (see
	// confidence.go). Distinct from MaterialityScore: Confidence measures
	// how much this FramedIssue itself should be trusted, not how
	// important the issue is to the case.
	Confidence float64 `json:"confidence"`

	// RuleLinkageCount is the number of governing RuleNodes found for this
	// issue at framing time, retained as a transparency signal for why
	// Confidence and Ambiguities came out the way they did.
	RuleLinkageCount int `json:"rule_linkage_count"`
}

// IssueAnalysisResult is the overall structured output of one issue-agent
// run for a case: every issue found in the tree, framed and ranked, plus
// the jurisdiction/legal-family context the framing was performed under.
type IssueAnalysisResult struct {
	// CaseID is the case this analysis was scoped to.
	CaseID string `json:"case_id"`

	// JurisdictionCode is the jurisdiction the framing prompt was selected
	// for (see prompt.go's jurisdiction-aware template selection). May be
	// empty when no jurisdiction was supplied.
	JurisdictionCode string `json:"jurisdiction_code,omitempty"`

	// LegalFamily is the legal tradition the framing prompt was selected
	// for. May be empty when no legal family was supplied.
	LegalFamily string `json:"legal_family,omitempty"`

	// Issues is every FramedIssue produced for this case, sorted by
	// MaterialityRank ascending (rank 1 first).
	Issues []FramedIssue `json:"issues"`

	// GeneratedAt records when this analysis was produced.
	GeneratedAt time.Time `json:"generated_at"`
}
