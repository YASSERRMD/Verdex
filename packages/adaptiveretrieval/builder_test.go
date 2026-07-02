package adaptiveretrieval_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/adaptiveretrieval"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

func TestNewBuilder_NilStore_ReturnsError(t *testing.T) {
	_, err := adaptiveretrieval.NewBuilder(nil, adaptiveretrieval.BuilderOptions{})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestBuild_EmptyCaseID_ReturnsError(t *testing.T) {
	store := newSeededStore(t)
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	_, err = b.Build(context.Background(), adaptiveretrieval.AdaptiveQuery{AnchorNodeID: "x"})
	if err == nil {
		t.Fatal("expected error for empty case id")
	}
}

func TestBuild_EmptyAnchorNodeID_ReturnsError(t *testing.T) {
	store := newSeededStore(t)
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	_, err = b.Build(context.Background(), adaptiveretrieval.AdaptiveQuery{CaseID: "case-anchor-required"})
	if err == nil {
		t.Fatal("expected error for empty anchor node id")
	}
}

func TestBuild_FreshBuild_ReturnsSubgraphFromWalker(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-fresh-build"
	issueID := seedCleanTree(t, store, caseID)

	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID).
		WithHop(hybridretrieval.ExpansionGoverningRule)

	sg, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sg.Source != adaptiveretrieval.SourceBuilt {
		t.Fatalf("Source = %q, want %q", sg.Source, adaptiveretrieval.SourceBuilt)
	}
	if len(sg.Paths) == 0 {
		t.Fatal("expected at least one path")
	}
	if sg.NodesVisited == 0 {
		t.Fatal("expected NodesVisited > 0")
	}
}

func TestBuild_SecondCallForSameQuery_IsCacheHit(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-cache-hit"
	issueID := seedCleanTree(t, store, caseID)

	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID).
		WithHop(hybridretrieval.ExpansionGoverningRule)

	if _, err := b.Build(context.Background(), q); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	sg2, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if sg2.Source != adaptiveretrieval.SourceCached {
		t.Fatalf("Source = %q, want %q", sg2.Source, adaptiveretrieval.SourceCached)
	}

	tel := b.Telemetry()
	if tel.CacheHits != 1 {
		t.Fatalf("CacheHits = %d, want 1", tel.CacheHits)
	}
	if tel.CacheMisses != 1 {
		t.Fatalf("CacheMisses = %d, want 1", tel.CacheMisses)
	}
	if tel.Builds != 1 {
		t.Fatalf("Builds = %d, want 1", tel.Builds)
	}
}

func TestBuild_StaleCache_TriggersRebuild(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-stale-refresh"
	issueID := seedCleanTree(t, store, caseID)

	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID).
		WithHop(hybridretrieval.ExpansionGoverningRule)

	if _, err := b.Build(context.Background(), q); err != nil {
		t.Fatalf("first Build: %v", err)
	}

	// Bump the case's tree revision: the cached subgraph is now stale.
	if err := b.SetRevision(irac.TreeRevision{CaseID: caseID, RevisionNumber: 2}); err != nil {
		t.Fatalf("SetRevision: %v", err)
	}

	sg, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("Build after revision bump: %v", err)
	}
	if sg.Source != adaptiveretrieval.SourceBuilt {
		t.Fatalf("Source = %q, want %q (stale cache should trigger a fresh build)", sg.Source, adaptiveretrieval.SourceBuilt)
	}

	tel := b.Telemetry()
	if tel.StaleRefreshes != 1 {
		t.Fatalf("StaleRefreshes = %d, want 1", tel.StaleRefreshes)
	}
	if tel.Builds != 2 {
		t.Fatalf("Builds = %d, want 2", tel.Builds)
	}
}

func TestBuild_ExceedsMaxNodes_FallsBackToTreeIndex(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-max-nodes"
	issueID := seedLargeFanout(t, store, caseID, 20)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(context.Background(), caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	budget := adaptiveretrieval.BuildBudget{MaxNodes: 2, MaxHops: 4, MaxWallClock: time.Second}
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{Budget: budget})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	b = b.WithFallback(idx)

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID)

	sg, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sg.Source != adaptiveretrieval.SourceFallback {
		t.Fatalf("Source = %q, want %q", sg.Source, adaptiveretrieval.SourceFallback)
	}

	tel := b.Telemetry()
	if tel.FallbacksTriggered != 1 {
		t.Fatalf("FallbacksTriggered = %d, want 1", tel.FallbacksTriggered)
	}
}

func TestBuild_ExceedsBudgetNoFallback_ReturnsError(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-no-fallback"
	issueID := seedLargeFanout(t, store, caseID, 20)

	budget := adaptiveretrieval.BuildBudget{MaxNodes: 2, MaxHops: 4, MaxWallClock: time.Second}
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{Budget: budget})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID)

	_, err = b.Build(context.Background(), q)
	if err == nil {
		t.Fatal("expected error when budget exceeded and no fallback configured")
	}
}

func TestBuild_WallClockTimeout_FallsBackGracefully(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-wallclock-timeout"
	issueID := seedLargeFanout(t, store, caseID, 50)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(context.Background(), caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	// An impossibly tight wall-clock budget: any real traversal exceeds
	// it, exercising the deadline-based safeguard rather than the
	// node-count check.
	budget := adaptiveretrieval.BuildBudget{MaxNodes: 1000, MaxHops: 4, MaxWallClock: time.Nanosecond}
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{Budget: budget})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	b = b.WithFallback(idx)

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID)

	sg, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if sg.Source != adaptiveretrieval.SourceFallback {
		t.Fatalf("Source = %q, want %q", sg.Source, adaptiveretrieval.SourceFallback)
	}
}

func TestBuild_FromHybridQuery_DerivesShapeCorrectly(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-from-hybrid-query"
	issueID := seedCleanTree(t, store, caseID)

	hq := hybridretrieval.NewHybridQuery(caseID, nil).
		WithExpansion(hybridretrieval.ExpansionGoverningRule)

	q := adaptiveretrieval.FromHybridQuery(hq, issueID, 0)

	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	sg, err := b.Build(context.Background(), q)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(sg.Paths) == 0 {
		t.Fatal("expected at least one path")
	}
}

func TestBuilder_WithFallback_NilClearsFallback(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-clear-fallback"
	issueID := seedLargeFanout(t, store, caseID, 20)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	budget := adaptiveretrieval.BuildBudget{MaxNodes: 2, MaxHops: 4, MaxWallClock: time.Second}
	b, err := adaptiveretrieval.NewBuilder(store, adaptiveretrieval.BuilderOptions{Budget: budget})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	b = b.WithFallback(idx).WithFallback(nil)

	q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID)
	_, err = b.Build(context.Background(), q)
	if err == nil {
		t.Fatal("expected error: fallback was cleared")
	}
}
