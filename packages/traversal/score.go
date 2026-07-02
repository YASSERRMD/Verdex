package traversal

// ScoreFunc computes a ranking score for one discovered Path. Higher
// scores rank first in a TraversalResult.Paths. Callers plug in whatever
// criteria matters for their domain — precedent AuthorityScore, statute
// specificity, recency, node Confidence — without this package importing
// packages/precedent, packages/statute, or any other package that would
// tie a general-purpose graph walker to one scoring policy. This is the
// same "accept a caller-supplied function instead of importing the
// concrete package" pattern packages/application uses for Origin (see
// origin.go) and this package uses for PrecedentResolver / (
// distinguish.go's) DistinguishingFactResolver.
//
// A ScoreFunc must be safe for concurrent use if the same Query is
// executed concurrently.
type ScoreFunc func(path Path) float64

// DefaultScoreFunc scores every Path by its inverse depth: shorter paths
// (fewer hops) score higher. This is a reasonable, dependency-free
// default — "the most direct route to an answer is usually the most
// relevant one" — that a caller can override via Query.RankBy for
// domain-specific criteria (e.g. weighting by precedent authority).
//
// Depth 0 (a single-node path, i.e. no hops at all) scores 1.0; each
// additional hop halves the score, so scores stay in (0, 1] and sort
// stably by hop count without requiring callers to know an upper bound
// on depth up front.
func DefaultScoreFunc(path Path) float64 {
	depth := path.Depth()
	score := 1.0
	for i := 0; i < depth; i++ {
		score /= 2
	}
	return score
}

// ConfidenceWeightedScoreFunc returns a ScoreFunc that scores a Path by
// the product of DefaultScoreFunc's depth-based score and the mean
// irac.Node.Confidence-derived weight supplied per node via weights. This
// is offered as a ready-to-use alternative to DefaultScoreFunc for
// callers that want confidence-aware ranking without writing their own
// ScoreFunc from scratch, while still not requiring this package to read
// irac.Node.Confidence itself (PathNode does not carry Confidence — see
// result.go — so the caller supplies it keyed by node ID).
//
// A node ID absent from weights contributes a neutral weight of 1.0.
func ConfidenceWeightedScoreFunc(weights map[string]float64) ScoreFunc {
	return func(path Path) float64 {
		base := DefaultScoreFunc(path)
		if len(path.Nodes) == 0 {
			return base
		}
		sum := 0.0
		for _, n := range path.Nodes {
			w, ok := weights[n.ID]
			if !ok {
				w = 1.0
			}
			sum += w
		}
		mean := sum / float64(len(path.Nodes))
		return base * mean
	}
}
