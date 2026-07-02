package adaptiveretrieval

import "github.com/YASSERRMD/verdex/packages/hybridretrieval"

// DefaultHopSequence is the hop sequence AdaptiveDepth walks when an
// AdaptiveQuery leaves Hops empty: the same three named legal-reasoning
// hops hybridretrieval's own expansion policy is built from (issue ->
// governing rule -> controlling precedent -> distinguishing facts), in
// the order a case's reasoning tree is naturally read.
var DefaultHopSequence = []hybridretrieval.ExpansionHop{
	hybridretrieval.ExpansionGoverningRule,
	hybridretrieval.ExpansionControllingPrecedent,
	hybridretrieval.ExpansionDistinguishingFacts,
}

// Vector-hit-count thresholds AdaptiveDepth uses to scale how deep an
// adaptive build walks. These are deliberately simple, fixed cutoffs
// rather than a continuous function: see AdaptiveDepth's doc comment for
// the rationale.
const (
	// FewVectorHits is the VectorHitCount at or below which AdaptiveDepth
	// treats semantic recall as having found little corroboration,
	// warranting the deepest configured walk to compensate structurally.
	FewVectorHits = 1

	// ManyVectorHits is the VectorHitCount at or above which
	// AdaptiveDepth treats semantic recall as already strong,
	// warranting the shallowest configured walk.
	ManyVectorHits = 5
)

// AdaptiveDepth resolves how many hops (of the query's configured Hops,
// or DefaultHopSequence if empty) an adaptive build should actually walk,
// and the hop sequence to walk, bounded by budget.MaxHops.
//
// # Why vector-hit count drives depth
//
// A query backed by many strong vector-recall hits (hybridretrieval's
// semantic signal) already has substantial corroboration before graph
// expansion even starts — walking deep is mostly redundant confirmation.
// A query with few or no vector hits has nothing else to lean on, so
// widening the structural walk is the only way to surface corroborating
// context. This mirrors the intuition behind hybridretrieval's own
// reciprocal-rank fusion (a node found by both signals is stronger
// evidence than one found by only one), applied one layer up: instead of
// fusing two fixed-depth signals, adaptiveretrieval varies the structural
// signal's depth based on how much the semantic signal already found.
//
// # Fixed thresholds over a continuous function
//
// A continuous formula (e.g. depth inversely proportional to hit count)
// was considered and rejected: it would produce depth values that change
// on every off-by-one hit count difference, which defeats
// AdaptiveQuery.shapeKey's cache-sharing goal (two queries landing on the
// same effective depth should share a cached subgraph) for no accuracy
// benefit — hit counts are noisy signals, not precise measurements, so a
// small number of coarse depth bands is a better fit than a formula that
// implies false precision.
func AdaptiveDepth(q AdaptiveQuery, budget BuildBudget) (hops []hybridretrieval.ExpansionHop, depth int) {
	budget = budget.withDefaults()

	sequence := q.Hops
	if len(sequence) == 0 {
		sequence = DefaultHopSequence
	}

	want := len(sequence)
	switch {
	case q.VectorHitCount >= ManyVectorHits:
		want = max(1, len(sequence)-2)
	case q.VectorHitCount > FewVectorHits:
		want = max(1, len(sequence)-1)
	default:
		want = len(sequence)
	}

	if want > budget.MaxHops {
		want = budget.MaxHops
	}
	if want > len(sequence) {
		want = len(sequence)
	}
	if want < 0 {
		want = 0
	}

	return sequence[:want], want
}
