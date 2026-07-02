package timeline

import "testing"

func TestRelationship_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rel     Relationship
		wantErr error
	}{
		{
			name:    "valid landlord-tenant",
			rel:     Relationship{ID: "r1", PartyAID: "p1", PartyBID: "p2", Kind: KindLandlordTenant},
			wantErr: nil,
		},
		{
			name:    "empty ID",
			rel:     Relationship{ID: "", PartyAID: "p1", PartyBID: "p2", Kind: KindLandlordTenant},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "empty party A",
			rel:     Relationship{ID: "r1", PartyAID: "", PartyBID: "p2", Kind: KindLandlordTenant},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "empty party B",
			rel:     Relationship{ID: "r1", PartyAID: "p1", PartyBID: "", Kind: KindLandlordTenant},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "same party twice",
			rel:     Relationship{ID: "r1", PartyAID: "p1", PartyBID: "p1", Kind: KindLandlordTenant},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "empty kind",
			rel:     Relationship{ID: "r1", PartyAID: "p1", PartyBID: "p2", Kind: ""},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "free-form kind allowed",
			rel:     Relationship{ID: "r1", PartyAID: "p1", PartyBID: "p2", Kind: "business-partners"},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rel.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRelationship_Involves(t *testing.T) {
	rel := Relationship{ID: "r1", PartyAID: "p1", PartyBID: "p2", Kind: KindLandlordTenant}

	if !rel.Involves("p1") {
		t.Error("Involves(p1) = false, want true")
	}
	if !rel.Involves("p2") {
		t.Error("Involves(p2) = false, want true")
	}
	if rel.Involves("p3") {
		t.Error("Involves(p3) = true, want false")
	}
}
