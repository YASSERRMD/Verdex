package application

import "github.com/YASSERRMD/verdex/packages/irac"

// matchWeight and familyWeight balance the two signals combined into an
// ApplicationNode's final Confidence: how textually relevant the rule is
// to the issue (matchWeight, the dominant signal — a rule that does not
// textually relate to the issue at all should not gain much confidence
// just from favorable legal-family weighting) and how strongly the
// rule's Origin is favored under the case's dominant legal family
// (familyWeight, a secondary adjustment). This mirrors
// packages/issue/confidence.go's weighted-aggregate convention.
const (
	matchWeight  = 0.7
	familyWeight = 0.3
)

// ComputeConfidence combines a RuleMatch's Score with
// WeightByLegalFamily's output into a single final confidence value in
// the closed interval [0, 1], suitable for setting on the built
// ApplicationNode's Confidence field.
func ComputeConfidence(match RuleMatch, dominantFamily string) float64 {
	familyScore := WeightByLegalFamily(match.Rule, dominantFamily)
	aggregate := matchWeight*clamp01(match.Score) + familyWeight*clamp01(familyScore)
	return clamp01(aggregate)
}

// ApplyConfidence returns a copy of node with Confidence set to the
// result of ComputeConfidence(match, dominantFamily). node's other
// fields are left unchanged.
func ApplyConfidence(node irac.ApplicationNode, match RuleMatch, dominantFamily string) irac.ApplicationNode {
	node.Confidence = ComputeConfidence(match, dominantFamily)
	return node
}

// clamp01 clamps v to the closed interval [0, 1], mirroring
// packages/issue/confidence.go's helper of the same name.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
