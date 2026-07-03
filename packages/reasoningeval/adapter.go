package reasoningeval

import (
	"github.com/YASSERRMD/verdex/packages/grounding"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// opinionAdapter adapts a synthesisagent.Opinion to OpinionLike.
type opinionAdapter struct {
	opinion synthesisagent.Opinion
}

// WrapOpinion adapts a synthesisagent.Opinion so it can be used as the
// Opinion field of a ScoreInput.
func WrapOpinion(opinion synthesisagent.Opinion) OpinionLike {
	return opinionAdapter{opinion: opinion}
}

func (a opinionAdapter) OpinionCaseID() string { return a.opinion.CaseID }

func (a opinionAdapter) ConclusionCount() int { return len(a.opinion.Conclusions) }

func (a opinionAdapter) ConclusionText(i int) string {
	if i < 0 || i >= len(a.opinion.Conclusions) {
		return ""
	}
	return a.opinion.Conclusions[i].Text
}

func (a opinionAdapter) ConclusionConfidence(i int) float64 {
	if i < 0 || i >= len(a.opinion.Conclusions) {
		return 0
	}
	return a.opinion.Conclusions[i].Confidence
}

func (a opinionAdapter) SkippedIssueCount() int { return len(a.opinion.SkippedIssueNodeIDs) }

// groundingReportAdapter adapts a packages/grounding.Report to
// GroundingReportLike.
type groundingReportAdapter struct {
	report grounding.Report
}

// WrapGroundingReport adapts a packages/grounding.Report so it can be
// used as the GroundingReport field of a ScoreInput.
func WrapGroundingReport(report grounding.Report) GroundingReportLike {
	return groundingReportAdapter{report: report}
}

func (a groundingReportAdapter) OpinionScoreValue() float64 {
	return a.report.OpinionScore
}

func (a groundingReportAdapter) CitationFindingCount() int {
	return len(a.report.AllCitationFindings())
}

func (a groundingReportAdapter) CriticalCitationFindingCount() int {
	count := 0
	for _, f := range a.report.AllCitationFindings() {
		if f.Severity == citationSeverityCritical {
			count++
		}
	}
	return count
}

// citationSeverityCritical mirrors packages/citation.SeverityCritical's
// string value locally so this file need not import packages/citation
// solely for one constant comparison; grounding.Report.AllCitationFindings
// already returns []citation.Finding, so the import is unavoidable at the
// call site above via the report package, but keeping the literal here
// documents the exact value being compared without adding a second import
// alias.
const citationSeverityCritical = "critical"
