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
//   - "mixed" jurisdictions are weighted at the midpoint between
//     CommonLawProfile and CivilLawProfile, rather than defaulting to
//     neutral, since a mixed-family jurisdiction still leans measurably
//     on both statute and precedent rather than having no lean at all.
//   - "islamic_law" jurisdictions are weighted per IslamicLawProfile; see
//     that constructor's doc comment for the rationale and limitations.
//   - Any other/unrecognized LegalFamily (including OriginUnknown-origin
//     rules) is treated as neutral: every Origin weights equally (1.0),
//     mirroring packages/application's and packages/evidenceweighing's
//     shared neutral-default convention.
//
// The weights below are this package's own copy of the canonical values
// defined in packages/reasoningprofile (Weights.StatuteEmphasis /
// Weights.PrecedentEmphasis) — see doc/law-application.md and
// packages/reasoningprofile/doc/jurisdiction-reasoning.md for the
// cross-package derivation. This package does not import
// packages/reasoningprofile; the values are kept in sync by convention
// and by packages/reasoningprofile's own cross-package alignment tests.
//
// | LegalFamily    | OriginStatute | OriginPrecedent | OriginUnknown |
// |----------------|---------------|-----------------|---------------|
// | "common_law"   | 0.8           | 1.0             | 1.0           |
// | "civil_law"    | 1.0           | 0.8             | 1.0           |
// | "mixed"        | 0.9           | 0.9             | 1.0           |
// | "islamic_law"  | 1.0           | 0.95            | 1.0           |
// | anything else  | 1.0           | 1.0             | 1.0           |
const (
	CommonLawFamily  LegalFamily = "common_law"
	CivilLawFamily   LegalFamily = "civil_law"
	MixedFamily      LegalFamily = "mixed"
	IslamicLawFamily LegalFamily = "islamic_law"

	dominantOriginWeight    = 1.0
	subordinateOriginWeight = 0.8
	neutralOriginWeight     = 1.0

	mixedStatuteWeight   = 0.9
	mixedPrecedentWeight = 0.9

	islamicLawStatuteWeight   = 1.0
	islamicLawPrecedentWeight = 0.95
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

// MixedProfile returns a blended weighting profile for LegalFamily
// MixedFamily: the midpoint between CommonLawProfile and CivilLawProfile,
// reflecting a jurisdiction that draws meaningfully on both the
// precedent-favoring and statute-favoring traditions rather than having
// no lean at all.
func MixedProfile() OriginProfile {
	return OriginProfile{
		Family:    MixedFamily,
		Statute:   mixedStatuteWeight,
		Precedent: mixedPrecedentWeight,
	}
}

// IslamicLawProfile returns a weighting profile for LegalFamily
// IslamicLawFamily. Many modern Islamic-law jurisdictions operate through
// heavy statutory codification (Gulf civil and commercial codes
// influenced by Sharia are frequently fully codified) while also
// affording strong, near-equal weight to established juristic
// consensus/precedent. This is a simplified computational model of a
// highly diverse family of legal systems, not a claim to legal or
// religious authority — see
// packages/reasoningprofile/doc/jurisdiction-reasoning.md's "Islamic-law
// profile rationale and limitations" section, which this package's
// weights are derived from, for the full discussion and caveats.
func IslamicLawProfile() OriginProfile {
	return OriginProfile{
		Family:    IslamicLawFamily,
		Statute:   islamicLawStatuteWeight,
		Precedent: islamicLawPrecedentWeight,
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
// CivilLawFamily, MixedProfile for MixedFamily, IslamicLawProfile for
// IslamicLawFamily, and NeutralProfile for anything else (including
// empty).
func ProfileForFamily(family LegalFamily) OriginProfile {
	switch family {
	case CommonLawFamily:
		return CommonLawProfile()
	case CivilLawFamily:
		return CivilLawProfile()
	case MixedFamily:
		return MixedProfile()
	case IslamicLawFamily:
		return IslamicLawProfile()
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
