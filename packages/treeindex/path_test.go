package treeindex_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// fanOutPath builds a small tree-shaped Path matching the reasoning-chain
// fan-out shape: Issue(0) -> Rule(1) -> Application(2) -> {Fact(3),
// Fact(4), Conclusion(5)}, all three tail nodes hopping from index 2.
func fanOutPath() treeindex.Path {
	return treeindex.Path{
		Kind:   treeindex.PathKindReasoningChain,
		CaseID: "case-1",
		Nodes: []treeindex.NodeRef{
			{ID: "issue-1", Type: irac.NodeIssue},
			{ID: "rule-1", Type: irac.NodeRule},
			{ID: "app-1", Type: irac.NodeApplication},
			{ID: "fact-1", Type: irac.NodeFact},
			{ID: "fact-2", Type: irac.NodeFact},
			{ID: "concl-1", Type: irac.NodeConclusion},
		},
		Hops: []treeindex.Hop{
			{FromIndex: 0, EdgeType: irac.EdgeGoverns, Reverse: true},
			{FromIndex: 1, EdgeType: irac.EdgeAppliesTo, Reverse: true},
			{FromIndex: 2, EdgeType: irac.EdgeSupports, Reverse: true},
			{FromIndex: 2, EdgeType: irac.EdgeSupports, Reverse: true},
			{FromIndex: 2, EdgeType: irac.EdgeConcludesFrom, Reverse: true},
		},
	}
}

func TestPath_RootID(t *testing.T) {
	p := fanOutPath()
	if got := p.RootID(); got != "issue-1" {
		t.Errorf("RootID() = %q, want %q", got, "issue-1")
	}
	if got := (treeindex.Path{}).RootID(); got != "" {
		t.Errorf("RootID() of empty path = %q, want empty", got)
	}
}

func TestPath_Depth(t *testing.T) {
	p := fanOutPath()
	// issue(0) -> rule(1) -> app(2) -> fact/concl(3) is 3 hops deep.
	if got := p.Depth(); got != 3 {
		t.Errorf("Depth() = %d, want 3", got)
	}
	if got := (treeindex.Path{}).Depth(); got != 0 {
		t.Errorf("Depth() of empty path = %d, want 0", got)
	}
}

func TestPath_Truncate(t *testing.T) {
	p := fanOutPath()

	t.Run("zero means unbounded", func(t *testing.T) {
		got := p.Truncate(0)
		if len(got.Nodes) != len(p.Nodes) {
			t.Errorf("Truncate(0) changed node count: got %d, want %d", len(got.Nodes), len(p.Nodes))
		}
	})

	t.Run("depth greater than path depth is a no-op", func(t *testing.T) {
		got := p.Truncate(99)
		if len(got.Nodes) != len(p.Nodes) {
			t.Errorf("Truncate(99) changed node count: got %d, want %d", len(got.Nodes), len(p.Nodes))
		}
	})

	t.Run("depth 1 keeps only the root and its direct hop", func(t *testing.T) {
		got := p.Truncate(1)
		if len(got.Nodes) != 2 {
			t.Fatalf("expected 2 nodes at depth 1, got %d: %+v", len(got.Nodes), got.Nodes)
		}
		if got.Nodes[0].ID != "issue-1" || got.Nodes[1].ID != "rule-1" {
			t.Errorf("unexpected nodes at depth 1: %+v", got.Nodes)
		}
		if len(got.Hops) != 1 || got.Hops[0].FromIndex != 0 {
			t.Errorf("unexpected hops at depth 1: %+v", got.Hops)
		}
	})

	t.Run("depth 2 stops before the fan-out", func(t *testing.T) {
		got := p.Truncate(2)
		if len(got.Nodes) != 3 {
			t.Fatalf("expected 3 nodes at depth 2, got %d: %+v", len(got.Nodes), got.Nodes)
		}
		if got.Nodes[2].ID != "app-1" {
			t.Errorf("expected app-1 as the deepest node at depth 2, got %+v", got.Nodes[2])
		}
	})

	t.Run("depth 3 includes the full fan-out with rewritten indices", func(t *testing.T) {
		got := p.Truncate(3)
		if len(got.Nodes) != 6 {
			t.Fatalf("expected all 6 nodes at depth 3, got %d", len(got.Nodes))
		}
		for _, hop := range got.Hops {
			if hop.FromIndex < 0 || hop.FromIndex >= len(got.Nodes) {
				t.Errorf("hop FromIndex %d out of range for %d nodes", hop.FromIndex, len(got.Nodes))
			}
		}
	})
}

func TestPathKind_IsValid(t *testing.T) {
	for _, k := range []treeindex.PathKind{treeindex.PathKindRuleGroupedIssues, treeindex.PathKindReasoningChain} {
		if !k.IsValid() {
			t.Errorf("expected %q to be valid", k)
		}
	}
	if treeindex.PathKind("bogus").IsValid() {
		t.Error("expected an unrecognized path kind to be invalid")
	}
}
