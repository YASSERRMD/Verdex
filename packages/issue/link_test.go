package issue_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/issue"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

func TestLinkIssues_LinksPartyByNameMention(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether Acme Corp breached the supply agreement"},
	}
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Acme Corp"},
		{ID: "party-2", Role: timeline.PartySecond, Name: "Beta Industries"},
	}

	links := issue.LinkIssues(issues, parties, nil)

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if len(links[0].PartyIDs) != 1 || links[0].PartyIDs[0] != "party-1" {
		t.Errorf("expected only party-1 linked, got %+v", links[0].PartyIDs)
	}
}

func TestLinkIssues_LinksRelatedFacts(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the shipment arrived damaged"},
	}
	facts := map[string]string{
		"fact-1": "The shipment arrived with visible water damage to the packaging.",
		"fact-2": "The invoice was issued on the fifteenth of March.",
	}

	links := issue.LinkIssues(issues, nil, facts)

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	found := false
	for _, id := range links[0].FactIDs {
		if id == "fact-1" {
			found = true
		}
		if id == "fact-2" {
			t.Errorf("did not expect unrelated fact-2 to be linked")
		}
	}
	if !found {
		t.Errorf("expected fact-1 to be linked, got %+v", links[0].FactIDs)
	}
}

func TestLinkIssues_NoMatchesReturnsEmptyLinks(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the vehicle was roadworthy"},
	}
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Jordan Lee"},
	}

	links := issue.LinkIssues(issues, parties, nil)

	if len(links) != 1 {
		t.Fatalf("expected 1 link entry (possibly empty), got %d", len(links))
	}
	if len(links[0].PartyIDs) != 0 {
		t.Errorf("expected no party match, got %+v", links[0].PartyIDs)
	}
}
