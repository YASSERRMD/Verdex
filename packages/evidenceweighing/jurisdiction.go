package evidenceweighing

// EvidenceKind classifies a fact for jurisdiction-weighting purposes: is it
// closer to testimony (an assertion attributed to a witness/party
// statement) or documentary evidence (a fact grounded in a record or
// instrument)? See classify.go for how a FactNode is mapped onto an
// EvidenceKind, and doc/evidence-weighing.md's "Testimony vs documentary
// evidence" section for the known limitation this heuristic accepts.
type EvidenceKind string

const (
	// EvidenceKindTestimony is a fact whose text or provenance suggests it
	// derives from a witness statement, deposition, or party assertion
	// rather than a document or instrument.
	EvidenceKindTestimony EvidenceKind = "testimony"

	// EvidenceKindDocumentary is a fact whose text or provenance suggests
	// it derives from a document, record, contract, or other written
	// instrument.
	EvidenceKindDocumentary EvidenceKind = "documentary"

	// EvidenceKindUnknown is used when neither signal is present. Treated
	// identically to EvidenceKindDocumentary by every JurisdictionProfile
	// in this package (see Profile.Multiplier), since defaulting to the
	// less volatile of the two categories is the more conservative
	// choice when the distinction cannot be made.
	EvidenceKindUnknown EvidenceKind = "unknown"
)

// Legal-family jurisdiction weighting profiles.
//
// Evidentiary standards differ meaningfully by legal tradition in how much
// weight testimony carries relative to documentary evidence:
//
//   - Under "common_law" (e.g. England & Wales, most US states),
//     adversarial live testimony and cross-examination are central to
//     fact-finding — oral witness evidence is weighted at full strength,
//     on par with or above documentary evidence of otherwise equal
//     reliability.
//   - Under "civil_law" (e.g. France, Germany, most of continental
//     Europe), inquisitorial procedure and a documentary/written-record
//     tradition dominate — written instruments and records are weighted
//     more heavily than oral testimony, which is treated as more prone to
//     memory/bias distortion absent corroborating documentation.
//   - "mixed" jurisdictions blend two or more traditions (e.g. common law
//     and Islamic law). This package weights them at the midpoint
//     between CommonLawProfile and CivilLawProfile, rather than
//     defaulting to neutral, since a mixed-family jurisdiction still
//     leans measurably toward both parent traditions rather than having
//     no evidentiary tradition at all.
//   - "islamic_law" jurisdictions are weighted per IslamicLawProfile; see
//     that constructor's doc comment for the rationale and its explicit
//     limitations.
//   - Any other/unrecognized LegalFamily is treated as neutral: both
//     evidence kinds weight equally (1.0), since this package has no
//     basis to prefer one over the other without a recognized
//     legal-family signal — mirroring packages/application's
//     WeightByLegalFamily neutral-default convention exactly.
//
// The weights below are this package's own copy of the canonical values
// defined in packages/reasoningprofile (Weights.TestimonyEmphasis /
// Weights.DocumentaryEmphasis) — see doc/evidence-weighing.md and
// packages/reasoningprofile/doc/jurisdiction-reasoning.md for the
// cross-package derivation. This package does not import
// packages/reasoningprofile; the values are kept in sync by convention
// and by packages/reasoningprofile's own cross-package alignment tests.
//
// | LegalFamily    | Testimony | Documentary |
// |----------------|-----------|-------------|
// | "common_law"   | 1.0       | 0.9         |
// | "civil_law"    | 0.8       | 1.0         |
// | "mixed"        | 0.9       | 0.95        |
// | "islamic_law"  | 0.85      | 1.0         |
// | anything else  | 1.0       | 1.0         |
const (
	CommonLawFamily  LegalFamily = "common_law"
	CivilLawFamily   LegalFamily = "civil_law"
	MixedFamily      LegalFamily = "mixed"
	IslamicLawFamily LegalFamily = "islamic_law"

	commonLawTestimonyWeight   = 1.0
	commonLawDocumentaryWeight = 0.9

	civilLawTestimonyWeight   = 0.8
	civilLawDocumentaryWeight = 1.0

	mixedTestimonyWeight   = 0.9
	mixedDocumentaryWeight = 0.95

	islamicLawTestimonyWeight   = 0.85
	islamicLawDocumentaryWeight = 1.0

	neutralWeight = 1.0
)

