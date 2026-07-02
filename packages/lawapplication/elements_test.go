package lawapplication_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestBuildElementFactMap_AggregatesFactsAndParties(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{
			ArgumentID:        "arg-p1",
			IssueNodeID:       "issue-1",
			PartyID:           "plaintiff",
			SupportingFactIDs: []string{"fact-1", "fact-2"},
			SupportingRuleIDs: []string{"rule-1"},
		},
		{
			ArgumentID:        "arg-p2",
			IssueNodeID:       "issue-1",
			PartyID:           "defendant",
			SupportingFactIDs: []string{"fact-1"},
			SupportingRuleIDs: []string{"rule-1"},
		},
		{
			ArgumentID:        "arg-other-rule",
			IssueNodeID:       "issue-1",
			PartyID:           "defendant",
			SupportingFactIDs: []string{"fact-99"},
			SupportingRuleIDs: []string{"rule-2"},
		},
	}

	evidence := evidenceweighing.Result{
		FactWeights: []evidenceweighing.FactWeight{
			{FactNodeID: "fact-1", Weight: 0.75, Contradicted: true},
			{FactNodeID: "fact-2", Weight: 0.4},
		},
	}

	got := lawapplication.BuildElementFactMap("issue-1", "rule-1", args, evidence)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	byFact := make(map[string]lawapplication.ElementFactEntry, len(got))
	for _, e := range got {
		byFact[e.FactNodeID] = e
	}

	fact1 := byFact["fact-1"]
	if fact1.FactWeight != 0.75 || !fact1.Contradicted {
		t.Errorf("fact-1 entry = %+v, want weight 0.75 contradicted true", fact1)
	}
	if len(fact1.CitingPartyIDs) != 2 {
		t.Errorf("fact-1 CitingPartyIDs = %v, want 2 parties", fact1.CitingPartyIDs)
	}

	fact2 := byFact["fact-2"]
	if fact2.FactWeight != 0.4 || fact2.Contradicted {
		t.Errorf("fact-2 entry = %+v, want weight 0.4 contradicted false", fact2)
	}
	if len(fact2.CitingPartyIDs) != 1 || fact2.CitingPartyIDs[0] != "plaintiff" {
		t.Errorf("fact-2 CitingPartyIDs = %v, want [plaintiff]", fact2.CitingPartyIDs)
	}
}

func TestBuildElementFactMap_MissingFactWeightDefaultsToZero(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-ghost"}, SupportingRuleIDs: []string{"rule-1"}},
	}

	got := lawapplication.BuildElementFactMap("issue-1", "rule-1", args, evidenceweighing.Result{})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].FactWeight != 0 || got[0].Contradicted {
		t.Errorf("got[0] = %+v, want zero weight, not contradicted", got[0])
	}
}

func TestBuildElementFactMap_NoCitingArguments(t *testing.T) {
	got := lawapplication.BuildElementFactMap("issue-1", "rule-1", nil, evidenceweighing.Result{})
	if len(got) != 0 {
		t.Errorf("got = %v, want empty", got)
	}
}
