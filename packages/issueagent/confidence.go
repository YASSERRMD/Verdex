package issueagent

import "fmt"

// thinRuleLinkageThreshold is the governing-rule count at or below which
// an issue is flagged as having thin rule linkage.
const thinRuleLinkageThreshold = 0

// lowExtractionConfidenceThreshold flags an issue whose source
// irac.IssueNode.Confidence (as extracted upstream by packages/issue) is
// at or below this value.
const lowExtractionConfidenceThreshold = 0.4

// heuristicAmbiguities derives structural ambiguity/gap flags for ic that
// do not require a model call: missing governing-rule linkage and low
// upstream extraction confidence. This can be extended to loosely
// integrate with citation/treevalidation-style signals later without
// widening this package's scope into reimplementing either package.
func heuristicAmbiguities(ic issueContext) []string {
	var out []string
	if len(ic.GoverningRule) <= thinRuleLinkageThreshold {
		out = append(out, "no governing rule is linked to this issue in the case tree")
	}
	if ic.Node.Confidence <= lowExtractionConfidenceThreshold {
		out = append(out, fmt.Sprintf("upstream extraction confidence is low (%.2f)", ic.Node.Confidence))
	}
	return out
}

// ruleLinkageConfidenceSignal returns a [0,1] signal for how well an
// issue's governing-rule linkage supports trusting its framing: richer
// linkage (more governing rules, up to saturation) yields a higher
// signal.
func ruleLinkageConfidenceSignal(ruleCount int) float64 {
	return clamp01(float64(ruleCount) / float64(materialityRuleSaturation))
}

// blendConfidence combines a model-reported confidence (when parseable)
// with the heuristic rule-linkage signal into the FramedIssue.Confidence
// field. Mirrors blendMateriality's weighting shape but is kept as its
// own function since materiality and confidence are conceptually
// distinct axes (see types.go's FramedIssue doc comment) and may need to
// diverge in weighting later.
func blendConfidence(ruleLinkageSignal, modelConfidence float64, modelConfidenceOK bool) float64 {
	if !modelConfidenceOK {
		return clamp01(0.5 + 0.5*ruleLinkageSignal)
	}
	const modelWeight = 0.6
	return clamp01(modelWeight*modelConfidence + (1-modelWeight)*ruleLinkageSignal)
}
