package reasoningprofile_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

// TestEvidenceWeighingAlignment cross-checks this package's canonical
// TestimonyEmphasis/DocumentaryEmphasis ordering against
// packages/evidenceweighing's own CommonLawProfile/CivilLawProfile, so
// the two packages cannot silently drift apart on the two families they
// both already encoded before this phase.
func TestEvidenceWeighingAlignment(t *testing.T) {
	commonWant := reasoningprofile.CommonLawWeights()
	commonGot := evidenceweighing.CommonLawProfile()
	if commonGot.Testimony != commonWant.TestimonyEmphasis || commonGot.Documentary != commonWant.DocumentaryEmphasis {
		t.Errorf("evidenceweighing.CommonLawProfile() = %+v, want testimony=%v documentary=%v",
			commonGot, commonWant.TestimonyEmphasis, commonWant.DocumentaryEmphasis)
	}

	civilWant := reasoningprofile.CivilLawWeights()
	civilGot := evidenceweighing.CivilLawProfile()
	if civilGot.Testimony != civilWant.TestimonyEmphasis || civilGot.Documentary != civilWant.DocumentaryEmphasis {
		t.Errorf("evidenceweighing.CivilLawProfile() = %+v, want testimony=%v documentary=%v",
			civilGot, civilWant.TestimonyEmphasis, civilWant.DocumentaryEmphasis)
	}
}

// TestLawApplicationAlignment cross-checks this package's canonical
// StatuteEmphasis/PrecedentEmphasis ordering against
// packages/lawapplication's own CommonLawProfile/CivilLawProfile.
func TestLawApplicationAlignment(t *testing.T) {
	commonWant := reasoningprofile.CommonLawWeights()
	commonGot := lawapplication.CommonLawProfile()
	if commonGot.Statute != commonWant.StatuteEmphasis || commonGot.Precedent != commonWant.PrecedentEmphasis {
		t.Errorf("lawapplication.CommonLawProfile() = %+v, want statute=%v precedent=%v",
			commonGot, commonWant.StatuteEmphasis, commonWant.PrecedentEmphasis)
	}

	civilWant := reasoningprofile.CivilLawWeights()
	civilGot := lawapplication.CivilLawProfile()
	if civilGot.Statute != civilWant.StatuteEmphasis || civilGot.Precedent != civilWant.PrecedentEmphasis {
		t.Errorf("lawapplication.CivilLawProfile() = %+v, want statute=%v precedent=%v",
			civilGot, civilWant.StatuteEmphasis, civilWant.PrecedentEmphasis)
	}
}
