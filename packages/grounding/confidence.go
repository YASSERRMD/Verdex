package grounding

import "github.com/YASSERRMD/verdex/packages/citation"

// scoreConclusion computes a single [0, 1] grounding confidence score for
// one conclusion from its Claim outcomes and citation findings.
//
// The score is the fraction of "checked" claims that came back grounded:
// OutcomeUnverifiable claims are excluded from both the numerator and
// denominator entirely (an unchecked claim should not silently count
// against — or for — grounding confidence; CodeUnverifiableClaim already
// surfaces the coverage gap as its own Finding). Every citation.Finding
// with SeverityCritical additionally counts as one more ungrounded check,
// since a hallucinated or wrong-case citation is exactly the kind of
// failure this score exists to catch, even though it was raised by
// packages/citation rather than one of this package's own Claim checks.
//
// A conclusion with nothing to check at all (zero checked claims and zero
// citation findings) scores 1.0 — an opinion that makes no checkable
// assertions has nothing ungrounded to report, mirroring
// packages/citation.ScoreConfidence's "absence of evidence is not
// evidence of fabrication" stance rather than penalizing a conclusion for
// having, say, only unverifiable claims.
func scoreConclusion(outcomes []VerificationOutcome, citationFindings []citation.Finding) float64 {
	checked, grounded := 0, 0
	for _, o := range outcomes {
		if o == OutcomeUnverifiable {
			continue
		}
		checked++
		if o == OutcomeGrounded {
			grounded++
		}
	}

	criticalCitations := 0
	for _, f := range citationFindings {
		if f.Severity == citation.SeverityCritical {
			criticalCitations++
		}
	}
	checked += criticalCitations

	if checked == 0 {
		return 1.0
	}
	return clamp01(float64(grounded) / float64(checked))
}

// scoreOpinion computes the overall [0, 1] grounding confidence for a
// whole opinion as the unweighted mean of every ConclusionResult's
// ConfidenceScore. An opinion with no conclusions scores 1.0 for the same
// "nothing to ground" reason scoreConclusion does.
func scoreOpinion(results []ConclusionResult) float64 {
	if len(results) == 0 {
		return 1.0
	}
	sum := 0.0
	for _, r := range results {
		sum += r.ConfidenceScore
	}
	return clamp01(sum / float64(len(results)))
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
