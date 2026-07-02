package traversal

import (
	"context"
	"fmt"
	"sort"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// Walker executes Query values against a graph.GraphStore, walking each
// hop step in sequence and shaping the discovered routes into a
// Result. Walker is the package's main entry point: build one
// with NewWalker, then call Execute (or ExecuteCached, cache.go) for each
// Query.
//
// Walker is safe for concurrent use: it holds no mutable state of its
// own beyond the store and cache references, both of which are safe for
// concurrent use in their own right.
type Walker struct {
	store graph.GraphStore
	cache *Cache // nil means "no caching" (see cache.go)
}

// NewWalker constructs a Walker over store. Returns ErrNilGraphStore if
// store is nil.
func NewWalker(store graph.GraphStore) (*Walker, error) {
	if store == nil {
		return nil, ErrNilGraphStore
	}
	return &Walker{store: store}, nil
}

// frontierNode tracks one live route during the walk: the path index
// chain leading to it (as indices into the accumulating nodes/hops
// slices) and the current node's full irac.Node (needed by the
// resolver-backed hops, which take an irac.RuleNode).
type frontierNode struct {
	nodeIndex int // index into the accumulated Path.Nodes this route currently ends at
	node      irac.Node
}

// Execute runs query against the Walker's graph.GraphStore, without
// caching. See ExecuteCached (cache.go) for a cached variant.
//
// # Traversal semantics
//
// Execute performs a bounded breadth-first walk: it starts at
// query.StartNodeID and, for each HopSpec in query.Hops (up to
// query.effectiveDepth() steps), expands every currently-live route by
// one hop. A HopKindCustom or HopKindGoverningRule hop walks a real
// irac.Edge (see caseEdgeIndex.neighbors); a HopKindControllingPrecedent
// or HopKindDistinguishingFacts hop instead calls the Query's configured
// resolver function once per live RuleNode/FactNode route endpoint (see
// precedent.go / distinguish.go).
//
// # Bounded depth and pruning
//
// The walk never takes more than query.effectiveDepth() steps
// (bounded-depth traversal), and a per-execution visited-set prevents any
// single route from revisiting a node it has already passed through
// (cycle guard) — a node can still appear in more than one Path (e.g. two
// distinct issues sharing a governing rule), but a single Path never
// contains the same node twice. If NodeTypeFilter is set on a HopSpec,
// candidates failing the filter are pruned before being added to the
// frontier (early termination of that branch, not of the whole walk).
//
// Returns ErrEmptyCaseID/ErrEmptyStartNodeID/ErrNoHops/ErrInvalidMaxDepth
// from query.validate() before touching the store, and
// ErrStartNodeNotFound if query.StartNodeID cannot be resolved in the
// GraphStore.
func (w *Walker) Execute(ctx context.Context, query Query) (Result, error) {
	if err := query.validate(); err != nil {
		return Result{}, err
	}

	startNode, err := w.store.GetNode(ctx, query.StartNodeID)
	if err != nil {
		return Result{}, fmt.Errorf("traversal: execute query: %w: %v", ErrStartNodeNotFound, err)
	}
	if startNode.CaseID != query.CaseID {
		return Result{}, fmt.Errorf("traversal: execute query: %w: node %q belongs to case %q, not %q", ErrStartNodeNotFound, query.StartNodeID, startNode.CaseID, query.CaseID)
	}

	edges, err := loadCaseEdges(ctx, w.store, query.CaseID)
	if err != nil {
		return Result{}, err
	}
	edgeIdx := newCaseEdgeIndex(edges)

	depth := query.effectiveDepth()

	nodes := []PathNode{pathNodeFromNode(startNode)}
	var hops []TraversedHop
	visited := map[string]struct{}{startNode.ID: {}}
	frontier := []frontierNode{{nodeIndex: 0, node: startNode}}

	truncated := false
	for step := 0; step < depth; step++ {
		hop := query.Hops[step]

		nextFrontier, err := w.expand(ctx, query, hop, edgeIdx, frontier, &nodes, &hops, visited)
		if err != nil {
			return Result{}, err
		}
		if len(nextFrontier) == 0 {
			// Nothing more to expand; the walk naturally ends before
			// consuming the full Hops sequence. This is not truncation
			// (there is nothing left to find), so Truncated stays false.
			frontier = nil
			break
		}
		frontier = nextFrontier
	}
	if len(frontier) > 0 && depth < len(query.Hops) {
		truncated = true
	}

	paths := assemblePaths(nodes, hops)

	scoreFn := query.ScoreFunc
	if scoreFn == nil {
		scoreFn = DefaultScoreFunc
	}
	for i := range paths {
		paths[i].Score = scoreFn(paths[i])
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return paths[i].Score > paths[j].Score
	})

	return Result{
		Paths:        paths,
		Truncated:    truncated,
		VisitedCount: len(visited),
	}, nil
}

// expand walks every frontier entry by one hop, appending newly
// discovered nodes/hops to nodes/hops and returning the new frontier.
// visited is mutated in place as a cycle guard shared across the whole
// query execution (not just the current step), so a route can never loop
// back through a node it already passed through.
func (w *Walker) expand(ctx context.Context, query Query, hop HopSpec, edgeIdx *caseEdgeIndex, frontier []frontierNode, nodes *[]PathNode, hops *[]TraversedHop, visited map[string]struct{}) ([]frontierNode, error) {
	var next []frontierNode

	for _, live := range frontier {
		candidates, err := w.resolveHop(ctx, query, hop, edgeIdx, live.node)
		if err != nil {
			return nil, err
		}

		for _, candidate := range candidates {
			if hop.NodeTypeFilter != "" && candidate.Type != hop.NodeTypeFilter {
				continue
			}
			if _, seen := visited[candidate.ID]; seen {
				continue
			}
			visited[candidate.ID] = struct{}{}

			*nodes = append(*nodes, pathNodeFromNode(candidate))
			*hops = append(*hops, TraversedHop{
				FromIndex: live.nodeIndex,
				Kind:      hop.Kind,
				EdgeType:  hop.EdgeType,
				Direction: hop.Direction,
			})
			next = append(next, frontierNode{nodeIndex: len(*nodes) - 1, node: candidate})
		}
	}

	return next, nil
}

