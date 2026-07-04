package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

// NewMultiJurisdictionScenario builds task 3's multi-jurisdiction
// variant: the SAME civil-case journey (identical seeded issue/rule/
// fact tree shape, identical ingestion payload, identical deterministic
// reasoning-model responses) run once per named
// jurisdiction.LegalFamily, asserting that each run resolves a
// genuinely different packages/reasoningprofile.Weights profile --
// not merely that every run completes without erroring.
//
// countryCode selects a real seeded jurisdiction.Jurisdiction (via
// jurisdiction.SeedData, referencing packages/jurisdiction's
// legal-family enum by value rather than duplicating its catalogue),
// so the LegalFamily driving this scenario's reasoning-profile
// selection is exactly what that package's own data says the named
// country's courts operate under.
func NewMultiJurisdictionScenario(countryCode string) (Scenario, error) {
	j, err := legalFamilyForJurisdiction(countryCode)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("civil/multi-jurisdiction/%s-%s", countryCode, j.LegalFamily)
	run := func(ctx context.Context) (ScenarioResult, error) {
		return runMultiJurisdictionCase(ctx, j)
	}
	return NewScenarioFunc(name, category.CodeCivil, run)
}

func runMultiJurisdictionCase(ctx context.Context, j jurisdiction.Jurisdiction) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	outcome, err := runFullJourney(ctx, "multi-jurisdiction-"+j.CountryCode, journeyOptions{
		JurisdictionCode: j.CountryCode,
		JurisdictionName: j.CourtName,
		LegalFamily:      string(j.LegalFamily),
	})
	if err != nil {
		return ScenarioResult{}, err
	}

	result := outcome.toScenarioResult(ctx, startedAt)
	if result.Outcome != OutcomePassed {
		return result, nil
	}
	result.LegalFamily = string(j.LegalFamily)

	expected := resolveScenarioWeights(string(j.LegalFamily))
	if expected == (ScenarioWeights{}) {
		result.Outcome = OutcomeErrored
		result.Detail = fmt.Sprintf("could not resolve a reasoningprofile.Weights for legal family %q", j.LegalFamily)
		return result, nil
	}
	if result.ResolvedWeights != expected {
		result.Outcome = OutcomeFailed
		result.Detail = fmt.Sprintf("resolved weights %+v did not match the expected %q profile %+v", result.ResolvedWeights, j.LegalFamily, expected)
		return result, nil
	}

	result.Detail = fmt.Sprintf("jurisdiction %s (%s, legal family %q) resolved profile %+v", j.CountryCode, j.CourtName, j.LegalFamily, result.ResolvedWeights)
	return result, nil
}

// AssertDistinctJurisdictionProfiles is a scenario-suite-level
// assertion (not a Scenario itself): given the ScenarioResults from
// running NewMultiJurisdictionScenario for two different legal
// families, it verifies the two runs actually resolved two distinct
// ResolvedWeights profiles -- the specific property task 3 requires
// ("the SAME scenario produces jurisdiction-appropriate different
// behavior... not just that it runs without erroring"). Returns
// ErrIsolationBreached's sibling concept via a plain error when the
// two profiles are identical, since two different legal families
// resolving to the identical profile would mean jurisdiction-aware
// weighting silently is not happening.
func AssertDistinctJurisdictionProfiles(a, b ScenarioResult) error {
	if a.LegalFamily == b.LegalFamily {
		return wrapf("AssertDistinctJurisdictionProfiles", fmt.Errorf("both results share legal family %q; pass two different families", a.LegalFamily))
	}
	if a.ResolvedWeights == b.ResolvedWeights {
		return wrapf("AssertDistinctJurisdictionProfiles", fmt.Errorf("legal families %q and %q resolved the identical weights profile %+v; expected jurisdiction-appropriate different behavior", a.LegalFamily, b.LegalFamily, a.ResolvedWeights))
	}
	return nil
}
