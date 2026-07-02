package evidenceweighing

// LegalFamily classifies the legal tradition a case's controlling
// jurisdiction derives from (e.g. "common_law", "civil_law"). It is an
// opaque, caller-defined string rather than a hard dependency on
// packages/jurisdiction — mirroring irac.RuleNode.LegalFamily's and
// packages/application's WeightByLegalFamily's own decoupling convention,
// so this package never has to reconcile its own notion of a legal family
// against packages/jurisdiction's richer domain model.
type LegalFamily string

// WeightFactors are the tunable coefficients Rubric applies when combining
// a FactNode's raw signals into a single [0, 1] weight. Coefficients are
// exported so a caller can tune the rubric for a deployment without
// forking this package, mirroring the *Config/*Options convention used
// elsewhere in Verdex (e.g. packages/adaptiveretrieval's Config).
type WeightFactors struct {
	// BaseConfidenceWeight is the blend weight applied to the fact's raw
	// irac.Node.Confidence — the closest analog available at this
	// reasoning stage to packages/fact's ingestion-time
	// ReliabilityScore's classification-confidence signal (that pipeline
	// is not re-run here; see doc/evidence-weighing.md).
	BaseConfidenceWeight float64

	// CorroborationWeight is the blend weight applied to the
	// corroboration signal: how many independent arguments (ideally from
	// both parties) cite this fact in support of a claim, normalized by
	// MaxCorroborationForScoring.
	CorroborationWeight float64

	// ContradictionPenalty is subtracted (as a fraction of the blended
	// pre-penalty score) for each Contradiction a fact is involved in,
	// capped so the total penalty never drives the score below zero.
	// Distinct from CorroborationWeight because a fact cited by both
	// parties for *compatible* claims is corroboration (a positive
	// signal), while a fact cited by both parties for *mutually
	// exclusive* claims is a contradiction (a negative signal) — the two
	// must never be conflated into one "cited more than once" bucket.
	ContradictionPenalty float64

	// CitationStrengthWeight is the blend weight applied to the average
	// Argument.Strength across every argument citing this fact, on the
	// theory that a fact anchoring a well-supported, well-cited argument
	// is itself more evidentially significant than one appearing only in
	// a weak, unsupported claim.
	CitationStrengthWeight float64

	// MaxCorroborationForScoring caps how many independent citing
	// arguments count toward the corroboration signal before it
	// saturates at 1.0, mirroring packages/fact's
	// maxCorroborationForScoring convention exactly.
	MaxCorroborationForScoring int
}

// DefaultWeightFactors returns the rubric's default coefficients.
// BaseConfidenceWeight dominates (0.45) because a FactNode's Confidence is
// the only signal fixed at ingestion time and unaffected by which/how many
// arguments happen to cite it; CorroborationWeight (0.30) and
// CitationStrengthWeight (0.25) are secondary signals derived from how the
// two parties' arguments actually use the fact. ContradictionPenalty
// (0.5) is a fractional penalty applied on top of the blended score, not a
// fourth blend component, so it can subtract from an already-high score
// rather than being diluted by summing to less than 1.0 with the other
// three weights.
func DefaultWeightFactors() WeightFactors {
	return WeightFactors{
		BaseConfidenceWeight:       0.45,
		CorroborationWeight:        0.30,
		ContradictionPenalty:       0.5,
		CitationStrengthWeight:     0.25,
		MaxCorroborationForScoring: 3,
	}
}

// Rubric bundles a WeightFactors coefficient set with the jurisdiction
// weighting profile (see jurisdiction.go) it applies on top. A zero-value
// Rubric is not directly usable — construct one with NewRubric or
// DefaultRubric.
type Rubric struct {
	// Factors are the base blend coefficients (see WeightFactors).
	Factors WeightFactors

	// Profile is the jurisdiction-aware weighting profile applied as a
	// multiplier on the blended base score (see jurisdiction.go). A
	// zero-value Profile is treated as neutral (multiplier 1.0 for every
	// EvidenceKind) by Profile.Multiplier.
	Profile JurisdictionProfile
}

// DefaultRubric returns a Rubric with DefaultWeightFactors and the neutral
// jurisdiction profile (see NeutralProfile), suitable when the caller has
// no LegalFamily signal to apply.
func DefaultRubric() Rubric {
	return Rubric{
		Factors: DefaultWeightFactors(),
		Profile: NeutralProfile(),
	}
}

// NewRubric constructs a Rubric from explicit factors and a jurisdiction
// profile.
func NewRubric(factors WeightFactors, profile JurisdictionProfile) Rubric {
	return Rubric{Factors: factors, Profile: profile}
}

// clampUnit clamps v into the closed interval [0, 1], mirroring
// packages/fact's clampUnit convention.
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