// resolveHop returns every candidate node reachable from from by walking
// hop, dispatching to a real edge walk or a resolver call depending on
// hop.Kind.
func (w *Walker) resolveHop(ctx context.Context, query Query, hop HopSpec, edgeIdx *caseEdgeIndex, from irac.Node) ([]irac.Node, error) {
	switch hop.Kind {
	case HopKindControllingPrecedent:
		return w.resolvePrecedentHop(ctx, query, from)
	case HopKindDistinguishingFacts:
		return w.resolveDistinguishingFactsHop(ctx, query, from)
	default:
		return w.resolveEdgeHop(ctx, edgeIdx, hop, from)
	}
}

// resolveEdgeHop walks a real irac.Edge from from's node using the
// case's edge index.
func (w *Walker) resolveEdgeHop(ctx context.Context, edgeIdx *caseEdgeIndex, hop HopSpec, from irac.Node) ([]irac.Node, error) {
	ids := edgeIdx.neighbors(from.ID, hop.EdgeType, hop.Direction)
	out := make([]irac.Node, 0, len(ids))
	for _, id := range ids {
		n, err := w.store.GetNode(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("traversal: resolve edge hop: get node %q: %w", id, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// resolvePrecedentHop invokes query.PrecedentResolver (or NoPrecedents)
// for from, which must be a RuleNode.
func (w *Walker) resolvePrecedentHop(ctx context.Context, query Query, from irac.Node) ([]irac.Node, error) {
	if from.Type != irac.NodeRule {
		return nil, nil
	}
	resolver := query.PrecedentResolver
	if resolver == nil {
		resolver = NoPrecedents
	}
	ruleNode := irac.RuleNode{Node: from}
	rules, err := resolver(ctx, ruleNode)
	if err != nil {
		return nil, fmt.Errorf("traversal: resolve controlling precedent for rule %q: %w", from.ID, err)
	}
	out := make([]irac.Node, len(rules))
	for i, r := range rules {
		out[i] = r.Node
	}
	return out, nil
}

// resolveDistinguishingFactsHop invokes query.DistinguishingFactResolver
// (or NoDistinguishingFacts) for from, which must be a RuleNode
// representing a precedent.
func (w *Walker) resolveDistinguishingFactsHop(ctx context.Context, query Query, from irac.Node) ([]irac.Node, error) {
	if from.Type != irac.NodeRule {
		return nil, nil
	}
	resolver := query.DistinguishingFactResolver
	if resolver == nil {
		resolver = NoDistinguishingFacts
	}
	ruleNode := irac.RuleNode{Node: from}
	facts, err := resolver(ctx, query.CaseID, ruleNode)
	if err != nil {
		return nil, fmt.Errorf("traversal: resolve distinguishing facts for rule %q: %w", from.ID, err)
	}
	out := make([]irac.Node, len(facts))
	for i, f := range facts {
		out[i] = f.Node
	}
	return out, nil
}

// assemblePaths reconstructs every root-to-leaf route through the
// (nodes, hops) tree built by Execute into a distinct Path. nodes[0] is
// always the tree's root (the query's start node); hops[i] describes how
// nodes[i+1] was reached (FromIndex names its parent). A node with no
// children (no hop's FromIndex points at it) is a leaf, and each leaf
// produces exactly one Path by walking parent pointers back to the root.
func assemblePaths(nodes []PathNode, hops []TraversedHop) []Path {
	if len(nodes) == 1 {
		// The start node had nothing reachable via the query's hops: a
		// single-node, zero-hop Path.
		return []Path{{Nodes: nodes}}
	}

	children := make(map[int]bool, len(nodes))
	parentHop := make(map[int]TraversedHop, len(hops))
	for i, h := range hops {
		nodeIndex := i + 1
		children[h.FromIndex] = true
		parentHop[nodeIndex] = h
	}

	var paths []Path
	for i := range nodes {
		if children[i] {
			continue // not a leaf
		}
		paths = append(paths, buildPathTo(nodes, parentHop, i))
	}
	return paths
}

// buildPathTo walks parent pointers from leafIndex back to the root
// (index 0), returning the resulting Path in root-first order.
func buildPathTo(nodes []PathNode, parentHop map[int]TraversedHop, leafIndex int) Path {
	var pathNodes []PathNode
	var pathHops []TraversedHop

	idx := leafIndex
	for {
		pathNodes = append([]PathNode{nodes[idx]}, pathNodes...)
		hop, ok := parentHop[idx]
		if !ok {
			break
		}
		pathHops = append([]TraversedHop{hop}, pathHops...)
		idx = hop.FromIndex
	}

	// Rewrite FromIndex values to be relative to this Path's own Nodes
	// slice rather than the shared accumulation buffer.
	for i := range pathHops {
		pathHops[i].FromIndex = i
	}

	return Path{Nodes: pathNodes, Hops: pathHops}
}
