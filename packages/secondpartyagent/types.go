package secondpartyagent

import "time"

// PartyID identifies the litigant this agent constructs arguments for. It
// is an opaque, caller-defined string rather than a hard dependency on
// packages/timeline's or any other package's party/role model — mirroring
// firstpartyagent.PartyID's own plain-string convention exactly, so a
// caller need not reconcile two different party-identifier types across
// the two adversarial agents.
type PartyID string

// CitationRef is a resolved, verified citation attached to one of an
// Argument's supporting RuleNodes, mirroring the fields of
// knowledgeapi.CitationDTO this package cares about without importing the
// wider knowledgeapi response envelope into the argument shape itself.
// This is a structural duplicate of firstpartyagent.CitationRef by design:
// the two adversarial agents' output shapes are intentionally independent
// types rather than sharing one via import, so this package's own
// ArgumentSet can evolve (e.g. the RebutsArgumentIDs addition below)
// without coupling firstpartyagent's shape to second-party concerns.
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

// Argument is one line of reasoning constructed in the second party's
// favor for a single FramedIssue: a claim, the FactNode/RuleNode IDs from
// the case's tree that support it, citations resolved for its supporting
// rules, the counterarguments it anticipates, a [0,1] strength score, and
// — unlike firstpartyagent.Argument — the set of first-party
// firstpartyagent.Argument.ID values it specifically rebuts.
//
// Every ID in SupportingFactIDs and SupportingRuleIDs is guaranteed, by
// the time an Argument appears in a finalized ArgumentSet, to have been
// cross-checked against the case's actual tree — see ground.go. Every ID
// in RebutsArgumentIDs is likewise guaranteed to have been cross-checked
// against the actual set of first-party Argument IDs supplied as input —
// see ground.go's groundRebuttalIDs.
type Argument struct {
	// ID is a stable identifier for this argument within its ArgumentSet,
	// unique per (IssueNodeID, index) pair.
	ID string `json:"id"`

	// IssueNodeID is the irac.IssueNode.ID (FramedIssue.SourceIssueNodeID)
	// this argument addresses.
	IssueNodeID string `json:"issue_node_id"`

	// PartyID is the party this argument favors — the second party.
	PartyID PartyID `json:"party_id"`

	// Claim is the argument's core assertion in the second party's favor.
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
	// downstream synthesis agent (Phase 055) has a documented starting
	// point rather than needing to discover them independently.
	Counterarguments []string `json:"counterarguments,omitempty"`

	// RebutsArgumentIDs are firstpartyagent.Argument.ID values this
	// argument specifically targets and rebuts. Every ID here is
	// guaranteed, by the time this Argument appears in a finalized
	// ArgumentSet, to reference a real Argument present in the
	// firstpartyagent.ArgumentSet supplied to New — see ground.go. A
	// second-party argument that does not target any specific first-party
	// argument (e.g. an affirmative argument raised independently of
	// rebuttal) legitimately carries an empty slice here.
	RebutsArgumentIDs []string `json:"rebuts_argument_ids,omitempty"`

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

	// FabricatedRebuttalIDs lists any RebutsArgumentIDs the model cited
	// that do not correspond to a real Argument in the first-party
	// ArgumentSet supplied to New, removed before this Argument was
	// finalized. Empty when every cited rebuttal target was real.
	FabricatedRebuttalIDs []string `json:"fabricated_rebuttal_ids,omitempty"`
}

// ArgumentSet is the final, structured output of one second-party
// argument-agent run for a case: every Argument constructed across every
// FramedIssue supplied as input, grouped implicitly by their IssueNodeID
// field (a caller wanting a per-issue grouping filters Arguments by
// IssueNodeID rather than this type nesting a second slice-of-slices
// shape) — mirroring firstpartyagent.ArgumentSet's shape exactly, plus
// the rebuttal-linkage addition carried on each Argument.
type ArgumentSet struct {
	// CaseID is the case this argument set was constructed for.
	CaseID string `json:"case_id"`

	// PartyID is the party every Argument in Arguments favors — the
	// second party.
	PartyID PartyID `json:"party_id"`

	// Arguments is every Argument constructed across every input
	// FramedIssue, in the order the issues were supplied.
	Arguments []Argument `json:"arguments"`

	// SkippedIssueNodeIDs lists FramedIssue.SourceIssueNodeID values for
	// which every proposed argument failed grounding, leaving no valid
	// Argument for that issue (the run as a whole still concludes as long
	// as at least one issue produced a grounded argument).
	SkippedIssueNodeIDs []string `json:"skipped_issue_node_ids,omitempty"`

	// GeneratedAt records when this argument set was produced.
	GeneratedAt time.Time `json:"generated_at"`
}
