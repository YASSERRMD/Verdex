package agentframework_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
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
const testCaseID = "case-agent-fw"

// newTestUser builds a minimal *identity.User holding roles, mirroring
// packages/knowledgeapi's own test helper convention.
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
// every permission a KnowledgeAPI-backed tool needs.
func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

// newTestKnowledgeAPI wires a full KnowledgeAPI over an in-memory
// GraphStore and VectorStore for testCaseID, seeded with a single Issue
// node, mirroring packages/knowledgeapi's own test fixture composition.
func newTestKnowledgeAPI(t *testing.T) *knowledgeapi.KnowledgeAPI {
	t.Helper()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, testCaseID, nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	vectors, err := knowledgeisolation.NewCaseScopedVectorStore(
		vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{}), testCaseID, nil,
	)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	ctx := authedContext()
	if err := store.CreateNode(ctx, irac.Node{
		ID:         "issue-1",
		Type:       irac.NodeIssue,
		CaseID:     testCaseID,
		Text:       "Was the contract validly formed?",
		CreatedAt:  time.Now(),
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("seed CreateNode: %v", err)
	}

	indexer, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	retriever, err := hybridretrieval.NewRetriever(vectors, store)
	if err != nil {
		t.Fatalf("NewRetriever: %v", err)
	}

	api, err := knowledgeapi.NewKnowledgeAPI(testCaseID, store, vectors, indexer, retriever)
	if err != nil {
		t.Fatalf("NewKnowledgeAPI: %v", err)
	}
	return api
}

// newTestRouter builds a *router.Router backed by a single
// provider.NoOpProvider registered for every task type, routed via a
// FallbackChain so no explicit TaskRoutes configuration is required.
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

// scriptedAgent is a fake agentframework.Agent whose Interpret decisions
// are driven by a caller-supplied sequence, one Decision per step. It
// records every BuildRequest/Interpret call for assertions.
type scriptedAgent struct {
	name     string
	taskType provider.TaskType

	decisions []Decision
	call      int

	buildErr     error
	interpretErr error

	buildCalls     int
	interpretCalls int
}

// Decision aliases agentframework.Decision so test files that don't
// import agentframework's package directly under a different name still
// read naturally; kept local to avoid stutter in call sites below.
type Decision = agentframework.Decision

func (a *scriptedAgent) Name() string                { return a.name }
func (a *scriptedAgent) TaskType() provider.TaskType { return a.taskType }
func (a *scriptedAgent) BuildRequest(_ context.Context, _ *agentframework.Scratchpad) (provider.ChatRequest, error) {
	a.buildCalls++
	if a.buildErr != nil {
		return provider.ChatRequest{}, a.buildErr
	}
	return provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "step"}},
	}, nil
}

func (a *scriptedAgent) Interpret(_ context.Context, _ *agentframework.Scratchpad, _ *provider.ChatResponse) (agentframework.Decision, error) {
	a.interpretCalls++
	if a.interpretErr != nil {
		return agentframework.Decision{}, a.interpretErr
	}
	if a.call >= len(a.decisions) {
		// Default to concluding so a misconfigured script doesn't spin.
		return agentframework.Decision{Conclude: true, FinalText: "done"}, nil
	}
	d := a.decisions[a.call]
	a.call++
	return d, nil
}

// newScriptedAgent returns a scriptedAgent that yields decisions in order.
func newScriptedAgent(name string, decisions ...agentframework.Decision) *scriptedAgent {
	return &scriptedAgent{name: name, taskType: provider.TaskReason, decisions: decisions}
}
