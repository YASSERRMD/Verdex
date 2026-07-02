package timeline

import "testing"

func strPtr(s string) *string { return &s }

func TestPartyRole_Valid(t *testing.T) {
	tests := []struct {
		name string
		role PartyRole
		want bool
	}{
		{"first", PartyFirst, true},
		{"second", PartySecond, true},
		{"third", PartyThird, true},
		{"unknown", PartyRole("bystander"), false},
		{"empty", PartyRole(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParty_Validate(t *testing.T) {
	tests := []struct {
		name    string
		party   Party
		wantErr error
	}{
		{
			name:    "valid first party",
			party:   Party{ID: "p1", Role: PartyFirst, Name: "Jane Doe"},
			wantErr: nil,
		},
		{
			name:    "valid with counsel",
			party:   Party{ID: "p2", Role: PartySecond, Name: "Acme Corp", Counsel: strPtr("John Smith")},
			wantErr: nil,
		},
		{
			name:    "empty ID",
			party:   Party{ID: "", Role: PartyFirst, Name: "Jane Doe"},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "whitespace ID",
			party:   Party{ID: "   ", Role: PartyFirst, Name: "Jane Doe"},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "empty name",
			party:   Party{ID: "p1", Role: PartyFirst, Name: ""},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "invalid role",
			party:   Party{ID: "p1", Role: PartyRole("plaintiff"), Name: "Jane Doe"},
			wantErr: ErrInvalidParty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.party.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParty_HasCounsel(t *testing.T) {
	tests := []struct {
		name  string
		party Party
		want  bool
	}{
		{"nil counsel", Party{ID: "p1"}, false},
		{"empty counsel", Party{ID: "p1", Counsel: strPtr("")}, false},
		{"whitespace counsel", Party{ID: "p1", Counsel: strPtr("   ")}, false},
		{"set counsel", Party{ID: "p1", Counsel: strPtr("John Smith")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.party.HasCounsel(); got != tt.want {
				t.Errorf("HasCounsel() = %v, want %v", got, tt.want)
			}
		})
	}
}
