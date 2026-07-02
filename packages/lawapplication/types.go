package lawapplication

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// RuleRef is the minimal shape this package needs for a rule node in a
// case's tree: its ID, text (used only for Origin inference — see
// origin.go), and jurisdiction/legal-family tags when a caller has them
// available. Kept as a dedicated, narrow input type (rather than
// depending on knowledgeapi.NodeDTO or irac.RuleNode directly) so the
// core mapping/weighting logic stays easy to unit test with in-memory
// fixtures, mirroring packages/evidenceweighing's FactRef convention
// exactly.
type RuleRef struct {
	// ID is the irac.RuleNode.ID (knowledgeapi.NodeDTO.ID) this RuleRef
	// describes.
	ID string

	// Text is the rule's statement text, used only to heuristically
	// infer Origin (see origin.go) when no OriginHint is supplied.
	Text string

	// LegalFamily is the rule's irac.RuleNode.LegalFamily, when known to
	// the caller. Not surfaced through knowledgeapi.NodeDTO today (see
	// doc/law-application.md's "Known limitation" section), so this is
	// typically empty unless a caller fetched it separately (e.g.
	// directly from irac.RuleNode rather than through knowledgeapi).
	LegalFamily string

	// OriginHint, when non-empty, is a caller-supplied Origin
	// (OriginStatute or OriginPrecedent) that overrides this package's
	// own text/citation-based inference entirely. Left empty, Origin is
	// inferred (see origin.go).
	OriginHint Origin
}

// ArgumentRef is this package's own, party-agnostic view of a single
// Argument from either firstpartyagent.ArgumentSet or
// secondpartyagent.ArgumentSet: just enough fields to drive rule-to-issue
// mapping and conflicting-authority detection, without this package
// depending on which of the two (structurally near-identical but
// independent) Argument types produced it — mirroring
// evidenceweighing.CitingArgument exactly.
type ArgumentRef struct {
	// ArgumentID is the originating Argument.ID.
	ArgumentID string

	// IssueNodeID is the originating Argument.IssueNodeID.
	IssueNodeID string

	// PartyID is the originating Argument.PartyID, carried as a plain
	// string so this package need not choose between
	// firstpartyagent.PartyID and secondpartyagent.PartyID.
	PartyID string

	// SupportingFactIDs is the originating Argument.SupportingFactIDs.
	SupportingFactIDs []string

	// SupportingRuleIDs is the originating Argument.SupportingRuleIDs —
	// the signal this package uses to find which RuleNodes each party
	// invoked for a given issue.
	SupportingRuleIDs []string
}

// argumentRefsFromFirstParty converts every Argument in set into
// ArgumentRefs.
func argumentRefsFromFirstParty(set firstpartyagent.ArgumentSet) []ArgumentRef {
	out := make([]ArgumentRef, 0, len(set.Arguments))
	for _, a := range set.Arguments {
		out = append(out, ArgumentRef{
			ArgumentID:        a.ID,
			IssueNodeID:       a.IssueNodeID,
			PartyID:           string(a.PartyID),
			SupportingFactIDs: a.SupportingFactIDs,
			SupportingRuleIDs: a.SupportingRuleIDs,
		})
	}
	return out
}

// argumentRefsFromSecondParty converts every Argument in set into
// ArgumentRefs.
func argumentRefsFromSecondParty(set secondpartyagent.ArgumentSet) []ArgumentRef {
	out := make([]ArgumentRef, 0, len(set.Arguments))
	for _, a := range set.Arguments {
		out = append(out, ArgumentRef{
			ArgumentID:        a.ID,
			IssueNodeID:       a.IssueNodeID,
			PartyID:           string(a.PartyID),
			SupportingFactIDs: a.SupportingFactIDs,
			SupportingRuleIDs: a.SupportingRuleIDs,
		})
	}
	return out
}

// ElementFactEntry records that FactNodeID (an evidenceweighing-weighed
// fact) was cited by one or more arguments invoking RuleID for a given
// issue: the "element-to-fact" bookkeeping this package's Apply produces
// per controlling rule. This is deterministic aggregation, not natural-
// language generation — see doc/law-application.md.
type ElementFactEntry struct {
	// RuleID is the controlling irac.RuleNode.ID this fact was cited in
	// support of.
	RuleID string `json:"rule_id"`

	// FactNodeID is the cited, weighed fact.
	FactNodeID string `json:"fact_node_id"`

	// FactWeight is the evidenceweighing.FactWeight.Weight computed for
	// FactNodeID, or 0 if the fact was not present in the supplied
	// evidenceweighing.Result (a defensively-handled gap, not a fatal
	// error).
	FactWeight float64 `json:"fact_weight"`

	// Contradicted mirrors evidenceweighing.FactWeight.Contradicted.
	Contradicted bool `json:"contradicted"`

	// CitingPartyIDs lists every distinct PartyID whose argument cited
	// this fact in support of RuleID for this issue.
	CitingPartyIDs []string `json:"citing_party_ids"`
}

