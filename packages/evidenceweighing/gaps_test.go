package evidenceweighing_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestDetectGaps_FlagsMissingFact(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", SupportingFactIDs: []string{"fact-1", "fact-ghost"}},
	}
	facts := []evidenceweighing.FactRef{{ID: "fact-1"}}

	gaps := evidenceweighing.DetectGaps(arguments, facts, []string{"issue-1"})

	found := false
	for _, g := range gaps {
		if g.Kind == evidenceweighing.GapKindMissingFact && g.FactNodeID == "fact-ghost" && g.ArgumentID == "arg-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a GapKindMissingFact for fact-ghost, got %+v", gaps)
	}
}

func TestDetectGaps_FlagsUncitedIssue(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", SupportingFactIDs: []string{"fact-1"}},
	}
	facts := []evidenceweighing.FactRef{{ID: "fact-1"}}

	gaps := evidenceweighing.DetectGaps(arguments, facts, []string{"issue-1", "issue-2"})

	found := false
	for _, g := range gaps {
		if g.Kind == evidenceweighing.GapKindUncitedIssue && g.IssueNodeID == "issue-2" {
			found = true
		}
		if g.Kind == evidenceweighing.GapKindUncitedIssue && g.IssueNodeID == "issue-1" {
			t.Errorf("issue-1 was cited and should not be flagged as uncited")
		}
	}
	if !found {
		t.Errorf("expected a GapKindUncitedIssue for issue-2, got %+v", gaps)
	}
}

func TestDetectGaps_IssueArguedWithNoFactsIsUncited(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", SupportingFactIDs: nil},
	}

	gaps := evidenceweighing.DetectGaps(arguments, nil, []string{"issue-1"})

	found := false
	for _, g := range gaps {
		if g.Kind == evidenceweighing.GapKindUncitedIssue && g.IssueNodeID == "issue-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("an argument with zero supporting facts should still leave its issue flagged as uncited, got %+v", gaps)
	}
}

func TestDetectGaps_NoGapsWhenFullyCovered(t *testing.T) {
	arguments := []evidenceweighing.CitingArgument{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", SupportingFactIDs: []string{"fact-1"}},
	}
	facts := []evidenceweighing.FactRef{{ID: "fact-1"}}

	gaps := evidenceweighing.DetectGaps(arguments, facts, []string{"issue-1"})

	if len(gaps) != 0 {
		t.Errorf("expected no gaps, got %+v", gaps)
	}
}
