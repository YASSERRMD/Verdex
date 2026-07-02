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
		{lawapplication.MixedFamily, lawapplication.MixedFamily},
		{lawapplication.IslamicLawFamily, lawapplication.IslamicLawFamily},
	}
	for _, tt := range tests {
		profile := lawapplication.ProfileForFamily(tt.family)
		if profile.Family != tt.want {
			t.Errorf("ProfileForFamily(%v).Family = %v, want %v", tt.family, profile.Family, tt.want)
		}
	}
}

func TestProfileForFamily_MixedIsBetweenCommonAndCivilLaw(t *testing.T) {
	mixed := lawapplication.ProfileForFamily(lawapplication.MixedFamily)
	common := lawapplication.CommonLawProfile()
	civil := lawapplication.CivilLawProfile()

	if mixed.Statute == common.Statute || mixed.Statute == civil.Statute {
		t.Errorf("mixed statute weight %.2f should differ from both common_law %.2f and civil_law %.2f", mixed.Statute, common.Statute, civil.Statute)
	}
	if mixed == lawapplication.NeutralProfile() {
		t.Errorf("mixed family must not silently fall back to NeutralProfile, got %+v", mixed)
	}
}

func TestProfileForFamily_IslamicLawIsDistinct(t *testing.T) {
	islamic := lawapplication.ProfileForFamily(lawapplication.IslamicLawFamily)

	if islamic == lawapplication.NeutralProfile() {
		t.Errorf("islamic_law must not silently fall back to NeutralProfile, got %+v", islamic)
	}
	if islamic == lawapplication.CommonLawProfile() {
		t.Errorf("islamic_law must not equal common_law profile, got %+v", islamic)
	}
	if islamic == lawapplication.CivilLawProfile() {
		t.Errorf("islamic_law must not equal civil_law profile, got %+v", islamic)
	}
}

func TestWeightByOrigin_MixedAndIslamicLawAreExhaustivelyHandled(t *testing.T) {
	statuteRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginStatute}
	precedentRule := lawapplication.RuleRef{OriginHint: lawapplication.OriginPrecedent}

	for _, family := range []lawapplication.LegalFamily{lawapplication.MixedFamily, lawapplication.IslamicLawFamily} {
		statuteWeight := lawapplication.WeightByOrigin(statuteRule, family)
		precedentWeight := lawapplication.WeightByOrigin(precedentRule, family)
		if statuteWeight == 1.0 && precedentWeight == 1.0 {
			t.Errorf("family %v should not weight statute and precedent both neutrally at 1.0", family)
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
