package treeindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// edgeLister is implemented by a graph.GraphStore that can additionally
// return every irac.Edge belonging to a case directly, without requiring
// one Traverse call per node. graph.InMemoryGraphStore implements this
// (see its EdgesForCase method) but it is not part of the graph.GraphStore
// interface itself — packages/graph's own backup.go type-asserts for the
// same capability opportunistically (see its Export function), and this
// package follows that precedent rather than widening GraphStore's
// interface just for a performance shortcut.
//
// When the underlying store does not implement edgeLister, loadEdges falls
// back to a Traverse-based reachability walk starting from every node in
// the case (see loadEdgesViaTraverse), which is correct but does more
// GraphStore round trips.
type edgeLister interface {
	EdgesForCase(caseID string) []irac.Edge
}

// loadEdges returns every irac.Edge belonging to caseID, preferring the
// direct edgeLister capability when the store exposes it and falling back
// to a per-node Traverse-based walk otherwise.
func loadEdges(ctx context.Context, store graph.GraphStore, caseID string) ([]irac.Edge, error) {
	if lister, ok := store.(edgeLister); ok {
		return lister.EdgesForCase(caseID), nil
	}
	return loadEdgesViaTraverse(ctx, store, caseID)
}

// loadEdgesViaTraverse reconstructs caseID's edges by calling
// GraphStore.Traverse once per node with FromNodeID set and MaxDepth 1,
// and recording an edge from that node to every node returned other than
// itself. This is the fallback path for a GraphStore implementation that
// does not expose edgeLister (e.g. a future Neo4j-backed store might
// prefer answering this via a direct Cypher MATCH instead of implementing
// EdgesForCase) — it is correct but issues O(nodes) Traverse calls rather
// than one.
func loadEdgesViaTraverse(ctx context.Context, store graph.GraphStore, caseID string) ([]irac.Edge, error) {
	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID})
	if err != nil {
		return nil, fmt.Errorf("treeindex: load edges for case %q: traverse case: %w", caseID, err)
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
			return nil, fmt.Errorf("treeindex: load edges for case %q: traverse from node %q: %w", caseID, n.ID, err)
		}
		for _, neighbor := range neighbors {
			if neighbor.ID == n.ID {
				continue
			}
			// The edge's actual EdgeType and direction cannot be recovered
			// from Traverse alone (it returns reachable nodes, not the
			// edges themselves), so this fallback path can only report
			// that neighbor is one hop from n, not which of the four
			// EdgeTypes connects them. Callers needing exact EdgeType
			// fidelity should prefer a store implementing edgeLister; see
			// the doc comment above.
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
