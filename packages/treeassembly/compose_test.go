package treeassembly

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestComposeTree_EmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		input AssemblyInput
	}{
		{"empty case id", AssemblyInput{CaseID: "", Issues: []irac.IssueNode{{}}}},
		{"no nodes at all", AssemblyInput{CaseID: "case-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ComposeTree(context.Background(), tt.input, nil)
			if !errors.Is(err, ErrEmptyInput) {
				t.Fatalf("expected ErrEmptyInput, got %v", err)
			}
		})
	}
}

func TestComposeTree_ProducesStructurallyValidTree(t *testing.T) {
	input := syntheticInput("case-1")

	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantNodes := len(input.Issues) + len(input.Rules) + len(input.Facts) + len(input.Applications)
	if len(tree.Nodes) != wantNodes {
		t.Fatalf("expected %d nodes, got %d", wantNodes, len(tree.Nodes))
	}

	issues := irac.ValidateTree(tree.Nodes, tree.Edges)
	if len(issues) != 0 {
		t.Fatalf("expected structurally valid tree, got issues: %v", issues)
	}

	// Expect: Rule--governs-->Issue, Application--applies_to-->Rule,
	// Application--applies_to-->Fact, Fact--supports-->Application.
	wantEdgeTypes := map[irac.EdgeType]int{
		irac.EdgeGoverns:   1,
		irac.EdgeAppliesTo: 2,
		irac.EdgeSupports:  1,
	}
	gotEdgeTypes := map[irac.EdgeType]int{}
	for _, e := range tree.Edges {
		gotEdgeTypes[e.Type]++
	}
	for et, want := range wantEdgeTypes {
		if gotEdgeTypes[et] != want {
			t.Errorf("edge type %q: got %d, want %d", et, gotEdgeTypes[et], want)
		}
	}
}

func TestComposeTree_NoOpConclusionProviderByDefault(t *testing.T) {
	input := syntheticInput("case-1")

	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range tree.Nodes {
		if n.GetType() == irac.NodeConclusion {
			t.Fatalf("expected no conclusion nodes with default provider, found %q", n.GetID())
		}
	}
}

func TestComposeTree_WithConclusionProvider(t *testing.T) {
	input := syntheticInput("case-1")

	conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "Draft analysis: breach likely occurred.", input.Applications[0].CreatedAt, 0.8, testProvenance(input.Applications[0].ID))
	provider := fixedConclusionProvider{conclusions: []irac.ConclusionNode{conclusion}}

	tree, err := ComposeTree(context.Background(), input, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundConclusion := false
	foundEdge := false
	for _, n := range tree.Nodes {
		if n.GetID() == conclusion.ID {
			foundConclusion = true
		}
	}
	for _, e := range tree.Edges {
		if e.Type == irac.EdgeConcludesFrom && e.FromID == conclusion.ID && e.ToID == input.Applications[0].ID {
			foundEdge = true
		}
	}
	if !foundConclusion {
		t.Error("expected conclusion node to be included in tree")
	}
	if !foundEdge {
		t.Error("expected concludes_from edge from conclusion to application")
	}

	issues := irac.ValidateTree(tree.Nodes, tree.Edges)
	if len(issues) != 0 {
		t.Fatalf("expected structurally valid tree, got issues: %v", issues)
	}
}

func TestComposeTree_ConclusionProviderError(t *testing.T) {
	input := syntheticInput("case-1")
	wantErr := errors.New("synthesis failed")
	provider := fixedConclusionProvider{err: wantErr}

	_, err := ComposeTree(context.Background(), input, provider)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestNoOpConclusionProvider_ReturnsEmptySlice(t *testing.T) {
	p := NoOpConclusionProvider{}
	conclusions, err := p.Provide(context.Background(), AssemblyInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conclusions) != 0 {
		t.Fatalf("expected empty slice, got %d conclusions", len(conclusions))
	}
}
