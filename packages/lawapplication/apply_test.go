package lawapplication_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

func TestApply_EmptyCaseID(t *testing.T) {
	_, err := lawapplication.Apply(lawapplication.Request{})
	if !errors.Is(err, lawapplication.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}

func TestApply_NoIssues(t *testing.T) {
	_, err := lawapplication.Apply(lawapplication.Request{CaseID: "case-1"})
	if !errors.Is(err, lawapplication.ErrNoIssues) {
		t.Errorf("err = %v, want ErrNoIssues", err)
	}
}

func TestApply_CaseIDMismatch_FirstParty(t *testing.T) {
	req := lawapplication.Request{
		CaseID: "case-1",
		Issues: []lawapplication.IssueInput{{Issue: issueagent.FramedIssue{SourceIssueNodeID: "issue-1"}}},
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID:    "case-OTHER",
			Arguments: []firstpartyagent.Argument{{ID: "arg-1", IssueNodeID: "issue-1"}},
		},
	}
	_, err := lawapplication.Apply(req)
	if !errors.Is(err, lawapplication.ErrCaseIDMismatch) {
		t.Errorf("err = %v, want ErrCaseIDMismatch", err)
	}
}

func TestApply_CaseIDMismatch_Evidence(t *testing.T) {
	req := lawapplication.Request{
		CaseID:   "case-1",
		Issues:   []lawapplication.IssueInput{{Issue: issueagent.FramedIssue{SourceIssueNodeID: "issue-1"}}},
		Evidence: evidenceweighing.Result{CaseID: "case-OTHER"},
	}
	_, err := lawapplication.Apply(req)
	if !errors.Is(err, lawapplication.ErrCaseIDMismatch) {
		t.Errorf("err = %v, want ErrCaseIDMismatch", err)
	}
}

func TestApply_EndToEnd(t *testing.T) {
	req := lawapplication.Request{
		CaseID: "case-1",
		Issues: []lawapplication.IssueInput{
			{
				Issue:            issueagent.FramedIssue{SourceIssueNodeID: "issue-1", GoverningQuestions: []string{"was notice reasonable?"}},
				GoverningRuleIDs: []string{"rule-statute"},
			},
		},
		Rules: []lawapplication.RuleRef{
			{ID: "rule-statute", Text: "42 U.S.C. § 1983 requires reasonable notice."},
			{ID: "rule-precedent", Text: "In Smith v. Jones, the court held notice must be actual."},
		},
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID:  "case-1",
			PartyID: "plaintiff",
			Arguments: []firstpartyagent.Argument{
				{
					ID:                "arg-p1",
					IssueNodeID:       "issue-1",
					PartyID:           "plaintiff",
					SupportingFactIDs: []string{"fact-1"},
					SupportingRuleIDs: []string{"rule-statute"},
				},
			},
		},
		SecondParty: secondpartyagent.ArgumentSet{
			CaseID:  "case-1",
			PartyID: "defendant",
			Arguments: []secondpartyagent.Argument{
				{
					ID:                "arg-p2",
					IssueNodeID:       "issue-1",
					PartyID:           "defendant",
					SupportingFactIDs: []string{"fact-2"},
					SupportingRuleIDs: []string{"rule-precedent"},
				},
			},
		},
		Evidence: evidenceweighing.Result{
			CaseID: "case-1",
			FactWeights: []evidenceweighing.FactWeight{
				{FactNodeID: "fact-1", Weight: 0.8},
				{FactNodeID: "fact-2", Weight: 0.6},
			},
		},
		LegalFamily: lawapplication.CommonLawFamily,
		CitationLookup: func(ruleID string) (string, lawapplication.Origin, bool, string, error) {
			return "Fake Reporter " + ruleID, lawapplication.OriginUnknown, true, "verified", nil
		},
	}

	result, err := lawapplication.Apply(req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if result.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", result.CaseID)
	}
	if len(result.IssueApplications) != 1 {
		t.Fatalf("len(IssueApplications) = %d, want 1", len(result.IssueApplications))
	}

	ia := result.IssueApplications[0]
	if ia.IssueNodeID != "issue-1" {
		t.Errorf("IssueNodeID = %q, want issue-1", ia.IssueNodeID)
	}
	if len(ia.ControllingRuleIDs) != 2 {
		t.Fatalf("ControllingRuleIDs = %v, want 2 entries", ia.ControllingRuleIDs)
	}
	if len(ia.Conflicts) != 1 {
		t.Errorf("Conflicts = %v, want 1 (rule-statute/plaintiff vs rule-precedent/defendant)", ia.Conflicts)
	}
	if len(ia.Citations) != 2 {
		t.Errorf("Citations = %v, want 2", ia.Citations)
	}
	for _, c := range ia.Citations {
		if !c.Resolved || !c.Verified {
			t.Errorf("citation %+v should be resolved and verified", c)
		}
	}
	if len(ia.ElementFactMap) != 2 {
		t.Errorf("ElementFactMap = %v, want 2 entries (fact-1 for rule-statute, fact-2 for rule-precedent)", ia.ElementFactMap)
	}
	if ia.Confidence <= 0 {
		t.Errorf("Confidence = %v, want > 0", ia.Confidence)
	}
	if len(ia.Steps) == 0 {
		t.Errorf("Steps should be non-empty")
	}

	if result.GeneratedAt.IsZero() {
		t.Errorf("GeneratedAt should be set")
	}
}

func TestApply_OnlyOnePartyArgued(t *testing.T) {
	req := lawapplication.Request{
		CaseID: "case-1",
		Issues: []lawapplication.IssueInput{
			{Issue: issueagent.FramedIssue{SourceIssueNodeID: "issue-1"}, GoverningRuleIDs: []string{"rule-1"}},
		},
		Rules: []lawapplication.RuleRef{{ID: "rule-1"}},
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID:  "case-1",
			PartyID: "plaintiff",
			Arguments: []firstpartyagent.Argument{
				{ID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}, SupportingRuleIDs: []string{"rule-1"}},
			},
		},
	}

	result, err := lawapplication.Apply(req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(result.IssueApplications[0].Conflicts) != 0 {
		t.Errorf("a single party's arguments cannot conflict, got %+v", result.IssueApplications[0].Conflicts)
	}
}

func TestApply_MultipleIssuesPreserveOrder(t *testing.T) {
	req := lawapplication.Request{
		CaseID: "case-1",
		Issues: []lawapplication.IssueInput{
			{Issue: issueagent.FramedIssue{SourceIssueNodeID: "issue-2"}},
			{Issue: issueagent.FramedIssue{SourceIssueNodeID: "issue-1"}},
		},
	}
	result, err := lawapplication.Apply(req)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(result.IssueApplications) != 2 {
		t.Fatalf("len(IssueApplications) = %d, want 2", len(result.IssueApplications))
	}
	if result.IssueApplications[0].IssueNodeID != "issue-2" || result.IssueApplications[1].IssueNodeID != "issue-1" {
		t.Errorf("issue order not preserved: %v", result.IssueApplications)
	}
}
