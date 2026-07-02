package uncertainty_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

func TestSurface_OverconfidencePhrasing_Detected(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Opinion: synthesisagent.Opinion{
			CaseID: "case-1",
			Conclusions: []synthesisagent.TentativeConclusion{
				{
					IssueNodeID: "issue-1",
					Text:        "The evidence definitely establishes the plaintiff's position beyond doubt.",
					Confidence:  0.95,
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	if len(report.OverconfidenceFlags) != 2 {
		t.Fatalf("expected 2 overconfidence flags (definitely, beyond doubt), got %d: %+v", len(report.OverconfidenceFlags), report.OverconfidenceFlags)
	}

	phrases := map[string]bool{}
	for _, f := range report.OverconfidenceFlags {
		phrases[f.Phrase] = true
		if f.IssueNodeID != "issue-1" {
			t.Errorf("flag.IssueNodeID = %q, want issue-1", f.IssueNodeID)
		}
		if f.Excerpt == "" {
			t.Errorf("expected non-empty excerpt for phrase %q", f.Phrase)
		}
	}
	if !phrases["definitely"] || !phrases["beyond doubt"] {
		t.Errorf("expected both 'definitely' and 'beyond doubt' flagged, got %+v", phrases)
	}
}

func TestSurface_OverconfidencePhrasing_CaseInsensitive(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Opinion: synthesisagent.Opinion{
			CaseID: "case-1",
			Conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "This CERTAINLY resolves the matter.", Confidence: 0.9},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.OverconfidenceFlags) != 1 {
		t.Fatalf("expected 1 overconfidence flag, got %d: %+v", len(report.OverconfidenceFlags), report.OverconfidenceFlags)
	}
	if report.OverconfidenceFlags[0].Phrase != "certainly" {
		t.Errorf("Phrase = %q, want certainly", report.OverconfidenceFlags[0].Phrase)
	}
}

// TestSurface_OverconfidencePhrasing_NoFalsePositives proves that
// carefully hedged, non-absolutist conclusion text produces zero
// overconfidence flags.
func TestSurface_OverconfidencePhrasing_NoFalsePositives(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Opinion: synthesisagent.Opinion{
			CaseID: "case-1",
			Conclusions: []synthesisagent.TentativeConclusion{
				{
					IssueNodeID: "issue-1",
					Text:        "The weight of the evidence appears to favor the plaintiff, though the record leaves some ambiguity.",
					Confidence:  0.7,
				},
				{
					IssueNodeID: "issue-2",
					Text:        "On balance, the applicable rule likely supports the defendant's position here.",
					Confidence:  0.6,
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.OverconfidenceFlags) != 0 {
		t.Errorf("expected no overconfidence flags for hedged text, got %+v", report.OverconfidenceFlags)
	}
}

func TestSurface_OverconfidencePhrasing_MultipleConclusions(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Opinion: synthesisagent.Opinion{
			CaseID: "case-1",
			Conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "This is a reasonable reading of the record.", Confidence: 0.8},
				{IssueNodeID: "issue-2", Text: "The facts undeniably support this outcome.", Confidence: 0.8},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.OverconfidenceFlags) != 1 {
		t.Fatalf("expected 1 flag, got %d: %+v", len(report.OverconfidenceFlags), report.OverconfidenceFlags)
	}
	if report.OverconfidenceFlags[0].IssueNodeID != "issue-2" {
		t.Errorf("IssueNodeID = %q, want issue-2", report.OverconfidenceFlags[0].IssueNodeID)
	}
}
