package reasoningprofile_test

import (
	"errors"
	"math"
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

func TestValidate_AcceptsAllFourCanonicalProfiles(t *testing.T) {
	profiles := []reasoningprofile.Weights{
		reasoningprofile.CommonLawWeights(),
		reasoningprofile.CivilLawWeights(),
		reasoningprofile.MixedWeights(),
		reasoningprofile.IslamicLawWeights(),
	}
	for _, w := range profiles {
		if err := reasoningprofile.Validate(w); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", w, err)
		}
	}
}

func TestValidate_RejectsNegative(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: -0.1, DocumentaryEmphasis: 0.5, StatuteEmphasis: 0.5, PrecedentEmphasis: 0.5}
	if err := reasoningprofile.Validate(w); !errors.Is(err, reasoningprofile.ErrInvalidWeight) {
		t.Fatalf("Validate(negative) = %v, want ErrInvalidWeight", err)
	}
}

func TestValidate_RejectsOutOfRangeHigh(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: 0.5, DocumentaryEmphasis: 1.5, StatuteEmphasis: 0.5, PrecedentEmphasis: 0.5}
	if err := reasoningprofile.Validate(w); !errors.Is(err, reasoningprofile.ErrInvalidWeight) {
		t.Fatalf("Validate(>1) = %v, want ErrInvalidWeight", err)
	}
}

func TestValidate_RejectsNaN(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: math.NaN(), DocumentaryEmphasis: 0.5, StatuteEmphasis: 0.5, PrecedentEmphasis: 0.5}
	if err := reasoningprofile.Validate(w); !errors.Is(err, reasoningprofile.ErrInvalidWeight) {
		t.Fatalf("Validate(NaN) = %v, want ErrInvalidWeight", err)
	}
}

func TestValidate_RejectsInf(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: 0.5, DocumentaryEmphasis: math.Inf(1), StatuteEmphasis: 0.5, PrecedentEmphasis: 0.5}
	if err := reasoningprofile.Validate(w); !errors.Is(err, reasoningprofile.ErrInvalidWeight) {
		t.Fatalf("Validate(+Inf) = %v, want ErrInvalidWeight", err)
	}
}

func TestValidate_AcceptsBoundaryValues(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: 0, DocumentaryEmphasis: 1, StatuteEmphasis: 0, PrecedentEmphasis: 1}
	if err := reasoningprofile.Validate(w); err != nil {
		t.Fatalf("Validate(boundary 0/1) = %v, want nil", err)
	}
}

func TestValidateFamily(t *testing.T) {
	valid := []reasoningprofile.Family{
		reasoningprofile.FamilyCommonLaw,
		reasoningprofile.FamilyCivilLaw,
		reasoningprofile.FamilyMixed,
		reasoningprofile.FamilyIslamicLaw,
	}
	for _, f := range valid {
		if err := reasoningprofile.ValidateFamily(f); err != nil {
			t.Errorf("ValidateFamily(%v) = %v, want nil", f, err)
		}
	}

	if err := reasoningprofile.ValidateFamily("napoleonic_customary"); !errors.Is(err, reasoningprofile.ErrUnknownFamily) {
		t.Errorf("ValidateFamily(unrecognized) = %v, want ErrUnknownFamily", err)
	}
}

func TestInvalidWeightError_MessageIncludesField(t *testing.T) {
	w := reasoningprofile.Weights{TestimonyEmphasis: 2, DocumentaryEmphasis: 0.5, StatuteEmphasis: 0.5, PrecedentEmphasis: 0.5}
	err := reasoningprofile.Validate(w)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var invalidErr *reasoningprofile.InvalidWeightError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error %v is not an *InvalidWeightError", err)
	}
	if invalidErr.Field != "TestimonyEmphasis" {
		t.Errorf("InvalidWeightError.Field = %q, want TestimonyEmphasis", invalidErr.Field)
	}
}
