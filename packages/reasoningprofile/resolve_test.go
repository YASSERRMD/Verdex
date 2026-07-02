package reasoningprofile_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

// TestWeightsForFamily_CoversAllFourExhaustively proves WeightsForFamily
// resolves every canonical Family without error and without falling back
// to a shared "neutral" value the way the sibling packages'
// ProfileForFamily functions do for unrecognized input.
func TestWeightsForFamily_CoversAllFourExhaustively(t *testing.T) {
	families := []reasoningprofile.Family{
		reasoningprofile.FamilyCommonLaw,
		reasoningprofile.FamilyCivilLaw,
		reasoningprofile.FamilyMixed,
		reasoningprofile.FamilyIslamicLaw,
	}

	seen := make(map[reasoningprofile.Weights]reasoningprofile.Family)
	for _, family := range families {
		w, err := reasoningprofile.WeightsForFamily(family)
		if err != nil {
			t.Fatalf("WeightsForFamily(%v) returned error: %v", family, err)
		}
		if err := reasoningprofile.Validate(w); err != nil {
			t.Errorf("WeightsForFamily(%v) produced invalid Weights: %v", family, err)
		}
		if prior, ok := seen[w]; ok {
			t.Errorf("Family %v produced identical Weights to %v; each family must be distinct", family, prior)
		}
		seen[w] = family
	}
}

func TestWeightsForFamily_UnknownFamilyIsError(t *testing.T) {
	_, err := reasoningprofile.WeightsForFamily("roman_dutch_law")
	if !errors.Is(err, reasoningprofile.ErrUnknownFamily) {
		t.Fatalf("WeightsForFamily(unrecognized) error = %v, want ErrUnknownFamily", err)
	}
}
