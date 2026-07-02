package citation

// certaintyWeight maps a Certainty to the multiplier ScoreConfidence
// applies for it. CertaintyExact contributes full weight, CertaintyNone
// contributes none (there is no citation text to have confidence in),
// and CertaintyHeuristic sits in between.
var certaintyWeight = map[Certainty]float64{
	CertaintyExact:     1.0,
	CertaintyHeuristic: 0.6,
	CertaintyNone:      0.0,
}

// statusWeight maps a VerificationStatus to the multiplier
// ScoreConfidence applies for it. Only StatusVerified contributes full
// weight; every failure mode collapses confidence toward zero, since an
// unverifiable citation should never be reported as trustworthy
// regardless of how confident the underlying node or resolver was.
var statusWeight = map[VerificationStatus]float64{
	StatusVerified:     1.0,
	StatusHallucinated: 0.0,
	StatusWrongCase:    0.0,
	StatusBroken:       0.0,
}

// Confidence is the outcome of ScoreConfidence: a single [0, 1] score for
// a CitedUnit, plus the three components it was combined from, so a
// caller can explain why a citation scored the way it did rather than
// treating the number as opaque.
type Confidence struct {
	// Score is the combined [0, 1] confidence score. See ScoreConfidence
	// for exactly how the three components below are combined.
	Score float64

	// NodeConfidence is the underlying node's own irac.Node.Confidence.
	// CitedUnit does not itself carry Confidence (see resolver.go), so
	// callers fetch it from the same irac.Node Resolve already looked up
	// and pass it in explicitly via ScoreConfidence/ScoreConfidenceWith.
	NodeConfidence float64

	// ResolutionCertainty is the Certainty the Resolver reported when it
	// produced the CitedUnit's Citation text.
	ResolutionCertainty Certainty

	// VerificationStatus is the outcome of verifying the CitedUnit
	// against the GraphStore.
	VerificationStatus VerificationStatus
}

// clamp01 clamps v into the closed interval [0, 1].
func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// ScoreConfidenceWith combines nodeConfidence (a node's own
// irac.Node.Confidence, expected in [0, 1] — out-of-range values are
// clamped), certainty (how the Resolver produced the citation text), and
// status (the outcome of verifying the citation against a GraphStore)
// into a single [0, 1] confidence score.
//
// The combination is a product of weights: Score = nodeConfidence *
// certaintyWeight[certainty] * statusWeight[status]. A product (rather
// than an average) is deliberate — this package's core guarantee is that
// an unverifiable or unresolved citation should never score highly no
// matter how confident the node extraction was, and a product collapses
// to zero the moment any single factor is zero, whereas an average would
// let a high node confidence mask a hallucinated citation.
func ScoreConfidenceWith(nodeConfidence float64, certainty Certainty, status VerificationStatus) Confidence {
	nc := clamp01(nodeConfidence)
	cw, ok := certaintyWeight[certainty]
	if !ok {
		cw = 0
	}
	sw, ok := statusWeight[status]
	if !ok {
		sw = 0
	}
	return Confidence{
		Score:               clamp01(nc * cw * sw),
		NodeConfidence:      nc,
		ResolutionCertainty: certainty,
		VerificationStatus:  status,
	}
}

// ScoreConfidence combines nodeConfidence (typically the irac.Node.
// Confidence fetched alongside the CitedUnit during Resolve),
// certainty (how the Resolver produced the citation text), and result's
// verification outcome into a single [0, 1] confidence score. See
// ScoreConfidenceWith for how the three components combine.
func ScoreConfidence(nodeConfidence float64, certainty Certainty, result VerificationResult) Confidence {
	return ScoreConfidenceWith(nodeConfidence, certainty, result.Status)
}
