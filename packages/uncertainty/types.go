package uncertainty

import "time"

// Source identifies which upstream pipeline stage an Uncertainty finding
// was derived from: the issue-framing stage, the evidence-weighing stage,
// the law-application stage, or the synthesis agent's own tentative
// conclusion.
type Source string

const (
	// SourceIssueFraming identifies an Uncertainty derived from a low
	// issueagent.FramedIssue.Confidence value.
	SourceIssueFraming Source = "issue_framing"

	// SourceEvidence identifies an Uncertainty derived from thin or
	// disputed evidence surfaced by packages/evidenceweighing: a
	// contradicted or low-weight evidenceweighing.FactWeight, an
	// evidenceweighing.Contradiction, or an evidenceweighing.Gap.
	SourceEvidence Source = "evidence"

	// SourceLawApplication identifies an Uncertainty derived from either
	// a low lawapplication.IssueApplication.Confidence value or an
	// unsettled/conflicting controlling authority
	// (lawapplication.ConflictingAuthority).
	SourceLawApplication Source = "law_application"

	// SourceConclusion identifies an Uncertainty derived from a low
	// synthesisagent.TentativeConclusion.Confidence value.
	SourceConclusion Source = "conclusion"
)

// Uncertainty is a single ranked finding describing one reason to doubt
// part of the draft analysis for one issue: which upstream Source raised
// it, how Severe the underlying signal is on its own, where it ranks
// relative to every other Uncertainty found for the case (ImpactRank,
// materiality-weighted — see rank.go), and a human-readable Caveat a
// reviewer can read directly.
type Uncertainty struct {
	// IssueNodeID is the irac.IssueNode.ID this Uncertainty concerns —
	// the same identifier used throughout the pipeline as
	// FramedIssue.SourceIssueNodeID, IssueApplication.IssueNodeID, and
	// TentativeConclusion.IssueNodeID. Grouping Uncertainties by this
	// field is how a caller answers "what's shaky about conclusion X" —
	// see Report.ByIssue.
	IssueNodeID string `json:"issue_node_id"`

	// Source identifies which upstream stage this Uncertainty came from.
	Source Source `json:"source"`

	// Severity is this finding's own [0,1] severity, independent of the
	// issue's materiality: how doubtful the underlying signal is in
	// isolation (e.g. 1 - Confidence for a low-confidence finding, or a
	// fixed severity for a structural finding like a Contradiction or a
	// ConflictingAuthority). See severity.go.
	Severity float64 `json:"severity"`

	// ImpactRank is this Uncertainty's 1-based rank among every
	// Uncertainty in the same Report, 1 being the highest-impact finding
	// on the case's outcome — Severity amplified by the issue's
	// issueagent.FramedIssue.MaterialityRank. Ties are broken
	// deterministically. See rank.go.
	ImpactRank int `json:"impact_rank"`

	// ImpactScore is the raw score ImpactRank was derived from, kept
	// alongside the rank so a caller can judge how close two findings'
	// rankings were rather than only seeing an ordinal — mirroring
	// issueagent.FramedIssue's MaterialityRank/MaterialityScore
	// convention exactly.
	ImpactScore float64 `json:"impact_score"`

	// Caveat is the human-readable explanation of this finding, suitable
	// for direct display to a reviewing judge (see caveat.go).
	Caveat string `json:"caveat"`

	// Detail carries the underlying, source-specific identifier the
	// finding concerns when one exists: a FactNodeID for
	// SourceEvidence, a RuleID pair for a conflicting-authority finding,
	// or empty when the finding concerns a confidence score with no
	// finer-grained identifier. Purely informational.
	Detail string `json:"detail,omitempty"`
}

// OverconfidencePhrasing is a single flagged occurrence of absolutist or
// over-confident language found in a synthesisagent.TentativeConclusion's
// Text. This is a quality signal only — see doc.go and
// doc/uncertainty-surfacing.md for why this package flags rather than
// blocks, and how it differs from irac.ContainsVerdictLanguage.
type OverconfidencePhrasing struct {
	// IssueNodeID is the TentativeConclusion.IssueNodeID this flag
	// concerns.
	IssueNodeID string `json:"issue_node_id"`

	// Phrase is the exact absolutist term matched (case-insensitively)
	// in the conclusion's Text, e.g. "definitely" or "beyond doubt".
	Phrase string `json:"phrase"`

	// Excerpt is a short snippet of Text surrounding the match, for a
	// reviewer to see the phrase in context without re-reading the full
	// conclusion.
	Excerpt string `json:"excerpt"`
}

// Report is the top-level output of one Surface/Analyze run for a case:
// every Uncertainty found across all four upstream result types, ranked
// by impact on the case's outcome, plus every OverconfidencePhrasing
// flagged in the synthesis agent's conclusion text.
type Report struct {
	// CaseID is the case this report was computed for.
	CaseID string `json:"case_id"`

	// Uncertainties is every Uncertainty found, sorted by ImpactRank
	// ascending (rank 1, the highest-impact finding, first).
	Uncertainties []Uncertainty `json:"uncertainties"`

	// OverconfidenceFlags is every OverconfidencePhrasing found across
	// every TentativeConclusion's Text. Empty means no over-confident
	// phrasing was detected — a legitimate, honestly-reported outcome,
	// not an error.
	OverconfidenceFlags []OverconfidencePhrasing `json:"overconfidence_flags,omitempty"`

	// GeneratedAt records when this report was produced.
	GeneratedAt time.Time `json:"generated_at"`
}

// ByIssue groups r.Uncertainties by IssueNodeID, preserving each group's
// relative ImpactRank ordering, so a caller can look up "what's shaky
// about conclusion X" without re-scanning the full, flat Uncertainties
// slice. See attach.go.
func (r Report) ByIssue() map[string][]Uncertainty {
	return groupByIssue(r.Uncertainties)
}
