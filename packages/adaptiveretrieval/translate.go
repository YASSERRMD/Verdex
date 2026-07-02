package adaptiveretrieval

import (
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/traversal"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// toTraversalQuery translates query's anchor and the AdaptiveDepth-resolved
// hop sequence into a traversal.Query, mirroring
// hybridretrieval.buildExpansionQuery's translation of the same
// hybridretrieval.ExpansionHop vocabulary. An unrecognized hop is skipped
// rather than erroring: AdaptiveDepth only ever produces hops drawn from
// query.Hops or DefaultHopSequence, both of which are always one of the
// three recognized ExpansionHop constants, so this can only be reached if
// a caller hand-builds an AdaptiveQuery with a non-standard
// hybridretrieval.ExpansionHop value — treated as "no hop" rather than a
// hard failure, since an adaptive build degrading to a shorter walk is
// preferable to erroring out entirely.
func toTraversalQuery(query AdaptiveQuery, hops []hybridretrieval.ExpansionHop, depth int) traversal.Query {
	tq := traversal.NewQuery(query.CaseID, query.AnchorNodeID)
	for _, hop := range hops {
		switch hop {
		case hybridretrieval.ExpansionGoverningRule:
			tq = tq.ViaGoverningRule()
		case hybridretrieval.ExpansionControllingPrecedent:
			tq = tq.ViaControllingPrecedent()
		case hybridretrieval.ExpansionDistinguishingFacts:
			tq = tq.ViaDistinguishingFacts()
		}
	}
	if depth > 0 {
		tq = tq.WithMaxDepth(depth)
	}
	return tq
}

// treeindexPathsToTraversalPaths projects treeindex.Path values (the
// fallback lookup's native shape) into traversal.Path values, so
// Subgraph.Paths has one consistent type regardless of whether it was
// produced by an adaptive build or a treeindex fallback.
func treeindexPathsToTraversalPaths(paths []treeindex.Path) []traversal.Path {
	out := make([]traversal.Path, len(paths))
	for i, p := range paths {
		nodes := make([]traversal.PathNode, len(p.Nodes))
		for j, n := range p.Nodes {
			nodes[j] = traversal.PathNode{ID: n.ID, Type: n.Type, CaseID: n.CaseID, Text: n.Text}
		}
		hops := make([]traversal.TraversedHop, len(p.Hops))
		for j, h := range p.Hops {
			direction := traversal.Forward
			if h.Reverse {
				direction = traversal.Reverse
			}
			hops[j] = traversal.TraversedHop{FromIndex: h.FromIndex, EdgeType: h.EdgeType, Direction: direction}
		}
		out[i] = traversal.Path{Nodes: nodes, Hops: hops}
	}
	return out
}

// countNodes returns the number of distinct node IDs referenced across
// every path in paths, used to populate Subgraph.NodesVisited for a
// treeindex fallback result (treeindex.Indexer does not itself report a
// visited-node count the way traversal.Result does).
func countNodes(paths []treeindex.Path) int {
	seen := make(map[string]struct{})
	for _, p := range paths {
		for _, n := range p.Nodes {
			seen[n.ID] = struct{}{}
		}
	}
	return len(seen)
}
