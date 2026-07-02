package traversal_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

func TestWalker_ViaGoverningRule_SingleHop(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-governs"
	issueID, ruleID, _, _, _ := seedCleanTree(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).ViaGoverningRule()
	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	if got := path.StartID(); got != issueID {
		t.Errorf("expected start id %q, got %q", issueID, got)
	}
	if got := path.EndID(); got != ruleID {
		t.Errorf("expected end id %q, got %q", ruleID, got)
	}
	if path.Depth() != 1 {
		t.Errorf("expected depth 1, got %d", path.Depth())
	}
	if len(path.Hops) != 1 || path.Hops[0].EdgeType != irac.EdgeGoverns || path.Hops[0].Direction != traversal.Reverse {
		t.Errorf("expected a single reverse EdgeGoverns hop, got %+v", path.Hops)
	}
}

func TestWalker_MultiHopChain_IssueToApplication(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-chain"
	issueID, ruleID, factID, appID, _ := seedCleanTree(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).
		ViaGoverningRule().
		Via(irac.EdgeAppliesTo, traversal.Reverse, irac.NodeApplication).
		Via(irac.EdgeSupports, traversal.Reverse, irac.NodeFact)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	wantIDs := []string{issueID, ruleID, appID, factID}
	if len(path.Nodes) != len(wantIDs) {
		t.Fatalf("expected %d nodes, got %d: %+v", len(wantIDs), len(path.Nodes), path.Nodes)
	}
	for i, want := range wantIDs {
		if path.Nodes[i].ID != want {
			t.Errorf("node %d: expected %q, got %q", i, want, path.Nodes[i].ID)
		}
	}
}

// TestWalker_MultiHopChain_IssueToConclusion exercises the other branch
// from the Application node (EdgeConcludesFrom, reverse), reaching the
// Conclusion rather than a supporting Fact, and uses conclusionID/factID
// from seedCleanTree's full return set to assert both branches are
// distinct and correctly reachable from Application.
func TestWalker_MultiHopChain_IssueToConclusion(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-chain-conclusion"
	issueID, ruleID, factID, appID, conclusionID := seedCleanTree(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).
		ViaGoverningRule().
		Via(irac.EdgeAppliesTo, traversal.Reverse, irac.NodeApplication).
		Via(irac.EdgeConcludesFrom, traversal.Reverse, irac.NodeConclusion)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	wantIDs := []string{issueID, ruleID, appID, conclusionID}
	if len(path.Nodes) != len(wantIDs) {
		t.Fatalf("expected %d nodes, got %d: %+v", len(wantIDs), len(path.Nodes), path.Nodes)
	}
	for i, want := range wantIDs {
		if path.Nodes[i].ID != want {
			t.Errorf("node %d: expected %q, got %q", i, want, path.Nodes[i].ID)
		}
	}
	if path.EndID() == factID {
		t.Errorf("expected the conclusion branch, not the fact branch, to be reached")
	}
}

func TestWalker_FanOut_TwoIssuesOneRule(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-fanout"
	_, _, ruleID := seedTwoIssuesOneRule(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	// From the rule, applies_to is not relevant; use the custom Via to
	// walk EdgeGoverns Forward (Rule -> Issue) to find both issues.
	query := traversal.NewQuery(caseID, ruleID).Via(irac.EdgeGoverns, traversal.Forward, irac.NodeIssue)
	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 2 {
		t.Fatalf("expected 2 paths (one per issue), got %d: %+v", len(result.Paths), result.Paths)
	}
}

func TestWalker_MaxDepth_Truncates(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-depth"
	issueID, _, _, appID, _ := seedCleanTree(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	fullQuery := traversal.NewQuery(caseID, issueID).
		ViaGoverningRule().
		Via(irac.EdgeAppliesTo, traversal.Reverse, irac.NodeApplication)

	boundedQuery := fullQuery.WithMaxDepth(1)
	result, err := walker.Execute(context.Background(), boundedQuery)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Truncated {
		t.Errorf("expected Truncated to be true when MaxDepth < len(Hops)")
	}
	for _, p := range result.Paths {
		if p.Depth() > 1 {
			t.Errorf("expected every path depth <= 1, got %d: %+v", p.Depth(), p)
		}
		if p.EndID() == appID {
			t.Errorf("expected the application node to be unreachable at depth 1, but found it in %+v", p)
		}
	}

	fullResult, err := walker.Execute(context.Background(), fullQuery)
	if err != nil {
		t.Fatalf("Execute (full): %v", err)
	}
	if fullResult.Truncated {
		t.Errorf("expected Truncated to be false when MaxDepth covers the full Hops sequence")
	}
}

func TestWalker_CycleGuard_NoRevisit(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-cycle"
	now := time.Now()

	// Build a small cycle: rule -> issue (governs), and issue -> rule via
	// a custom edge type reuse to simulate a back-reference an
	// adversarial or buggy upstream producer might create. We use two
	// distinct legal edges walked in opposite directions from the same
	// pair to exercise the visited-set guard without needing an illegal
	// edge triple.
	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Issue", now, 0.9, testProvenance(), testSpan())
	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "Rule", "us-ny", "common_law", now, 0.9, testProvenance(), testSpan())
	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, rule.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	// A query that would walk Governs Reverse then Forward would, without
	// a cycle guard, bounce issue -> rule -> issue -> rule indefinitely.
	// Bound it generously and confirm no path revisits a node.
	query := traversal.NewQuery(caseID, issue.ID).
		ViaGoverningRule().
		Via(irac.EdgeGoverns, traversal.Forward, irac.NodeIssue).
		Via(irac.EdgeGoverns, traversal.Reverse, irac.NodeRule)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, p := range result.Paths {
		seen := make(map[string]bool)
		for _, n := range p.Nodes {
			if seen[n.ID] {
				t.Fatalf("path revisits node %q: %+v", n.ID, p)
			}
			seen[n.ID] = true
		}
	}
}

func TestWalker_StartNodeNotFound(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery("case-1", "does-not-exist").ViaGoverningRule()
	_, err = walker.Execute(context.Background(), query)
	if err == nil {
		t.Fatalf("expected an error for a nonexistent start node")
	}
}

func TestNewWalker_NilStore(t *testing.T) {
	_, err := traversal.NewWalker(nil)
	if err != traversal.ErrNilGraphStore {
		t.Fatalf("expected ErrNilGraphStore, got %v", err)
	}
}

func TestWalker_ZeroHopFrontier_StopsCleanly(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-leaf"
	now := time.Now()
	// A single issue with no governing rule: ViaGoverningRule should
	// yield zero paths beyond the (unreachable) hop, not an error.
	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Lonely issue", now, 0.9, testProvenance(), testSpan())
	mustCreateNode(t, store, issue.Node)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issue.ID).ViaGoverningRule()
	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected a single-node path when nothing is reachable, got %d: %+v", len(result.Paths), result.Paths)
	}
	if result.Paths[0].Depth() != 0 {
		t.Errorf("expected depth 0, got %d", result.Paths[0].Depth())
	}
	if result.Truncated {
		t.Errorf("expected Truncated to be false when the walk ends naturally, not by hitting MaxDepth")
	}
}
