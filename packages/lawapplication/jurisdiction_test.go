package lawapplication_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestWeightByOrigin_FlipsBetweenCommonAndCivilLaw(t *testing.T) {
	statuteRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginStatute}
	precedentRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginPrecedent}

	commonStatute := lawapplication.WeightByOrigin(statuteRule, lawapplication.CommonLawFamily)
	commonPrecedent := lawapplication.WeightByOrigin(precedentRule, lawapplication.CommonLawFamily)
	if commonPrecedent <= commonStatute {
		t.Errorf("under common_law, precedent weight (%v) should exceed statute weight (%v)", commonPrecedent, commonStatute)
	}

	civilStatute := lawapplication.WeightByOrigin(statuteRule, lawapplication.CivilLawFamily)
	civilPrecedent := lawapplication.WeightByOrigin(precedentRule, lawapplication.CivilLawFamily)
	if civilStatute <= civilPrecedent {
		t.Errorf("under civil_law, statute weight (%v) should exceed precedent weight (%v)", civilStatute, civilPrecedent)
	}
}

func TestWeightByOrigin_NeutralForUnknownFamily(t *testing.T) {
	statuteRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginStatute}
	precedentRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginPrecedent}

	statuteWeight := lawapplication.WeightByOrigin(statuteRule, "unknown_family")
	precedentWeight := lawapplication.WeightByOrigin(precedentRule, "unknown_family")

	if statuteWeight != precedentWeight {
		t.Errorf("unknown family should weight statute and precedent equally, got %v vs %v", statuteWeight, precedentWeight)
	}
}

func TestProfileForFamily(t *testing.T) {
	tests := []struct {
		family lawapplication.LegalFamily
		want   lawapplication.LegalFamily
	}{
		{lawapplication.CommonLawFamily, lawapplication.CommonLawFamily},
		{lawapplication.CivilLawFamily, lawapplication.CivilLawFamily},
	}
	for _, tt := range tests {
		profile := lawapplication.ProfileForFamily(tt.family)
		if profile.Family != tt.want {
			t.Errorf("ProfileForFamily(%v).Family = %v, want %v", tt.family, profile.Family, tt.want)
		}
	}
}

func TestOriginProfile_MultiplierDefensiveZeroValue(t *testing.T) {
	var zero lawapplication.OriginProfile
	if got := zero.Multiplier(lawapplication.OriginStatute); got != 1.0 {
		t.Errorf("zero-value OriginProfile.Multiplier = %v, want 1.0 (neutral default)", got)
	}
}

func TestNeutralProfile_WeightsEqually(t *testing.T) {
	profile := lawapplication.NeutralProfile()
	if profile.Multiplier(lawapplication.OriginStatute) != profile.Multiplier(lawapplication.OriginPrecedent) {
		t.Errorf("NeutralProfile should weight statute and precedent equally")
	}
}
