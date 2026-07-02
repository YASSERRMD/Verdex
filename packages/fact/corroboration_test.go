package fact_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/fact"
)

func TestDetectCorroboration_OverlappingFactsFromDifferentParties(t *testing.T) {
	candidates := []fact.CorroborationCandidate{
		{ID: "fact-1", Text: "The car ran the red light at the intersection.", PartyID: "party-1"},
		{ID: "fact-2", Text: "The car ran the red light at the intersection.", PartyID: "party-2"},
		{ID: "fact-3", Text: "Completely unrelated statement about the weather.", PartyID: "party-2"},
	}

	links := fact.DetectCorroboration(candidates)
	if len(links) != 1 {
		t.Fatalf("expected 1 corroboration link, got %d: %+v", len(links), links)
	}
	if links[0].FactAID != "fact-1" || links[0].FactBID != "fact-2" {
		t.Errorf("unexpected link: %+v", links[0])
	}
	if links[0].Overlap <= 0 {
		t.Errorf("expected positive overlap, got %v", links[0].Overlap)
	}
}

func TestDetectCorroboration_SamePartySkipped(t *testing.T) {
	candidates := []fact.CorroborationCandidate{
		{ID: "fact-1", Text: "The car ran the red light at the intersection.", PartyID: "party-1"},
		{ID: "fact-2", Text: "The car ran the red light at the intersection.", PartyID: "party-1"},
	}

	links := fact.DetectCorroboration(candidates)
	if len(links) != 0 {
		t.Errorf("expected no links for same-party facts, got %+v", links)
	}
}

func TestDetectCorroboration_UnattributedFactsStillCompared(t *testing.T) {
	candidates := []fact.CorroborationCandidate{
		{ID: "fact-1", Text: "The car ran the red light at the intersection.", PartyID: ""},
		{ID: "fact-2", Text: "The car ran the red light at the intersection.", PartyID: ""},
	}

	links := fact.DetectCorroboration(candidates)
	if len(links) != 1 {
		t.Errorf("expected 1 link for unattributed overlapping facts, got %+v", links)
	}
}

func TestCorroborationCount(t *testing.T) {
	links := []fact.CorroborationLink{
		{FactAID: "fact-1", FactBID: "fact-2"},
		{FactAID: "fact-1", FactBID: "fact-3"},
		{FactAID: "fact-4", FactBID: "fact-5"},
	}

	if got := fact.CorroborationCount("fact-1", links); got != 2 {
		t.Errorf("expected count 2 for fact-1, got %d", got)
	}
	if got := fact.CorroborationCount("fact-3", links); got != 1 {
		t.Errorf("expected count 1 for fact-3, got %d", got)
	}
	if got := fact.CorroborationCount("fact-9", links); got != 0 {
		t.Errorf("expected count 0 for unreferenced fact, got %d", got)
	}
}
