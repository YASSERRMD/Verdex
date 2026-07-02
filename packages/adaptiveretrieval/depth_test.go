package adaptiveretrieval_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/adaptiveretrieval"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
)

func TestAdaptiveDepth_FewVectorHits_WalksFullSequence(t *testing.T) {
	q := adaptiveretrieval.NewAdaptiveQuery("case-1", "node-1").WithVectorHitCount(0)
	budget := adaptiveretrieval.DefaultBuildBudget()

	hops, depth := adaptiveretrieval.AdaptiveDepth(q, budget)

	if depth != len(adaptiveretrieval.DefaultHopSequence) {
		t.Fatalf("depth = %d, want %d (full sequence)", depth, len(adaptiveretrieval.DefaultHopSequence))
	}
	if len(hops) != depth {
		t.Fatalf("len(hops) = %d, want %d", len(hops), depth)
	}
}

func TestAdaptiveDepth_ManyVectorHits_WalksShallower(t *testing.T) {
	few := adaptiveretrieval.NewAdaptiveQuery("case-1", "node-1").WithVectorHitCount(0)
	many := adaptiveretrieval.NewAdaptiveQuery("case-1", "node-1").WithVectorHitCount(adaptiveretrieval.ManyVectorHits)
	budget := adaptiveretrieval.DefaultBuildBudget()

	_, fewDepth := adaptiveretrieval.AdaptiveDepth(few, budget)
	_, manyDepth := adaptiveretrieval.AdaptiveDepth(many, budget)

	if manyDepth >= fewDepth {
		t.Fatalf("manyDepth = %d, want strictly less than fewDepth = %d", manyDepth, fewDepth)
	}
}

func TestAdaptiveDepth_BoundedByBudgetMaxHops(t *testing.T) {
	q := adaptiveretrieval.NewAdaptiveQuery("case-1", "node-1").WithVectorHitCount(0)
	budget := adaptiveretrieval.BuildBudget{MaxHops: 1}

	hops, depth := adaptiveretrieval.AdaptiveDepth(q, budget)

	if depth != 1 {
		t.Fatalf("depth = %d, want 1 (bounded by MaxHops)", depth)
	}
	if len(hops) != 1 {
		t.Fatalf("len(hops) = %d, want 1", len(hops))
	}
}

func TestAdaptiveDepth_RespectsExplicitQueryHops(t *testing.T) {
	q := adaptiveretrieval.NewAdaptiveQuery("case-1", "node-1").
		WithHop(hybridretrieval.ExpansionGoverningRule).
		WithVectorHitCount(0)
	budget := adaptiveretrieval.DefaultBuildBudget()

	hops, depth := adaptiveretrieval.AdaptiveDepth(q, budget)

	if depth != 1 {
		t.Fatalf("depth = %d, want 1 (single explicit hop)", depth)
	}
	if len(hops) != 1 || hops[0] != hybridretrieval.ExpansionGoverningRule {
		t.Fatalf("hops = %v, want [ExpansionGoverningRule]", hops)
	}
}
