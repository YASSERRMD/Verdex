package hybridretrieval

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// vectorHit is one ranked candidate from the vector-recall phase, before
// fusion with graph expansion.
type vectorHit struct {
	result vectorindex.ScoredResult
	rank   int // 1-based
}

// runVectorRecall queries store for the top-K semantic candidates matching
// query, applying query.Filter. Returns hits ordered by descending
// VectorScore (the order vectorindex.VectorStore.Query already guarantees).
func runVectorRecall(ctx context.Context, store vectorindex.VectorStore, query HybridQuery) ([]vectorHit, error) {
	if len(query.Vector) == 0 {
		// A pure structural query (AnchorNodeID set, no Vector): nothing
		// to recall semantically, and this is not an error — see
		// HybridQuery.AnchorNodeID's doc comment.
		return nil, nil
	}

	topK := query.VectorTopK
	if topK == 0 {
		topK = DefaultVectorTopK
	}

	req := vectorindex.QueryRequest{
		Vector: query.Vector,
		TopK:   topK,
		Filter: query.Filter.toMetadataFilter(),
		CaseID: query.CaseID,
	}

	results, err := store.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("hybridretrieval: vector recall: %w", err)
	}

	hits := make([]vectorHit, len(results))
	for i, r := range results {
		hits[i] = vectorHit{result: r, rank: i + 1}
	}
	return hits, nil
}
