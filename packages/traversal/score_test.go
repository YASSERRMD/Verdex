package traversal_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

func TestDefaultScoreFunc_ShorterPathsScoreHigher(t *testing.T) {
	shallow := traversal.Path{Nodes: make([]traversal.PathNode, 2), Hops: make([]traversal.TraversedHop, 1)}
	deep := traversal.Path{Nodes: make([]traversal.PathNode, 4), Hops: make([]traversal.TraversedHop, 3)}

	shallowScore := traversal.DefaultScoreFunc(shallow)
	deepScore := traversal.DefaultScoreFunc(deep)

	if shallowScore <= deepScore {
		t.Fatalf("expected shallow path to score higher: shallow=%f deep=%f", shallowScore, deepScore)
	}
}

func TestWalker_RankBy_OrdersResultsByScore(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	caseID := "case-rank"
	issue1ID, issue2ID, ruleID := seedTwoIssuesOneRule(t, store, caseID)

	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	// Rank issue2 higher than issue1 via a custom ScoreFunc.
	scoreFn := func(p traversal.Path) float64 {
		if p.EndID() == issue2ID {
			return 10.0
		}
		return 1.0
	}

	query := traversal.NewQuery(caseID, ruleID).
		Via(irac.EdgeGoverns, traversal.Forward, irac.NodeIssue).
		RankBy(scoreFn)

	result, err := walker.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result.Paths))
	}
	if result.Paths[0].EndID() != issue2ID {
		t.Errorf("expected issue2 (%q) ranked first, got %q", issue2ID, result.Paths[0].EndID())
	}
	if result.Paths[1].EndID() != issue1ID {
		t.Errorf("expected issue1 (%q) ranked second, got %q", issue1ID, result.Paths[1].EndID())
	}
	if result.Paths[0].Score != 10.0 {
		t.Errorf("expected score 10.0, got %f", result.Paths[0].Score)
	}
}

func TestConfidenceWeightedScoreFunc(t *testing.T) {
	weights := map[string]float64{
		"a": 2.0,
		"b": 1.0,
	}
	scoreFn := traversal.ConfidenceWeightedScoreFunc(weights)

	path := traversal.Path{
		Nodes: []traversal.PathNode{{ID: "a"}, {ID: "b"}},
		Hops:  []traversal.TraversedHop{{}},
	}
	got := scoreFn(path)

	base := traversal.DefaultScoreFunc(path)
	want := base * 1.5 // mean of weights 2.0 and 1.0
	if got != want {
		t.Errorf("expected score %f, got %f", want, got)
	}
}

func TestConfidenceWeightedScoreFunc_UnknownNodeDefaultsToNeutral(t *testing.T) {
	scoreFn := traversal.ConfidenceWeightedScoreFunc(map[string]float64{})
	path := traversal.Path{Nodes: []traversal.PathNode{{ID: "unknown"}}}
	got := scoreFn(path)
	want := traversal.DefaultScoreFunc(path) // neutral weight of 1.0
	if got != want {
		t.Errorf("expected neutral-weighted score %f, got %f", want, got)
	}
}
