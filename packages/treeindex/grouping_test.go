package treeindex_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// TestIndexer_RuleGroupedIssues_Correctness seeds two issues governed by
// the same rule, plus one issue governed by a different rule, and asserts
// RebuildCase + LookupPaths group them correctly: the shared rule's Path
// contains both issues it governs; the other rule's Path contains only
// its own issue.
func TestIndexer_RuleGroupedIssues_Correctness(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	caseID := "case-grouping"
	now := time.Now()

	sharedRule := irac.NewRuleNode("rule-shared", caseID, "shared rule text", "us-ny", "common_law", now, 0.9, testProvenance())
	issueA := irac.NewIssueNode("issue-a", caseID, "issue A", now, 0.9, testProvenance())
	issueB := irac.NewIssueNode("issue-b", caseID, "issue B", now, 0.9, testProvenance())

	otherRule := irac.NewRuleNode("rule-other", caseID, "other rule text", "us-ny", "common_law", now, 0.9, testProvenance())
	issueC := irac.NewIssueNode("issue-c", caseID, "issue C", now, 0.9, testProvenance())

	for _, n := range []irac.Node{sharedRule.Node, issueA.Node, issueB.Node, otherRule.Node, issueC.Node} {
		mustCreateNode(t, store, n)
	}
	mustCreateEdge(t, store, irac.Edge{FromID: sharedRule.ID, ToID: issueA.ID, Type: irac.EdgeGoverns})
	mustCreateEdge(t, store, irac.Edge{FromID: sharedRule.ID, ToID: issueB.ID, Type: irac.EdgeGoverns})
	mustCreateEdge(t, store, irac.Edge{FromID: otherRule.ID, ToID: issueC.ID, Type: irac.EdgeGoverns})

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	sharedPaths, err := idx.LookupPaths(ctx, caseID, sharedRule.ID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths(sharedRule): %v", err)
	}
	if len(sharedPaths) != 1 {
		t.Fatalf("expected 1 path rooted at the shared rule, got %d", len(sharedPaths))
	}
	if got := len(sharedPaths[0].Nodes); got != 3 {
		t.Fatalf("expected 3 nodes (rule + 2 issues) in the shared rule's path, got %d", got)
	}
	governedIDs := map[string]bool{}
	for _, n := range sharedPaths[0].Nodes[1:] {
		governedIDs[n.ID] = true
	}
	if !governedIDs[issueA.ID] || !governedIDs[issueB.ID] {
		t.Errorf("expected the shared rule's path to govern both issue-a and issue-b, got %+v", sharedPaths[0].Nodes)
	}

	otherPaths, err := idx.LookupPaths(ctx, caseID, otherRule.ID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths(otherRule): %v", err)
	}
	if len(otherPaths) != 1 || len(otherPaths[0].Nodes) != 2 {
		t.Fatalf("expected otherRule's path to govern exactly issue-c, got %+v", otherPaths)
	}
	if otherPaths[0].Nodes[1].ID != issueC.ID {
		t.Errorf("expected issue-c in otherRule's path, got %+v", otherPaths[0].Nodes[1])
	}
}
