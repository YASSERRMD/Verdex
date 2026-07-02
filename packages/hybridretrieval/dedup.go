package hybridretrieval

import "strings"

// dedupAndDiversify trims items (already sorted by descending
// CombinedScore) down to at most topK results, applying two independent
// controls along the way:
//
//  1. Per-anchor cap: at most maxPerAnchor items sharing the same
//     non-empty AnchorNodeID are kept, in score order, so one
//     densely-connected seed node cannot crowd out every other
//     candidate's results. Items with no anchor (vector-only hits that
//     were never used as an expansion seed) are never capped by this
//     rule.
//  2. Near-duplicate text collapse: items whose Text normalizes to the
//     same value (case-insensitive, whitespace-trimmed) as an
//     already-kept item are dropped, keeping only the first (highest-
//     scoring) occurrence. This is the "avoid returning near-duplicate
//     or redundant nodes" requirement — a lightweight, dependency-free
//     stand-in for a full MMR/embedding-distance diversity pass (see
//     doc/hybrid-retrieval.md for why exact-normalized-text dedup was
//     chosen over embedding-distance MMR for this phase).
//
// items must already be sorted by descending CombinedScore (fuse
// guarantees this); dedupAndDiversify does not re-sort.
func dedupAndDiversify(items []Item, topK, maxPerAnchor int) []Item {
	if topK <= 0 {
		topK = DefaultTopK
	}
	if maxPerAnchor <= 0 {
		maxPerAnchor = DefaultMaxPerAnchor
	}

	seenText := make(map[string]struct{}, len(items))
	anchorCount := make(map[string]int, len(items))
	out := make([]Item, 0, topK)

	for _, item := range items {
		if len(out) >= topK {
			break
		}

		norm := normalizeText(item.Text)
		if norm != "" {
			if _, dup := seenText[norm]; dup {
				continue
			}
		}

		if item.AnchorNodeID != "" && anchorCount[item.AnchorNodeID] >= maxPerAnchor {
			continue
		}

		if norm != "" {
			seenText[norm] = struct{}{}
		}
		if item.AnchorNodeID != "" {
			anchorCount[item.AnchorNodeID]++
		}
		out = append(out, item)
	}

	return out
}

// normalizeText lowercases and collapses surrounding/interior whitespace
// so two near-identical node texts (differing only by case or incidental
// spacing) are recognized as duplicates.
func normalizeText(s string) string {
	fields := strings.Fields(s)
	return strings.ToLower(strings.Join(fields, " "))
}
