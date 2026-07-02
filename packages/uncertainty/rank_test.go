package uncertainty_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

// TestSurface_MaterialityWeightedRanking confirms that an equally severe
// finding on a more material issue (MaterialityRank 1) outranks one on a
// less material issue (MaterialityRank 3), per the plan's "rank
// uncertainties by impact on outcome" requirement.
func TestSurface_MaterialityWeightedRanking(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-1",
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-most-material", MaterialityRank: 1, Confidence: 0.9},
				{SourceIssueNodeID: "issue-mid", MaterialityRank: 2, Confidence: 0.9},
				{SourceIssueNodeID: "issue-least-material", MaterialityRank: 3, Confidence: 0.9},
			},
		},
		Law: lawapplication.Result{
			CaseID: "case-1",
			IssueApplications: []lawapplication.IssueApplication{
				// Identical Confidence -> identical Severity. Only
				// materiality should differentiate their ImpactRank.
				{IssueNodeID: "issue-most-material", Confidence: 0.1},
				{IssueNodeID: "issue-least-material", Confidence: 0.1},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.Uncertainties) != 2 {
		t.Fatalf("expected 2 uncertainties, got %d: %+v", len(report.Uncertainties), report.Uncertainties)
	}

	var mostMaterialRank, leastMaterialRank int
	for _, u := range report.Uncertainties {
		switch u.IssueNodeID {
		case "issue-most-material":
			mostMaterialRank = u.ImpactRank
		case "issue-least-material":
			leastMaterialRank = u.ImpactRank
		}
	}

	if mostMaterialRank == 0 || leastMaterialRank == 0 {
		t.Fatalf("expected both issues to produce a ranked finding, got %+v", report.Uncertainties)
	}
	if mostMaterialRank >= leastMaterialRank {
		t.Errorf("expected the most-material issue's finding to outrank the least-material one: mostMaterialRank=%d leastMaterialRank=%d", mostMaterialRank, leastMaterialRank)
	}
}

// TestSurface_ImpactRankIsContiguousAndOrdered confirms ImpactRank values
// are assigned 1..n in descending ImpactScore order with no gaps.
func TestSurface_ImpactRankIsContiguousAndOrdered(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-1",
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-1", MaterialityRank: 1, Confidence: 0.05},
				{SourceIssueNodeID: "issue-2", MaterialityRank: 2, Confidence: 0.1},
				{SourceIssueNodeID: "issue-3", MaterialityRank: 3, Confidence: 0.4},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.Uncertainties) != 3 {
		t.Fatalf("expected 3 uncertainties, got %d", len(report.Uncertainties))
	}

	for i, u := range report.Uncertainties {
		if u.ImpactRank != i+1 {
			t.Errorf("uncertainties[%d].ImpactRank = %d, want %d", i, u.ImpactRank, i+1)
		}
		if i > 0 && report.Uncertainties[i-1].ImpactScore < u.ImpactScore {
			t.Errorf("uncertainties not sorted by descending ImpactScore at index %d", i)
		}
	}
}

// TestSurface_ByIssueGroupsFindings confirms Report.ByIssue groups
// findings correctly, enabling a caller to look up "what's shaky about
// conclusion X".
func TestSurface_ByIssueGroupsFindings(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-1",
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-1", MaterialityRank: 1, Confidence: 0.1},
				{SourceIssueNodeID: "issue-2", MaterialityRank: 2, Confidence: 0.1},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	byIssue := report.ByIssue()
	if len(byIssue["issue-1"]) != 1 {
		t.Errorf("expected 1 finding for issue-1, got %d", len(byIssue["issue-1"]))
	}
	if len(byIssue["issue-2"]) != 1 {
		t.Errorf("expected 1 finding for issue-2, got %d", len(byIssue["issue-2"]))
	}
}
