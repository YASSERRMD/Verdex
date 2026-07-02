package uncertainty_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

func baseIssues() issueagent.IssueAnalysisResult {
	return issueagent.IssueAnalysisResult{
		CaseID: "case-1",
		Issues: []issueagent.FramedIssue{
			{SourceIssueNodeID: "issue-1", MaterialityRank: 1, Confidence: 0.9},
			{SourceIssueNodeID: "issue-2", MaterialityRank: 2, Confidence: 0.9},
		},
	}
}

func TestSurface_LowConfidenceIssueFraming(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-1",
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-1", MaterialityRank: 1, Confidence: 0.2},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceIssueFraming && u.IssueNodeID == "issue-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SourceIssueFraming uncertainty for issue-1, got %+v", report.Uncertainties)
	}
}

func TestSurface_LowConfidenceLawApplication(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Law: lawapplication.Result{
			CaseID: "case-1",
			IssueApplications: []lawapplication.IssueApplication{
				{IssueNodeID: "issue-1", Confidence: 0.1},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceLawApplication && u.IssueNodeID == "issue-1" && u.Detail == "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SourceLawApplication low-confidence uncertainty for issue-1, got %+v", report.Uncertainties)
	}
}

func TestSurface_LowConfidenceConclusion(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Opinion: synthesisagent.Opinion{
			CaseID: "case-1",
			Conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-2", Text: "The record favors the plaintiff.", Confidence: 0.15},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceConclusion && u.IssueNodeID == "issue-2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SourceConclusion uncertainty for issue-2, got %+v", report.Uncertainties)
	}
}

func TestSurface_NoLowConfidenceAboveThreshold(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}
	if len(report.Uncertainties) != 0 {
		t.Errorf("expected no uncertainties for high-confidence issues, got %+v", report.Uncertainties)
	}
}

func TestSurface_ThinEvidence_LowWeight(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Evidence: evidenceweighing.Result{
			CaseID: "case-1",
			FactWeights: []evidenceweighing.FactWeight{
				{FactNodeID: "fact-1", Weight: 0.1, Contradicted: false},
			},
		},
		Law: lawapplication.Result{
			CaseID: "case-1",
			IssueApplications: []lawapplication.IssueApplication{
				{
					IssueNodeID: "issue-1",
					Confidence:  0.9,
					ElementFactMap: []lawapplication.ElementFactEntry{
						{RuleID: "rule-1", FactNodeID: "fact-1"},
					},
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceEvidence && u.Detail == "fact-1" && u.IssueNodeID == "issue-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SourceEvidence uncertainty for fact-1 on issue-1, got %+v", report.Uncertainties)
	}
}

func TestSurface_ThinEvidence_Contradicted(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Evidence: evidenceweighing.Result{
			CaseID: "case-1",
			FactWeights: []evidenceweighing.FactWeight{
				{FactNodeID: "fact-1", Weight: 0.9, Contradicted: true},
			},
			Contradictions: []evidenceweighing.Contradiction{
				{
					FactNodeID:  "fact-1",
					IssueNodeID: "issue-1",
					ArgumentAID: "arg-a",
					ArgumentBID: "arg-b",
					PartyAID:    "plaintiff",
					PartyBID:    "defendant",
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	contradictionFindings := 0
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceEvidence && u.IssueNodeID == "issue-1" {
			contradictionFindings++
		}
	}
	// One from the Contradicted FactWeight (unattached to an issue via
	// ElementFactMap, so IssueNodeID is empty) and one from the
	// Contradiction record itself (IssueNodeID set directly).
	if contradictionFindings == 0 {
		t.Errorf("expected at least one SourceEvidence uncertainty for issue-1, got %+v", report.Uncertainties)
	}
}

func TestSurface_EvidenceGap(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Evidence: evidenceweighing.Result{
			CaseID: "case-1",
			Gaps: []evidenceweighing.Gap{
				{Kind: evidenceweighing.GapKindUncitedIssue, IssueNodeID: "issue-2", Description: "no evidence cited"},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceEvidence && u.IssueNodeID == "issue-2" {
			found = true
			if u.Caveat == "" {
				t.Errorf("expected non-empty caveat for gap finding")
			}
		}
	}
	if !found {
		t.Errorf("expected a SourceEvidence uncertainty for the gap on issue-2, got %+v", report.Uncertainties)
	}
}

func TestSurface_ConflictingLaw(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-1",
		Issues: baseIssues(),
		Law: lawapplication.Result{
			CaseID: "case-1",
			IssueApplications: []lawapplication.IssueApplication{
				{
					IssueNodeID: "issue-1",
					Confidence:  0.9,
					Conflicts: []lawapplication.ConflictingAuthority{
						{
							IssueNodeID:   "issue-1",
							FirstRuleID:   "rule-a",
							SecondRuleID:  "rule-b",
							FirstPartyID:  "plaintiff",
							SecondPartyID: "defendant",
							Rationale:     "opposing parties invoke conflicting rules",
						},
					},
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	found := false
	for _, u := range report.Uncertainties {
		if u.Source == uncertainty.SourceLawApplication && u.Detail == "rule-a vs rule-b" {
			found = true
			if u.Caveat == "" {
				t.Errorf("expected non-empty caveat for conflicting authority finding")
			}
		}
	}
	if !found {
		t.Errorf("expected a conflicting-authority uncertainty, got %+v", report.Uncertainties)
	}
}
