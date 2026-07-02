package issueagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// newInternalFixture builds a KnowledgeAPI fixture for whitebox tests
// within the issueagent package itself (as opposed to helpers_test.go's
// fixture, which lives in the issueagent_test external test package).
func newInternalFixture(t *testing.T) (caseID string, inner *graph.InMemoryGraphStore, api *knowledgeapi.KnowledgeAPI) {
	t.Helper()

	caseID = "case-fetch-internal"
	inner = graph.NewInMemoryGraphStore()
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
	api, err = knowledgeapi.NewKnowledgeAPI(caseID, store, vectors, indexer, retriever)
	if err != nil {
		t.Fatalf("NewKnowledgeAPI: %v", err)
	}
	return caseID, inner, api
}

func internalAuthedContext() context.Context {
	return identity.WithUser(context.Background(), &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     []identity.Role{identity.RoleAdvocate},
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}

func TestFetchIssueContexts_NoIssues_ReturnsErrNoIssueNodes(t *testing.T) {
	caseID, _, api := newInternalFixture(t)
	_, err := fetchIssueContexts(internalAuthedContext(), api, caseID)
	if !errors.Is(err, ErrNoIssueNodes) {
		t.Fatalf("fetchIssueContexts() error = %v, want ErrNoIssueNodes", err)
	}
}

func TestFetchIssueContexts_ResolvesGoverningRules(t *testing.T) {
	caseID, inner, api := newInternalFixture(t)
	ctx := internalAuthedContext()

	issue := irac.NewIssueNode("issue-1", caseID, "Was notice given?", time.Now(), 0.8, irac.Provenance{})
	if err := inner.CreateNode(ctx, issue.Node); err != nil {
		t.Fatalf("CreateNode issue: %v", err)
	}
	rule := irac.NewRuleNode("rule-1", caseID, "Notice must be given within 30 days.", "US-NY", "common_law", time.Now(), 0.9, irac.Provenance{})
	if err := inner.CreateNode(ctx, rule.Node); err != nil {
		t.Fatalf("CreateNode rule: %v", err)
	}
	if err := inner.CreateEdge(ctx, irac.Edge{FromID: "rule-1", ToID: "issue-1", Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	contexts, err := fetchIssueContexts(ctx, api, caseID)
	if err != nil {
		t.Fatalf("fetchIssueContexts: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("len(contexts) = %d, want 1", len(contexts))
	}
	if contexts[0].Node.ID != "issue-1" {
		t.Fatalf("contexts[0].Node.ID = %q, want issue-1", contexts[0].Node.ID)
	}
	if len(contexts[0].GoverningRule) != 1 || contexts[0].GoverningRule[0].ID != "rule-1" {
		t.Fatalf("contexts[0].GoverningRule = %+v, want [rule-1]", contexts[0].GoverningRule)
	}
}

func TestFetchIssueContexts_IssueWithoutRule_HasEmptyGoverningRule(t *testing.T) {
	caseID, inner, api := newInternalFixture(t)
	ctx := internalAuthedContext()

	issue := irac.NewIssueNode("issue-lonely", caseID, "An issue with no rule linked.", time.Now(), 0.5, irac.Provenance{})
	if err := inner.CreateNode(ctx, issue.Node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	contexts, err := fetchIssueContexts(ctx, api, caseID)
	if err != nil {
		t.Fatalf("fetchIssueContexts: %v", err)
	}
	if len(contexts) != 1 {
		t.Fatalf("len(contexts) = %d, want 1", len(contexts))
	}
	if len(contexts[0].GoverningRule) != 0 {
		t.Fatalf("GoverningRule = %+v, want empty", contexts[0].GoverningRule)
	}
}
