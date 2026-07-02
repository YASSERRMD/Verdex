package hybridretrieval

import (
	"errors"
	"sort"

	"github.com/YASSERRMD/verdex/packages/traversal"
)

// sortGraphHitsByScoreDesc sorts hits by descending score, breaking ties
// by nodeID for a deterministic order (map iteration order is otherwise
// randomized, and this package's tests assert exact ordering).
func sortGraphHitsByScoreDesc(hits []graphHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].nodeID < hits[j].nodeID
	})
}

// isErrStartNodeNotFound reports whether err wraps
// traversal.ErrStartNodeNotFound.
func isErrStartNodeNotFound(err error) bool {
	return errors.Is(err, traversal.ErrStartNodeNotFound)
}
