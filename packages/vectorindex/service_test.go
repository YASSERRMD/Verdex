package vectorindex_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func newTestService(t *testing.T) (vectorindex.IndexingService, graph.GraphStore, *vectorindex.InMemoryVectorStore) {
	t.Helper()
	g := graph.NewInMemoryGraphStore()
	v := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	svc := vectorindex.IndexingService{
		Graph:      g,
		Embeddings: newFakeEmbeddingService("contract", "warranty", "tort", "negligence"),
		Vectors:    v,
		ProjectionOptions: vectorindex.ProjectionOptions{
			CategoryCode: vectorindex.CategoryCode("contract"),
		},
	}
	return svc, g, v
}

func TestIndexingService_MissingDependencies(t *testing.T) {
	ctx := context.Background()

	_, err := (vectorindex.IndexingService{}).IndexCase(ctx, "case-1")
	if err != vectorindex.ErrNilGraphStore {
		t.Errorf("expected ErrNilGraphStore, got %v", err)
	}

	svc := vectorindex.IndexingService{Graph: graph.NewInMemoryGraphStore()}
	if _, err := svc.IndexCase(ctx, "case-1"); err != vectorindex.ErrNilEmbeddingService {
		t.Errorf("expected ErrNilEmbeddingService, got %v", err)
	}

	svc.Embeddings = newFakeEmbeddingService()
	if _, err := svc.IndexCase(ctx, "case-1"); err != vectorindex.ErrNilVectorStore {
		t.Errorf("expected ErrNilVectorStore, got %v", err)
	}
}

func TestIndexingService_IndexCase_EmptyCaseID(t *testing.T) {
	svc, _, _ := newTestService(t)
	if _, err := svc.IndexCase(context.Background(), ""); err != vectorindex.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestIndexingService_IndexCase_EndToEnd(t *testing.T) {
	svc, g, v := newTestService(t)
	ctx := context.Background()
	now := time.Now()

	mustCreateNode(t, g, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-1", Text: "Is there a breach of contract?", CreatedAt: now})
	mustCreateNode(t, g, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-1", Text: "A contract requires a warranty of good faith.", CreatedAt: now})
	mustCreateNode(t, g, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-1", Text: "The parties signed a contract with a warranty clause.", CreatedAt: now})
	mustCreateNode(t, g, irac.Node{ID: "app-1", Type: irac.NodeApplication, CaseID: "case-1", Text: "Applying warranty rule to the fact.", CreatedAt: now})
	mustCreateNode(t, g, irac.Node{ID: "conclusion-1", Type: irac.NodeConclusion, CaseID: "case-1", Text: "The warranty was breached.", CreatedAt: now})

	n, err := svc.IndexCase(ctx, "case-1")
	if err != nil {
		t.Fatalf("IndexCase: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 leaves indexed (rule, fact, conclusion), got %d", n)
	}
	if v.Len() != 3 {
		t.Fatalf("expected 3 records in the vector store, got %d", v.Len())
	}

	// A query for "contract warranty" text should recall the rule/fact/
	// conclusion leaves (all mention warranty/contract) ranked above an
	// unrelated tort/negligence leaf seeded from a different case.
	mustCreateNode(t, g, irac.Node{ID: "fact-unrelated", Type: irac.NodeFact, CaseID: "case-2", Text: "The defendant committed negligence in a tort claim.", CreatedAt: now})
	if _, err := svc.IndexCase(ctx, "case-2"); err != nil {
		t.Fatalf("IndexCase(case-2): %v", err)
	}

	queryVec, err := svc.Embeddings.Embed(ctx, []string{"contract warranty dispute"})
	if err != nil {
		t.Fatalf("Embed query: %v", err)
	}

	results, err := v.Query(ctx, vectorindex.QueryRequest{
		Vector: queryVec[0].Vector,
		TopK:   4,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results across both cases, got %d", len(results))
	}
	if results[len(results)-1].Record.ID != "fact-unrelated" {
		t.Errorf("expected the unrelated tort fact to rank last, got %q last", results[len(results)-1].Record.ID)
	}

	// Metadata filter narrows to case-1 only.
	scoped, err := v.Query(ctx, vectorindex.QueryRequest{
		Vector: queryVec[0].Vector,
		TopK:   10,
		CaseID: "case-1",
	})
	if err != nil {
		t.Fatalf("Query scoped: %v", err)
	}
	if len(scoped) != 3 {
		t.Fatalf("expected 3 case-1 results, got %d", len(scoped))
	}
}

func TestReindexOnRevision_EmptyCaseID(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := vectorindex.ReindexOnRevision(context.Background(), svc, irac.TreeRevision{})
	if err != vectorindex.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestReindexOnRevision_RepopulatesAfterNewRevision(t *testing.T) {
	svc, g, v := newTestService(t)
	ctx := context.Background()
	now := time.Now()

	mustCreateNode(t, g, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-1", Text: "Initial fact about the contract.", CreatedAt: now})

	rev1 := irac.NewInitialRevision("case-1", now)
	n, err := vectorindex.ReindexOnRevision(ctx, svc, rev1)
	if err != nil {
		t.Fatalf("ReindexOnRevision (rev1): %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 leaf indexed after rev1, got %d", n)
	}
	if v.Len() != 1 {
		t.Fatalf("expected 1 record after rev1, got %d", v.Len())
	}

	// A new revision adds a fact node; re-indexing should pick it up
	// without requiring a second, separate call for the new node alone.
	mustCreateNode(t, g, irac.Node{ID: "fact-2", Type: irac.NodeFact, CaseID: "case-1", Text: "A second fact added in the new revision.", CreatedAt: now})

	rev2 := irac.NextRevision(rev1, now)
	n, err = vectorindex.ReindexOnRevision(ctx, svc, rev2)
	if err != nil {
		t.Fatalf("ReindexOnRevision (rev2): %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 leaves indexed after rev2, got %d", n)
	}
	if v.Len() != 2 {
		t.Fatalf("expected 2 records in the store after rev2 (upsert, not append), got %d", v.Len())
	}
}
