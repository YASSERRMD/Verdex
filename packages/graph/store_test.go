package graph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func testNode(id, caseID string, nodeType irac.NodeType) irac.Node {
	return irac.Node{
		ID:         id,
		Type:       nodeType,
		CaseID:     caseID,
		Text:       id + " text",
		CreatedAt:  time.Now(),
		Confidence: 0.75,
		Provenance: irac.Provenance{GeneratedBy: "test", GeneratedAt: time.Now()},
	}
}

func TestInMemoryGraphStore_CreateAndGetNode(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := store.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := store.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Text != n.Text {
		t.Fatalf("expected text %q, got %q", n.Text, got.Text)
	}
}

func TestInMemoryGraphStore_CreateNode_Upsert(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := store.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	n.Text = "updated text"
	if err := store.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode (overwrite): %v", err)
	}

	got, err := store.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Text != "updated text" {
		t.Fatalf("expected upsert to overwrite text, got %q", got.Text)
	}
}

func TestInMemoryGraphStore_GetNode_NotFound(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	_, err := store.GetNode(context.Background(), "missing")
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestInMemoryGraphStore_GetNode_EmptyID(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	_, err := store.GetNode(context.Background(), "")
	if !errors.Is(err, graph.ErrEmptyNodeID) {
		t.Fatalf("expected ErrEmptyNodeID, got %v", err)
	}
}

func TestInMemoryGraphStore_CreateEdge_RequiresExistingNodes(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	err := store.CreateEdge(ctx, irac.Edge{FromID: "missing1", ToID: "missing2", Type: irac.EdgeGoverns})
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestInMemoryGraphStore_CreateEdge_RejectsCrossCase(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	n1 := testNode("n1", "case1", irac.NodeRule)
	n2 := testNode("n2", "case2", irac.NodeIssue)
	if err := store.CreateNode(ctx, n1); err != nil {
		t.Fatalf("CreateNode n1: %v", err)
	}
	if err := store.CreateNode(ctx, n2); err != nil {
		t.Fatalf("CreateNode n2: %v", err)
	}

	err := store.CreateEdge(ctx, irac.Edge{FromID: "n1", ToID: "n2", Type: irac.EdgeGoverns})
	if err == nil {
		t.Fatal("expected error creating a cross-case edge, got nil")
	}
}

func TestInMemoryGraphStore_FullCRUD(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	rule := testNode("rule1", "case1", irac.NodeRule)
	issue := testNode("issue1", "case1", irac.NodeIssue)
	if err := store.CreateNode(ctx, rule); err != nil {
		t.Fatalf("CreateNode rule: %v", err)
	}
	if err := store.CreateNode(ctx, issue); err != nil {
		t.Fatalf("CreateNode issue: %v", err)
	}
	if err := store.CreateEdge(ctx, irac.Edge{FromID: "rule1", ToID: "issue1", Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: "case1"})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if err := store.DeleteTree(ctx, "case1"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	nodesAfter, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: "case1"})
	if err != nil {
		t.Fatalf("Traverse after delete: %v", err)
	}
	if len(nodesAfter) != 0 {
		t.Fatalf("expected 0 nodes after DeleteTree, got %d", len(nodesAfter))
	}

	if _, err := store.GetNode(ctx, "rule1"); !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected rule1 to be gone after DeleteTree, got %v", err)
	}
}

func TestInMemoryGraphStore_Traverse_EmptyCaseID(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	_, err := store.Traverse(context.Background(), graph.TraversalQuery{})
	if !errors.Is(err, graph.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestInMemoryGraphStore_DeleteTree_EmptyCaseID(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	err := store.DeleteTree(context.Background(), "")
	if !errors.Is(err, graph.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestInMemoryGraphStore_DeleteTree_NoNodes_NotAnError(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	if err := store.DeleteTree(context.Background(), "no-such-case"); err != nil {
		t.Fatalf("expected nil error deleting an empty case, got %v", err)
	}
}

func TestInMemoryGraphStore_Traverse_FromNodeID_RespectsMaxDepth(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	fact := testNode("fact1", "case1", irac.NodeFact)
	app := testNode("app1", "case1", irac.NodeApplication)
	conclusion := testNode("conclusion1", "case1", irac.NodeConclusion)

	for _, n := range []irac.Node{fact, app, conclusion} {
		if err := store.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}
	if err := store.CreateEdge(ctx, irac.Edge{FromID: "fact1", ToID: "app1", Type: irac.EdgeSupports}); err != nil {
		t.Fatalf("CreateEdge fact->app: %v", err)
	}
	if err := store.CreateEdge(ctx, irac.Edge{FromID: "conclusion1", ToID: "app1", Type: irac.EdgeConcludesFrom}); err != nil {
		t.Fatalf("CreateEdge conclusion->app: %v", err)
	}

	// Depth 1 from fact1 should only reach app1, not conclusion1 (which
	// is reached by walking outward from conclusion1, not from app1).
	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: "case1", FromNodeID: "fact1", MaxDepth: 1})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	ids := map[string]bool{}
	for _, n := range nodes {
		ids[n.ID] = true
	}
	if !ids["fact1"] || !ids["app1"] {
		t.Fatalf("expected fact1 and app1 reachable within depth 1, got %v", ids)
	}
	if ids["conclusion1"] {
		t.Fatalf("did not expect conclusion1 reachable from fact1 (wrong edge direction), got %v", ids)
	}
}
