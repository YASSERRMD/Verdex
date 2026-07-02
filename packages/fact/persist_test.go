package fact_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/fact"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func buildTestFact(t *testing.T, id, caseID, text string) irac.FactNode {
	t.Helper()
	node, err := fact.BuildFactNode(
		evidence.Classification{SegmentID: id + "-seg", Confidence: 0.7},
		text,
		fact.SourceSpan{Start: 0, End: len(text)},
		id,
		caseID,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("failed to build fixture fact node: %v", err)
	}
	return node
}

func TestPersistFacts_RoundTripsViaGetNode(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	facts := []irac.FactNode{
		buildTestFact(t, "fact-1", "case-1", "The contract was signed on March 15."),
		buildTestFact(t, "fact-2", "case-1", "The invoice was never paid."),
	}

	persisted, err := fact.PersistFacts(context.Background(), store, facts, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected 2 persisted nodes, got %d", len(persisted))
	}

	for _, want := range persisted {
		got, err := store.GetNode(context.Background(), want.ID)
		if err != nil {
			t.Fatalf("GetNode(%q) failed: %v", want.ID, err)
		}
		if got.Text != want.Text || got.CaseID != want.CaseID || got.Type != irac.NodeFact {
			t.Errorf("round-tripped node mismatch: got %+v want %+v", got, want.Node)
		}
	}
}

func TestPersistFacts_CreatesSupportsEdgeToExistingApplication(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	createdAt := time.Now()

	application := irac.NewApplicationNode("app-1", "case-1", "Applying the breach rule to the facts.", createdAt, 0.9, irac.Provenance{GeneratedBy: "test", GeneratedAt: createdAt})
	if err := store.CreateNode(context.Background(), application.Node); err != nil {
		t.Fatalf("failed to seed application node: %v", err)
	}

	facts := []irac.FactNode{buildTestFact(t, "fact-1", "case-1", "The contract was signed.")}
	supports := map[string][]string{"fact-1": {"app-1"}}

	_, err := fact.PersistFacts(context.Background(), store, facts, []string{"app-1"}, supports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes, err := store.Traverse(context.Background(), graph.TraversalQuery{CaseID: "case-1"})
	if err != nil {
		t.Fatalf("Traverse failed: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (application + fact) in case, got %d", len(nodes))
	}
}

func TestPersistFacts_SkipsEdgeToUnknownApplication(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	facts := []irac.FactNode{buildTestFact(t, "fact-1", "case-1", "The contract was signed.")}
	supports := map[string][]string{"fact-1": {"app-does-not-exist"}}

	persisted, err := fact.PersistFacts(context.Background(), store, facts, []string{"app-real"}, supports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("expected fact node to still persist, got %d", len(persisted))
	}
}

func TestPersistFacts_FailurePropagatesErrPersistFailed(t *testing.T) {
	facts := []irac.FactNode{{Node: irac.Node{ID: "", CaseID: "case-1", Type: irac.NodeFact}}}

	_, err := fact.PersistFacts(context.Background(), graph.NewInMemoryGraphStore(), facts, nil, nil)
	if !errors.Is(err, fact.ErrPersistFailed) {
		t.Fatalf("expected ErrPersistFailed for empty node id, got %v", err)
	}
}
