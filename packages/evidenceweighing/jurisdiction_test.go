package evidenceweighing_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestProfileForFamily_CommonLawFavorsTestimony(t *testing.T) {
	profile := evidenceweighing.ProfileForFamily(evidenceweighing.CommonLawFamily)

	testimony := profile.Multiplier(evidenceweighing.EvidenceKindTestimony)
	documentary := profile.Multiplier(evidenceweighing.EvidenceKindDocumentary)

	if testimony <= documentary {
		t.Errorf("common_law: testimony multiplier %.2f should exceed documentary %.2f", testimony, documentary)
	}
}

func TestProfileForFamily_CivilLawFavorsDocumentary(t *testing.T) {
	profile := evidenceweighing.ProfileForFamily(evidenceweighing.CivilLawFamily)

	testimony := profile.Multiplier(evidenceweighing.EvidenceKindTestimony)
	documentary := profile.Multiplier(evidenceweighing.EvidenceKindDocumentary)

	if documentary <= testimony {
		t.Errorf("civil_law: documentary multiplier %.2f should exceed testimony %.2f", documentary, testimony)
	}
}

func TestProfileForFamily_UnknownFamilyIsNeutral(t *testing.T) {
	profile := evidenceweighing.ProfileForFamily("some_unrecognized_family")

	testimony := profile.Multiplier(evidenceweighing.EvidenceKindTestimony)
	documentary := profile.Multiplier(evidenceweighing.EvidenceKindDocumentary)

	if testimony != 1.0 || documentary != 1.0 {
		t.Errorf("unrecognized family should be neutral, got testimony=%.2f documentary=%.2f", testimony, documentary)
	}
}

func TestJurisdictionProfile_ZeroValueIsNeutral(t *testing.T) {
	var profile evidenceweighing.JurisdictionProfile

	if got := profile.Multiplier(evidenceweighing.EvidenceKindTestimony); got != 1.0 {
		t.Errorf("zero-value profile testimony multiplier = %.2f, want 1.0", got)
	}
	if got := profile.Multiplier(evidenceweighing.EvidenceKindDocumentary); got != 1.0 {
		t.Errorf("zero-value profile documentary multiplier = %.2f, want 1.0", got)
	}
}

func TestJurisdictionProfile_UnknownKindUsesDocumentaryWeight(t *testing.T) {
	profile := evidenceweighing.CivilLawProfile()

	documentary := profile.Multiplier(evidenceweighing.EvidenceKindDocumentary)
	unknown := profile.Multiplier(evidenceweighing.EvidenceKindUnknown)

	if documentary != unknown {
		t.Errorf("unknown kind multiplier %.2f should equal documentary %.2f", unknown, documentary)
	}
}

// TestJurisdictionWeightingDiffersByFamily is the plan's explicit
// "jurisdiction-profile weighting differences" test: the same fact scores
// differently under CommonLawProfile vs CivilLawProfile depending on its
// EvidenceKind.
func TestJurisdictionWeightingDiffersByFamily(t *testing.T) {
	factors := evidenceweighing.DefaultWeightFactors()
	testimonyFact := evidenceweighing.FactRef{
		ID:         "fact-1",
		Text:       "The witness testified that the signal was green.",
		Confidence: 0.9,
	}

	commonLawWeight := evidenceweighing.ScoreFact(
		evidenceweighing.NewRubric(factors, evidenceweighing.CommonLawProfile()),
		testimonyFact, 0, false, 0,
	).Weight

	civilLawWeight := evidenceweighing.ScoreFact(
		evidenceweighing.NewRubric(factors, evidenceweighing.CivilLawProfile()),
		testimonyFact, 0, false, 0,
	).Weight

	if commonLawWeight <= civilLawWeight {
		t.Errorf("testimony fact should score higher under common_law (%.4f) than civil_law (%.4f)", commonLawWeight, civilLawWeight)
	}
}
