package reasoningeval

import "strings"

// CoherenceDimension returns a Dimension that scores an Opinion's
// structural coherence: how completely and substantively it addresses
// the issues it was asked to resolve.
//
// This dimension is purely structural — it counts conclusions, checks
// text length, and inspects per-conclusion confidence — and never
// evaluates or rewrites the Opinion's own prose for verdict-like
// language. That check belongs exclusively to packages/guardrail
// (irac.ContainsVerdictLanguage), which synthesisagent already runs
// before a TentativeConclusion is ever constructed (see
// synthesisagent's ConclusionProvider adapter). Duplicating that check
// here would risk this dimension silently drifting out of sync with
// guardrail's own verdict-language ban; instead this dimension composes
// with it by construction: an Opinion this package ever sees has already
// passed guardrail's check, so nothing here needs to re-verify it.
//
// Score is the average of three sub-scores, each in [0,1]:
//   - coverage: 1 - (skipped issues / (conclusions + skipped issues)),
//     i.e. the fraction of issues that reached a conclusion at all. An
//     opinion with no issues and no skips scores 1.0 (nothing expected,
//     nothing missing).
//   - substance: fraction of conclusions whose Text is non-trivial
//     (at least minSubstantiveLength characters after trimming
//     whitespace), penalising empty or near-empty conclusion text.
//   - confidence: the mean of each conclusion's own Confidence field,
//     reflecting how confidently the reasoning pipeline itself rated its
//     output (0 when there are no conclusions).
func CoherenceDimension(weight float64) Dimension {
	return Dimension{
		Name:   DimensionCoherence,
		Weight: weight,
		Scorer: func(input ScoreInput) (float64, error) {
			if input.Opinion == nil {
				return 0, nil
			}
			return scoreCoherence(input.Opinion), nil
		},
	}
}

// minSubstantiveLength is the minimum trimmed character length for a
// conclusion's Text to count as "substantive" rather than trivial or
// empty.
const minSubstantiveLength = 40

func scoreCoherence(op OpinionLike) float64 {
	n := op.ConclusionCount()
	skipped := op.SkippedIssueCount()

	coverage := coverageScore(n, skipped)
	substance := substanceScore(op, n)
	confidence := confidenceScore(op, n)

	return clamp01((coverage + substance + confidence) / 3.0)
}

func coverageScore(conclusions, skipped int) float64 {
	total := conclusions + skipped
	if total == 0 {
		return 1.0
	}
	return float64(conclusions) / float64(total)
}

func substanceScore(op OpinionLike, n int) float64 {
	if n == 0 {
		return 0
	}
	substantive := 0
	for i := 0; i < n; i++ {
		if len(strings.TrimSpace(op.ConclusionText(i))) >= minSubstantiveLength {
			substantive++
		}
	}
	return float64(substantive) / float64(n)
}

func confidenceScore(op OpinionLike, n int) float64 {
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += clamp01(op.ConclusionConfidence(i))
	}
	return sum / float64(n)
}
