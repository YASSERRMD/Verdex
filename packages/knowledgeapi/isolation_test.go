package knowledgeapi_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

// TestGetNode_ForeignCaseNode_Rejected is a regression guard on top of
// Phase 047's own knowledgeisolation tests: it proves the cross-case
// isolation guarantee still holds when reached through this facade, not
// just when CaseScopedStore is exercised directly. A KnowledgeAPI scoped
// to "case-a" must never return a node that belongs to "case-b", even
// when directly asked for it by ID.
func TestGetNode_ForeignCaseNode_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "fact-foreign", Type: irac.NodeFact, CaseID: "case-b", Text: "Belongs to case-b."})

	ctx := authedContext(identity.RoleJudge)
	_, err := f.api.GetNode(ctx, knowledgeapi.GetNodeRequest{CaseID: "case-a", NodeID: "fact-foreign"})
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}
}

// TestGetTree_ForeignCaseNodes_NeverAppear proves GetTree's Traverse-based
// listing never surfaces a foreign case's nodes, even when both cases
// share the same underlying GraphStore instance (the realistic deployment
// shape: one store, many cases, isolation enforced entirely by the
// case-scoped wrapper).
func TestGetTree_ForeignCaseNodes_NeverAppear(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "issue-a", Type: irac.NodeIssue, CaseID: "case-a", Text: "Case A's issue."})
	f.seedNode(t, irac.Node{ID: "issue-b", Type: irac.NodeIssue, CaseID: "case-b", Text: "Case B's issue."})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a"})
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}

	if len(resp.Nodes) != 1 || resp.Nodes[0].ID != "issue-a" {
		t.Fatalf("expected only case-a's node, got %+v", resp.Nodes)
	}
}

// TestGetNode_SharedLawNode_ReadableAcrossCases proves shared-law nodes
// (RuleNodes) remain readable regardless of which case they were
// originally attributed to, per knowledgeisolation's documented
// exception — this facade must not accidentally widen or narrow that
// exception.
func TestGetNode_SharedLawNode_ReadableAcrossCases(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "rule-shared", Type: irac.NodeRule, CaseID: "case-b", Text: "Shared statute text."})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.GetNode(ctx, knowledgeapi.GetNodeRequest{CaseID: "case-a", NodeID: "rule-shared"})
	if err != nil {
		t.Fatalf("expected shared-law node to be readable, got %v", err)
	}
	if resp.Node.ID != "rule-shared" {
		t.Fatalf("unexpected node: %+v", resp.Node)
	}
}

// TestLookupPaths_NeverSurfacesForeignCasePaths proves treeindex-backed
// LookupPaths, reached through this facade, only ever materializes paths
// for the case the KnowledgeAPI instance is scoped to, even though the
// underlying store holds another case's nodes/edges too.
func TestLookupPaths_NeverSurfacesForeignCasePaths(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{ID: "rule-a", Type: irac.NodeRule, CaseID: "case-a"})
	f.seedNode(t, irac.Node{ID: "issue-a", Type: irac.NodeIssue, CaseID: "case-a"})
	f.seedEdge(t, irac.Edge{FromID: "rule-a", ToID: "issue-a", Type: irac.EdgeGoverns})

	// A second case's identically-shaped tree in the same underlying
	// store, which must never be reachable from case-a's KnowledgeAPI.
	f.seedNode(t, irac.Node{ID: "rule-b", Type: irac.NodeRule, CaseID: "case-b"})
	f.seedNode(t, irac.Node{ID: "issue-b", Type: irac.NodeIssue, CaseID: "case-b"})
	f.seedEdge(t, irac.Edge{FromID: "rule-b", ToID: "issue-b", Type: irac.EdgeGoverns})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{
		CaseID:     "case-a",
		FromNodeID: "rule-a",
		EdgeType:   string(irac.EdgeGoverns),
	})
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}
	for _, p := range resp.Paths {
		for _, n := range p.Nodes {
			if n.CaseID == "case-b" {
				t.Fatalf("case-b node leaked into case-a's LookupPaths result: %+v", p)
			}
		}
	}

	// Looking up paths rooted at a foreign case's node ID must never
	// return that foreign case's data: case-a's index was built without
	// ever reading case-b's nodes/edges, so a root ID belonging to
	// case-b simply matches nothing.
	foreignResp, err := f.api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{
		CaseID:     "case-a",
		FromNodeID: "rule-b",
		EdgeType:   string(irac.EdgeGoverns),
	})
	if err != nil {
		t.Fatalf("LookupPaths for foreign root: %v", err)
	}
	if len(foreignResp.Paths) != 0 {
		t.Fatalf("expected no paths rooted at a foreign case's node, got %+v", foreignResp.Paths)
	}
}
