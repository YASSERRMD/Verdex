package perf

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
)

// BenchmarkHybridRetrieval_VectorOnly benchmarks
// hybridretrieval.Retriever.Retrieve over an in-memory
// vectorindex.VectorStore fixture (1000 synthetic records, built by
// buildVectorFixture) and an in-memory graph.GraphStore (built by
// buildGraphFixture) with no ExpansionHops configured, i.e. the
// vector-recall-only path. This isolates vector-recall cost from graph-
// expansion cost (see BenchmarkHybridRetrieval_WithGraphExpansion below).
func BenchmarkHybridRetrieval_VectorOnly(b *testing.B) {
	const recordCount = 1000
	vectorStore, queryVector := buildVectorFixture(b, recordCount)
	graphStore, _ := buildGraphFixture(b, 50)

	retriever, err := hybridretrieval.NewRetriever(vectorStore, graphStore)
	if err != nil {
		b.Fatalf("NewRetriever: %v", err)
	}

	query := hybridretrieval.NewHybridQuery(benchCaseID, queryVector).
		WithTopK(10).
		WithVectorTopK(20)

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := retriever.Retrieve(ctx, query)
		if err != nil {
			b.Fatalf("Retrieve: %v", err)
		}
		if result.VectorHitCount == 0 {
			b.Fatal("expected at least one vector-recall hit")
		}
	}
}

// BenchmarkHybridRetrieval_WithGraphExpansion benchmarks
// hybridretrieval.Retriever.Retrieve with ExpansionGoverningRule configured,
// so every call also runs the graph-expansion phase (a traversal.Walker
// walk from each vector-recall hit), fusing both signals with reciprocal
// rank fusion. Uses an anchor node id drawn from the graph fixture so
// expansion has a real governing-rule edge to walk from at least one seed.
func BenchmarkHybridRetrieval_WithGraphExpansion(b *testing.B) {
	const recordCount = 1000
	vectorStore, queryVector := buildVectorFixture(b, recordCount)
	graphStore, issueIDs := buildGraphFixture(b, 500)

	retriever, err := hybridretrieval.NewRetriever(vectorStore, graphStore)
	if err != nil {
		b.Fatalf("NewRetriever: %v", err)
	}

	query := hybridretrieval.NewHybridQuery(benchCaseID, queryVector).
		WithAnchor(issueIDs[0]).
		WithExpansion(hybridretrieval.ExpansionGoverningRule).
		WithTopK(10).
		WithVectorTopK(20).
		WithMaxPerAnchor(5)

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := retriever.Retrieve(ctx, query); err != nil {
			b.Fatalf("Retrieve: %v", err)
		}
	}
}

// BenchmarkHybridRetrieval_ScalingCorpusSize sweeps the vector-store
// fixture size to show how vector-recall-only Retrieve latency scales with
// corpus size under InMemoryVectorStore's exhaustive brute-force cosine
// scan (see packages/vectorindex/inmemory.go's doc comment: every Query
// scores every matching record regardless of IndexConfig).
func BenchmarkHybridRetrieval_ScalingCorpusSize(b *testing.B) {
	for _, size := range []int{100, 1000, 5000} {
		b.Run(benchName("records", size), func(b *testing.B) {
			vectorStore, queryVector := buildVectorFixture(b, size)
			graphStore, _ := buildGraphFixture(b, 10)

			retriever, err := hybridretrieval.NewRetriever(vectorStore, graphStore)
			if err != nil {
				b.Fatalf("NewRetriever: %v", err)
			}
			query := hybridretrieval.NewHybridQuery(benchCaseID, queryVector).WithTopK(10)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := retriever.Retrieve(ctx, query); err != nil {
					b.Fatalf("Retrieve: %v", err)
				}
			}
		})
	}
}
