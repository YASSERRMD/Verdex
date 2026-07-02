package graph_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestExportImport_RoundTripsLosslessly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()

	rule := testNode("rule1", "case1", irac.NodeRule)
	issue := testNode("issue1", "case1", irac.NodeIssue)
	fact := testNode("fact1", "case1", irac.NodeFact)
	app := testNode("app1", "case1", irac.NodeApplication)
	conclusion := testNode("conclusion1", "case1", irac.NodeConclusion)

	for _, n := range []irac.Node{rule, issue, fact, app, conclusion} {
		if err := store.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}

	edges := []irac.Edge{
		{FromID: "rule1", ToID: "issue1", Type: irac.EdgeGoverns},
		{FromID: "app1", ToID: "fact1", Type: irac.EdgeAppliesTo},
		{FromID: "app1", ToID: "rule1", Type: irac.EdgeAppliesTo},
		{FromID: "fact1", ToID: "app1", Type: irac.EdgeSupports},
		{FromID: "conclusion1", ToID: "app1", Type: irac.EdgeConcludesFrom},
	}
	for _, e := range edges {
		if err := store.CreateEdge(ctx, e); err != nil {
			t.Fatalf("CreateEdge %s->%s: %v", e.FromID, e.ToID, err)
		}
	}

	data, err := graph.Export(ctx, store, "case1")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	restored := graph.NewInMemoryGraphStore()
	if err := graph.Import(ctx, restored, data); err != nil {
		t.Fatalf("Import: %v", err)
	}

	restoredNodes, err := restored.Traverse(ctx, graph.TraversalQuery{CaseID: "case1"})
	if err != nil {
		t.Fatalf("Traverse restored: %v", err)
	}
	if len(restoredNodes) != 5 {
		t.Fatalf("expected 5 restored nodes, got %d", len(restoredNodes))
	}

	for _, want := range []irac.Node{rule, issue, fact, app, conclusion} {
		got, err := restored.GetNode(ctx, want.ID)
		if err != nil {
			t.Fatalf("GetNode %s after restore: %v", want.ID, err)
		}
		if got.Text != want.Text {
			t.Fatalf("node %s: expected text %q, got %q", want.ID, want.Text, got.Text)
		}
		if got.Type != want.Type {
			t.Fatalf("node %s: expected type %q, got %q", want.ID, want.Type, got.Type)
		}
		if got.CaseID != want.CaseID {
			t.Fatalf("node %s: expected case id %q, got %q", want.ID, want.CaseID, got.CaseID)
		}
	}

	restoredEdges := restored.EdgesForCase("case1")
	if len(restoredEdges) != len(edges) {
		t.Fatalf("expected %d restored edges, got %d", len(edges), len(restoredEdges))
	}
}

func TestExport_EmptyCaseID(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	_, err := graph.Export(context.Background(), store, "")
	if err == nil {
		t.Fatal("expected error for empty case id, got nil")
	}
}

func TestExport_NoNodes_ProducesEmptyButValidEnvelope(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	data, err := graph.Export(context.Background(), store, "no-such-case")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	restored := graph.NewInMemoryGraphStore()
	if err := graph.Import(context.Background(), restored, data); err != nil {
		t.Fatalf("Import: %v", err)
	}

	nodes, err := restored.Traverse(context.Background(), graph.TraversalQuery{CaseID: "no-such-case"})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestImport_NilStore(t *testing.T) {
	t.Parallel()

	if err := graph.Import(context.Background(), nil, []byte(`{}`)); err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

func TestImport_InvalidJSON(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	if err := graph.Import(context.Background(), store, []byte(`not json`)); err == nil {
		t.Fatal("expected error decoding invalid JSON, got nil")
	}
}
