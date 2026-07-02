package fact_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/fact"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

func TestAttributeParty(t *testing.T) {
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Acme Corp"},
		{ID: "party-2", Role: timeline.PartySecond, Name: "Jane Doe"},
	}

	tests := []struct {
		name        string
		partyRole   evidence.PartyRole
		wantPartyID string
		wantRole    timeline.PartyRole
	}{
		{
			name:        "first party role resolves to matching party",
			partyRole:   evidence.PartyFirst,
			wantPartyID: "party-1",
			wantRole:    timeline.PartyFirst,
		},
		{
			name:        "second party role resolves to matching party",
			partyRole:   evidence.PartySecond,
			wantPartyID: "party-2",
			wantRole:    timeline.PartySecond,
		},
		{
			name:        "unattributed role resolves to nothing",
			partyRole:   evidence.PartyUnattributed,
			wantPartyID: "",
			wantRole:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attribution := fact.AttributeParty("fact-1", tt.partyRole, parties)
			if attribution.FactID != "fact-1" {
				t.Errorf("expected FactID fact-1, got %q", attribution.FactID)
			}
			if attribution.PartyID != tt.wantPartyID {
				t.Errorf("expected PartyID %q, got %q", tt.wantPartyID, attribution.PartyID)
			}
			if attribution.PartyRole != tt.wantRole {
				t.Errorf("expected PartyRole %q, got %q", tt.wantRole, attribution.PartyRole)
			}
		})
	}
}

func TestAttributeParty_NoMatchingPartyInRoster(t *testing.T) {
	attribution := fact.AttributeParty("fact-1", evidence.PartyFirst, nil)
	if attribution.PartyID != "" {
		t.Errorf("expected empty PartyID when no parties supplied, got %q", attribution.PartyID)
	}
	if attribution.PartyRole != timeline.PartyFirst {
		t.Errorf("expected PartyRole to still be resolved from the evidence role, got %q", attribution.PartyRole)
	}
}
