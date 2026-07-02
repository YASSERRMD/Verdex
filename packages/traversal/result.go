package traversal

import "github.com/YASSERRMD/verdex/packages/irac"

// PathNode is a minimal, immutable projection of an irac.Node captured as
// one step of a materialized Path. Mirrors packages/treeindex's NodeRef
// convention (a snapshot decoupled from the live irac.Node, so a Path
// never risks drifting from a node's full field set) but is defined
// independently here rather than imported, since treeindex is a sibling
// package this one does not depend on.
type PathNode struct {
	// ID is the referenced node's irac.Node.ID.
	ID string

	// Type is the referenced node's irac.NodeType.
	Type irac.NodeType

	// CaseID is the referenced node's case.
	CaseID string

	// Text is the referenced node's irac.Node.Text, captured at traversal
	// time.
	Text string
}

// pathNodeFromNode projects an irac.Node into a PathNode.
func pathNodeFromNode(n irac.Node) PathNode {
	return PathNode{ID: n.ID, Type: n.Type, CaseID: n.CaseID, Text: n.Text}
}

// TraversedHop describes one edge actually walked while assembling a
// Path, as opposed to HopSpec which describes a step the caller asked
// for. FromIndex names which element of the owning Path's Nodes this hop
// originates from, mirroring packages/treeindex's Hop.FromIndex — a
// traversal can fan out (e.g. a governing rule reached by more than one
// distinguishing fact), so hops do not assume a strict linear chain.
type TraversedHop struct {
	// FromIndex is the index into the owning Path's Nodes slice that this
	// hop originates from.
	FromIndex int

	// Kind is the HopKind of the HopSpec that produced this hop.
	Kind HopKind

	// EdgeType is the irac.EdgeType walked, or "" for a resolver-backed
	// hop (HopKindControllingPrecedent, HopKindDistinguishingFacts).
	EdgeType irac.EdgeType

	// Direction is the Direction the edge was walked in. Meaningless
	// (zero value Forward) for resolver-backed hops.
	Direction Direction
}

// Path is one concrete route discovered by executing a Query: an ordered
// sequence of PathNodes starting at the query's start node, plus the
// TraversedHops describing how each subsequent node was reached, and a
// cumulative Score assigned by the Query's ScoreFunc (see score.go).
type Path struct {
	// Nodes is the ordered set of node references making up this Path,
	// start node first (index 0).
	Nodes []PathNode

	// Hops describes the edge (or resolver call) that produced each
	// non-start element of Nodes: Hops[i] produced Nodes[i+1]. len(Hops)
	// == len(Nodes)-1.
	Hops []TraversedHop

	// Score is this Path's cumulative ranking score, computed by the
	// Query's ScoreFunc. Zero if the Query used the default (unweighted)
	// scoring.
	Score float64
}

// StartID returns the ID of the first node in the path, or "" if the
// path has no nodes.
func (p Path) StartID() string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return p.Nodes[0].ID
}

// EndID returns the ID of the last node in the path, or "" if the path
// has no nodes.
func (p Path) EndID() string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return p.Nodes[len(p.Nodes)-1].ID
}

// Depth returns the number of hops in the path (len(Hops)), i.e. how many
// edges were walked from the start node to reach the last node appended.
// This is a simple hop count rather than a fanned-out-tree depth
// computation (contrast packages/treeindex's Path.Depth, which handles a
// fan-out tree shape): a traversal.Path's Hops are appended in strict
// per-step order by the walker (walk.go), so the maximum FromIndex chain
// length equals len(Hops) for any path this package produces from a
// single Query execution.
func (p Path) Depth() int {
	return len(p.Hops)
}

// Explain renders a short, human-readable description of the hop
// sequence that produced this Path, e.g.
// "issue-1 --governing_rule(reverse:governs)--> rule-1
//
//	--controlling_precedent--> rule-9".
//
// Intended for logs, debugging, and surfacing "why was this result
// returned" explanations to a caller (the same spirit as the future
// Phase 044 hybrid retrieval consumer's "explanation of retrieval path"
// requirement).
func (p Path) Explain() string {
	if len(p.Nodes) == 0 {
		return ""
	}
	out := p.Nodes[0].ID
	for i, hop := range p.Hops {
		out += " --" + string(hop.Kind)
		if hop.EdgeType != "" {
			out += "(" + hop.Direction.String() + ":" + string(hop.EdgeType) + ")"
		}
		out += "--> " + p.Nodes[i+1].ID
	}
	return out
}

// TraversalResult is the outcome of executing a Query: every discovered
// Path, already ordered by the Query's ScoreFunc (highest score first;
// see score.go), plus bookkeeping about how the traversal was bounded.
type TraversalResult struct {
	// Paths is every Path discovered by the traversal, sorted by
	// descending Score. Empty (not nil) when nothing was reachable.
	Paths []Path

	// Truncated reports whether the traversal stopped early because it
	// hit the Query's MaxDepth bound before exhausting every reachable
	// node. It does NOT report cycle-guard pruning (that is expected,
	// routine behavior, not a sign of an incomplete result).
	Truncated bool

	// VisitedCount is the number of distinct node IDs the traversal
	// visited while executing the query, including the start node.
	// Exposed for observability/telemetry, mirroring
	// packages/treeindex.Stats's role for that package.
	VisitedCount int
}
