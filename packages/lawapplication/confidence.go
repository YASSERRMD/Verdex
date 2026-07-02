package lawapplication

import "fmt"

// Confidence blend coefficients.
//
// ComputeConfidence blends three signals already computed elsewhere in
// this package's pipeline into a single [0,1] confidence score for an
// IssueApplication:
//
//   - ruleWeightAvg: the mean WeightByOrigin multiplier across every
//     controlling rule, reflecting how strongly the case's legal family
//     favors the origins actually found.
//   - citationHealth: the fraction of AppliedCitations that are both
//     Resolved and Verified — an issue resting on unresolved or
//     unverified authority is less trustworthy than one whose every
//     citation checks out.
//   - factWeightAvg: the mean FactWeight across every ElementFactEntry,
//     reflecting how strong the underlying evidence is.
//
// A conflict penalty is then applied: each ConflictingAuthority found
// for the issue reduces the blended score, capped so the total penalty
// never drives confidence below zero, mirroring
// evidenceweighing.WeightFactors.ContradictionPenalty's fractional-
// penalty-on-top convention exactly.
const (
	ruleWeightBlend      = 0.35
	citationHealthBlend  = 0.35
	factWeightBlend      = 0.30
	perConflictPenalty   = 0.25
	maxConflictPenalty   = 0.75
	confidenceWhenNoData = 0.0
)

// ComputeConfidence derives an IssueApplication's Confidence and appends
// the reasoning Steps explaining the derivation, given the already-
// computed controlling rules, their AppliedCitations, their
// ElementFactEntries, any ConflictingAuthority findings, and the
// LegalFamily applied.
func ComputeConfidence(
	controllingRuleIDs []string,
	rules []RuleRef,
	citations []AppliedCitation,
	elementFacts []ElementFactEntry,
	conflicts []ConflictingAuthority,
	family LegalFamily,
) (float64, []Step) {
	var steps []Step

	if len(controllingRuleIDs) == 0 {
		steps = append(steps, Step{Description: "no controlling rules found for this issue; confidence set to 0"})
		return confidenceWhenNoData, steps
	}

	idx := ruleRefIndex(rules)
	profile := ProfileForFamily(family)

	var ruleWeightSum float64
	for _, ruleID := range controllingRuleIDs {
		origin := InferOrigin(idx[ruleID])
		ruleWeightSum += profile.Multiplier(origin)
	}
	ruleWeightAvg := ruleWeightSum / float64(len(controllingRuleIDs))
	steps = append(steps, Step{Description: fmt.Sprintf(
		"averaged WeightByOrigin across %d controlling rule(s) under legal family %q -> %.3f",
		len(controllingRuleIDs), string(family), ruleWeightAvg,
	)})

	citationHealth := 1.0
	if len(citations) > 0 {
		var healthy int
		for _, c := range citations {
			if c.Resolved && c.Verified {
				healthy++
			}
		}
		citationHealth = float64(healthy) / float64(len(citations))
	}
	steps = append(steps, Step{Description: fmt.Sprintf(
		"citation health: %d/%d controlling rule citation(s) resolved and verified -> %.3f",
		countHealthy(citations), len(citations), citationHealth,
	)})

	factWeightAvg := 1.0
	if len(elementFacts) > 0 {
		var sum float64
		for _, ef := range elementFacts {
			sum += ef.FactWeight
		}
		factWeightAvg = sum / float64(len(elementFacts))
	} else {
		steps = append(steps, Step{Description: "no element-fact entries available; fact weight signal defaults to neutral 1.000"})
	}
	if len(elementFacts) > 0 {
		steps = append(steps, Step{Description: fmt.Sprintf(
			"averaged evidenceweighing fact weight across %d element-fact entrie(s) -> %.3f",
			len(elementFacts), factWeightAvg,
		)})
	}

	base := ruleWeightBlend*ruleWeightAvg + citationHealthBlend*citationHealth + factWeightBlend*factWeightAvg

	if len(conflicts) > 0 {
		penalty := clampUnit(float64(len(conflicts)) * perConflictPenalty)
		if penalty > maxConflictPenalty {
			penalty = maxConflictPenalty
		}
		adjusted := base * (1 - penalty)
		steps = append(steps, Step{Description: fmt.Sprintf(
			"%d conflicting authority finding(s) detected; applied %.0f%% penalty -> %.3f",
			len(conflicts), penalty*100, adjusted,
		)})
		base = adjusted
	}

	final := clampUnit(base)
	steps = append(steps, Step{Description: fmt.Sprintf("final confidence clamped to [0,1] -> %.3f", final)})

	return final, steps
}

// countHealthy returns how many AppliedCitations are both Resolved and
// Verified.
func countHealthy(citations []AppliedCitation) int {
	var n int
	for _, c := range citations {
		if c.Resolved && c.Verified {
			n++
		}
	}
	return n
}

// clampUnit clamps v into the closed interval [0, 1], mirroring
// packages/evidenceweighing's clampUnit convention.
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
