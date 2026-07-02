package graph

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestInMemoryIndex_AddRemoveLookup(t *testing.T) {
	t.Parallel()

	idx := newInMemoryIndex()
	idx.addType("issue", "n1")
	idx.addType("issue", "n2")
	idx.addType("rule", "n3")

	issues := idx.nodeIDsByType("issue")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issue-typed ids, got %d: %v", len(issues), issues)
	}

	idx.removeType("issue", "n1")
	issuesAfterRemove := idx.nodeIDsByType("issue")
	if len(issuesAfterRemove) != 1 || issuesAfterRemove[0] != "n2" {
		t.Fatalf("expected only n2 to remain under 'issue', got %v", issuesAfterRemove)
	}

	rules := idx.nodeIDsByType("rule")
	if len(rules) != 1 || rules[0] != "n3" {
		t.Fatalf("expected [n3] under 'rule', got %v", rules)
	}
}

func TestInMemoryIndex_UnknownType(t *testing.T) {
	t.Parallel()

	idx := newInMemoryIndex()
	got := idx.nodeIDsByType("does-not-exist")
	if len(got) != 0 {
		t.Fatalf("expected empty result for unknown type, got %v", got)
	}
}

// TestInMemoryGraphStore_Traverse_UsesTypeIndex verifies Traverse's
// NodeType filter, backed by the typeIndex secondary index: creating
// nodes of several types in one case and filtering by one NodeType
// returns exactly that type's nodes.
func TestInMemoryGraphStore_Traverse_UsesTypeIndex(t *testing.T) {
	t.Parallel()

	s := NewInMemoryGraphStore()
	ctx := context.Background()

	issue := irac.Node{ID: "issue1", Type: irac.NodeIssue, CaseID: "case1", CreatedAt: time.Now()}
	rule := irac.Node{ID: "rule1", Type: irac.NodeRule, CaseID: "case1", CreatedAt: time.Now()}
	fact := irac.Node{ID: "fact1", Type: irac.NodeFact, CaseID: "case1", CreatedAt: time.Now()}

	for _, n := range []irac.Node{issue, rule, fact} {
		if err := s.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}

	nodes, err := s.Traverse(ctx, TraversalQuery{CaseID: "case1", NodeType: irac.NodeRule})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "rule1" {
		t.Fatalf("expected only rule1 filtered by NodeType, got %+v", nodes)
	}
}

// TestInMemoryGraphStore_Traverse_TypeIndexUpdatedOnRetype verifies the
// typeIndex is kept in sync when CreateNode overwrites an existing node
// with a different NodeType (an edge case in the upsert path).
func TestInMemoryGraphStore_Traverse_TypeIndexUpdatedOnRetype(t *testing.T) {
	t.Parallel()

	s := NewInMemoryGraphStore()
	ctx := context.Background()

	n := irac.Node{ID: "n1", Type: irac.NodeFact, CaseID: "case1", CreatedAt: time.Now()}
	if err := s.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Re-create with a different NodeType (fact -> issue). Not a
	// realistic real-world transition, but exercises the retype path.
	n.Type = irac.NodeIssue
	if err := s.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode (retype): %v", err)
	}

	factNodes, err := s.Traverse(ctx, TraversalQuery{CaseID: "case1", NodeType: irac.NodeFact})
	if err != nil {
		t.Fatalf("Traverse fact: %v", err)
	}
	if len(factNodes) != 0 {
		t.Fatalf("expected n1 no longer indexed under NodeFact after retype, got %+v", factNodes)
	}

	issueNodes, err := s.Traverse(ctx, TraversalQuery{CaseID: "case1", NodeType: irac.NodeIssue})
	if err != nil {
		t.Fatalf("Traverse issue: %v", err)
	}
	if len(issueNodes) != 1 || issueNodes[0].ID != "n1" {
		t.Fatalf("expected n1 indexed under NodeIssue after retype, got %+v", issueNodes)
	}
}
