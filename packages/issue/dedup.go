package issue

import "github.com/YASSERRMD/verdex/packages/irac"

// dedupSimilarityThreshold is the minimum Jaccard similarity between two
// CandidateIssues' normalized token sets for them to be considered
// near-duplicates and merged by Dedup.
const dedupSimilarityThreshold = 0.6

// Dedup merges near-duplicate CandidateIssues in issues using a
// token-overlap (Jaccard) similarity heuristic over each issue's
// normalized word set: two issues whose Jaccard similarity is at or above
// dedupSimilarityThreshold are merged into one, keeping the union of their
// SourceSpans and the maximum of their Confidence scores.
//
// Merging is transitive within a single pass: issues are compared
// pairwise in input order, and once an issue is merged into an earlier
// survivor it is not itself compared against later issues (its merged
// spans/confidence carry forward on the survivor instead).
func Dedup(issues []CandidateIssue) []CandidateIssue {
	if len(issues) == 0 {
		return nil
	}

	merged := make([]bool, len(issues))
	tokens := make([]map[string]struct{}, len(issues))
	for i, iss := range issues {
		tokens[i] = tokenSet(iss.Text)
	}

	out := make([]CandidateIssue, 0, len(issues))
	for i := range issues {
		if merged[i] {
			continue
		}
		survivor := issues[i]
		for j := i + 1; j < len(issues); j++ {
			if merged[j] {
				continue
			}
			if jaccardSimilarity(tokens[i], tokens[j]) >= dedupSimilarityThreshold {
				survivor = mergeCandidateIssues(survivor, issues[j])
				merged[j] = true
			}
		}
		out = append(out, survivor)
	}

	return out
}

// mergeCandidateIssues combines a and b into a single CandidateIssue: a's
// Text and ID are kept (a is the earlier-seen, "canonical" survivor), the
// SourceSpans are the union (deduplicated by value), and Confidence is the
// max of the two.
func mergeCandidateIssues(a, b CandidateIssue) CandidateIssue {
	a.SourceSpans = unionSpans(a.SourceSpans, b.SourceSpans)
	if b.Confidence > a.Confidence {
		a.Confidence = b.Confidence
	}
	return a
}

// unionSpans returns the union of a and b, skipping spans already present
// (by value equality) in a.
func unionSpans(a, b []irac.SourceSpan) []irac.SourceSpan {
	seen := make(map[irac.SourceSpan]struct{}, len(a))
	out := make([]irac.SourceSpan, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// jaccardSimilarity returns the Jaccard similarity |A ∩ B| / |A ∪ B|
// between two token sets. Two empty sets are defined as fully similar
// (1.0) since neither carries any distinguishing content.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	intersection := 0
	for t := range a {
		if _, ok := b[t]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
