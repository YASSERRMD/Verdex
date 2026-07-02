package hybridretrieval_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// testProvenance builds a minimal irac.Provenance, mirroring the
// convention used by packages/traversal's own test helpers.
func testProvenance() irac.Provenance {
	return irac.Provenance{GeneratedBy: "hybridretrieval-test", GeneratedAt: time.Now()}
}

// testSpan returns a single valid irac.SourceSpan.
func testSpan() irac.SourceSpan {
	return irac.SourceSpan{Start: 0, End: 10}
}

// mustCreateNode persists node into store, failing the test on error.
func mustCreateNode(t *testing.T, store graph.GraphStore, node irac.Node) {
	t.Helper()
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode(%s): %v", node.ID, err)
	}
}

// mustCreateEdge persists edge into store, failing the test on error.
func mustCreateEdge(t *testing.T, store graph.GraphStore, edge irac.Edge) {
	t.Helper()
	if err := store.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("CreateEdge(%s -> %s): %v", edge.FromID, edge.ToID, err)
	}
}

// mustUpsertVector upserts record into store, failing the test on error.
func mustUpsertVector(t *testing.T, store vectorindex.VectorStore, record vectorindex.VectorRecord) {
	t.Helper()
	if err := store.Upsert(context.Background(), record); err != nil {
		t.Fatalf("Upsert(%s): %v", record.ID, err)
	}
}

// testVectorDims is the embedding dimensionality every fixture in this
// package's tests uses. A single fixed dimensionality keeps every
// InMemoryVectorStore in this test suite mutually comparable without
// tripping ErrDimensionMismatch.
const testVectorDims = 4

// unitVector returns a testVectorDims-length vector with a 1.0 at index 0
// and noise elsewhere, so two calls with different noise values are
// semantically close (same dominant direction) but not identical.
// Deterministic for a given noise value so tests are reproducible.
func unitVector(noise float64) embedding.EmbeddingVector {
	v := make(embedding.EmbeddingVector, testVectorDims)
	for i := range v {
		v[i] = noise
	}
	v[0] = 1.0
	return v
}

// testCaseID is the case ID every fixture in this package's tests uses.
const testCaseID = "case-1"

// seedCaseTree builds a small IRAC reasoning tree in store for testCaseID:
// one Issue, one Rule governing it, and two Facts. Returns every node's
// ID. Mirrors packages/traversal's seedCleanTree fixture shape, trimmed to
// what this package's tests need (no Application/Conclusion, since hybrid
// retrieval's expansion hops only exercise ViaGoverningRule in these
// tests).
func seedCaseTree(t *testing.T, store graph.GraphStore) (issueID, ruleID, fact1ID, fact2ID string) {
	t.Helper()
	now := time.Now()
	caseID := testCaseID

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Was the contract breached?", now, 0.9, testProvenance(), testSpan())
	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "A contract is breached when a party fails to perform.", "us-ny", "common_law", now, 0.9, testProvenance(), testSpan())
	fact1 := irac.NewFactNode(caseID+"-fact-1", caseID, "The seller did not deliver the goods.", now, 0.9, testProvenance(), testSpan())
	fact2 := irac.NewFactNode(caseID+"-fact-2", caseID, "The buyer paid on time.", now, 0.9, testProvenance(), testSpan())

	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, rule.Node)
	mustCreateNode(t, store, fact1.Node)
	mustCreateNode(t, store, fact2.Node)

	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})

	return issue.ID, rule.ID, fact1.ID, fact2.ID
}

// seedSecondGoverningRule adds a second RuleNode governing issueID within
// testCaseID, distinct from the tree's primary rule. Useful for tests
// that need a node reachable via ViaGoverningRule (which filters to
// irac.NodeRule) but deliberately never indexed into the vector store, so
// it is forced to be graph-only.
func seedSecondGoverningRule(t *testing.T, store graph.GraphStore, issueID string) (ruleID string) {
	t.Helper()
	now := time.Now()
	caseID := testCaseID
	rule := irac.NewRuleNode(caseID+"-rule-2", caseID, "A second governing rule.", "us-ny", "common_law", now, 0.9, testProvenance(), testSpan())
	mustCreateNode(t, store, rule.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issueID, Type: irac.EdgeGoverns})
	return rule.ID
}
