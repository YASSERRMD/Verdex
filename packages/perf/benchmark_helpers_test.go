package perf

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// benchCaseID is the fixed case ID every benchmark fixture below builds
// its reasoning tree/vectors under.
const benchCaseID = "bench-case-1"

// benchVectorDimensions is the embedding dimensionality synthetic vectors
// use across the benchmark fixtures.
const benchVectorDimensions = 32

// buildGraphFixture populates an in-memory graph.GraphStore with
// issueCount IssueNodes, each governed by its own RuleNode via a real
// irac.Edge{Type: irac.EdgeGoverns} (Rule -> Issue, per
// packages/irac/edge.go's legalEdgeTriples), so
// traversal.Query.ViaGoverningRule (which walks EdgeGoverns in Reverse
// from an Issue to find its governing Rule) has real, representative data
// to walk. Returns the store plus the list of created issue node IDs.
func buildGraphFixture(tb testing.TB, issueCount int) (graph.GraphStore, []string) {
	tb.Helper()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	now := time.Now()

	issueIDs := make([]string, 0, issueCount)
	for i := 0; i < issueCount; i++ {
		issueID := fmt.Sprintf("issue-%d", i)
		ruleID := fmt.Sprintf("rule-%d", i)

		issue := irac.NewIssueNode(issueID, benchCaseID, fmt.Sprintf("issue text %d", i), now, 0.9, irac.Provenance{})
		rule := irac.NewRuleNode(ruleID, benchCaseID, fmt.Sprintf("rule text %d", i), "US-CA", "common_law", now, 0.9, irac.Provenance{})

		if err := store.CreateNode(ctx, issue.Node); err != nil {
			tb.Fatalf("CreateNode(issue): %v", err)
		}
		if err := store.CreateNode(ctx, rule.Node); err != nil {
			tb.Fatalf("CreateNode(rule): %v", err)
		}
		if err := store.CreateEdge(ctx, irac.Edge{FromID: ruleID, ToID: issueID, Type: irac.EdgeGoverns}); err != nil {
			tb.Fatalf("CreateEdge: %v", err)
		}

		issueIDs = append(issueIDs, issueID)
	}

	return store, issueIDs
}

// buildVectorFixture populates an in-memory vectorindex.VectorStore with
// recordCount synthetic VectorRecords under benchCaseID, each carrying a
// deterministic pseudo-random unit-ish embedding vector of dimension
// benchVectorDimensions, plus a query vector nudged close to record 0's
// vector so a similarity query returns a meaningful (non-empty, non-random)
// top-K.
func buildVectorFixture(tb testing.TB, recordCount int) (vectorindex.VectorStore, embedding.EmbeddingVector) {
	tb.Helper()

	store := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	var firstVector embedding.EmbeddingVector
	for i := 0; i < recordCount; i++ {
		vec := syntheticVector(i)
		if i == 0 {
			firstVector = vec
		}
		record := vectorindex.VectorRecord{
			ID:        fmt.Sprintf("vec-node-%d", i),
			NodeType:  irac.NodeIssue,
			CaseID:    benchCaseID,
			Text:      fmt.Sprintf("synthetic indexable text %d", i),
			Vector:    vec,
			UpdatedAt: time.Now(),
		}
		if err := store.Upsert(ctx, record); err != nil {
			tb.Fatalf("Upsert: %v", err)
		}
	}

	// A query vector very close to (but not identical to) the first
	// record's vector, so cosine-similarity ranking has a clear top
	// candidate rather than a tie across every record.
	query := make(embedding.EmbeddingVector, len(firstVector))
	for i, v := range firstVector {
		query[i] = v + 0.001
	}
	return store, query
}

// syntheticVector deterministically derives a benchVectorDimensions-length
// embedding vector from seed, spreading values via sin/cos so distinct
// seeds produce distinct, non-degenerate (non-zero-norm) vectors without
// needing a real embedding model or a math/rand dependency.
func syntheticVector(seed int) embedding.EmbeddingVector {
	vec := make(embedding.EmbeddingVector, benchVectorDimensions)
	for d := 0; d < benchVectorDimensions; d++ {
		angle := float64(seed+1) * float64(d+1) * 0.137
		vec[d] = math.Sin(angle)
	}
	return vec
}

// benchName builds a b.Run sub-benchmark name of the form "label=value",
// e.g. benchName("records", 1000) -> "records=1000".
func benchName(label string, value int) string {
	return fmt.Sprintf("%s=%d", label, value)
}
