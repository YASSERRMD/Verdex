package traversal

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// edgeLister is implemented by a graph.GraphStore that can return every
// irac.Edge belonging to a case directly. graph.InMemoryGraphStore
// implements this via its EdgesForCase method. This is the same
// opportunistic, type-asserted capability pattern packages/graph's own
// backup.go (Export) and packages/treeindex's edges.go already use —
// this package follows that established precedent rather than widening
// the graph.GraphStore interface just for a performance shortcut.
//
// A store not implementing edgeLister falls back to a Traverse-based
// one-hop walk (see stepViaTraverse in walk.go), which is correct for
// connectivity but, like treeindex's fallback, cannot recover exact
// EdgeType/direction fidelity beyond what Traverse itself reports.
type edgeLister interface {
	EdgesForCase(caseID string) []irac.Edge
}

// caseEdgeIndex indexes a case's edges by both endpoints, so a single
// Query execution walking several hops from different nodes does not
// re-scan the full edge list on every step.
type caseEdgeIndex struct {
	outbound map[string][]irac.Edge // keyed by FromID
	inbound  map[string][]irac.Edge // keyed by ToID
}

// newCaseEdgeIndex builds a caseEdgeIndex over edges.
func newCaseEdgeIndex(edges []irac.Edge) *caseEdgeIndex {
	idx := &caseEdgeIndex{
		outbound: make(map[string][]irac.Edge),
		inbound:  make(map[string][]irac.Edge),
	}
	for _, e := range edges {
		idx.outbound[e.FromID] = append(idx.outbound[e.FromID], e)
		idx.inbound[e.ToID] = append(idx.inbound[e.ToID], e)
	}
	return idx
}

// neighbors returns every node ID reachable from nodeID by walking one
// edge of edgeType in the given direction: Forward follows FromID ->
// ToID edges originating at nodeID, Reverse follows FromID -> ToID edges
// terminating at nodeID (returning the FromID side).
func (idx *caseEdgeIndex) neighbors(nodeID string, edgeType irac.EdgeType, direction Direction) []string {
	var edges []irac.Edge
	if direction == Reverse {
		edges = idx.inbound[nodeID]
	} else {
		edges = idx.outbound[nodeID]
	}

	out := make([]string, 0, len(edges))
	for _, e := range edges {
		if e.Type != edgeType {
			continue
		}
		if direction == Reverse {
			out = append(out, e.FromID)
		} else {
			out = append(out, e.ToID)
		}
	}
	return out
}

// loadCaseEdges returns every irac.Edge belonging to caseID, preferring
// the direct edgeLister capability when store exposes it and falling
// back to a per-node Traverse-based reconstruction otherwise (see
// loadCaseEdgesViaTraverse).
func loadCaseEdges(ctx context.Context, store graph.GraphStore, caseID string) ([]irac.Edge, error) {
	if lister, ok := store.(edgeLister); ok {
		return lister.EdgesForCase(caseID), nil
	}
	return loadCaseEdgesViaTraverse(ctx, store, caseID)
}

// loadCaseEdgesViaTraverse reconstructs caseID's edges by calling
// graph.GraphStore.Traverse once per node with FromNodeID set and
// MaxDepth 1. Like packages/treeindex's identical fallback, this cannot
// recover the exact EdgeType connecting two nodes (Traverse reports
// reachable nodes, not edges), so every reconstructed edge is reported
// with an empty EdgeType; callers of this package needing exact
// EdgeType-aware hops (i.e. every named hop except the resolver-backed
// ones) should prefer a store implementing edgeLister.
func loadCaseEdgesViaTraverse(ctx context.Context, store graph.GraphStore, caseID string) ([]irac.Edge, error) {
	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID})
	if err != nil {
		return nil, fmt.Errorf("traversal: load edges for case %q: traverse case: %w", caseID, err)
	}

	var edges []irac.Edge
	seen := make(map[irac.Edge]struct{})
	for _, n := range nodes {
		neighbors, err := store.Traverse(ctx, graph.TraversalQuery{
			CaseID:     caseID,
			FromNodeID: n.ID,
			MaxDepth:   1,
		})
		if err != nil {
			return nil, fmt.Errorf("traversal: load edges for case %q: traverse from node %q: %w", caseID, n.ID, err)
		}
		for _, neighbor := range neighbors {
			if neighbor.ID == n.ID {
				continue
			}
			e := irac.Edge{FromID: n.ID, ToID: neighbor.ID}
			if _, ok := seen[e]; ok {
				continue
			}
			seen[e] = struct{}{}
			edges = append(edges, e)
		}
	}
	return edges, nil
}
