package synthesisagent_test

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
const testCaseID = "case-synthesisagent"

// newTestUser builds a minimal *identity.User holding roles, mirroring
// packages/issueagent's own test helper convention.
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
// VectorStore for testCaseID, mirroring packages/issueagent's and
// packages/firstpartyagent's own test fixture composition exactly.
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

// seedFact creates a FactNode directly on the fixture's underlying store.
func (f *fixture) seedFact(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewFactNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed fact %s: %v", id, err)
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

// seedApplication creates an ApplicationNode directly on the fixture's
// underlying store, with upstream provenance linking it to ruleID and
// factIDs so provider.go's upstream-application matching can be
// exercised.
func (f *fixture) seedApplication(t *testing.T, id, text, ruleID string, factIDs []string, confidence float64) {
	t.Helper()
	upstream := append([]string{ruleID}, factIDs...)
	node := irac.NewApplicationNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{
		GeneratedBy:     "test-fixture",
		GeneratedAt:     time.Now(),
		UpstreamNodeIDs: upstream,
	})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed application %s: %v", id, err)
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

// seedAppliesTo links an ApplicationNode to a Rule or Fact it applies to
// (Application --applies_to--> Rule/Fact, see irac.EdgeAppliesTo).
func (f *fixture) seedAppliesTo(t *testing.T, applicationID, targetID string) {
	t.Helper()
	edge := irac.Edge{FromID: applicationID, ToID: targetID, Type: irac.EdgeAppliesTo}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed applies_to %s->%s: %v", applicationID, targetID, err)
	}
}

// seedSupports links a FactNode to the ApplicationNode it supports
// (Fact --supports--> Application, see irac.EdgeSupports).
func (f *fixture) seedSupports(t *testing.T, factID, applicationID string) {
	t.Helper()
	edge := irac.Edge{FromID: factID, ToID: applicationID, Type: irac.EdgeSupports}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed supports %s->%s: %v", factID, applicationID, err)
	}
}

// newTestRouter builds a *router.Router backed by p (or a
// provider.DefaultNoOpProvider if p is nil), mirroring
// packages/issueagent's own test helper exactly.
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
// fixed Content string for every Chat call, mirroring
// packages/issueagent's own test helper exactly.
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
