package evidenceweighing_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestDetectContradictions_FlagsSameFactOpposingParties(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{
			ArgumentID:        "arg-p1-1",
			IssueNodeID:       "issue-1",
			PartyID:           "plaintiff",
			SupportingFactIDs: []string{"fact-1", "fact-2"},
		},
		{
			ArgumentID:        "arg-p2-1",
			IssueNodeID:       "issue-1",
			PartyID:           "defendant",
			SupportingFactIDs: []string{"fact-1"},
		},
	}

	contradictions := evidenceweighing.DetectContradictions(arguments)

	if len(contradictions) != 1 {
		t.Fatalf("len(contradictions) = %d, want 1", len(contradictions))
	}
	c := contradictions[0]
	if c.FactNodeID != "fact-1" || c.IssueNodeID != "issue-1" {
		t.Errorf("unexpected contradiction: %+v", c)
	}
	if c.ArgumentAID != "arg-p1-1" || c.ArgumentBID != "arg-p2-1" {
		t.Errorf("expected deterministic ordering (first-seen = A), got %+v", c)
	}
}

func TestDetectContradictions_NoFlagForSameParty(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}},
		{ArgumentID: "arg-2", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}},
	}

	if got := evidenceweighing.DetectContradictions(arguments); len(got) != 0 {
		t.Errorf("expected no contradictions for same-party citations, got %d", len(got))
	}
}

func TestDetectContradictions_NoFlagForDifferentIssues(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingFactIDs: []string{"fact-1"}},
		{ArgumentID: "arg-2", IssueNodeID: "issue-2", PartyID: "defendant", SupportingFactIDs: []string{"fact-1"}},
	}

	if got := evidenceweighing.DetectContradictions(arguments); len(got) != 0 {
		t.Errorf("expected no contradictions across different issues, got %d", len(got))
	}
}

func TestDetectContradictions_SkipsEmptyPartyID(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", PartyID: "", SupportingFactIDs: []string{"fact-1"}},
		{ArgumentID: "arg-2", IssueNodeID: "issue-1", PartyID: "defendant", SupportingFactIDs: []string{"fact-1"}},
	}

	if got := evidenceweighing.DetectContradictions(arguments); len(got) != 0 {
		t.Errorf("expected no contradictions when one side has empty PartyID, got %d", len(got))
	}
}

func TestCorroborationCounts(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", SupportingFactIDs: []string{"fact-1", "fact-2"}},
		{ArgumentID: "arg-2", SupportingFactIDs: []string{"fact-1"}},
		{ArgumentID: "arg-3", SupportingFactIDs: []string{"fact-1", "fact-1"}}, // duplicate within one argument
	}

	counts := evidenceweighing.CorroborationCounts(arguments)

	if counts["fact-1"] != 3 {
		t.Errorf("fact-1 corroboration count = %d, want 3 (dedup within a single argument)", counts["fact-1"])
	}
	if counts["fact-2"] != 1 {
		t.Errorf("fact-2 corroboration count = %d, want 1", counts["fact-2"])
	}
	if counts["fact-missing"] != 0 {
		t.Errorf("uncited fact should have count 0, got %d", counts["fact-missing"])
	}
}
