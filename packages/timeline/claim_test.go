package timeline

import "testing"

func TestClaim_Validate(t *testing.T) {
	tests := []struct {
		name    string
		claim   Claim
		wantErr error
	}{
		{
			name:    "valid with events",
			claim:   Claim{ID: "c1", PartyID: "p1", Description: "breach of lease", EventIDs: []string{"e1"}},
			wantErr: nil,
		},
		{
			name:    "valid with facts",
			claim:   Claim{ID: "c1", PartyID: "p1", Description: "breach of lease", FactIDs: []string{"f1"}},
			wantErr: nil,
		},
		{
			name:    "empty ID",
			claim:   Claim{ID: "", PartyID: "p1", Description: "x", EventIDs: []string{"e1"}},
			wantErr: ErrInvalidClaim,
		},
		{
			name:    "empty party ID",
			claim:   Claim{ID: "c1", PartyID: "", Description: "x", EventIDs: []string{"e1"}},
			wantErr: ErrInvalidClaim,
		},
		{
			name:    "empty description",
			claim:   Claim{ID: "c1", PartyID: "p1", Description: "", EventIDs: []string{"e1"}},
			wantErr: ErrInvalidClaim,
		},
		{
			name:    "no support",
			claim:   Claim{ID: "c1", PartyID: "p1", Description: "x"},
			wantErr: ErrInvalidClaim,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claim.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaim_SupportCount(t *testing.T) {
	c := Claim{EventIDs: []string{"e1", "e2"}, FactIDs: []string{"f1"}}
	if got := c.SupportCount(); got != 3 {
		t.Errorf("SupportCount() = %d, want 3", got)
	}
}

func TestValidateClaimLinkage(t *testing.T) {
	knownEvents := map[string]bool{"e1": true, "e2": true}
	knownFacts := map[string]bool{"f1": true}

	tests := []struct {
		name    string
		claim   Claim
		wantErr error
	}{
		{
			name:    "all known",
			claim:   Claim{EventIDs: []string{"e1", "e2"}, FactIDs: []string{"f1"}},
			wantErr: nil,
		},
		{
			name:    "unknown event",
			claim:   Claim{EventIDs: []string{"e99"}},
			wantErr: ErrEventNotFound,
		},
		{
			name:    "unknown fact",
			claim:   Claim{FactIDs: []string{"f99"}},
			wantErr: ErrEmptyInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClaimLinkage(tt.claim, knownEvents, knownFacts)
			if err != tt.wantErr {
				t.Errorf("ValidateClaimLinkage() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
