package traversal_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

func TestWalker_ViaDistinguishingFacts_ResolvesViaCallback(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-distinguish"
	now := time.Now()

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Does the precedent control?", now, 0.9, testProvenance(), testSpan())
	precedentRule := irac.NewRuleNode(caseID+"-precedent-1", caseID, "Smith v. Jones", "us-ny", "common_law", now, 0.85, testProvenance(), testSpan())
	distinguishingFact := irac.NewFactNode(caseID+"-fact-1", caseID, "Here the agreement was oral, not written.", now, 0.9, testProvenance(), testSpan())

	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, precedentRule.Node)
	mustCreateNode(t, store, distinguishingFact.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: precedentRule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})

	resolver := func(_ context.Context, gotCaseID string, rule irac.RuleNode) ([]irac.FactNode, error) {
		if gotCaseID != caseID || rule.ID != precedentRule.ID {
			return nil, nil
		}
		return []irac.FactNode{distinguishingFact}, nil
	}

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issue.ID).
		ViaGoverningRule().
		ViaDistinguishingFacts().
		WithDistinguishingFactResolver(resolver)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	if got := path.EndID(); got != distinguishingFact.ID {
		t.Errorf("expected path to end at the distinguishing fact %q, got %q", distinguishingFact.ID, got)
	}
	if path.Hops[len(path.Hops)-1].Kind != traversal.HopKindDistinguishingFacts {
		t.Errorf("expected last hop to be HopKindDistinguishingFacts, got %q", path.Hops[len(path.Hops)-1].Kind)
	}
}

func TestNoDistinguishingFacts_ReturnsEmpty(t *testing.T) {
	facts, err := traversal.NoDistinguishingFacts(context.Background(), "case-1", irac.RuleNode{})
	if err != nil {
		t.Fatalf("NoDistinguishingFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected no facts, got %d", len(facts))
	}
}

// TestWalker_FullLegalReasoningChain exercises the plan's headline
// scenario end to end: issue -> governing rule -> controlling precedent
// -> distinguishing facts, all in one Query.
func TestWalker_FullLegalReasoningChain(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-full-chain"
	now := time.Now()

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Was performance excused?", now, 0.9, testProvenance(), testSpan())
	statuteRule := irac.NewRuleNode(caseID+"-statute-1", caseID, "Performance may be excused by impossibility.", "us-ny", "civil_law", now, 0.9, testProvenance(), testSpan())
	precedentRule := irac.NewRuleNode(caseID+"-precedent-1", caseID, "Taylor v. Caldwell", "uk", "common_law", now, 0.85, testProvenance(), testSpan())
	distinguishingFact := irac.NewFactNode(caseID+"-fact-1", caseID, "Here, performance was merely more expensive, not impossible.", now, 0.9, testProvenance(), testSpan())

	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, statuteRule.Node)
	mustCreateNode(t, store, precedentRule.Node)
	mustCreateNode(t, store, distinguishingFact.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: statuteRule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})

	precedentResolver := func(_ context.Context, rule irac.RuleNode) ([]irac.RuleNode, error) {
		if rule.ID != statuteRule.ID {
			return nil, nil
		}
		return []irac.RuleNode{precedentRule}, nil
	}
	factResolver := func(_ context.Context, _ string, rule irac.RuleNode) ([]irac.FactNode, error) {
		if rule.ID != precedentRule.ID {
			return nil, nil
		}
		return []irac.FactNode{distinguishingFact}, nil
	}

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	query := traversal.NewQuery(caseID, issue.ID).
		ViaGoverningRule().
		ViaControllingPrecedent().
		ViaDistinguishingFacts().
		WithPrecedentResolver(precedentResolver).
		WithDistinguishingFactResolver(factResolver)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %+v", len(result.Paths), result.Paths)
	}
	path := result.Paths[0]
	wantIDs := []string{issue.ID, statuteRule.ID, precedentRule.ID, distinguishingFact.ID}
	if len(path.Nodes) != len(wantIDs) {
		t.Fatalf("expected %d nodes, got %d: %+v", len(wantIDs), len(path.Nodes), path.Nodes)
	}
	for i, want := range wantIDs {
		if path.Nodes[i].ID != want {
			t.Errorf("node %d: expected %q, got %q", i, want, path.Nodes[i].ID)
		}
	}

	t.Logf("explained path: %s", path.Explain())
}
