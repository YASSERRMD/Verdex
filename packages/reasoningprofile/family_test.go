package reasoningprofile_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

func TestResolveFamily_MatchesJurisdictionLegalFamily(t *testing.T) {
	tests := []struct {
		name string
		lf   jurisdiction.LegalFamily
		want reasoningprofile.Family
	}{
		{"common_law", jurisdiction.LegalFamilyCommonLaw, reasoningprofile.FamilyCommonLaw},
		{"civil_law", jurisdiction.LegalFamilyCivilLaw, reasoningprofile.FamilyCivilLaw},
		{"mixed", jurisdiction.LegalFamilyMixed, reasoningprofile.FamilyMixed},
		{"islamic_law", jurisdiction.LegalFamilyIslamicLaw, reasoningprofile.FamilyIslamicLaw},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := jurisdiction.Jurisdiction{
				CountryCode: "XX",
				LegalFamily: tt.lf,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			got := reasoningprofile.ResolveFamily(j)
			if got != tt.want {
				t.Errorf("ResolveFamily(%v) = %v, want %v", tt.lf, got, tt.want)
			}
			if string(got) != string(tt.lf) {
				t.Errorf("Family string value %q does not match jurisdiction.LegalFamily value %q", got, tt.lf)
			}
		})
	}
}

func TestFamily_IsValid(t *testing.T) {
	valid := []reasoningprofile.Family{
		reasoningprofile.FamilyCommonLaw,
		reasoningprofile.FamilyCivilLaw,
		reasoningprofile.FamilyMixed,
		reasoningprofile.FamilyIslamicLaw,
	}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("Family(%q).IsValid() = false, want true", f)
		}
	}

	invalid := []reasoningprofile.Family{"", "socialist_law", "COMMON_LAW"}
	for _, f := range invalid {
		if f.IsValid() {
			t.Errorf("Family(%q).IsValid() = true, want false", f)
		}
	}
}
