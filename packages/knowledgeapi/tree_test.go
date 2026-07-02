package knowledgeapi_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestGetTree_ReturnsSeededNodesAndEdges proves GetTree delegates to the
// underlying case-scoped store and correctly converts irac.Node/irac.Edge
// into this package's own NodeDTO/EdgeDTO wire types (a DTO round-trip
// through the real store, not a hand-built fixture).
func TestGetTree_ReturnsSeededNodesAndEdges(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	now := time.Now().UTC().Truncate(time.Second)

	issue := irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a", Text: "Was notice given?", CreatedAt: now, Confidence: 0.9}
	rule := irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a", Text: "Notice must be in writing.", CreatedAt: now, Confidence: 1.0}
	f.seedNode(t, issue)
	f.seedNode(t, rule)
	f.seedEdge(t, irac.Edge{FromID: "rule-1", ToID: "issue-1", Type: irac.EdgeGoverns})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a"})
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}

	if resp.Version != knowledgeapi.APIVersionV1 {
		t.Errorf("expected version %q, got %q", knowledgeapi.APIVersionV1, resp.Version)
	}
	if resp.CaseID != "case-a" {
		t.Errorf("expected case-a, got %q", resp.CaseID)
	}
	if len(resp.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %+v", len(resp.Nodes), resp.Nodes)
	}
	if len(resp.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %+v", len(resp.Edges), resp.Edges)
	}
	if resp.Edges[0].FromID != "rule-1" || resp.Edges[0].ToID != "issue-1" {
		t.Errorf("unexpected edge: %+v", resp.Edges[0])
	}
	if resp.Meta.Total != 2 {
		t.Errorf("expected meta.Total 2, got %d", resp.Meta.Total)
	}

	var gotIssue, gotRule bool
	for _, n := range resp.Nodes {
		if n.Version != knowledgeapi.APIVersionV1 {
			t.Errorf("node %q: expected version %q, got %q", n.ID, knowledgeapi.APIVersionV1, n.Version)
		}
		switch n.ID {
		case "issue-1":
			gotIssue = true
			if n.Text != issue.Text || n.Type != string(irac.NodeIssue) {
				t.Errorf("issue node mismatch: %+v", n)
			}
		case "rule-1":
			gotRule = true
		}
	}
	if !gotIssue || !gotRule {
		t.Fatalf("missing expected nodes in response: %+v", resp.Nodes)
	}
}

// TestGetTree_NodeTypeFilter proves the NodeTypeFilter request field is
// forwarded to the underlying graph.TraversalQuery.
func TestGetTree_NodeTypeFilter(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	now := time.Now()
	f.seedNode(t, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a", CreatedAt: now})
	f.seedNode(t, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a", CreatedAt: now})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a", NodeTypeFilter: string(irac.NodeFact)})
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}

	if len(resp.Nodes) != 1 || resp.Nodes[0].ID != "fact-1" {
		t.Fatalf("expected only fact-1, got %+v", resp.Nodes)
	}
}

// TestGetTree_WrongCaseID_Rejected proves a request naming a different
// case than the KnowledgeAPI instance is scoped to is rejected
// structurally, before ever touching the store.
func TestGetTree_WrongCaseID_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-b"})
	if err != knowledgeapi.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID for mismatched case, got %v", err)
	}
}

// TestGetNode_DelegatesToStore proves GetNode delegates to the
// case-scoped store's GetNode and converts the result to a NodeDTO.
func TestGetNode_DelegatesToStore(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a", Text: "The lease began in March.", Confidence: 0.8})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.GetNode(ctx, knowledgeapi.GetNodeRequest{CaseID: "case-a", NodeID: "fact-1"})
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if resp.Node.ID != "fact-1" || resp.Node.Text != "The lease began in March." {
		t.Fatalf("unexpected node: %+v", resp.Node)
	}
}

// TestGetNode_EmptyNodeID_Rejected proves the request-shape validation
// path is exercised before any store call.
func TestGetNode_EmptyNodeID_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.GetNode(ctx, knowledgeapi.GetNodeRequest{CaseID: "case-a"})
	if err != knowledgeapi.ErrEmptyNodeID {
		t.Fatalf("expected ErrEmptyNodeID, got %v", err)
	}
}

// TestLookupPaths_DelegatesToIndexer proves LookupPaths surfaces
// treeindex's rule-grouped-issue path materialization through this
// package's own PathDTO shape.
func TestLookupPaths_DelegatesToIndexer(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	now := time.Now()
	f.seedNode(t, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a", CreatedAt: now})
	f.seedNode(t, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a", CreatedAt: now})
	f.seedEdge(t, irac.Edge{FromID: "rule-1", ToID: "issue-1", Type: irac.EdgeGoverns})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{
		CaseID:     "case-a",
		FromNodeID: "rule-1",
		EdgeType:   string(irac.EdgeGoverns),
	})
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}
	if len(resp.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(resp.Paths), resp.Paths)
	}
	if resp.Paths[0].Root != "rule-1" {
		t.Errorf("expected root rule-1, got %q", resp.Paths[0].Root)
	}
}
