package perf

import (
	"context"
	"fmt"
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

// BenchmarkTraversal_ViaGoverningRule benchmarks the real
// traversal.Walker.Execute code path over an in-memory graph.GraphStore
// fixture (packages/graph.NewInMemoryGraphStore, populated by
// buildGraphFixture) with a representative fan-out: 500 IssueNodes, each
// governed by its own RuleNode via a real irac.Edge{Type:
// irac.EdgeGoverns}. Every b.N iteration walks the single-hop
// ViaGoverningRule query (Reverse EdgeGoverns walk, per
// packages/traversal/query.go) from a different issue node in round-robin
// order, so the benchmark exercises real store lookups rather than
// repeatedly hitting the same cached path.
func BenchmarkTraversal_ViaGoverningRule(b *testing.B) {
	const issueCount = 500
	store, issueIDs := buildGraphFixture(b, issueCount)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		b.Fatalf("NewWalker: %v", err)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startID := issueIDs[i%len(issueIDs)]
		query := traversal.NewQuery(benchCaseID, startID).ViaGoverningRule()

		result, err := walker.Execute(ctx, query)
		if err != nil {
			b.Fatalf("Execute: %v", err)
		}
		if len(result.Paths) == 0 {
			b.Fatal("expected at least one path from a governed issue to its rule")
		}
	}
}

// BenchmarkTraversal_CustomEdgeWalk benchmarks Walker.Execute using the
// general-purpose Via(EdgeType, Direction, NodeTypeFilter) builder method
// instead of the named ViaGoverningRule hop, over the same fixture, so both
// of Query's construction styles are represented in this suite.
func BenchmarkTraversal_CustomEdgeWalk(b *testing.B) {
	const issueCount = 500
	store, issueIDs := buildGraphFixture(b, issueCount)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		b.Fatalf("NewWalker: %v", err)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startID := issueIDs[i%len(issueIDs)]
		query := traversal.NewQuery(benchCaseID, startID).
			Via(irac.EdgeGoverns, traversal.Reverse, irac.NodeRule).
			WithMaxDepth(1)

		if _, err := walker.Execute(ctx, query); err != nil {
			b.Fatalf("Execute: %v", err)
		}
	}
}

// BenchmarkTraversal_ScalingFanOut sweeps the graph fixture size to show
// how Walker.Execute's latency scales with the number of issue/rule pairs
// present in the store (not walked -- each iteration still only walks one
// hop from one issue), which mostly exercises the fixture-populated
// in-memory GraphStore's own lookup cost (see packages/graph/index.go's
// byCase/typeIndex secondary indexes, referenced by
// doc/graph-optimization-checklist.md).
func BenchmarkTraversal_ScalingFanOut(b *testing.B) {
	for _, size := range []int{50, 500, 2000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			store, issueIDs := buildGraphFixture(b, size)
			walker, err := traversal.NewWalker(store)
			if err != nil {
				b.Fatalf("NewWalker: %v", err)
			}
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				startID := issueIDs[i%len(issueIDs)]
				query := traversal.NewQuery(benchCaseID, startID).ViaGoverningRule()
				if _, err := walker.Execute(ctx, query); err != nil {
					b.Fatalf("Execute: %v", err)
				}
			}
		})
	}
}
