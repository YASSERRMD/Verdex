package uncertainty_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

// TestSurface_EndToEnd exercises Surface against realistic fixture data
// spanning all four upstream result types for a single, multi-issue
// case: a well-supported issue that should produce no findings, and a
// second, more material issue carrying a low-confidence framing, thin
// and contradicted evidence, conflicting authority, a low-confidence law
// application, and an over-confidently worded tentative conclusion — the
// full breadth of what this package is meant to surface, together.
func TestSurface_EndToEnd(t *testing.T) {
	req := uncertainty.Request{
		CaseID: "case-42",
		Issues: issueagent.IssueAnalysisResult{
			CaseID: "case-42",
			Issues: []issueagent.FramedIssue{
				{
					SourceIssueNodeID: "issue-liability",
					MaterialityRank:   1,
					MaterialityScore:  0.95,
					GoverningQuestions: []string{
						"Did the defendant breach the duty of care?",
					},
					Confidence: 0.25, // low: flags SourceIssueFraming
				},
				{
					SourceIssueNodeID:  "issue-damages",
					MaterialityRank:    2,
					MaterialityScore:   0.4,
					GoverningQuestions: []string{"What damages, if any, follow?"},
					Confidence:         0.9, // well-supported, no finding
				},
			},
		},
		Evidence: evidenceweighing.Result{
			CaseID: "case-42",
			FactWeights: []evidenceweighing.FactWeight{
				{FactNodeID: "fact-skid-marks", Weight: 0.2, Kind: evidenceweighing.EvidenceKindDocumentary, Contradicted: false, CorroborationCount: 1, Rationale: "single uncorroborated document"},
				{FactNodeID: "fact-witness-a", Weight: 0.85, Contradicted: true, CorroborationCount: 2, Rationale: "corroborated but contradicted by opposing witness"},
				{FactNodeID: "fact-damages-invoice", Weight: 0.9, Contradicted: false, CorroborationCount: 3, Rationale: "strong documentary support"},
			},
			Contradictions: []evidenceweighing.Contradiction{
				{
					FactNodeID:  "fact-witness-a",
					IssueNodeID: "issue-liability",
					ArgumentAID: "arg-plaintiff-1",
					ArgumentBID: "arg-defendant-1",
					PartyAID:    "plaintiff",
					PartyBID:    "defendant",
				},
			},
			Gaps: []evidenceweighing.Gap{
				{
					Kind:        evidenceweighing.GapKindUncitedIssue,
					IssueNodeID: "issue-liability",
					Description: "no fact was cited in support of causation",
				},
			},
			LegalFamily: evidenceweighing.CommonLawFamily,
		},
		Law: lawapplication.Result{
			CaseID: "case-42",
			IssueApplications: []lawapplication.IssueApplication{
				{
					IssueNodeID:        "issue-liability",
					ControllingRuleIDs: []string{"rule-negligence", "rule-strict-liability"},
					ElementFactMap: []lawapplication.ElementFactEntry{
						{RuleID: "rule-negligence", FactNodeID: "fact-skid-marks", FactWeight: 0.2, Contradicted: false, CitingPartyIDs: []string{"plaintiff"}},
						{RuleID: "rule-negligence", FactNodeID: "fact-witness-a", FactWeight: 0.85, Contradicted: true, CitingPartyIDs: []string{"plaintiff", "defendant"}},
					},
					Conflicts: []lawapplication.ConflictingAuthority{
						{
							IssueNodeID:   "issue-liability",
							FirstRuleID:   "rule-negligence",
							SecondRuleID:  "rule-strict-liability",
							FirstPartyID:  "plaintiff",
							SecondPartyID: "defendant",
							Rationale:     "opposing parties invoke different liability theories",
						},
					},
					Confidence: 0.3, // low: flags SourceLawApplication
				},
				{
					IssueNodeID:        "issue-damages",
					ControllingRuleIDs: []string{"rule-compensatory-damages"},
					ElementFactMap: []lawapplication.ElementFactEntry{
						{RuleID: "rule-compensatory-damages", FactNodeID: "fact-damages-invoice", FactWeight: 0.9, Contradicted: false, CitingPartyIDs: []string{"plaintiff"}},
					},
					Confidence: 0.85,
				},
			},
		},
		Opinion: synthesisagent.Opinion{
			CaseID: "case-42",
			Conclusions: []synthesisagent.TentativeConclusion{
				{
					IssueNodeID:       "issue-liability",
					Text:              "The evidence undeniably establishes the defendant's negligence beyond doubt.",
					FavoredParty:      "plaintiff",
					Confidence:        0.3,
					WeakestLink:       "fact-witness-a is contradicted by the defendant's own witness",
					SupportingFactIDs: []string{"fact-skid-marks", "fact-witness-a"},
					SupportingRuleIDs: []string{"rule-negligence"},
					Grounded:          true,
				},
				{
					IssueNodeID:       "issue-damages",
					Text:              "The damages invoice supports an award of compensatory damages.",
					FavoredParty:      "plaintiff",
					Confidence:        0.85,
					WeakestLink:       "damages invoice is single-sourced",
					SupportingFactIDs: []string{"fact-damages-invoice"},
					SupportingRuleIDs: []string{"rule-compensatory-damages"},
					Grounded:          true,
				},
			},
		},
	}

	report, err := uncertainty.Surface(req)
	if err != nil {
		t.Fatalf("Surface() error = %v", err)
	}

	if report.CaseID != "case-42" {
		t.Errorf("CaseID = %q, want case-42", report.CaseID)
	}
	if report.GeneratedAt.IsZero() {
		t.Errorf("expected GeneratedAt to be populated")
	}
	if len(report.Uncertainties) == 0 {
		t.Fatalf("expected uncertainties, got none")
	}

	// The top-ranked finding must be on the most material issue.
	if report.Uncertainties[0].IssueNodeID != "issue-liability" {
		t.Errorf("top-ranked finding.IssueNodeID = %q, want issue-liability", report.Uncertainties[0].IssueNodeID)
	}

	// issue-damages, the well-supported issue, must carry no findings.
	byIssue := report.ByIssue()
	if findings := byIssue["issue-damages"]; len(findings) != 0 {
		t.Errorf("expected no findings for the well-supported issue-damages, got %+v", findings)
	}

	// issue-liability should have accumulated multiple distinct kinds of
	// findings: low framing confidence, thin/contradicted evidence, an
	// evidentiary gap, conflicting authority, and low law-application
	// confidence.
	liabilityFindings := byIssue["issue-liability"]
	if len(liabilityFindings) < 4 {
		t.Errorf("expected at least 4 distinct findings for issue-liability, got %d: %+v", len(liabilityFindings), liabilityFindings)
	}
	sources := map[uncertainty.Source]bool{}
	for _, u := range liabilityFindings {
		sources[u.Source] = true
		if u.Caveat == "" {
			t.Errorf("finding %+v has an empty caveat", u)
		}
	}
	if !sources[uncertainty.SourceIssueFraming] {
		t.Errorf("expected a SourceIssueFraming finding for issue-liability")
	}
	if !sources[uncertainty.SourceEvidence] {
		t.Errorf("expected a SourceEvidence finding for issue-liability")
	}
	if !sources[uncertainty.SourceLawApplication] {
		t.Errorf("expected a SourceLawApplication finding for issue-liability")
	}

	// The over-confidently worded conclusion on issue-liability must be
	// flagged; the plainly worded issue-damages conclusion must not be.
	if len(report.OverconfidenceFlags) != 2 {
		t.Fatalf("expected 2 overconfidence flags (undeniably, beyond doubt), got %d: %+v", len(report.OverconfidenceFlags), report.OverconfidenceFlags)
	}
	for _, f := range report.OverconfidenceFlags {
		if f.IssueNodeID != "issue-liability" {
			t.Errorf("overconfidence flag on unexpected issue %q", f.IssueNodeID)
		}
	}
}