// JurisdictionProfile is a LegalFamily-keyed weighting profile: a
// multiplier applied per EvidenceKind on top of a fact's blended base
// score, reflecting how strongly the profile's legal tradition weights
// testimony versus documentary evidence. Constructed via CommonLawProfile,
// CivilLawProfile, or NeutralProfile — a zero-value JurisdictionProfile
// behaves identically to NeutralProfile (every multiplier defaults to the
// Go zero value 0.0 rather than 1.0, so always construct one via a
// constructor rather than a bare literal).
type JurisdictionProfile struct {
	// Family identifies which legal tradition this profile represents,
	// for display/rationale purposes only — Multiplier keys off the
	// Testimony/Documentary fields below, not Family.
	Family LegalFamily

	// Testimony is the multiplier applied to facts classified
	// EvidenceKindTestimony.
	Testimony float64

	// Documentary is the multiplier applied to facts classified
	// EvidenceKindDocumentary or EvidenceKindUnknown.
	Documentary float64
}

// CommonLawProfile returns the testimony-heavy weighting profile for
// LegalFamily CommonLawFamily.
func CommonLawProfile() JurisdictionProfile {
	return JurisdictionProfile{
		Family:      CommonLawFamily,
		Testimony:   commonLawTestimonyWeight,
		Documentary: commonLawDocumentaryWeight,
	}
}

// CivilLawProfile returns the documentary-heavy weighting profile for
// LegalFamily CivilLawFamily.
func CivilLawProfile() JurisdictionProfile {
	return JurisdictionProfile{
		Family:      CivilLawFamily,
		Testimony:   civilLawTestimonyWeight,
		Documentary: civilLawDocumentaryWeight,
	}
}

// MixedProfile returns a blended weighting profile for LegalFamily
// MixedFamily: the midpoint between CommonLawProfile and CivilLawProfile,
// reflecting a jurisdiction that draws meaningfully on both the
// adversarial/testimony-heavy and inquisitorial/documentary-heavy
// traditions rather than having no lean at all.
func MixedProfile() JurisdictionProfile {
	return JurisdictionProfile{
		Family:      MixedFamily,
		Testimony:   mixedTestimonyWeight,
		Documentary: mixedDocumentaryWeight,
	}
}

// IslamicLawProfile returns a weighting profile for LegalFamily
// IslamicLawFamily. Many modern Islamic-law jurisdictions blend strong
// documentary formality (contracts, deeds, and instruments carry
// particular evidentiary weight in Islamic commercial and family law)
// with a still-significant, if slightly reduced relative to documentary
// evidence, role for witness testimony. This is a simplified
// computational model of a highly diverse family of legal systems, not a
// claim to legal or religious authority — see
// packages/reasoningprofile/doc/jurisdiction-reasoning.md's "Islamic-law
// profile rationale and limitations" section, which this package's
// weights are derived from, for the full discussion and caveats.
func IslamicLawProfile() JurisdictionProfile {
	return JurisdictionProfile{
		Family:      IslamicLawFamily,
		Testimony:   islamicLawTestimonyWeight,
		Documentary: islamicLawDocumentaryWeight,
	}
}

// NeutralProfile returns a profile that weights every EvidenceKind
// equally, used as the default when no LegalFamily signal is available.
func NeutralProfile() JurisdictionProfile {
	return JurisdictionProfile{
		Testimony:   neutralWeight,
		Documentary: neutralWeight,
	}
}

// ProfileForFamily resolves the JurisdictionProfile for a given
// LegalFamily: CommonLawProfile for CommonLawFamily, CivilLawProfile for
// CivilLawFamily, MixedProfile for MixedFamily, IslamicLawProfile for
// IslamicLawFamily, and NeutralProfile for anything else (including
// empty), mirroring WeightByLegalFamily's exhaustive-switch-with-
// neutral-default convention.
func ProfileForFamily(family LegalFamily) JurisdictionProfile {
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

// Multiplier returns this profile's weighting multiplier for kind. A
// zero-value JurisdictionProfile (Testimony and Documentary both 0.0)
// would incorrectly zero out every score, so Multiplier treats an
// all-zero profile as NeutralProfile — defensive handling for a caller
// that constructed a JurisdictionProfile{} literal instead of using a
// constructor.
func (p JurisdictionProfile) Multiplier(kind EvidenceKind) float64 {
	if p.Testimony == 0 && p.Documentary == 0 {
		return neutralWeight
	}
	switch kind {
	case EvidenceKindTestimony:
		return p.Testimony
	case EvidenceKindDocumentary, EvidenceKindUnknown:
		return p.Documentary
	default:
		return neutralWeight
	}
}
