package knowledgeapi_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestRetrieve_StructuralAnchorOnly_DelegatesToRetriever proves Retrieve
// delegates to hybridretrieval.Retriever.Retrieve for a purely structural
// query (an AnchorNodeID with no query Vector), converting the fused
// Result into this package's own DTO shape.
func TestRetrieve_StructuralAnchorOnly_DelegatesToRetriever(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	now := time.Now()
	f.seedNode(t, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a", Text: "Rule text", CreatedAt: now})
	f.seedNode(t, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a", Text: "Issue text", CreatedAt: now})
	f.seedEdge(t, irac.Edge{FromID: "rule-1", ToID: "issue-1", Type: irac.EdgeGoverns})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.Retrieve(ctx, knowledgeapi.RetrieveRequest{
		CaseID:        "case-a",
		AnchorNodeID:  "issue-1",
		ExpansionHops: []string{"governing_rule"},
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if resp.Version != knowledgeapi.APIVersionV1 {
		t.Errorf("expected version %q, got %q", knowledgeapi.APIVersionV1, resp.Version)
	}
	if resp.CaseID != "case-a" {
		t.Errorf("expected case-a, got %q", resp.CaseID)
	}
	// Expanding "governing_rule" from the issue-1 anchor walks
	// Issue --governs(reverse)--> Rule, so rule-1 should surface as a
	// graph-sourced item.
	foundGoverning := false
	for _, item := range resp.Items {
		if item.NodeID == "rule-1" {
			foundGoverning = true
		}
	}
	if !foundGoverning {
		t.Fatalf("expected governing node rule-1 among retrieved items, got %+v", resp.Items)
	}
}

// TestRetrieve_EmptyQuery_Rejected proves a request with neither a Vector
// nor an AnchorNodeID is rejected before calling the underlying
// Retriever.
func TestRetrieve_EmptyQuery_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.Retrieve(ctx, knowledgeapi.RetrieveRequest{CaseID: "case-a"})
	if err != knowledgeapi.ErrEmptyQuery {
		t.Fatalf("expected ErrEmptyQuery, got %v", err)
	}
}

// TestRetrieve_WrongCaseID_Rejected proves a request naming a different
// case than the KnowledgeAPI instance is scoped to is rejected
// structurally.
func TestRetrieve_WrongCaseID_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.Retrieve(ctx, knowledgeapi.RetrieveRequest{CaseID: "case-b", AnchorNodeID: "n1"})
	if err != knowledgeapi.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}
