package reasoningprofile_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

func TestCommonLawWeights_PrecedentAndTestimonyHeavy(t *testing.T) {
	w := reasoningprofile.CommonLawWeights()

	if w.PrecedentEmphasis <= w.StatuteEmphasis {
		t.Errorf("common law should favor precedent over statute: precedent=%v statute=%v", w.PrecedentEmphasis, w.StatuteEmphasis)
	}
	if w.TestimonyEmphasis <= w.DocumentaryEmphasis {
		t.Errorf("common law should favor testimony over documentary: testimony=%v documentary=%v", w.TestimonyEmphasis, w.DocumentaryEmphasis)
	}
}

func TestCivilLawWeights_StatuteAndDocumentaryHeavy(t *testing.T) {
	w := reasoningprofile.CivilLawWeights()

	if w.StatuteEmphasis <= w.PrecedentEmphasis {
		t.Errorf("civil law should favor statute over precedent: statute=%v precedent=%v", w.StatuteEmphasis, w.PrecedentEmphasis)
	}
	if w.DocumentaryEmphasis <= w.TestimonyEmphasis {
		t.Errorf("civil law should favor documentary over testimony: documentary=%v testimony=%v", w.DocumentaryEmphasis, w.TestimonyEmphasis)
	}
}

// TestMixedWeights_IsGenuinelyBlended proves MixedWeights sits strictly
// between the common-law and civil-law values on the dimensions where
// those two traditions diverge, and is not simply an alias of a neutral
// 1.0-across-the-board profile.
func TestMixedWeights_IsGenuinelyBlended(t *testing.T) {
	common := reasoningprofile.CommonLawWeights()
	civil := reasoningprofile.CivilLawWeights()
	mixed := reasoningprofile.MixedWeights()

	if mixed == common || mixed == civil {
		t.Fatalf("MixedWeights must differ from both parent profiles, got %+v", mixed)
	}
	if mixed == (reasoningprofile.Weights{TestimonyEmphasis: 1, DocumentaryEmphasis: 1, StatuteEmphasis: 1, PrecedentEmphasis: 1}) {
		t.Fatalf("MixedWeights must not be an alias of an all-neutral profile")
	}

	between := func(name string, mixedVal, a, b float64) {
		lo, hi := a, b
		if lo > hi {
			lo, hi = hi, lo
		}
		if mixedVal < lo || mixedVal > hi {
			t.Errorf("%s: mixed value %v not between common-law %v and civil-law %v", name, mixedVal, a, b)
		}
	}
	between("TestimonyEmphasis", mixed.TestimonyEmphasis, common.TestimonyEmphasis, civil.TestimonyEmphasis)
	between("DocumentaryEmphasis", mixed.DocumentaryEmphasis, common.DocumentaryEmphasis, civil.DocumentaryEmphasis)
	between("StatuteEmphasis", mixed.StatuteEmphasis, common.StatuteEmphasis, civil.StatuteEmphasis)
	between("PrecedentEmphasis", mixed.PrecedentEmphasis, common.PrecedentEmphasis, civil.PrecedentEmphasis)
}

// TestIslamicLawWeights_DistinctFromEveryOtherProfile proves
// IslamicLawWeights is not a silent alias for any of the other three
// canonical profiles.
func TestIslamicLawWeights_DistinctFromEveryOtherProfile(t *testing.T) {
	islamic := reasoningprofile.IslamicLawWeights()
	others := map[string]reasoningprofile.Weights{
		"common_law": reasoningprofile.CommonLawWeights(),
		"civil_law":  reasoningprofile.CivilLawWeights(),
		"mixed":      reasoningprofile.MixedWeights(),
	}
	for name, w := range others {
		if islamic == w {
			t.Errorf("IslamicLawWeights must not equal %s profile, both are %+v", name, w)
		}
	}

	if err := reasoningprofile.Validate(islamic); err != nil {
		t.Errorf("IslamicLawWeights() is invalid: %v", err)
	}
}
