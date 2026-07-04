package e2e_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/e2e"
)

// TestMultiJurisdictionScenario_DistinctProfiles is task 3's core
// assertion: running the identical civil setup-to-opinion journey for
// two different jurisdictions (the US, common-law, and Saudi Arabia,
// islamic_law -- both real entries in jurisdiction.SeedData()) must
// resolve two genuinely different reasoningprofile.Weights profiles,
// not merely "both runs completed without erroring."
func TestMultiJurisdictionScenario_DistinctProfiles(t *testing.T) {
	usScenario, err := e2e.NewMultiJurisdictionScenario("US")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(US): %v", err)
	}
	saScenario, err := e2e.NewMultiJurisdictionScenario("SA")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(SA): %v", err)
	}

	usResult := runAndRequirePassed(t, usScenario)
	saResult := runAndRequirePassed(t, saScenario)

	if usResult.LegalFamily != "common_law" {
		t.Fatalf("US result LegalFamily = %q, want common_law", usResult.LegalFamily)
	}
	if saResult.LegalFamily != "islamic_law" {
		t.Fatalf("SA result LegalFamily = %q, want islamic_law", saResult.LegalFamily)
	}

	if err := e2e.AssertDistinctJurisdictionProfiles(usResult, saResult); err != nil {
		t.Fatalf("AssertDistinctJurisdictionProfiles: %v", err)
	}

	// Guard against a degenerate implementation that always resolves
	// the zero ScenarioWeights value (which would trivially satisfy
	// "distinct from itself" checks incorrectly if compared wrongly
	// elsewhere): both profiles must carry genuinely non-zero weights.
	if usResult.ResolvedWeights.TestimonyEmphasis == 0 || saResult.ResolvedWeights.TestimonyEmphasis == 0 {
		t.Fatalf("expected non-zero TestimonyEmphasis for both profiles, got US=%v SA=%v",
			usResult.ResolvedWeights.TestimonyEmphasis, saResult.ResolvedWeights.TestimonyEmphasis)
	}
}

// TestMultiJurisdictionScenario_SameFamilySameProfile is the negative
// control for AssertDistinctJurisdictionProfiles itself: two
// jurisdictions sharing the SAME legal family (UK and US, both
// common_law) must resolve the IDENTICAL profile -- proving the
// per-family resolution is deterministic and not accidentally
// per-country.
func TestMultiJurisdictionScenario_SameFamilySameProfile(t *testing.T) {
	usScenario, err := e2e.NewMultiJurisdictionScenario("US")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(US): %v", err)
	}
	gbScenario, err := e2e.NewMultiJurisdictionScenario("GB")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(GB): %v", err)
	}

	usResult := runAndRequirePassed(t, usScenario)
	gbResult := runAndRequirePassed(t, gbScenario)

	if usResult.LegalFamily != gbResult.LegalFamily {
		t.Fatalf("expected both US and GB to resolve legal family common_law, got US=%q GB=%q", usResult.LegalFamily, gbResult.LegalFamily)
	}
	if usResult.ResolvedWeights != gbResult.ResolvedWeights {
		t.Fatalf("expected identical resolved weights for the same legal family, got US=%+v GB=%+v", usResult.ResolvedWeights, gbResult.ResolvedWeights)
	}

	// AssertDistinctJurisdictionProfiles must correctly reject this
	// same-family pairing as "not distinct" rather than passing it.
	if err := e2e.AssertDistinctJurisdictionProfiles(usResult, gbResult); err == nil {
		t.Fatalf("AssertDistinctJurisdictionProfiles(US, GB) returned nil, want an error (both share common_law)")
	}
}

func TestNewMultiJurisdictionScenario_UnknownCountryCode(t *testing.T) {
	if _, err := e2e.NewMultiJurisdictionScenario("ZZ"); err == nil {
		t.Fatalf("expected an error for an unknown country code, got nil")
	}
}
