package issueagent_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/provider"
	"github.com/YASSERRMD/verdex/packages/router"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// testCaseID is the case ID used by every test fixture in this package.
const testCaseID = "case-issueagent"

// newTestUser builds a minimal *identity.User holding roles, mirroring
// packages/agentframework's own test helper convention.
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

// authedContext returns a context carrying an authenticated user holding
// every permission a KnowledgeAPI method needs.
func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

// fixture wires a full KnowledgeAPI over an in-memory GraphStore and
// VectorStore for testCaseID, mirroring packages/knowledgeapi's and
// packages/agentframework's own test fixture composition (graph.
// NewInMemoryGraphStore + knowledgeisolation wrappers + treeindex.
// NewIndexer + hybridretrieval.NewRetriever).
type fixture struct {
	caseID string
	inner  *graph.InMemoryGraphStore
	store  *knowledgeisolation.CaseScopedStore
	api    *knowledgeapi.KnowledgeAPI
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	caseID := testCaseID
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

	return &fixture{caseID: caseID, inner: inner, store: store, api: api}
}

// seedIssue creates an IssueNode directly on the fixture's underlying
// store.
func (f *fixture) seedIssue(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewIssueNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed issue %s: %v", id, err)
	}
}

// seedRule creates a RuleNode directly on the fixture's underlying store.
func (f *fixture) seedRule(t *testing.T, id, text, jurisdictionCode, legalFamily string, confidence float64) {
	t.Helper()
	node := irac.NewRuleNode(id, f.caseID, text, jurisdictionCode, legalFamily, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed rule %s: %v", id, err)
	}
}

// seedGoverns links a RuleNode to the IssueNode it governs
// (Rule --governs--> Issue, see irac.EdgeGoverns).
func (f *fixture) seedGoverns(t *testing.T, ruleID, issueID string) {
	t.Helper()
	edge := irac.Edge{FromID: ruleID, ToID: issueID, Type: irac.EdgeGoverns}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed governs %s->%s: %v", ruleID, issueID, err)
	}
}

// newTestRouter builds a *router.Router backed by p (or a
// provider.DefaultNoOpProvider if p is nil), routed via a FallbackChain so
// no explicit TaskRoutes configuration is required, mirroring
// packages/agentframework's own test helper.
func newTestRouter(t *testing.T, p provider.LLMProvider) *router.Router {
	t.Helper()

	if p == nil {
		p = provider.DefaultNoOpProvider()
	}

	registry := provider.NewRegistry()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}

	r, err := router.NewRouter(router.RouterConfig{
		Registry: registry,
		Policy: router.RoutingPolicy{
			FallbackChain: []string{p.ID()},
		},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

// fakeProvider is a fake provider.LLMProvider returning a caller-supplied
// fixed Content string for every Chat call, so tests can exercise this
// package's JSON-parsing code (parse.go) against a realistic structured
// fake completion rather than provider.NoOpProvider's canned response.
type fakeProvider struct {
	provider.NoOpProvider
	content string
	err     error
}

func (f *fakeProvider) ID() string { return "fake" }

func (f *fakeProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &provider.ChatResponse{
		ID:           "fake-1",
		Content:      f.content,
		FinishReason: "stop",
	}, nil
}
