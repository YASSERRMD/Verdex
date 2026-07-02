package fact_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/fact"
)

func TestDetermineDisputeStatus(t *testing.T) {
	tests := []struct {
		name       string
		candidate  fact.FactWithParty
		peers      []fact.FactWithParty
		wantStatus fact.DisputeStatus
	}{
		{
			name:      "contradictory facts from different parties are disputed",
			candidate: fact.FactWithParty{ID: "fact-1", Text: "The defendant did not pay the invoice.", PartyID: "party-1"},
			peers: []fact.FactWithParty{
				{ID: "fact-1", Text: "The defendant did not pay the invoice.", PartyID: "party-1"},
				{ID: "fact-2", Text: "The defendant paid the invoice in full.", PartyID: "party-2"},
			},
			wantStatus: fact.Disputed,
		},
		{
			name:      "non-contradictory facts from different parties are undisputed",
			candidate: fact.FactWithParty{ID: "fact-1", Text: "The meeting occurred on March 3rd.", PartyID: "party-1"},
			peers: []fact.FactWithParty{
				{ID: "fact-1", Text: "The meeting occurred on March 3rd.", PartyID: "party-1"},
				{ID: "fact-2", Text: "The invoice was sent by mail.", PartyID: "party-2"},
			},
			wantStatus: fact.Undisputed,
		},
		{
			name:      "same-party facts never dispute each other",
			candidate: fact.FactWithParty{ID: "fact-1", Text: "The defendant did not pay the invoice.", PartyID: "party-1"},
			peers: []fact.FactWithParty{
				{ID: "fact-1", Text: "The defendant did not pay the invoice.", PartyID: "party-1"},
				{ID: "fact-2", Text: "The defendant paid the invoice in full.", PartyID: "party-1"},
			},
			wantStatus: fact.Undisputed,
		},
		{
			name:       "no party id is unknown",
			candidate:  fact.FactWithParty{ID: "fact-1", Text: "The defendant did not pay.", PartyID: ""},
			peers:      []fact.FactWithParty{{ID: "fact-2", Text: "The defendant paid.", PartyID: "party-2"}},
			wantStatus: fact.Unknown,
		},
		{
			name:       "no peers is unknown",
			candidate:  fact.FactWithParty{ID: "fact-1", Text: "The defendant did not pay.", PartyID: "party-1"},
			peers:      nil,
			wantStatus: fact.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := fact.DetermineDisputeStatus(tt.candidate, tt.peers)
			if status != tt.wantStatus {
				t.Errorf("expected status %v, got %v", tt.wantStatus, status)
			}
		})
	}
}

func TestDetermineDisputeStatus_ReturnsContradictingPeerID(t *testing.T) {
	candidate := fact.FactWithParty{ID: "fact-1", Text: "The witness denied being present.", PartyID: "party-1"}
	peers := []fact.FactWithParty{
		candidate,
		{ID: "fact-2", Text: "The witness admitted being present.", PartyID: "party-2"},
	}

	status, peerID := fact.DetermineDisputeStatus(candidate, peers)
	if status != fact.Disputed {
		t.Fatalf("expected Disputed, got %v", status)
	}
	if peerID != "fact-2" {
		t.Errorf("expected contradicting peer id fact-2, got %q", peerID)
	}
}
