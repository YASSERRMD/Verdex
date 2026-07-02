package evidenceweighing

import (
	"fmt"
	"strings"
)

// ScoreFact computes the FactWeight for a single fact, given the rubric to
// apply, the fact's own data, how many arguments corroborate it, whether
// it is contradicted, and the average Strength across every argument
// citing it (0 if no argument cites it).
//
// The blended pre-penalty, pre-jurisdiction score is:
//
//	base = BaseConfidenceWeight*confidence
//	     + CorroborationWeight*min(corroborationCount/MaxCorroborationForScoring, 1)
//	     + CitationStrengthWeight*avgCitingStrength
//
// If contradicted, base is reduced by ContradictionPenalty as a fraction
// of itself: base = base * (1 - ContradictionPenalty). The jurisdiction
// profile's Multiplier for the fact's classified EvidenceKind is then
// applied: weight = clampUnit(base * profile.Multiplier(kind)).
//
// The score is monotonic in its inputs for a fixed contradiction/
// jurisdiction state: more corroboration or higher citing-argument
// strength never lowers the score, mirroring packages/fact.
// ReliabilityScore's own documented monotonicity guarantee.
func ScoreFact(rubric Rubric, fact FactRef, corroborationCount int, contradicted bool, avgCitingStrength float64) FactWeight {
	factors := rubric.Factors
	confidence := clampUnit(fact.Confidence)

	corroboration := 0.0
	if factors.MaxCorroborationForScoring > 0 {
		corroboration = float64(corroborationCount) / float64(factors.MaxCorroborationForScoring)
	}
	corroboration = clampUnit(corroboration)

	strength := clampUnit(avgCitingStrength)

	base := factors.BaseConfidenceWeight*confidence +
		factors.CorroborationWeight*corroboration +
		factors.CitationStrengthWeight*strength

	var rationale strings.Builder
	fmt.Fprintf(&rationale, "base score %.3f (confidence=%.2f*w%.2f + corroboration=%.2f*w%.2f + citation_strength=%.2f*w%.2f)",
		clampUnit(base), confidence, factors.BaseConfidenceWeight, corroboration, factors.CorroborationWeight, strength, factors.CitationStrengthWeight)

	if contradicted {
		penalized := base * (1 - factors.ContradictionPenalty)
		fmt.Fprintf(&rationale, "; contradiction penalty %.0f%% applied -> %.3f", factors.ContradictionPenalty*100, clampUnit(penalized))
		base = penalized
	}

	kind := ClassifyEvidenceKind(fact.Text)
	multiplier := rubric.Profile.Multiplier(kind)
	final := clampUnit(base * multiplier)
	if multiplier != 1.0 {
		fmt.Fprintf(&rationale, "; jurisdiction profile %q applies %.2fx multiplier for %s evidence -> %.3f",
			rubric.Profile.Family, multiplier, kind, final)
	}

	return FactWeight{
		FactNodeID:         fact.ID,
		Weight:             final,
		Kind:               kind,
		Contradicted:       contradicted,
		CorroborationCount: corroborationCount,
		Rationale:          rationale.String(),
	}
}

// ScoreFacts computes a FactWeight for every fact in facts, deriving
// corroboration counts, contradiction status, and average citing strength
// from arguments. Facts not cited by any argument still receive a
// FactWeight (corroborationCount 0, not contradicted, avgCitingStrength
// 0), so a caller can see every fact known to the tree, not only cited
// ones.
func ScoreFacts(rubric Rubric, facts []FactRef, arguments []CitingArgument) []FactWeight {
	corroborationCounts := CorroborationCounts(arguments)
	contradictions := DetectContradictions(arguments)
	contradicted := factsInContradiction(contradictions)

	strengthSums := make(map[string]float64)
	strengthCounts := make(map[string]int)
	for _, arg := range arguments {
		for _, factID := range arg.SupportingFactIDs {
			if factID == "" {
				continue
			}
			strengthSums[factID] += arg.Strength
			strengthCounts[factID]++
		}
	}

	out := make([]FactWeight, 0, len(facts))
	for _, fact := range facts {
		avgStrength := 0.0
		if n := strengthCounts[fact.ID]; n > 0 {
			avgStrength = strengthSums[fact.ID] / float64(n)
		}
		_, isContradicted := contradicted[fact.ID]
		out = append(out, ScoreFact(rubric, fact, corroborationCounts[fact.ID], isContradicted, avgStrength))
	}
	return out
}
