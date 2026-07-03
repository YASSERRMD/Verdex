package casesearch_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/casesearch"
	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// knowledgeAPIFixture wires up a full knowledgeapi.KnowledgeAPI over an
// in-memory GraphStore and VectorStore for a single case, mirroring
// packages/knowledgeapi/knowledgeapi_test.go's testFixture composition
// contract exactly: this proves KnowledgeAPISearcher (this package's
// CaseSearcher reference adapter) genuinely composes with the real
// hybridretrieval/treeindex/vectorindex stack rather than a test double.
type knowledgeAPIFixture struct {
	caseID  string
	inner   *graph.InMemoryGraphStore
	vectors *knowledgeisolation.CaseScopedVectorStore
	api     *knowledgeapi.KnowledgeAPI
}

func newKnowledgeAPIFixture(t *testing.T, caseID string) *knowledgeAPIFixture {
	t.Helper()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, caseID, nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	vectors, err := knowledgeisolation.NewCaseScopedVectorStore(
		vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{}), caseID, nil,
	)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	indexer, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	retriever, err := hybridretrieval.NewRetriever(vectors, store)
	if err != nil {
		t.Fatalf("NewRetriever: %v", err)
	}

	api, err := knowledgeapi.NewKnowledgeAPI(caseID, store, vectors, indexer, retriever)
	if err != nil {
		t.Fatalf("NewKnowledgeAPI: %v", err)
	}

	return &knowledgeAPIFixture{caseID: caseID, inner: inner, vectors: vectors, api: api}
}

func (f *knowledgeAPIFixture) seedNode(t *testing.T, n irac.Node) {
	t.Helper()
	if err := f.inner.CreateNode(context.Background(), n); err != nil {
		t.Fatalf("seed CreateNode(%s): %v", n.ID, err)
	}
}

func (f *knowledgeAPIFixture) seedEdge(t *testing.T, e irac.Edge) {
	t.Helper()
	if err := f.inner.CreateEdge(context.Background(), e); err != nil {
		t.Fatalf("seed CreateEdge(%s->%s): %v", e.FromID, e.ToID, err)
	}
}

// seedVector inserts a vector record directly, bypassing embedding, so
// tests can control exactly which node a query vector is closest to.
func (f *knowledgeAPIFixture) seedVector(t *testing.T, id string, nodeType irac.NodeType, text string, vec embedding.EmbeddingVector) {
	t.Helper()
	err := f.vectors.Upsert(context.Background(), vectorindex.VectorRecord{
		ID:       id,
		NodeType: nodeType,
		CaseID:   f.caseID,
		Text:     text,
		Vector:   vec,
	})
	if err != nil {
		t.Fatalf("seedVector(%s): %v", id, err)
	}
}

func authedContext(roles ...identity.Role) context.Context {
	user := &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return identity.WithUser(context.Background(), user)
}

func TestKnowledgeAPISearcher_SearchSemantic_ComposesRealHybridRetrieval(t *testing.T) {
	f := newKnowledgeAPIFixture(t, "case-a")

	// Two candidate vectors: one very close to the query, one far away.
	f.seedVector(t, "fact-close", irac.NodeFact, "the tenant paid rent on time every month", embedding.EmbeddingVector{1, 0, 0})
	f.seedVector(t, "fact-far", irac.NodeFact, "unrelated criminal procedure text", embedding.EmbeddingVector{0, 1, 0})

	embed := func(_ context.Context, _ string) (embedding.EmbeddingVector, error) {
		return embedding.EmbeddingVector{1, 0, 0}, nil
	}

	searcher, err := casesearch.NewKnowledgeAPISearcher(f.api, embed)
	if err != nil {
		t.Fatalf("NewKnowledgeAPISearcher: %v", err)
	}

	hits, err := searcher.SearchSemantic(authedContext(identity.RoleJudge), "rent payment history", 5)
	if err != nil {
		t.Fatalf("SearchSemantic: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	if hits[0].NodeID != "fact-close" {
		t.Fatalf("expected closest vector match first, got %+v", hits)
	}
}

func TestKnowledgeAPISearcher_SearchSemantic_NoEmbedFunc_ReturnsError(t *testing.T) {
	f := newKnowledgeAPIFixture(t, "case-a")
	searcher, err := casesearch.NewKnowledgeAPISearcher(f.api, nil)
	if err != nil {
		t.Fatalf("NewKnowledgeAPISearcher: %v", err)
	}

	_, err = searcher.SearchSemantic(authedContext(identity.RoleJudge), "query", 5)
	if err != casesearch.ErrNilEmbedFunc {
		t.Fatalf("expected ErrNilEmbedFunc, got %v", err)
	}
}

func TestKnowledgeAPISearcher_SearchKeyword_MatchesNodeText(t *testing.T) {
	f := newKnowledgeAPIFixture(t, "case-a")
	now := time.Now()
	f.seedNode(t, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a", Text: "The lease was breached in March.", CreatedAt: now})
	f.seedNode(t, irac.Node{ID: "fact-2", Type: irac.NodeFact, CaseID: "case-a", Text: "Unrelated fact about parking.", CreatedAt: now})

	searcher, err := casesearch.NewKnowledgeAPISearcher(f.api, nil)
	if err != nil {
		t.Fatalf("NewKnowledgeAPISearcher: %v", err)
	}

	hits, err := searcher.SearchKeyword(authedContext(identity.RoleJudge), "breached", 5)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(hits) != 1 || hits[0].NodeID != "fact-1" {
		t.Fatalf("expected exactly fact-1 to match, got %+v", hits)
	}
}

func TestKnowledgeAPISearcher_SearchIssueOrRule_FindsPathFromRoot(t *testing.T) {
	f := newKnowledgeAPIFixture(t, "case-a")
	now := time.Now()
	f.seedNode(t, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a", Text: "Rule on lease obligations", CreatedAt: now})
	f.seedNode(t, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a", Text: "Was the lease breached?", CreatedAt: now})
	f.seedEdge(t, irac.Edge{FromID: "rule-1", ToID: "issue-1", Type: irac.EdgeGoverns})

	// No explicit index build is needed: KnowledgeAPI.LookupPaths
	// transparently calls treeindex.Indexer.RebuildCase on first use for
	// a case that has never been indexed (see tree.go's doc comment).
	searcher, err := casesearch.NewKnowledgeAPISearcher(f.api, nil)
	if err != nil {
		t.Fatalf("NewKnowledgeAPISearcher: %v", err)
	}

	hits, herr := searcher.SearchIssueOrRule(authedContext(identity.RoleJudge), "rule-1", 5)
	if herr != nil {
		t.Fatalf("SearchIssueOrRule: %v", herr)
	}
	found := false
	for _, h := range hits {
		if h.NodeID == "issue-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issue-1 reachable from rule-1, got %+v", hits)
	}
}

func TestKnowledgeAPISearcher_SearchIssueOrRule_UnknownRoot_ReturnsEmptyNotError(t *testing.T) {
	f := newKnowledgeAPIFixture(t, "case-a")
	searcher, err := casesearch.NewKnowledgeAPISearcher(f.api, nil)
	if err != nil {
		t.Fatalf("NewKnowledgeAPISearcher: %v", err)
	}

	hits, herr := searcher.SearchIssueOrRule(authedContext(identity.RoleJudge), "does-not-exist", 5)
	if herr != nil {
		t.Fatalf("expected no error for an unknown root, got %v", herr)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits, got %+v", hits)
	}
}
