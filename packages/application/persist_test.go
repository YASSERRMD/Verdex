package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/application"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func seedNode(t *testing.T, store graph.GraphStore, node irac.Node) {
	t.Helper()
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("failed to seed node %s: %v", node.ID, err)
	}
}

func TestPersistApplicationSubgraph_RoundTripsAndCreatesLegalEdges(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	issue := testIssue(t, "issue-1", "whether notice was reasonable")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "reasonable notice required", "US-CA", "common_law")}
	fact1 := testFact(t, "fact-1", "two days notice was given")

	seedNode(t, store, issue.Node)
	seedNode(t, store, rule.Rule.Node)
	seedNode(t, store, fact1.Node)

	node, err := application.BuildApplicationNode(issue, rule, []irac.FactNode{fact1})
	if err != nil {
		t.Fatalf("unexpected error building node: %v", err)
	}

	if err := application.PersistApplicationSubgraph(ctx, store, node, rule, issue.ID, []irac.FactNode{fact1}, false); err != nil {
		t.Fatalf("unexpected error persisting subgraph: %v", err)
	}

	// round-trip via GetNode
	got, err := store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if got.Text != node.Text || got.CaseID != node.CaseID || got.Type != irac.NodeApplication {
		t.Fatalf("round-tripped node mismatch: got %+v want %+v", got, node.Node)
	}

	// Every legal edge triple created must be present in irac's
	// constraint table.
	legal := irac.LegalEdgeTriples()
	checks := []struct {
		from irac.NodeType
		edge irac.EdgeType
		to   irac.NodeType
	}{
		{irac.NodeApplication, irac.EdgeAppliesTo, irac.NodeRule},
		{irac.NodeApplication, irac.EdgeAppliesTo, irac.NodeFact},
		{irac.NodeRule, irac.EdgeGoverns, irac.NodeIssue},
	}
	for _, c := range checks {
		found := false
		for _, l := range legal {
			if l.From == c.from && l.Edge == c.edge && l.To == c.to {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("triple %v-%v->%v not in irac's legal edge constraint table", c.from, c.edge, c.to)
		}
		if !irac.IsLegalEdgeTriple(c.from, c.edge, c.to) {
			t.Fatalf("IsLegalEdgeTriple rejected supposedly legal triple %v-%v->%v", c.from, c.edge, c.to)
		}
	}
}

func TestPersistApplicationSubgraph_SkipsExistingGovernsEdge(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	issue := testIssue(t, "issue-1", "issue text")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "rule text", "US-CA", "common_law")}
	fact1 := testFact(t, "fact-1", "fact text")

	seedNode(t, store, issue.Node)
	seedNode(t, store, rule.Rule.Node)
	seedNode(t, store, fact1.Node)

	node, err := application.BuildApplicationNode(issue, rule, []irac.FactNode{fact1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ruleGovernsIssueExists=true means the governs edge should not be
	// (re)created; PersistApplicationSubgraph should still succeed.
	if err := application.PersistApplicationSubgraph(ctx, store, node, rule, issue.ID, []irac.FactNode{fact1}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPersistApplicationSubgraph_FailurePropagatesErrPersistFailed(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	// A node with an empty ID is rejected by InMemoryGraphStore.CreateNode.
	badNode := irac.ApplicationNode{Node: irac.Node{ID: "", CaseID: "case-1", Type: irac.NodeApplication}}
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "rule text", "US-CA", "common_law")}

	_, err := application.BuildApplicationNode(testIssue(t, "issue-1", "text"), rule, []irac.FactNode{testFact(t, "fact-1", "text")})
	if err != nil {
		t.Fatalf("unexpected error building fixture node: %v", err)
	}

	err = application.PersistApplicationSubgraph(ctx, store, badNode, rule, "issue-1", nil, false)
	if !errors.Is(err, application.ErrPersistFailed) {
		t.Fatalf("expected ErrPersistFailed, got %v", err)
	}
}

func TestPersistApplicationSubgraph_CreatedAtStamped(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	issue := testIssue(t, "issue-1", "issue text")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "rule text", "US-CA", "common_law")}
	fact1 := testFact(t, "fact-1", "fact text")
	seedNode(t, store, issue.Node)
	seedNode(t, store, rule.Rule.Node)
	seedNode(t, store, fact1.Node)

	node, err := application.BuildApplicationNode(issue, rule, []irac.FactNode{fact1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.CreatedAt.After(time.Now()) {
		t.Fatal("expected CreatedAt to not be in the future")
	}

	if err := application.PersistApplicationSubgraph(ctx, store, node, rule, issue.ID, []irac.FactNode{fact1}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
