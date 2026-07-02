package issue

// confidence.go aggregates the several independent confidence signals
// produced across the extraction pipeline — identification (identify.go),
// claim-mapping (claim_map.go), and dedup merging (dedup.go) — into a
// single, normalized final Confidence on each CandidateIssue.
//
// Weights are chosen so identification confidence (the strongest single
// signal, since it reflects how specific the matched dispute-language
// pattern was) dominates, while claim support and dedup corroboration
// each nudge the score up, never down: an issue that also has claim
// support or was corroborated by a merge is, if anything, more likely to
// be a genuine issue, not less.
const (
	identificationWeight = 0.6
	claimSupportWeight   = 0.25
	dedupWeight          = 0.15
)

// ScoreConfidence recomputes issues[i].Confidence for every i as a
// weighted aggregate of:
//
//   - identification confidence: the CandidateIssue's own Confidence as
//     set by IssueIdentifier.Identify (or by Dedup's max-of-merged
//     rule) — this is the base signal.
//   - claim support: whether any ClaimLink in claimLinks references
//     issues[i], and how strong the best such link's Overlap is.
//   - dedup corroboration: whether issues[i] carries more than one
//     SourceSpan, which Dedup.mergeCandidateIssues produces when two or
//     more independently identified candidates were merged — multiple
//     independent spans agreeing on the same issue is itself a
//     confidence signal.
//
// The result is clamped to the closed interval [0, 1] (see
// irac.ValidConfidence) and returned as a new slice; issues is not
// mutated in place.
func ScoreConfidence(issues []CandidateIssue, claimLinks []ClaimLink) []CandidateIssue {
	bestClaimOverlap := make(map[int]float64, len(claimLinks))
	for _, link := range claimLinks {
		if link.Overlap > bestClaimOverlap[link.IssueIndex] {
			bestClaimOverlap[link.IssueIndex] = link.Overlap
		}
	}

	out := make([]CandidateIssue, len(issues))
	for i, iss := range issues {
		identificationScore := clamp01(iss.Confidence)
		claimScore := clamp01(bestClaimOverlap[i])

		dedupScore := 0.0
		if len(iss.SourceSpans) > 1 {
			dedupScore = 1.0
		}

		aggregate := identificationWeight*identificationScore +
			claimSupportWeight*claimScore +
			dedupWeight*dedupScore

		iss.Confidence = clamp01(aggregate)
		out[i] = iss
	}
	return out
}

// clamp01 clamps v to the closed interval [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
