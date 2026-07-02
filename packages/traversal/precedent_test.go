package traversal_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

// seedRuleWithPrecedent builds a case with an Issue governed by a
// statute-origin Rule, plus a separate precedent-origin RuleNode that is
// not connected by any literal irac.Edge (the schema has none for this
// relationship) — resolved instead via a PrecedentResolver in the test.
func seedRuleWithPrecedent(t *testing.T, store graph.GraphStore, caseID string) (issueID, statuteRuleID, precedentRuleID string) {
	t.Helper()
	now := time.Now()

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Was the notice adequate?", now, 0.9, testProvenance(), testSpan())
	statuteRule := irac.NewRuleNode(caseID+"-statute-1", caseID, "Notice must be in writing.", "us-ny", "civil_law", now, 0.9, testProvenance(), testSpan())
	precedentRule := irac.NewRuleNode(caseID+"-precedent-1", caseID, "Smith v. Jones: written notice must be signed.", "us-ny", "common_law", now, 0.85, testProvenance(), testSpan())

	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, statuteRule.Node)
	mustCreateNode(t, store, precedentRule.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: statuteRule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})

	return issue.ID, statuteRule.ID, precedentRule.ID
}

func TestWalker_ViaControllingPrecedent_ResolvesViaCallback(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-precedent"
	issueID, statuteRuleID, precedentRuleID := seedRuleWithPrecedent(t, store, caseID)

	precedentNode, err := store.GetNode(context.Background(), precedentRuleID)
	if err != nil {
		t.Fatalf("GetNode(precedent): %v", err)
	}

	resolver := func(_ context.Context, rule irac.RuleNode) ([]irac.RuleNode, error) {
		if rule.ID != statuteRuleID {
			return nil, nil
		}
		return []irac.RuleNode{{Node: precedentNode}}, nil
	}

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).
		ViaGoverningRule().
		ViaControllingPrecedent().
		WithPrecedentResolver(resolver)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	wantIDs := []string{issueID, statuteRuleID, precedentRuleID}
	if len(path.Nodes) != len(wantIDs) {
		t.Fatalf("expected %d nodes, got %d: %+v", len(wantIDs), len(path.Nodes), path.Nodes)
	}
	for i, want := range wantIDs {
		if path.Nodes[i].ID != want {
			t.Errorf("node %d: expected %q, got %q", i, want, path.Nodes[i].ID)
		}
	}
	if path.Hops[1].Kind != traversal.HopKindControllingPrecedent {
		t.Errorf("expected second hop to be HopKindControllingPrecedent, got %q", path.Hops[1].Kind)
	}
}

func TestWalker_ViaControllingPrecedent_NoResolverYieldsNoResults(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-precedent-none"
	issueID, _, _ := seedRuleWithPrecedent(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issueID).ViaGoverningRule().ViaControllingPrecedent()
	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path (issue -> rule, precedent hop yields nothing), got %d: %+v", len(result.Paths), result.Paths)
	}
	if result.Paths[0].Depth() != 1 {
		t.Errorf("expected depth 1 (precedent hop found nothing to add), got %d", result.Paths[0].Depth())
	}
}

func TestNoPrecedents_ReturnsEmpty(t *testing.T) {
	rules, err := traversal.NoPrecedents(context.Background(), irac.RuleNode{})
	if err != nil {
		t.Fatalf("NoPrecedents: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected no rules, got %d", len(rules))
	}
}
