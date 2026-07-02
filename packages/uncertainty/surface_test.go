package uncertainty_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

func TestSurface_EmptyCaseID(t *testing.T) {
	_, err := uncertainty.Surface(uncertainty.Request{})
	if !errors.Is(err, uncertainty.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}

func TestSurface_NoFramedIssues(t *testing.T) {
	_, err := uncertainty.Surface(uncertainty.Request{CaseID: "case-1"})
	if !errors.Is(err, uncertainty.ErrNoFramedIssues) {
		t.Errorf("err = %v, want ErrNoFramedIssues", err)
	}
}

func TestSurface_CaseIDMismatch(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Evidence: evidenceweighing.Result{
			CaseID: "case-OTHER",
		},
	}
	_, err := uncertainty.Surface(req)
	if !errors.Is(err, uncertainty.ErrCaseIDMismatch) {
		t.Errorf("err = %v, want ErrCaseIDMismatch", err)
	}
}

func TestAnalyze_IsAliasForSurface(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-1",
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-1", MaterialityRank: 1, Confidence: 0.9},
			},
		},
	}

	surfaced, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	analyzed, err := uncertainty.Analyze(req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if surfaced.CaseID != analyzed.CaseID {
		t.Errorf("Analyze() and Surface() produced different CaseIDs: %q vs %q", analyzed.CaseID, surfaced.CaseID)
	}
	if len(surfaced.Uncertainties) != len(analyzed.Uncertainties) {
		t.Errorf("Analyze() and Surface() produced different uncertainty counts")
	}
}

func TestSurface_GeneratedAtIsPopulated(t *testing.T) {
	report, err := uncertainty.Surface(uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
	})
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if report.GeneratedAt.IsZero() {
		t.Errorf("expected GeneratedAt to be populated")
	}
}
