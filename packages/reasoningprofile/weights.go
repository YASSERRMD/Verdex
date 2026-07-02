package reasoningprofile

// Weights is the canonical reasoning-weight profile for a legal family,
// spanning both dimensions the reasoning pipeline models separately in
// sibling packages:
//
//   - TestimonyEmphasis / DocumentaryEmphasis mirror
//     packages/evidenceweighing.JurisdictionProfile's Testimony/
//     Documentary shape: how strongly oral witness testimony is weighted
//     relative to documentary/written evidence.
//   - StatuteEmphasis / PrecedentEmphasis mirror
//     packages/lawapplication.OriginProfile's Statute/Precedent shape:
//     how strongly codified statute is weighted relative to judicial
//     precedent.
//
// All four fields are multipliers in [0, 1]; see Validate for the
// acceptance criteria. There is no zero-value convenience constructor —
// unlike the sibling packages' JurisdictionProfile/OriginProfile (which
// defensively treat an all-zero value as neutral for backward-compatible
// literal construction), this package always resolves a Weights value
// through WeightsForFamily, so a bare Weights{} is simply invalid input
// to Validate rather than a silently-accepted neutral default.
type Weights struct {
	// TestimonyEmphasis is the relative weight given to oral
	// witness/party testimony.
	TestimonyEmphasis float64

	// DocumentaryEmphasis is the relative weight given to documentary or
	// written-record evidence.
	DocumentaryEmphasis float64

	// StatuteEmphasis is the relative weight given to codified statute.
	StatuteEmphasis float64

	// PrecedentEmphasis is the relative weight given to judicial
	// precedent.
	PrecedentEmphasis float64
}

// Weight constants underlying the four canonical profiles. Common-law and
// civil-law values match packages/evidenceweighing's and
// packages/lawapplication's existing encodings exactly (see
// doc/jurisdiction-reasoning.md for the side-by-side table); mixed and
// islamic_law are new to this phase.
const (
	commonLawTestimony   = 1.0
	commonLawDocumentary = 0.9
	commonLawStatute     = 0.8
	commonLawPrecedent   = 1.0

	civilLawTestimony   = 0.8
	civilLawDocumentary = 1.0
	civilLawStatute     = 1.0
	civilLawPrecedent   = 0.8

	// mixedTestimony..mixedPrecedent are the midpoint of the common-law
	// and civil-law values above, rounded to two decimal places — a
	// genuinely blended profile rather than an alias of either parent
	// tradition or of neutral (1.0/1.0/1.0/1.0).
	mixedTestimony   = 0.9
	mixedDocumentary = 0.95
	mixedStatute     = 0.9
	mixedPrecedent   = 0.9

	// islamicLawTestimony..islamicLawPrecedent: see doc/jurisdiction-reasoning.md
	// "Islamic-law profile rationale and limitations" for the full
	// discussion. In brief: many modern Islamic-law jurisdictions
	// (e.g. Gulf civil codes influenced by Sharia) operate through heavy
	// statutory codification while also affording strong weight to
	// established juristic consensus/precedent (ijma/precedent-like
	// reasoning) and to documentary instruments (contracts, deeds) that
	// Islamic commercial and family law both treat with particular
	// evidentiary formality. This is a simplified computational model,
	// not a claim to legal or religious authority — see the doc's
	// explicit humility note.
	islamicLawTestimony   = 0.85
	islamicLawDocumentary = 1.0
	islamicLawStatute     = 1.0
	islamicLawPrecedent   = 0.95
)

// CommonLawWeights returns the precedent-heavy, testimony-heavy profile
// for FamilyCommonLaw.
func CommonLawWeights() Weights {
	return Weights{
		TestimonyEmphasis:   commonLawTestimony,
		DocumentaryEmphasis: commonLawDocumentary,
		StatuteEmphasis:     commonLawStatute,
		PrecedentEmphasis:   commonLawPrecedent,
	}
}

// CivilLawWeights returns the statute-heavy, documentary-heavy profile
// for FamilyCivilLaw.
func CivilLawWeights() Weights {
	return Weights{
		TestimonyEmphasis:   civilLawTestimony,
		DocumentaryEmphasis: civilLawDocumentary,
		StatuteEmphasis:     civilLawStatute,
		PrecedentEmphasis:   civilLawPrecedent,
	}
}

// MixedWeights returns a genuinely blended profile for FamilyMixed: each
// dimension sits at the midpoint between CommonLawWeights and
// CivilLawWeights, reflecting a jurisdiction that draws meaningfully on
// both traditions rather than defaulting to a neutral 1.0 across the
// board (see doc/jurisdiction-reasoning.md for why "midpoint of the two
// parent traditions" was chosen over "neutral").
func MixedWeights() Weights {
	return Weights{
		TestimonyEmphasis:   mixedTestimony,
		DocumentaryEmphasis: mixedDocumentary,
		StatuteEmphasis:     mixedStatute,
		PrecedentEmphasis:   mixedPrecedent,
	}
}

// IslamicLawWeights returns a distinct profile for FamilyIslamicLaw. This
// is a simplified computational model of a highly diverse family of
// legal systems, not a claim to legal or religious authority — see
// doc/jurisdiction-reasoning.md's "Islamic-law profile rationale and
// limitations" section before relying on these specific values in a
// real deployment.
func IslamicLawWeights() Weights {
	return Weights{
		TestimonyEmphasis:   islamicLawTestimony,
		DocumentaryEmphasis: islamicLawDocumentary,
		StatuteEmphasis:     islamicLawStatute,
		PrecedentEmphasis:   islamicLawPrecedent,
	}
}