// ConflictingAuthority flags two controlling rules for the same issue
// that were invoked by opposing parties: one or more arguments favoring
// FirstRuleID's party cite FirstRuleID, and one or more arguments
// favoring a different party cite SecondRuleID, for the same issue. This
// package does not attempt to resolve which rule prevails — it surfaces
// the conflict as a finding for Phase 055's synthesis agent, per the
// plan's explicit "handle conflicting authority... flag it... rather
// than silently picking one" requirement.
type ConflictingAuthority struct {
	// IssueNodeID is the shared issue both rules were invoked to
	// resolve.
	IssueNodeID string `json:"issue_node_id"`

	// FirstRuleID and SecondRuleID are the two controlling rules in
	// tension. FirstRuleID is always the lexicographically smaller of
	// the two IDs, for deterministic, reproducible output regardless of
	// input ordering.
	FirstRuleID  string `json:"first_rule_id"`
	SecondRuleID string `json:"second_rule_id"`

	// FirstPartyID and SecondPartyID are the respective parties whose
	// arguments invoked FirstRuleID and SecondRuleID.
	FirstPartyID  string `json:"first_party_id"`
	SecondPartyID string `json:"second_party_id"`

	// Rationale explains why this pair was flagged.
	Rationale string `json:"rationale"`
}

// AppliedCitation is a controlling rule's resolved citation, attached so
// every ControllingRuleIDs entry in an IssueApplication carries the
// authority it cites — per the plan's "cite every applied authority"
// requirement. Unresolved or unverified citations are tracked (not
// silently dropped) via Resolved/Verified.
type AppliedCitation struct {
	// RuleID is the controlling rule this citation was resolved for.
	RuleID string `json:"rule_id"`

	// Citation is the resolved citation text, empty if Resolved is
	// false.
	Citation string `json:"citation"`

	// Origin is the inferred or resolved Origin for RuleID (see
	// origin.go).
	Origin Origin `json:"origin"`

	// Resolved is true if a citation lookup was attempted and returned
	// without error. False indicates the lookup itself failed (e.g. the
	// node could not be found) — a quality signal distinct from
	// Verified.
	Resolved bool `json:"resolved"`

	// Verified mirrors knowledgeapi.CitationDTO.Verified: whether the
	// underlying node was independently confirmed to exist in the
	// case's tree. False (with Resolved true) means the citation
	// resolved but failed verification — a quality signal a caller
	// should not silently ignore.
	Verified bool `json:"verified"`

	// VerificationStatus mirrors knowledgeapi.CitationDTO.VerificationStatus.
	VerificationStatus string `json:"verification_status"`
}

// Step is one entry in an IssueApplication's reasoning trail: a single,
// human-readable record of how ControllingRuleIDs, Conflicts, or
// Confidence were derived, mirroring evidenceweighing.FactWeight's
// human-readable Rationale-string convention but kept as a slice here
// since law application accumulates multiple distinct reasoning moves
// (rule mapping, element-fact aggregation, conflict detection, weighting)
// per issue rather than one blended score.
type Step struct {
	// Description explains what this step did and why.
	Description string `json:"description"`
}

