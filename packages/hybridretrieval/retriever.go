package hybridretrieval

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/traversal"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// Retriever runs HybridQuerys against a vectorindex.VectorStore and a
// graph.GraphStore, fusing vector recall with graph-traversal expansion
// into a single ranked Result. This is the package's main entry point:
// build one with NewRetriever, then call Retrieve for each HybridQuery.
//
// Retriever is a thin orchestrator over vectorindex.VectorStore and
// traversal.Walker — it holds no index or graph state of its own, and is
// safe for concurrent use as long as the underlying store/walker are (both
// InMemoryVectorStore and Walker are).
type Retriever struct {
	vectors vectorindex.VectorStore
	walker  *traversal.Walker
}

// NewRetriever constructs a Retriever over vectors and graphStore. Returns
// ErrNilVectorStore or ErrNilGraphStore if either is nil.
func NewRetriever(vectors vectorindex.VectorStore, graphStore graph.GraphStore) (*Retriever, error) {
	if vectors == nil {
		return nil, ErrNilVectorStore
	}
	if graphStore == nil {
		return nil, ErrNilGraphStore
	}
	walker, err := traversal.NewWalker(graphStore)
	if err != nil {
		return nil, fmt.Errorf("hybridretrieval: new retriever: %w", err)
	}
	return &Retriever{vectors: vectors, walker: walker}, nil
}

// Retrieve executes query: it runs vector recall over r.vectors, expands
// the resulting hits (and query.AnchorNodeID, if set) via graph traversal
// over r.walker, fuses both ranked lists with reciprocal rank fusion,
// applies dedup/diversity and the query's TopK cap, and returns the fused
// Result.
//
// # Latency budget
//
// If query.Budget is set, Retrieve tracks a deadline from the moment
// Retrieve is called. If vector recall alone already consumes the entire
// budget, graph expansion is skipped outright (Result.ExpansionSkipped is
// true) and the result is vector-recall-only. If the budget runs out
// partway through expanding seeds, remaining seeds are skipped and
// Result.ExpansionTruncated is set. Vector recall itself is never
// skipped or truncated by the budget — it is assumed to be the fast,
// always-available floor of a hybrid query's latency, with graph
// expansion as the optional, budget-sensitive enrichment on top (see
// doc/hybrid-retrieval.md).
func (r *Retriever) Retrieve(ctx context.Context, query HybridQuery) (Result, error) {
	if err := query.validate(); err != nil {
		return Result{}, err
	}

	budget := newBudgetTracker(query.Budget)

	vectorHits, err := runVectorRecall(ctx, r.vectors, query)
	if err != nil {
		return Result{}, err
	}

	seeds := expansionSeeds(query, vectorHits)

	if len(query.ExpansionHops) == 0 || len(seeds) == 0 {
		items := fuse(query, vectorHits, nil)
		items = dedupAndDiversify(items, query.TopK, query.MaxPerAnchor)
		return Result{
			Items:              items,
			VectorHitCount:     len(vectorHits),
			ExpansionSeedCount: 0,
		}, nil
	}

	if budget.exhausted() {
		items := fuse(query, vectorHits, nil)
		items = dedupAndDiversify(items, query.TopK, query.MaxPerAnchor)
		return Result{
			Items:              items,
			VectorHitCount:     len(vectorHits),
			ExpansionSeedCount: len(seeds),
			ExpansionSkipped:   true,
		}, nil
	}

	expCtx, cancel := budget.withDeadline(ctx)
	defer cancel()

	var graphHits []graphHit
	truncated := false
	for _, seed := range seeds {
		if budget.exhausted() {
			truncated = true
			break
		}
		hits, err := expandSeed(expCtx, r.walker, query, seed, query.MaxPerAnchor)
		if err != nil {
			return Result{}, err
		}
		graphHits = append(graphHits, hits...)
	}
	graphHits = filterGraphHits(query, graphHits)

	items := fuse(query, vectorHits, graphHits)
	items = dedupAndDiversify(items, query.TopK, query.MaxPerAnchor)

	return Result{
		Items:              items,
		VectorHitCount:     len(vectorHits),
		ExpansionSeedCount: len(seeds),
		ExpansionTruncated: truncated,
	}, nil
}

// expansionSeeds computes the ordered, deduplicated set of node IDs graph
// expansion should walk from: query.AnchorNodeID (if set) first, followed
// by every distinct vector-recall hit's node ID in rank order.
func expansionSeeds(query HybridQuery, vectorHits []vectorHit) []string {
	seen := make(map[string]struct{})
	var seeds []string

	if query.AnchorNodeID != "" {
		seeds = append(seeds, query.AnchorNodeID)
		seen[query.AnchorNodeID] = struct{}{}
	}
	for _, vh := range vectorHits {
		id := vh.result.Record.ID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		seeds = append(seeds, id)
	}
	return seeds
}
