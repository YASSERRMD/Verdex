package knowledgeapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"

	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
)

// newTestUser builds a minimal *identity.User holding roles, mirroring
// packages/identity's own test helper convention (see
// packages/identity/middleware_test.go's newTestUser).
func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// authedContext returns a context carrying an authenticated user with the
// given roles, as identity.AuthMiddleware would have set it up.
func authedContext(roles ...identity.Role) context.Context {
	return identity.WithUser(context.Background(), newTestUser(roles...))
}

// testFixtureCaseID is the case every newTestFixture call is scoped to.
// Every test in this package that needs a second, foreign case (for
// isolation regression tests) seeds that other case's data directly on
// the fixture's shared inner store rather than constructing a second
// KnowledgeAPI, so a single fixture case ID is sufficient across the
// whole suite.
const testFixtureCaseID = "case-a"

// testFixture wires up a full KnowledgeAPI over an in-memory GraphStore
// and VectorStore for a single case, mirroring the composition contract
// documented on NewKnowledgeAPI: the same case-scoped store/vector-store
// pair backs the indexer and the retriever.
type testFixture struct {
	caseID string
	inner  *graph.InMemoryGraphStore
	store  *knowledgeisolation.CaseScopedStore
	api    *knowledgeapi.KnowledgeAPI
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()

	caseID := testFixtureCaseID
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

	return &testFixture{caseID: caseID, inner: inner, store: store, api: api}
}

// seedNode creates a node directly on the fixture's underlying store
// (bypassing case-scoping checks, since seeding a foreign-case node is a
// deliberate part of some isolation tests).
func (f *testFixture) seedNode(t *testing.T, n irac.Node) {
	t.Helper()
	if err := f.inner.CreateNode(context.Background(), n); err != nil {
		t.Fatalf("seed CreateNode(%s): %v", n.ID, err)
	}
}

// seedEdge creates an edge directly on the fixture's underlying store.
func (f *testFixture) seedEdge(t *testing.T, e irac.Edge) {
	t.Helper()
	if err := f.inner.CreateEdge(context.Background(), e); err != nil {
		t.Fatalf("seed CreateEdge(%s->%s): %v", e.FromID, e.ToID, err)
	}
}
