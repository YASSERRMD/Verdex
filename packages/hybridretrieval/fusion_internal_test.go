package hybridretrieval

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func TestRRFScore_ZeroRankContributesNothing(t *testing.T) {
	if got := rrfScore(60, 0); got != 0 {
		t.Errorf("rrfScore(60, 0) = %v, want 0", got)
	}
}

func TestRRFScore_LowerRankScoresHigher(t *testing.T) {
	r1 := rrfScore(60, 1)
	r2 := rrfScore(60, 2)
	if r1 <= r2 {
		t.Errorf("expected rank 1 to score higher than rank 2: r1=%v r2=%v", r1, r2)
	}
}

func TestFuse_ItemPresentInBothListsSumsContributions(t *testing.T) {
	query := HybridQuery{RRFConstant: 60}
	vectorHits := []vectorHit{
		{result: vectorindex.ScoredResult{Record: vectorindex.VectorRecord{ID: "n1", NodeType: irac.NodeRule}, VectorScore: 0.9}, rank: 1},
	}
	graphHits := []graphHit{
		{nodeID: "n1", node: traversal.PathNode{ID: "n1", Type: irac.NodeRule}, score: 1.0, anchorID: "seed-1", explain: "seed-1 --governing_rule--> n1"},
	}

	items := fuse(query, vectorHits, graphHits)
	if len(items) != 1 {
		t.Fatalf("expected 1 fused item, got %d", len(items))
	}
	item := items[0]
	if item.Path != RetrievalPathBoth {
		t.Errorf("expected Path RetrievalPathBoth, got %q", item.Path)
	}
	want := rrfScore(60, 1) + rrfScore(60, 1)
	if item.CombinedScore != want {
		t.Errorf("CombinedScore = %v, want %v", item.CombinedScore, want)
	}
}

func TestFuse_VectorOnlyItemHasZeroGraphRank(t *testing.T) {
	query := HybridQuery{}
	vectorHits := []vectorHit{
		{result: vectorindex.ScoredResult{Record: vectorindex.VectorRecord{ID: "n1"}}, rank: 1},
	}
	items := fuse(query, vectorHits, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].GraphRank != 0 || items[0].Path != RetrievalPathVector {
		t.Errorf("expected vector-only item, got %+v", items[0])
	}
}

func TestFuse_GraphOnlyItemHasZeroVectorRank(t *testing.T) {
	query := HybridQuery{}
	graphHits := []graphHit{
		{nodeID: "n1", node: traversal.PathNode{ID: "n1"}, score: 0.5, anchorID: "seed-1"},
	}
	items := fuse(query, nil, graphHits)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].VectorRank != 0 || items[0].Path != RetrievalPathGraph {
		t.Errorf("expected graph-only item, got %+v", items[0])
	}
}

func TestFuse_SortedByDescendingCombinedScore(t *testing.T) {
	query := HybridQuery{}
	vectorHits := []vectorHit{
		{result: vectorindex.ScoredResult{Record: vectorindex.VectorRecord{ID: "weak"}}, rank: 5},
		{result: vectorindex.ScoredResult{Record: vectorindex.VectorRecord{ID: "strong"}}, rank: 1},
	}
	items := fuse(query, vectorHits, nil)
	if len(items) != 2 || items[0].NodeID != "strong" {
		t.Fatalf("expected 'strong' first, got %+v", items)
	}
}

func TestRankGraphHitsForFusion_DedupsByBestScorePerNode(t *testing.T) {
	hits := []graphHit{
		{nodeID: "n1", score: 0.2, anchorID: "seed-a"},
		{nodeID: "n1", score: 0.8, anchorID: "seed-b"},
	}
	ranked := rankGraphHitsForFusion(hits)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 deduplicated hit, got %d", len(ranked))
	}
	if ranked[0].hit.score != 0.8 || ranked[0].hit.anchorID != "seed-b" {
		t.Errorf("expected the higher-scoring hit to win, got %+v", ranked[0].hit)
	}
}

func TestIsZeroFilter(t *testing.T) {
	if !isZeroFilter(Filter{}) {
		t.Errorf("expected zero Filter to be reported as zero")
	}
	if isZeroFilter(Filter{JurisdictionCode: "us-ny"}) {
		t.Errorf("expected non-empty Filter to be reported as non-zero")
	}
}

func TestFilterGraphHits_NoFilterPassesThrough(t *testing.T) {
	hits := []graphHit{{nodeID: "n1"}, {nodeID: "n2"}}
	out := filterGraphHits(HybridQuery{}, hits)
	if len(out) != 2 {
		t.Errorf("expected all hits to pass with a zero filter, got %d", len(out))
	}
}

func TestFilterGraphHits_ExcludesWithoutLookup(t *testing.T) {
	hits := []graphHit{{nodeID: "n1"}}
	q := HybridQuery{Filter: Filter{JurisdictionCode: "us-ny"}}
	out := filterGraphHits(q, hits)
	if len(out) != 0 {
		t.Errorf("expected hits excluded when no MetadataLookup is set, got %d", len(out))
	}
}
