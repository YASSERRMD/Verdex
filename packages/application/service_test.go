package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/application"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestApplicationService_ApplyRules_FullPipeline(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &application.ApplicationService{Store: store}
	ctx := context.Background()

	issue := testIssue(t, "issue-1", "whether the landlord gave reasonable notice before eviction")
	statuteRule := application.OriginatedRule{
		Rule:   testRule(t, "rule-statute", "a landlord must give reasonable notice before eviction", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}
	precedentRule := application.OriginatedRule{
		Rule:   testRule(t, "rule-precedent", "reasonable notice for eviction was previously held to be 30 days", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}
	fact1 := testFact(t, "fact-1", "the landlord gave two days notice before eviction")

	seedNode(t, store, issue.Node)
	seedNode(t, store, statuteRule.Rule.Node)
	seedNode(t, store, precedentRule.Rule.Node)
	seedNode(t, store, fact1.Node)

	req := application.ApplyRequest{
		Issue:          issue,
		Rules:          []application.OriginatedRule{statuteRule, precedentRule},
		Facts:          []irac.FactNode{fact1},
		DominantFamily: "common_law",
		PrecedentRationales: map[string]string{
			"rule-precedent": "same contested element: reasonableness of notice period",
		},
		DistinguishingRationales: map[string]string{
			"rule-precedent:fact-1": "precedent involved 30 days notice; here only two days were given",
		},
	}

	result, err := svc.ApplyRulesDetailed(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) == 0 {
		t.Fatal("expected at least one application node")
	}
	if len(result.PrecedentLinks) != 1 {
		t.Fatalf("expected 1 precedent link, got %d", len(result.PrecedentLinks))
	}
	if len(result.DistinguishingFacts) != 1 {
		t.Fatalf("expected 1 distinguishing fact, got %d", len(result.DistinguishingFacts))
	}

	for _, node := range result.Nodes {
		got, err := store.GetNode(ctx, node.ID)
		if err != nil {
			t.Fatalf("expected node %s to be persisted: %v", node.ID, err)
		}
		if got.Type != irac.NodeApplication {
			t.Fatalf("expected NodeApplication, got %s", got.Type)
		}
		if node.Confidence <= 0 {
			t.Fatalf("expected positive confidence, got %f", node.Confidence)
		}
	}
}

func TestApplicationService_ApplyRules_NoMatchingRules(t *testing.T) {
	svc := application.NewApplicationService()
	issue := testIssue(t, "issue-1", "alpha beta gamma")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "delta epsilon zeta", "US-CA", "common_law")}
	fact1 := testFact(t, "fact-1", "eta theta iota")

	_, err := svc.ApplyRules(context.Background(), application.ApplyRequest{
		Issue: issue,
		Rules: []application.OriginatedRule{rule},
		Facts: []irac.FactNode{fact1},
	})
	if !errors.Is(err, application.ErrNoMatchingRules) {
		t.Fatalf("expected ErrNoMatchingRules, got %v", err)
	}
}

func TestApplicationService_ApplyRules_EmptyInput(t *testing.T) {
	svc := application.NewApplicationService()

	_, err := svc.ApplyRules(context.Background(), application.ApplyRequest{})
	if !errors.Is(err, application.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestApplicationService_ApplyRules_RejectsCyclicChain(t *testing.T) {
	svc := application.NewApplicationService()
	issue := testIssue(t, "issue-1", "notice was reasonable")
	repeated := testRule(t, "rule-1", "text", "US-CA", "common_law")
	rule := application.OriginatedRule{Rule: repeated}
	fact1 := testFact(t, "fact-1", "notice text")

	_, err := svc.ApplyRules(context.Background(), application.ApplyRequest{
		Issue: issue,
		Rules: []application.OriginatedRule{rule},
		Facts: []irac.FactNode{fact1},
		Chain: application.RuleChain{
			Rules: []application.OriginatedRule{{Rule: repeated}, {Rule: repeated}},
		},
	})
	if !errors.Is(err, application.ErrCyclicChain) {
		t.Fatalf("expected ErrCyclicChain, got %v", err)
	}
}

func TestApplicationService_ApplyRules_TopNLimitsResults(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &application.ApplicationService{Store: store}
	issue := testIssue(t, "issue-1", "reasonable notice before eviction")
	rule1 := application.OriginatedRule{Rule: testRule(t, "rule-1", "reasonable notice before eviction", "US-CA", "common_law")}
	rule2 := application.OriginatedRule{Rule: testRule(t, "rule-2", "reasonable notice eviction rule", "US-CA", "common_law")}
	fact1 := testFact(t, "fact-1", "notice was given")

	seedNode(t, store, issue.Node)
	seedNode(t, store, rule1.Rule.Node)
	seedNode(t, store, rule2.Rule.Node)
	seedNode(t, store, fact1.Node)

	nodes, err := svc.ApplyRules(context.Background(), application.ApplyRequest{
		Issue: issue,
		Rules: []application.OriginatedRule{rule1, rule2},
		Facts: []irac.FactNode{fact1},
		TopN:  1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected TopN=1 to limit results to 1 node, got %d", len(nodes))
	}
}
