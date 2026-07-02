package firstpartyagent

import "github.com/YASSERRMD/verdex/packages/knowledgeapi"

// ruleLinkageSaturation is the number of supporting rules at which the
// rule-linkage-richness signal saturates to 1.0, mirroring
// packages/issueagent/rank.go's materialityRuleSaturation convention of a
// small saturating count rather than an unbounded linear signal.
const ruleLinkageSaturation = 3

// scoreWeights are the fixed weights strengthScore blends its three
// signals with. They sum to 1.0 so the result stays in [0,1] by
// construction (each signal itself already clamped to [0,1]).
const (
	citationWeight       = 0.4
	factConfidenceWeight = 0.35
	ruleLinkageWeight    = 0.25
)

// strengthScore computes an Argument's [0,1] strength score, combining:
//   - citation verification status across arg's resolved Citations (mean
//     ConfidenceScore for verified citations; an argument with rules but
//     no verified citation at all scores zero on this axis),
//   - the mean Confidence of arg's SupportingFactIDs' underlying
//     FactNodes (facts drawn straight from the case record — the more the
//     tree itself trusts them, the stronger the argument built on them),
//   - rule-linkage richness: how many distinct SupportingRuleIDs back the
//     argument, saturating at ruleLinkageSaturation.
//
// An argument with no supporting rules at all (so no citations are even
// attempted) still gets a defined score from the other two signals rather
// than being penalized to zero purely for citationWeight's share, by
// redistributing citationWeight's share onto the other two signals
// whenever there are no rules to cite.
func strengthScore(arg Argument, factsByID map[string]knowledgeapi.NodeDTO) float64 {
	citationSignal, haveCitationSignal := citationVerificationSignal(arg.Citations)
	factSignal := meanFactConfidence(arg.SupportingFactIDs, factsByID)
	ruleSignal := clamp01(float64(len(arg.SupportingRuleIDs)) / float64(ruleLinkageSaturation))

	if !haveCitationSignal {
		// Redistribute citationWeight proportionally across the remaining
		// two signals so their relative weighting is preserved.
		remaining := factConfidenceWeight + ruleLinkageWeight
		return clamp01((factConfidenceWeight/remaining)*factSignal + (ruleLinkageWeight/remaining)*ruleSignal)
	}

	return clamp01(citationWeight*citationSignal + factConfidenceWeight*factSignal + ruleLinkageWeight*ruleSignal)
}

// citationVerificationSignal returns the mean ConfidenceScore across
// citations, and false if citations is empty (no citation signal to
// contribute at all, as opposed to a citation signal of exactly zero from
// a resolved-but-unverified citation).
func citationVerificationSignal(citations []CitationRef) (float64, bool) {
	if len(citations) == 0 {
		return 0, false
	}
	var sum float64
	for _, c := range citations {
		if !c.Verified {
			continue
		}
		sum += c.ConfidenceScore
	}
	return clamp01(sum / float64(len(citations))), true
}

// meanFactConfidence returns the mean Confidence of factIDs' underlying
// FactNodes as resolved in factsByID. Unresolvable IDs (should not occur
// for a grounded Argument, since ground.go only keeps IDs verified to
// exist) are skipped rather than treated as zero, to keep this function
// robust even if called on a not-yet-grounded modelArgument.
func meanFactConfidence(factIDs []string, factsByID map[string]knowledgeapi.NodeDTO) float64 {
	if len(factIDs) == 0 {
		return 0
	}
	var sum float64
	var n int
	for _, id := range factIDs {
		if f, ok := factsByID[id]; ok {
			sum += f.Confidence
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return clamp01(sum / float64(n))
}

// clamp01 clamps v into the closed interval [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
