package secondpartyagent_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/citation"
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
const testCaseID = "case-secondpartyagent"

// newTestUser builds a minimal *identity.User holding roles, mirroring
// packages/firstpartyagent's own test helper convention.
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
// VectorStore for testCaseID, mirroring packages/firstpartyagent's own
// test fixture composition exactly.
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
	api = api.WithCitationResolver(fakeResolver)

	return &fixture{caseID: caseID, inner: inner, store: store, api: api}
}

// fakeResolver is a citation.Resolver that resolves every node into a
// deterministic, "verifiable" citation, so citation resolution in tests
// exercises the real citation.Verify anti-hallucination path (a node ID
// that does not exist in the tree still fails verification) without
// depending on any real external citation source.
func fakeResolver(_ context.Context, node irac.Node) (citation.ResolvedCitation, error) {
	return citation.ResolvedCitation{
		Text:      "Fake Reporter " + node.ID,
		Origin:    citation.OriginStatute,
		Certainty: citation.CertaintyExact,
	}, nil
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

// seedFact creates a FactNode directly on the fixture's underlying store.
func (f *fixture) seedFact(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewFactNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed fact %s: %v", id, err)
	}
}

// seedApplication creates an ApplicationNode directly on the fixture's
// underlying store.
func (f *fixture) seedApplication(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewApplicationNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed application %s: %v", id, err)
	}
}

// seedGoverns links a RuleNode to the IssueNode it governs
// (Rule --governs--> Issue).
func (f *fixture) seedGoverns(t *testing.T, ruleID, issueID string) {
	t.Helper()
	f.seedEdge(t, ruleID, issueID, irac.EdgeGoverns)
}

// seedAppliesToRule links an ApplicationNode to the RuleNode it applies
// (Application --applies_to--> Rule).
func (f *fixture) seedAppliesToRule(t *testing.T, applicationID, ruleID string) {
	t.Helper()
	f.seedEdge(t, applicationID, ruleID, irac.EdgeAppliesTo)
}

// seedSupports links a FactNode to the ApplicationNode it supports
// (Fact --supports--> Application).
func (f *fixture) seedSupports(t *testing.T, factID, applicationID string) {
	t.Helper()
	f.seedEdge(t, factID, applicationID, irac.EdgeSupports)
}

func (f *fixture) seedEdge(t *testing.T, fromID, toID string, edgeType irac.EdgeType) {
	t.Helper()
	edge := irac.Edge{FromID: fromID, ToID: toID, Type: edgeType}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed edge %s-%s->%s: %v", edgeType, fromID, toID, err)
	}
}

// newTestRouter builds a *router.Router backed by p (or a
// provider.DefaultNoOpProvider if p is nil), routed via a FallbackChain so
// no explicit TaskRoutes configuration is required, mirroring
// packages/firstpartyagent's own test helper.
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
// fake completion, mirroring packages/firstpartyagent's own test helper.
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
