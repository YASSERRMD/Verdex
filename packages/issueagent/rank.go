package issueagent

import "sort"

// heuristicMaterialityFloor is the minimum materiality score assigned to
// every issue before any model-reported score is blended in, so an issue
// the model fails to score (e.g. a malformed/partial response) never
// drops to zero materiality purely from parser gaps.
const heuristicMaterialityFloor = 0.1

// heuristicMaterialityScore derives a baseline [0,1] materiality signal
// from an issueContext alone, independent of any model output: an issue
// with more governing rules linked to it is treated as more central to
// the case (it is where the bulk of the tree's legal reasoning already
// converges), and an issue's own extraction Confidence contributes a
// smaller weight. This is intentionally a coarse prior — the model's
// materiality_score (see prompt.go/parse.go) is expected to dominate the
// blended MaterialityScore in rankIssues; the heuristic exists so a
// legitimate case (e.g. a fake/no-op provider in tests, or a model
// response missing an issue) still yields a stable, sensible order.
func heuristicMaterialityScore(ic issueContext) float64 {
	ruleSignal := clamp01(float64(len(ic.GoverningRule)) / float64(materialityRuleSaturation))
	score := heuristicMaterialityFloor + 0.6*ruleSignal + 0.3*ic.Node.Confidence
	return clamp01(score)
}

// materialityRuleSaturation is the number of governing rules at which the
// heuristic rule-linkage signal saturates to 1.0.
const materialityRuleSaturation = 3

// blendMateriality combines the heuristic score with a model-reported
// score (when present) into the final MaterialityScore used for ranking.
// modelScoreOK is false when the model's response did not include a
// parseable score for this issue, in which case the heuristic alone is
// used.
func blendMateriality(heuristic, modelScore float64, modelScoreOK bool) float64 {
	if !modelScoreOK {
		return heuristic
	}
	const modelWeight = 0.7
	return clamp01(modelWeight*modelScore + (1-modelWeight)*heuristic)
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

// rankIssues sorts issues by MaterialityScore descending (most material
// first), breaking ties deterministically by SourceIssueNodeID so a
// re-run over identical input always yields identical ordering, and
// assigns each issue's 1-based MaterialityRank in place.
func rankIssues(issues []FramedIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].MaterialityScore != issues[j].MaterialityScore {
			return issues[i].MaterialityScore > issues[j].MaterialityScore
		}
		return issues[i].SourceIssueNodeID < issues[j].SourceIssueNodeID
	})
	for i := range issues {
		issues[i].MaterialityRank = i + 1
	}
}
