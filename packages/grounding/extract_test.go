package grounding_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/grounding"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

func TestExtractClaims_ReferencesNumericsAndDates(t *testing.T) {
	tc := synthesisagent.TentativeConclusion{
		IssueNodeID:       "issue-1",
		Text:              "The parties signed a written memorandum on 2024-03-15 for $4,500.00.",
		SupportingFactIDs: []string{"fact-1"},
		SupportingRuleIDs: []string{"rule-1"},
	}

	claims := grounding.ExtractClaims(tc)

	var refs, numerics, dates int
	for _, c := range claims {
		if c.IssueNodeID != "issue-1" {
			t.Fatalf("expected IssueNodeID issue-1, got %q", c.IssueNodeID)
		}
		switch c.Kind {
		case grounding.ClaimReference:
			refs++
		case grounding.ClaimNumeric:
			numerics++
			if c.Value != "4,500.00" && c.Value != "$4,500.00" {
				t.Fatalf("unexpected numeric claim value %q", c.Value)
			}
		case grounding.ClaimDate:
			dates++
			if c.Value != "2024-03-15" {
				t.Fatalf("unexpected date claim value %q", c.Value)
			}
		case grounding.ClaimCitation:
			t.Fatalf("ExtractClaims never produces ClaimCitation claims (citation verification runs directly over SupportingRuleIDs, see citations.go), got %+v", c)
		}
	}

	if refs != 2 {
		t.Fatalf("expected 2 reference claims (1 fact + 1 rule), got %d", refs)
	}
	if numerics != 1 {
		t.Fatalf("expected 1 numeric claim, got %d", numerics)
	}
	if dates != 1 {
		t.Fatalf("expected 1 date claim, got %d", dates)
	}
}

func TestExtractClaims_NoFiguresInPlainText(t *testing.T) {
	tc := synthesisagent.TentativeConclusion{
		IssueNodeID: "issue-1",
		Text:        "The evidence favors the plaintiff on this issue.",
	}

	claims := grounding.ExtractClaims(tc)
	for _, c := range claims {
		if c.Kind == grounding.ClaimNumeric || c.Kind == grounding.ClaimDate {
			t.Fatalf("expected no numeric/date claims in plain prose, got %+v", c)
		}
	}
}

func TestExtractOpinionClaims_OnePerConclusion(t *testing.T) {
	opinion := synthesisagent.Opinion{
		CaseID: testCaseID,
		Conclusions: []synthesisagent.TentativeConclusion{
			{IssueNodeID: "issue-1", SupportingFactIDs: []string{"fact-1"}},
			{IssueNodeID: "issue-2", SupportingFactIDs: []string{"fact-2"}},
		},
	}

	claimSets := grounding.ExtractOpinionClaims(opinion)
	if len(claimSets) != 2 {
		t.Fatalf("expected 2 claim sets, got %d", len(claimSets))
	}
	if len(claimSets[0]) != 1 || claimSets[0][0].Value != "fact-1" {
		t.Fatalf("unexpected claims for conclusion 0: %+v", claimSets[0])
	}
	if len(claimSets[1]) != 1 || claimSets[1][0].Value != "fact-2" {
		t.Fatalf("unexpected claims for conclusion 1: %+v", claimSets[1])
	}
}
