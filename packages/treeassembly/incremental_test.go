package treeassembly

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestReassembleIncremental_NilPrev(t *testing.T) {
	_, err := ReassembleIncremental(nil, nil, nil)
	if !errors.Is(err, ErrNilPrevTree) {
		t.Fatalf("expected ErrNilPrevTree, got %v", err)
	}
}

func TestReassembleIncremental_NoChanges_StillBumpsRevision(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := ReassembleIncremental(tree, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tree.Revision.RevisionNumber != tree.Revision.RevisionNumber+1 {
		t.Fatalf("expected revision bump, got %d -> %d", tree.Revision.RevisionNumber, result.Tree.Revision.RevisionNumber)
	}
	if len(result.Tree.Nodes) != len(tree.Nodes) {
		t.Fatalf("expected same node count with no new evidence, got %d vs %d", len(result.Tree.Nodes), len(tree.Nodes))
	}
}

func TestReassembleIncremental_OnlyTouchesDelta(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newFact := irac.NewFactNode("fact-2", input.CaseID, "A second, corroborating fact.", time.Now(), 0.85, testProvenance())
	newApp := irac.NewApplicationNode("app-2", input.CaseID, "A second application using the new fact and existing rule.", time.Now(), 0.85, testProvenance(input.Rules[0].ID, newFact.ID))

	result, err := ReassembleIncremental(tree, []irac.FactNode{newFact}, []irac.ApplicationNode{newApp})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantNodeCount := len(tree.Nodes) + 2
	if len(result.Tree.Nodes) != wantNodeCount {
		t.Fatalf("expected %d nodes, got %d", wantNodeCount, len(result.Tree.Nodes))
	}

	// New edges: app-2 --applies_to--> rule-1, app-2 --applies_to--> fact-2,
	// fact-2 --supports--> app-2.
	wantEdgeCount := len(tree.Edges) + 3
	if len(result.Tree.Edges) != wantEdgeCount {
		t.Fatalf("expected %d edges, got %d", wantEdgeCount, len(result.Tree.Edges))
	}

	// Revision bumped exactly once.
	if result.Tree.Revision.RevisionNumber != tree.Revision.RevisionNumber+1 {
		t.Fatalf("expected single revision bump, got %d -> %d", tree.Revision.RevisionNumber, result.Tree.Revision.RevisionNumber)
	}

	// The full updated tree should still be structurally valid.
	issues := irac.ValidateTree(result.Tree.Nodes, result.Tree.Edges)
	if len(issues) != 0 {
		t.Fatalf("expected valid tree, got issues: %v", issues)
	}

	// The delta-only validation/gap results should not report anything
	// about the pre-existing (already-valid) subgraph.
	if len(result.ValidationIssues) != 0 {
		t.Fatalf("expected no validation issues on delta, got %v", result.ValidationIssues)
	}
}

func TestReassembleIncremental_DuplicateNodesIgnored(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-submitting the same fact should not duplicate it.
	result, err := ReassembleIncremental(tree, input.Facts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tree.Nodes) != len(tree.Nodes) {
		t.Fatalf("expected no duplicate nodes, got %d vs %d", len(result.Tree.Nodes), len(tree.Nodes))
	}
}
