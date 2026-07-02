package evidenceweighing_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

func TestWeigh_EmptyCaseID(t *testing.T) {
	_, err := evidenceweighing.Weigh(evidenceweighing.WeighRequest{})
	if !errors.Is(err, evidenceweighing.ErrEmptyCaseID) {
		t.Errorf("err = %v, want ErrEmptyCaseID", err)
	}
}

func TestWeigh_NoArgumentSets(t *testing.T) {
	_, err := evidenceweighing.Weigh(evidenceweighing.WeighRequest{CaseID: "case-1"})
	if !errors.Is(err, evidenceweighing.ErrNoArgumentSets) {
		t.Errorf("err = %v, want ErrNoArgumentSets", err)
	}
}

func TestWeigh_CaseIDMismatch(t *testing.T) {
	req := evidenceweighing.WeighRequest{
		CaseID: "case-1",
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID: "case-OTHER",
			Arguments: []firstpartyagent.Argument{
				{ID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}},
			},
		},
	}

	_, err := evidenceweighing.Weigh(req)
	if !errors.Is(err, evidenceweighing.ErrCaseIDMismatch) {
		t.Errorf("err = %v, want ErrCaseIDMismatch", err)
	}
}

func TestWeigh_EndToEnd(t *testing.T) {
	req := evidenceweighing.WeighRequest{
		CaseID: "case-1",
		Facts: []evidenceweighing.FactRef{
			{ID: "fact-1", Text: "The witness testified the light was red.", Confidence: 0.8},
			{ID: "fact-2", Text: "The contract was signed on May 1.", Confidence: 0.9},
			{ID: "fact-3", Confidence: 0.4},
		},
		IssueNodeIDs: []string{"issue-1", "issue-2"},
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID:  "case-1",
			PartyID: "plaintiff",
			Arguments: []firstpartyagent.Argument{
				{
					ID:                "arg-p1-1",
					IssueNodeID:       "issue-1",
					PartyID:           "plaintiff",
					SupportingFactIDs: []string{"fact-1", "fact-2", "fact-ghost"},
					Strength:          0.7,
				},
			},
		},
		SecondParty: secondpartyagent.ArgumentSet{
			CaseID:  "case-1",
			PartyID: "defendant",
			Arguments: []secondpartyagent.Argument{
				{
					ID:                "arg-p2-1",
					IssueNodeID:       "issue-1",
					PartyID:           "defendant",
					SupportingFactIDs: []string{"fact-1"},
					Strength:          0.5,
				},
			},
		},
		LegalFamily: evidenceweighing.CommonLawFamily,
	}

	result, err := evidenceweighing.Weigh(req)
	if err != nil {
		t.Fatalf("Weigh returned error: %v", err)
	}

	if result.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", result.CaseID)
	}
	if len(result.FactWeights) != 3 {
		t.Fatalf("len(FactWeights) = %d, want 3", len(result.FactWeights))
	}
	if len(result.Contradictions) != 1 {
		t.Fatalf("len(Contradictions) = %d, want 1 (fact-1 cited by both parties on issue-1)", len(result.Contradictions))
	}

	foundMissingFact := false
	foundUncitedIssue := false
	for _, g := range result.Gaps {
		switch g.Kind {
		case evidenceweighing.GapKindMissingFact:
			if g.FactNodeID == "fact-ghost" {
				foundMissingFact = true
			}
		case evidenceweighing.GapKindUncitedIssue:
			if g.IssueNodeID == "issue-2" {
				foundUncitedIssue = true
			}
		}
	}
	if !foundMissingFact {
		t.Errorf("expected a missing-fact gap for fact-ghost, got %+v", result.Gaps)
	}
	if !foundUncitedIssue {
		t.Errorf("expected an uncited-issue gap for issue-2, got %+v", result.Gaps)
	}

	if result.GeneratedAt.IsZero() {
		t.Errorf("GeneratedAt should be set")
	}
}

func TestWeigh_OnlyOnePartyArgued(t *testing.T) {
	req := evidenceweighing.WeighRequest{
		CaseID: "case-1",
		Facts:  []evidenceweighing.FactRef{{ID: "fact-1", Confidence: 0.5}},
		FirstParty: firstpartyagent.ArgumentSet{
			CaseID: "case-1",
			Arguments: []firstpartyagent.Argument{
				{ID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}},
			},
		},
	}

	result, err := evidenceweighing.Weigh(req)
	if err != nil {
		t.Fatalf("Weigh returned error: %v", err)
	}
	if len(result.Contradictions) != 0 {
		t.Errorf("a single party's arguments cannot contradict, got %+v", result.Contradictions)
	}
}
