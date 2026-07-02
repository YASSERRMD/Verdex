package adaptiveretrieval

import (
	"github.com/YASSERRMD/verdex/packages/traversal"
)

// Subgraph is the minimal on-demand-built structure produced for one
// AdaptiveQuery: every traversal.Path discovered walking outward from the
// query's anchor, plus bookkeeping about how the build was bounded.
// Unlike treeindex.PathIndex (a full-case materialization), a Subgraph
// only ever covers one query's anchor and hop sequence.
type Subgraph struct {
	// CaseID identifies the case this Subgraph belongs to.
	CaseID string

	// AnchorNodeID is the node the build started from.
	AnchorNodeID string

	// Paths is every traversal.Path discovered from AnchorNodeID, sorted
	// by descending Score (see traversal.Walker.Execute).
	Paths []traversal.Path

	// Depth is the number of hops actually walked to produce Paths (the
	// AdaptiveDepth-resolved depth, not necessarily the query's full Hops
	// length).
	Depth int

	// NodesVisited is the number of distinct nodes the build visited
	// (traversal.Result.VisitedCount) producing this Subgraph.
	NodesVisited int

	// Truncated reports whether the build stopped early because it hit
	// the BuildBudget's MaxHops, MaxNodes, or MaxWallClock bound before
	// the underlying traversal.Walker would otherwise have stopped on its
	// own.
	Truncated bool

	// Source records how this Subgraph was produced: a fresh adaptive
	// build, a cache hit, or a treeindex fallback. See SubgraphSource.
	Source SubgraphSource

	// Revision is the irac.TreeRevision.RevisionNumber the case's tree
	// was at when this Subgraph was built, used to detect staleness on a
	// later cache read (see Cache).
	Revision int
}

// SubgraphSource identifies which code path produced a Subgraph, so a
// caller (or test) can distinguish "this came from a real adaptive
// traversal" from "this was served from cache" or "this came from the
// treeindex fallback" without re-deriving it from other fields.
type SubgraphSource string

const (
	// SourceBuilt labels a Subgraph produced by a fresh adaptive
	// traversal.Walker walk.
	SourceBuilt SubgraphSource = "built"

	// SourceCached labels a Subgraph served from the Cache without
	// running a new traversal.
	SourceCached SubgraphSource = "cached"

	// SourceFallback labels a Subgraph produced by falling back to
	// treeindex.Indexer.LookupPaths instead of an adaptive build.
	SourceFallback SubgraphSource = "fallback"
)

// allSubgraphSources is the exhaustive set of recognized SubgraphSource
// values, used by IsValid.
var allSubgraphSources = map[SubgraphSource]struct{}{
	SourceBuilt:    {},
	SourceCached:   {},
	SourceFallback: {},
}

// IsValid reports whether s is one of the recognized SubgraphSource
// constants.
func (s SubgraphSource) IsValid() bool {
	_, ok := allSubgraphSources[s]
	return ok
}
