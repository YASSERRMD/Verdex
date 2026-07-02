package synthesisagent

import "time"

// TentativeConclusion is this agent's per-issue, non-binding synthesis of
// both parties' arguments, the issue's weighed evidence, and its law
// application: a draft resolution favoring at most one party (or neither,
// when the issue is genuinely unresolved on the record), the single
// weakest element threatening that resolution's reliability, and every
// fact/rule node it traces back to.
//
// A TentativeConclusion is deliberately not a verdict: FavoredParty names
// which party's position this draft analysis currently favors, not a
// binding determination of liability or outcome. See doc/synthesis-agent.md
// for the non-binding guardrail this type's downstream ConclusionProvider
// (provider.go) enforces at the tree-assembly boundary.
type TentativeConclusion struct {
	// IssueNodeID is the irac.IssueNode.ID (FramedIssue.SourceIssueNodeID)
	// this conclusion resolves.
	IssueNodeID string `json:"issue_node_id"`

	// Text is the reasoned, non-binding draft analysis text for this
	// issue: why the evidence and law point the way they do. Never
	// verdict or directive language — see irac.ContainsVerdictLanguage,
	// enforced by this package's ConclusionProvider adapter (provider.go)
	// before Text ever reaches an irac.ConclusionNode.
	Text string `json:"text"`

	// FavoredParty is the PartyID (carried as a plain string, mirroring
	// lawapplication.ArgumentRef.PartyID's party-agnostic convention so
	// this package need not choose between firstpartyagent.PartyID and
	// secondpartyagent.PartyID) whose position this conclusion currently
	// favors. Empty when the issue is genuinely unresolved on the current
	// record — a legitimate, honestly-reported outcome, not an error.
	FavoredParty string `json:"favored_party,omitempty"`

	// Confidence is this conclusion's own [0,1] confidence, reflecting how
	// well-supported it is by the underlying arguments, evidence weights,
	// and law application — not how important the issue is to the case.
	Confidence float64 `json:"confidence"`

	// WeakestLink names the single supporting element that most threatens
	// this conclusion's reliability: the lowest-weight or contradicted
	// fact it relies on, an unverified citation among its supporting
	// rules, or a lawapplication.ConflictingAuthority affecting the
	// issue's controlling rules. See weakestlink.go. Always populated
	// (never empty) for a conclusion with at least one supporting fact or
	// rule, since every real-world evidentiary record has some weakest
	// point; empty only when a conclusion has no supporting facts or
	// rules to evaluate at all.
	WeakestLink string `json:"weakest_link,omitempty"`

	// SupportingFactIDs are irac.FactNode IDs this conclusion traces back
	// to, cross-checked against the case's actual tree (see ground.go). A
	// conclusion carries only verified IDs here — fabricated references
	// are stripped, see FabricatedNodeIDs.
	SupportingFactIDs []string `json:"supporting_fact_ids"`

	// SupportingRuleIDs are irac.RuleNode IDs this conclusion traces back
	// to, cross-checked the same way as SupportingFactIDs.
	SupportingRuleIDs []string `json:"supporting_rule_ids"`

	// Grounded is false if one or more of the model's originally proposed
	// supporting node IDs did not exist in the case's tree (or were not
	// offered as evidence for this issue) and had to be stripped by the
	// anti-fabrication check. A Grounded=false TentativeConclusion still
	// only ever carries verified IDs in SupportingFactIDs/
	// SupportingRuleIDs.
	Grounded bool `json:"grounded"`

	// FabricatedNodeIDs lists any node IDs the model cited that do not
	// exist in the case's tree (or were not offered as evidence for this
	// issue), removed before this TentativeConclusion was finalized. Empty
	// when Grounded is true.
	FabricatedNodeIDs []string `json:"fabricated_node_ids,omitempty"`
}

// Opinion is the top-level, structured draft output of one synthesis-agent
// run for a case: a TentativeConclusion for every issue the run addressed.
// Opinion is intentionally named to avoid the "SynthesisResult" stutter
// against this package's own name (synthesisagent.SynthesisResult would
// repeat "synthesis"), mirroring how lawapplication.Result and
// evidenceweighing.Result avoid stuttering against their own package
// names.
//
// Opinion is a draft, non-binding analysis, not a judgment: every
// TentativeConclusion it carries is produced from
// irac.NewConclusionNode's mandatory draft_analysis label once passed
// through this package's ConclusionProvider adapter (provider.go) — see
// doc/synthesis-agent.md.
type Opinion struct {
	// CaseID is the case this opinion was synthesized for.
	CaseID string `json:"case_id"`

	// Conclusions is one TentativeConclusion per issue addressed, in the
	// order the issues were supplied to New.
	Conclusions []TentativeConclusion `json:"conclusions"`

	// SkippedIssueNodeIDs lists FramedIssue.SourceIssueNodeID values for
	// which every proposed conclusion failed grounding, leaving no valid
	// TentativeConclusion for that issue — mirroring
	// firstpartyagent.ArgumentSet.SkippedIssueNodeIDs's non-fatal,
	// per-issue convention exactly.
	SkippedIssueNodeIDs []string `json:"skipped_issue_node_ids,omitempty"`

	// GeneratedAt records when this opinion was produced.
	GeneratedAt time.Time `json:"generated_at"`
}
