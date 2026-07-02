package uncertainty

import (
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// defaultLowConfidenceThreshold is the [0,1] confidence value at or below
// which a reasoning step (a FramedIssue, an IssueApplication, or a
// TentativeConclusion) is flagged as a low-confidence Uncertainty when a
// Request does not override it via LowConfidenceThreshold.
const defaultLowConfidenceThreshold = 0.5

// defaultThinEvidenceWeightThreshold is the [0,1] evidenceweighing.
// FactWeight.Weight value at or below which a fact is treated as thin
// evidence when a Request does not override it via
// ThinEvidenceWeightThreshold.
const defaultThinEvidenceWeightThreshold = 0.4

// Request bundles the full pipeline's output for one case: the framed
// issues (for materiality-weighted ranking context), the weighed
// evidence, the applied law, and the synthesized draft opinion. Surface
// reads all four; it never fetches anything itself.
type Request struct {
	// CaseID is the case being analyzed. Must match every non-zero
	// CaseID carried by Issues/Evidence/Law/Opinion.
	CaseID string

	// Issues is the issue-agent's analysis for this case, supplying each
	// issue's MaterialityRank — the amplification signal ranking uses
	// (see rank.go) — and each FramedIssue's own Confidence.
	Issues issueagent.IssueAnalysisResult

	// Evidence is the evidence-weighing result for this case, supplying
	// FactWeights, Contradictions, and Gaps.
	Evidence evidenceweighing.Result

	// Law is the law-application result for this case, supplying each
	// IssueApplication's Confidence and Conflicts.
	Law lawapplication.Result

	// Opinion is the synthesis agent's draft opinion for this case,
	// supplying each TentativeConclusion's Confidence and Text.
	Opinion synthesisagent.Opinion

	// LowConfidenceThreshold overrides defaultLowConfidenceThreshold when
	// non-zero.
	LowConfidenceThreshold float64

	// ThinEvidenceWeightThreshold overrides
	// defaultThinEvidenceWeightThreshold when non-zero.
	ThinEvidenceWeightThreshold float64
}

// lowConfidenceThreshold returns req's effective low-confidence
// threshold: LowConfidenceThreshold if set, otherwise
// defaultLowConfidenceThreshold.
func (req Request) lowConfidenceThreshold() float64 {
	if req.LowConfidenceThreshold > 0 {
		return req.LowConfidenceThreshold
	}
	return defaultLowConfidenceThreshold
}

// thinEvidenceWeightThreshold returns req's effective thin-evidence
// weight threshold: ThinEvidenceWeightThreshold if set, otherwise
// defaultThinEvidenceWeightThreshold.
func (req Request) thinEvidenceWeightThreshold() float64 {
	if req.ThinEvidenceWeightThreshold > 0 {
		return req.ThinEvidenceWeightThreshold
	}
	return defaultThinEvidenceWeightThreshold
}

// materialityRankByIssue indexes req.Issues.Issues by
// SourceIssueNodeID, for O(1) lookup of an issue's MaterialityRank while
// scoring impact (see rank.go).
func (req Request) materialityRankByIssue() map[string]int {
	out := make(map[string]int, len(req.Issues.Issues))
	for _, fi := range req.Issues.Issues {
		out[fi.SourceIssueNodeID] = fi.MaterialityRank
	}
	return out
}

// groupByIssue groups uncertainties by IssueNodeID, preserving relative
// order within each group.
func groupByIssue(uncertainties []Uncertainty) map[string][]Uncertainty {
	out := make(map[string][]Uncertainty)
	for _, u := range uncertainties {
		out[u.IssueNodeID] = append(out[u.IssueNodeID], u)
	}
	return out
}
