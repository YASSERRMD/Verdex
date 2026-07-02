package reasoningorchestration_test

import (
	"context"
	"fmt"
	"sync"
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
const testCaseID = "case-reasoningorchestration"

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

func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

// fixture wires a full KnowledgeAPI over an in-memory GraphStore and
// VectorStore for testCaseID, mirroring every Part-5 package's own test
// fixture composition exactly (see e.g.
// packages/firstpartyagent/helpers_test.go).
type fixture struct {
	caseID string
	inner  *graph.InMemoryGraphStore
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

	return &fixture{caseID: caseID, inner: inner, api: api}
}

// fakeResolver resolves every node into a deterministic, verifiable
// citation, mirroring packages/firstpartyagent/helpers_test.go's own
// fakeResolver exactly.
func fakeResolver(_ context.Context, node irac.Node) (citation.ResolvedCitation, error) {
	return citation.ResolvedCitation{
		Text:      "Fake Reporter " + node.ID,
		Origin:    citation.OriginStatute,
		Certainty: citation.CertaintyExact,
	}, nil
}

func (f *fixture) seedIssue(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewIssueNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed issue %s: %v", id, err)
	}
}

func (f *fixture) seedRule(t *testing.T, id, text, jurisdictionCode, legalFamily string, confidence float64) {
	t.Helper()
	node := irac.NewRuleNode(id, f.caseID, text, jurisdictionCode, legalFamily, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed rule %s: %v", id, err)
	}
}

func (f *fixture) seedFact(t *testing.T, id, text string, confidence float64) {
	t.Helper()
	node := irac.NewFactNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	if err := f.inner.CreateNode(context.Background(), node.Node); err != nil {
		t.Fatalf("seed fact %s: %v", id, err)
	}
}

func (f *fixture) seedGoverns(t *testing.T, ruleID, issueID string) {
	t.Helper()
	f.seedEdge(t, ruleID, issueID, irac.EdgeGoverns)
}

func (f *fixture) seedEdge(t *testing.T, fromID, toID string, edgeType irac.EdgeType) {
	t.Helper()
	edge := irac.Edge{FromID: fromID, ToID: toID, Type: edgeType}
	if err := f.inner.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("seed edge %s-%s->%s: %v", edgeType, fromID, toID, err)
	}
}

// newTestRouter builds a *router.Router backed by p, routed via a
// FallbackChain so no explicit TaskRoutes configuration is required,
// mirroring every Part-5 package's own test helper.
func newTestRouter(t *testing.T, p provider.LLMProvider) *router.Router {
	t.Helper()

	registry := provider.NewRegistry()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}

	r, err := router.NewRouter(router.RouterConfig{
		Registry: registry,
		Policy:   router.RoutingPolicy{FallbackChain: []string{p.ID()}},
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

// sequencedProvider is a fake provider.LLMProvider that returns one
// fixed response per call, advancing through responses in call order.
// This models the pipeline's real call sequence: StageIssueFraming,
// StageFirstPartyArguments, StageSecondPartyArguments, and StageSynthesis
// are each single-step agents (BuildRequest+Interpret concludes on the
// first model turn — see e.g. issueagent.Agent's own doc comment), so
// across one full Run they call the Router's Chat method exactly once
// each, in exactly this order. A call past the end of responses returns
// errNoMoreResponses so a test bug (an unexpected extra call) fails
// loudly instead of silently reusing the last response.
type sequencedProvider struct {
	provider.NoOpProvider
	mu        sync.Mutex
	responses []string
	errs      []error
	calls     int
}

func (p *sequencedProvider) ID() string { return "sequenced-fake" }

func (p *sequencedProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.calls
	p.calls++
	if idx < len(p.errs) && p.errs[idx] != nil {
		return nil, p.errs[idx]
	}
	if idx >= len(p.responses) {
		return nil, errNoMoreResponses
	}
	return &provider.ChatResponse{
		ID:           fmt.Sprintf("fake-%d", idx),
		Content:      p.responses[idx],
		FinishReason: "stop",
	}, nil
}

var errNoMoreResponses = fmt.Errorf("sequencedProvider: no more responses configured")

// failAtCallProvider wraps a sequencedProvider's responses but forces the
// call at index failAt to return failErr regardless of what response was
// configured there, so a test can inject a mid-pipeline failure at a
// specific stage without needing a bespoke provider per test.
type failAtCallProvider struct {
	*sequencedProvider
	failAt  int
	failErr error
}

func (p *failAtCallProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	p.mu.Lock()
	idx := p.calls
	p.mu.Unlock()

	if idx == p.failAt {
		// Deliberately does NOT advance p.calls: the forced failure must
		// not consume this call index's queued response, so a later
		// retry (e.g. after Resume "fixes" the outage by resetting
		// failAt) replays the same response index rather than skipping
		// past it.
		return nil, p.failErr
	}
	return p.sequencedProvider.Chat(ctx, req)
}

// fakeIssueFramingJSON is a realistic structured completion matching
// issueagent's documented JSON schema, covering one seeded issue.
const fakeIssueFramingJSON = `{
  "framed_issues": [
    {
      "source_issue_node_id": "issue-1",
      "materiality_score": 0.9,
      "governing_questions": ["Was the contract validly formed?"],
      "ambiguities": [],
      "confidence": 0.85
    }
  ]
}`

// fakeFirstPartyArgumentJSON is a realistic structured completion
// matching firstpartyagent's documented JSON schema.
const fakeFirstPartyArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The contract was validly formed in writing.",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "counterarguments": ["The signature was not witnessed."],
      "confidence": 0.8
    }
  ]
}`

// fakeSecondPartyArgumentJSON is a realistic structured completion
// matching secondpartyagent's documented JSON schema.
const fakeSecondPartyArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The contract fails the Statute of Frauds.",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "rebuts_argument_ids": ["issue-1-0"],
      "counterarguments": [],
      "confidence": 0.7
    }
  ]
}`

// fakeSynthesisJSON is a realistic structured completion matching
// synthesisagent's documented JSON schema. Text avoids verdict/directive
// language so it passes guardrail.CheckText at StageGuardrailCheck.
const fakeSynthesisJSON = `{
  "conclusions": [
    {
      "issue_node_id": "issue-1",
      "text": "The record suggests the contract was likely validly formed, though the Statute of Frauds argument raises a genuine question the evidence does not fully resolve.",
      "favored_party": "first-party",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "weakest_link": "fact-1 is not independently corroborated",
      "confidence": 0.65
    }
  ]
}`
