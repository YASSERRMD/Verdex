package lawapplication

// LegalFamily classifies the legal tradition a case's controlling
// jurisdiction derives from (e.g. "common_law", "civil_law"). It is an
// opaque, caller-defined string rather than a hard dependency on
// packages/jurisdiction — mirroring irac.RuleNode.LegalFamily's,
// packages/application.WeightByLegalFamily's, and
// packages/evidenceweighing.LegalFamily's own decoupling convention
// exactly, so this package never has to reconcile its own notion of a
// legal family against packages/jurisdiction's richer domain model.
type LegalFamily string

// Legal-family origin weighting profiles.
//
// packages/application.WeightByLegalFamily (Phase 037) establishes the
// weighting concept this package reuses at the reasoning stage: legal
// traditions differ in how much authority statute carries relative to
// precedent.
//
//   - Under "common_law" (e.g. England & Wales, most US states), judicial
//     precedent (stare decisis) is itself a primary, binding source of
//     law — a precedent-origin rule is weighted higher than a
//     statute-origin rule of otherwise equal applicability.
//   - Under "civil_law" (e.g. France, Germany, most of continental
//     Europe), codified statute is the primary source of law and
//     judicial decisions are persuasive but not formally binding — the
//     weighting is reversed: statute-origin rules outweigh
//     precedent-origin rules.
//   - Any other/unrecognized LegalFamily (including OriginUnknown-origin
//     rules) is treated as neutral: every Origin weights equally (1.0),
//     mirroring packages/application's and packages/evidenceweighing's
//     shared neutral-default convention.
//
// | LegalFamily   | OriginStatute | OriginPrecedent | OriginUnknown |
// |---------------|---------------|-----------------|---------------|
// | "common_law"  | 0.8           | 1.0             | 1.0           |
// | "civil_law"   | 1.0           | 0.8             | 1.0           |
// | anything else | 1.0           | 1.0             | 1.0           |
const (
	CommonLawFamily LegalFamily = "common_law"
	CivilLawFamily  LegalFamily = "civil_law"

	dominantOriginWeight    = 1.0
	subordinateOriginWeight = 0.8
	neutralOriginWeight     = 1.0
)

// OriginProfile is a LegalFamily-keyed weighting profile: a multiplier
// applied per Origin, reflecting how strongly the profile's legal
// tradition favors statute versus precedent. Constructed via
// CommonLawProfile, CivilLawProfile, or NeutralProfile — a zero-value
// OriginProfile behaves identically to NeutralProfile (Multiplier
// defensively treats an all-zero profile as neutral).
type OriginProfile struct {
	// Family identifies which legal tradition this profile represents,
	// for display/rationale purposes only — Multiplier keys off the
	// Statute/Precedent fields below, not Family.
	Family LegalFamily

	// Statute is the multiplier applied to rules with Origin
	// OriginStatute.
	Statute float64

	// Precedent is the multiplier applied to rules with Origin
	// OriginPrecedent or OriginUnknown.
	Precedent float64
}

// CommonLawProfile returns the precedent-favoring weighting profile for
// LegalFamily CommonLawFamily.
func CommonLawProfile() OriginProfile {
	return OriginProfile{
		Family:    CommonLawFamily,
		Statute:   subordinateOriginWeight,
		Precedent: dominantOriginWeight,
	}
}

// CivilLawProfile returns the statute-favoring weighting profile for
// LegalFamily CivilLawFamily.
func CivilLawProfile() OriginProfile {
	return OriginProfile{
		Family:    CivilLawFamily,
		Statute:   dominantOriginWeight,
		Precedent: subordinateOriginWeight,
	}
}

// NeutralProfile returns a profile that weights every Origin equally,
// used as the default when no LegalFamily signal is available.
func NeutralProfile() OriginProfile {
	return OriginProfile{
		Statute:   neutralOriginWeight,
		Precedent: neutralOriginWeight,
	}
}

// ProfileForFamily resolves the OriginProfile for a given LegalFamily:
// CommonLawProfile for CommonLawFamily, CivilLawProfile for
// CivilLawFamily, and NeutralProfile for anything else (including
// empty).
func ProfileForFamily(family LegalFamily) OriginProfile {
	switch family {
	case CommonLawFamily:
		return CommonLawProfile()
	case CivilLawFamily:
		return CivilLawProfile()
	default:
		return NeutralProfile()
	}
}

// Multiplier returns this profile's weighting multiplier for origin. A
// zero-value OriginProfile (Statute and Precedent both 0.0) is treated
// as NeutralProfile, defensively, in case a caller constructs one
// without going through a constructor.
func (p OriginProfile) Multiplier(origin Origin) float64 {
	if p.Statute == 0 && p.Precedent == 0 {
		return neutralOriginWeight
	}
	switch origin {
	case OriginStatute:
		return p.Statute
	case OriginPrecedent, OriginUnknown:
		return p.Precedent
	default:
		return neutralOriginWeight
	}
}

// WeightByOrigin returns a multiplier in (0, 1] reflecting how strongly
// rule's Origin is favored under family, per ProfileForFamily's
// weighting table. Mirrors packages/application.WeightByLegalFamily's
// contract exactly (same signature shape, same neutral-default
// behavior), reimplemented locally against this package's own Origin/
// RuleRef types rather than importing packages/application, since no
// application.Origin/OriginatedRule survives into the tree by the time
// this package's reasoning stage runs (see doc/law-application.md).
func WeightByOrigin(rule RuleRef, family LegalFamily) float64 {
	origin := InferOrigin(rule)
	return ProfileForFamily(family).Multiplier(origin)
}