// IssueApplication is the per-issue legal analysis Apply produces: every
// controlling rule found for the issue, the element-to-fact mapping
// backing their application, any conflicting authority detected, the
// resolved citations for every controlling rule, and a Confidence score
// with an explicit reasoning Steps trail.
type IssueApplication struct {
	// IssueNodeID is the irac.IssueNode.ID (FramedIssue.SourceIssueNodeID)
	// this analysis addresses.
	IssueNodeID string `json:"issue_node_id"`

	// ControllingRuleIDs are every irac.RuleNode.ID found to govern this
	// issue, via the Rule--governs-->Issue edge or either party's
	// SupportingRuleIDs (see map.go), in deterministic sorted order.
	ControllingRuleIDs []string `json:"controlling_rule_ids"`

	// ElementFactMap is the element-to-fact bookkeeping for every
	// controlling rule (see ElementFactEntry).
	ElementFactMap []ElementFactEntry `json:"element_fact_map"`

	// Conflicts is every ConflictingAuthority detected among this
	// issue's controlling rules.
	Conflicts []ConflictingAuthority `json:"conflicts"`

	// Citations is one AppliedCitation per ControllingRuleIDs entry.
	Citations []AppliedCitation `json:"citations"`

	// Confidence is this analysis's own [0,1] confidence, derived from
	// how well-supported the controlling rules are (citation
	// verification, fact weights, absence of conflicts) — see
	// confidence.go.
	Confidence float64 `json:"confidence"`

	// Steps is the explicit reasoning trail explaining how
	// ControllingRuleIDs/Confidence were derived.
	Steps []Step `json:"steps"`
}

// IssueInput bundles one FramedIssue with the RuleRefs available to
// Apply as candidate controlling rules for that issue (typically loaded
// by the caller via knowledgeapi.GetTree with a rule NodeTypeFilter).
// Rules not linked to this issue via a governs edge or an argument's
// SupportingRuleIDs are simply not selected as controlling for it — see
// map.go.
type IssueInput struct {
	// Issue is the framed issue being analyzed.
	Issue issueagent.FramedIssue

	// GoverningRuleIDs are irac.RuleNode.IDs linked to Issue via a
	// Rule--governs-->Issue edge in the case's tree (see map.go for how
	// this unions with argument-cited rules).
	GoverningRuleIDs []string
}

// Request bundles everything Apply needs for one case: the issues to
// analyze (each with its known governing rules), the full RuleRef catalog
// (for element-fact mapping, origin inference, and citation lookup),
// both parties' ArgumentSets, the weighed-facts Result from
// packages/evidenceweighing, and the LegalFamily governing this case
// (used to select the origin-weighting profile — see jurisdiction.go).
//
// Either ArgumentSet may be its zero value if that party produced no
// arguments; Apply still runs against whichever ArgumentSet is
// non-empty.
type Request struct {
	// CaseID is the case being analyzed.
	CaseID string

	// Issues is every issue to analyze.
	Issues []IssueInput

	// Rules is every RuleRef known in the case's tree, typically loaded
	// via knowledgeapi.GetTree with NodeTypeFilter set to rule nodes.
	Rules []RuleRef

	// FirstParty is the first party's ArgumentSet
	// (packages/firstpartyagent, Phase 051).
	FirstParty firstpartyagent.ArgumentSet

	// SecondParty is the second party's ArgumentSet
	// (packages/secondpartyagent, Phase 052).
	SecondParty secondpartyagent.ArgumentSet

	// Evidence is the evidenceweighing.Result carrying per-fact weights
	// for this case (Phase 053), used to populate ElementFactMap's
	// FactWeight/Contradicted fields.
	Evidence evidenceweighing.Result

	// LegalFamily selects the OriginProfile applied by WeightByOrigin
	// (see jurisdiction.go).
	LegalFamily LegalFamily

	// CitationLookup resolves a RuleID to its citation, typically backed
	// by knowledgeapi.ResolveCitation. A nil CitationLookup causes every
	// AppliedCitation to be recorded as unresolved rather than causing
	// Apply to fail — citation resolution is an I/O boundary this
	// package's core logic does not require to compute the rest of an
	// IssueApplication.
	CitationLookup CitationLookupFunc
}

// CitationLookupFunc resolves ruleID to an AppliedCitation's Citation/
// Origin/Verified/VerificationStatus fields. Implementations are
// expected to wrap knowledgeapi.KnowledgeAPI.ResolveCitation; this
// package depends only on this narrow function type so its core logic
// never imports knowledgeapi or performs I/O directly, mirroring
// evidenceweighing's "callers fetch, this package computes" boundary.
type CitationLookupFunc func(ruleID string) (citation string, origin Origin, verified bool, verificationStatus string, err error)

// Result is the full output of one Apply call for a case: one
// IssueApplication per input issue.
type Result struct {
	// CaseID is the case this result was computed for.
	CaseID string `json:"case_id"`

	// IssueApplications is one IssueApplication per Request.Issues entry,
	// in the same order supplied.
	IssueApplications []IssueApplication `json:"issue_applications"`

	// GeneratedAt records when this result was computed.
	GeneratedAt time.Time `json:"generated_at"`
}
