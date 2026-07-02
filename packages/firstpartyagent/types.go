package firstpartyagent

import "time"

// PartyID identifies the litigant this agent constructs arguments for. It
// is an opaque, caller-defined string rather than a hard dependency on
// packages/timeline's or any other package's party/role model — mirroring
// CaseID's own plain-string convention across Verdex's agent packages.
// A caller mapping PartyID onto a richer domain concept (e.g.
// packages/timeline's notion of a case participant) does so at its own
// boundary; this package only ever treats it as an opaque label carried
// through into the rendered prompt and the resulting ArgumentSet.
type PartyID string

// CitationRef is a resolved, verified citation attached to one of an
// Argument's supporting RuleNodes, mirroring the fields of
// knowledgeapi.CitationDTO this package cares about without importing the
// wider knowledgeapi response envelope into the argument shape itself.
type CitationRef struct {
	// NodeID is the RuleNode this citation was resolved for.
	NodeID string `json:"node_id"`

	// Citation is the resolved citation text (e.g. a statute or case
	// reporter cite).
	Citation string `json:"citation"`

	// VerificationStatus mirrors knowledgeapi.CitationDTO.VerificationStatus.
	VerificationStatus string `json:"verification_status"`

	// Verified mirrors knowledgeapi.CitationDTO.Verified: whether the
	// underlying node was independently confirmed to exist in the case's
	// tree under the claimed case, per packages/citation's
	// anti-hallucination guarantee.
	Verified bool `json:"verified"`

	// ConfidenceScore mirrors knowledgeapi.CitationDTO.ConfidenceScore.
	ConfidenceScore float64 `json:"confidence_score"`
}

// Argument is one line of reasoning constructed in the first party's
// favor for a single FramedIssue: a claim, the FactNode/RuleNode IDs from
// the case's tree that support it, citations resolved for its supporting
// rules, the counterarguments it anticipates, and a [0,1] strength score.
//
// Every ID in SupportingFactIDs and SupportingRuleIDs is guaranteed, by
// the time an Argument appears in a finalized ArgumentSet, to have been
// cross-checked against the case's actual tree — see ground.go. An
// Argument that failed grounding either has its ungrounded references
// stripped (leaving Grounded false and FabricatedNodeIDs populated) or is
// dropped entirely, per GroundingIssues on the containing ArgumentSet.
type Argument struct {
	// ID is a stable identifier for this argument within its ArgumentSet,
	// unique per (IssueNodeID, index) pair.
	ID string `json:"id"`

	// IssueNodeID is the irac.IssueNode.ID (FramedIssue.SourceIssueNodeID)
	// this argument addresses.
	IssueNodeID string `json:"issue_node_id"`

	// PartyID is the party this argument favors.
	PartyID PartyID `json:"party_id"`

	// Claim is the argument's core assertion in the first party's favor.
	Claim string `json:"claim"`

	// SupportingFactIDs are irac.FactNode IDs backing Claim.
	SupportingFactIDs []string `json:"supporting_fact_ids"`

	// SupportingRuleIDs are irac.RuleNode IDs backing Claim.
	SupportingRuleIDs []string `json:"supporting_rule_ids"`

	// Citations are resolved citations for entries in SupportingRuleIDs,
	// one per rule that a citation could be resolved for (a rule with no
	// resolvable citation is simply absent here, not an error).
	Citations []CitationRef `json:"citations,omitempty"`

	// Counterarguments are likely rebuttals this argument anticipates,
	// surfaced by the model alongside its own construction of Claim so a
	// downstream second-party agent (Phase 052) has a documented starting
	// point rather than needing to discover them independently, and so a
	// human reviewer can see this agent already reasoned about its own
	// weaknesses.
	Counterarguments []string `json:"counterarguments,omitempty"`

	// Strength is this argument's [0,1] strength score, combining
	// citation verification status, supporting-fact confidence, and
	// rule-linkage richness. See score.go.
	Strength float64 `json:"strength"`

	// Grounded is false if one or more of the model's originally proposed
	// supporting node IDs did not exist in the case's tree and had to be
	// stripped by the anti-fabrication check. A Grounded=false Argument
	// still only ever carries verified IDs in SupportingFactIDs/
	// SupportingRuleIDs — FabricatedNodeIDs records what was removed, for
	// transparency.
	Grounded bool `json:"grounded"`

	// FabricatedNodeIDs lists any node IDs the model cited that do not
	// exist in the case's tree, removed from SupportingFactIDs/
	// SupportingRuleIDs before this Argument was finalized. Empty when
	// Grounded is true.
	FabricatedNodeIDs []string `json:"fabricated_node_ids,omitempty"`
}

// ArgumentSet is the final, structured output of one first-party
// argument-agent run for a case: every Argument constructed across every
// FramedIssue supplied as input, grouped implicitly by their
// IssueNodeID field (a caller wanting a per-issue grouping filters
// Arguments by IssueNodeID rather than this type nesting a second
// slice-of-slices shape).
type ArgumentSet struct {
	// CaseID is the case this argument set was constructed for.
	CaseID string `json:"case_id"`

	// PartyID is the party every Argument in Arguments favors.
	PartyID PartyID `json:"party_id"`

	// Arguments is every Argument constructed across every input
	// FramedIssue, in the order the issues were supplied.
	Arguments []Argument `json:"arguments"`

	// SkippedIssueNodeIDs lists FramedIssue.SourceIssueNodeID values for
	// which every proposed argument failed grounding, leaving no valid
	// Argument for that issue (see ErrNoGroundedArguments's per-issue,
	// non-fatal counterpart: the run as a whole still concludes as long as
	// at least one issue produced a grounded argument).
	SkippedIssueNodeIDs []string `json:"skipped_issue_node_ids,omitempty"`

	// GeneratedAt records when this argument set was produced.
	GeneratedAt time.Time `json:"generated_at"`
}
