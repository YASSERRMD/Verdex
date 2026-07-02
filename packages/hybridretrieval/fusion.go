package hybridretrieval

import (
	"fmt"
	"sort"
)

// fuse combines vectorHits and graphHits into a single ranked, deduplicated
// []Item using reciprocal rank fusion (RRF): each item's CombinedScore is
// the sum of 1/(k+rank) over every ranked list it appears in, where k is
// query's RRFConstant (or DefaultRRFConstant). This is the standard RRF
// formula (Cormack, Clarke & Buettcher, 2009) — see doc/hybrid-retrieval.md
// for why RRF was chosen over a weighted-sum-of-raw-scores fusion.
//
// graphHits that fail query.Filter (via query.MetadataLookup) are excluded
// before fusion — see filterGraphHits. Vector hits are never re-filtered
// here: vectorindex.VectorStore.Query already applied query.Filter during
// recall (see vectorrecall.go), so re-checking here would be redundant.
func fuse(query HybridQuery, vectorHits []vectorHit, graphHits []graphHit) []Item {
	k := query.RRFConstant
	if k == 0 {
		k = DefaultRRFConstant
	}

	items := make(map[string]*Item)

	for _, vh := range vectorHits {
		id := vh.result.Record.ID
		items[id] = &Item{
			NodeID:      id,
			NodeType:    vh.result.Record.NodeType,
			Text:        vh.result.Record.Text,
			Path:        RetrievalPathVector,
			VectorScore: vh.result.VectorScore,
			VectorRank:  vh.rank,
			Explanation: fmt.Sprintf("vector similarity (rank %d, score %.4f)", vh.rank, vh.result.VectorScore),
		}
	}

	graphRanked := rankGraphHitsForFusion(graphHits)
	for _, gh := range graphRanked {
		if existing, ok := items[gh.hit.nodeID]; ok {
			existing.Path = RetrievalPathBoth
			existing.GraphScore = gh.hit.score
			existing.GraphRank = gh.rank
			existing.AnchorNodeID = gh.hit.anchorID
			existing.Explanation = fmt.Sprintf(
				"%s; graph expansion from %s (rank %d): %s",
				existing.Explanation, gh.hit.anchorID, gh.rank, gh.hit.explain,
			)
			continue
		}
		items[gh.hit.nodeID] = &Item{
			NodeID:       gh.hit.nodeID,
			NodeType:     gh.hit.node.Type,
			Text:         gh.hit.node.Text,
			Path:         RetrievalPathGraph,
			GraphScore:   gh.hit.score,
			GraphRank:    gh.rank,
			AnchorNodeID: gh.hit.anchorID,
			Explanation:  fmt.Sprintf("graph expansion from %s (rank %d): %s", gh.hit.anchorID, gh.rank, gh.hit.explain),
		}
	}

	out := make([]Item, 0, len(items))
	for _, item := range items {
		item.CombinedScore = rrfScore(k, item.VectorRank) + rrfScore(k, item.GraphRank)
		out = append(out, *item)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CombinedScore != out[j].CombinedScore {
			return out[i].CombinedScore > out[j].CombinedScore
		}
		return out[i].NodeID < out[j].NodeID
	})

	return out
}

// rrfScore returns 1/(k+rank) for a positive rank, or 0 if rank is 0
// (meaning "this item did not appear in this ranked list at all" — RRF
// only sums contributions from lists an item actually appears in).
func rrfScore(k float64, rank int) float64 {
	if rank <= 0 {
		return 0
	}
	return 1.0 / (k + float64(rank))
}

// rankedGraphHit pairs a graphHit with its 1-based rank in the merged,
// deduplicated-by-node graph-expansion result set.
type rankedGraphHit struct {
	hit  graphHit
	rank int
}

// rankGraphHitsForFusion merges every seed's graphHits into one list
// (keeping, per node ID, only the best-scoring hit across all seeds — the
// same node can be reached from more than one seed), sorted by descending
// score, and assigns 1-based ranks. This mirrors how vectorHits are
// already ranked by vectorindex.VectorStore.Query, so both ranked lists
// feeding fuse are shaped consistently.
func rankGraphHitsForFusion(graphHits []graphHit) []rankedGraphHit {
	best := make(map[string]graphHit, len(graphHits))
	for _, gh := range graphHits {
		if existing, ok := best[gh.nodeID]; !ok || gh.score > existing.score {
			best[gh.nodeID] = gh
		}
	}

	merged := make([]graphHit, 0, len(best))
	for _, gh := range best {
		merged = append(merged, gh)
	}
	sortGraphHitsByScoreDesc(merged)

	ranked := make([]rankedGraphHit, len(merged))
	for i, gh := range merged {
		ranked[i] = rankedGraphHit{hit: gh, rank: i + 1}
	}
	return ranked
}
