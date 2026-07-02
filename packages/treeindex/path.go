package treeindex

import "github.com/YASSERRMD/verdex/packages/irac"

// NodeRef is a minimal, immutable projection of an irac.Node captured as
// one hop in a materialized Path. treeindex stores NodeRef rather than a
// full irac.Node in its paths so that re-fetching or copying a Path never
// risks drifting from the source node's full field set (Confidence,
// Provenance, Spans, ...); callers that need the full node can look it up
// by ID via graph.GraphStore.GetNode.
type NodeRef struct {
	// ID is the referenced node's irac.Node.ID.
	ID string

	// Type is the referenced node's irac.NodeType.
	Type irac.NodeType

	// CaseID is the referenced node's case, copied for convenience so a
	// NodeRef can be filtered/grouped without a second lookup.
	CaseID string

	// Text is the referenced node's irac.Node.Text, copied at index-build
	// time. Like packages/vectorindex's VectorRecord.Text, this is a
	// snapshot: it can go stale if the source node's Text changes without
	// a RebuildCase/ReindexOnRevision call (see maintenance.go).
	Text string
}

// nodeRefFromNode projects an irac.Node into a NodeRef.
func nodeRefFromNode(n irac.Node) NodeRef {
	return NodeRef{
		ID:     n.ID,
		Type:   n.Type,
		CaseID: n.CaseID,
		Text:   n.Text,
	}
}

// PathKind classifies which materialization strategy produced a Path. See
// doc.go and doc/tree-indexing.md for the full rationale behind these two
// kinds.
type PathKind string

const (
	// PathKindRuleGroupedIssues identifies a Path grouping a RuleNode
	// (Nodes[0]) with every IssueNode it governs (Nodes[1:], connected via
	// irac.EdgeGoverns). This is treeindex's substitute for an
	// "issue -> sub-issue" relationship, since irac.IssueNode carries no
	// literal parent/sub-issue edge.
	PathKindRuleGroupedIssues PathKind = "rule_grouped_issues"

	// PathKindReasoningChain identifies a Path assembling one IssueNode's
	// full reasoning chain: the issue, its governing rule(s), the
	// applications that apply those rules, and the facts/conclusions
	// connected to those applications. This is the "rule -> application ->
	// conclusion" structure the plan calls for.
	PathKindReasoningChain PathKind = "reasoning_chain"
)

// allPathKinds is the exhaustive set of recognized PathKind values, used
// by IsValid.
var allPathKinds = map[PathKind]struct{}{
	PathKindRuleGroupedIssues: {},
	PathKindReasoningChain:    {},
}

// IsValid reports whether k is one of the recognized PathKind constants.
func (k PathKind) IsValid() bool {
	_, ok := allPathKinds[k]
	return ok
}

// Hop describes one edge that produced a node in a Path. A Path is not
// always a strict linear chain — buildReasoningChainPaths (chain.go) fans
// out from a single ApplicationNode to every supporting FactNode and every
// derived ConclusionNode, so a Hop records which node it originated from
// (FromIndex) rather than assuming it always originates from the
// immediately preceding element of Nodes.
type Hop struct {
	// FromIndex is the index into the owning Path's Nodes slice that this
	// hop originates from. The hop's destination is implicitly the Nodes
	// element at the same position as this Hop in Hops (i.e. Hops[i]
	// produced Nodes[i+1]).
	FromIndex int

	// EdgeType is the irac.EdgeType connecting the two hops.
	EdgeType irac.EdgeType

	// Reverse reports whether this hop was walked against the edge's
	// declared irac.Edge.FromID -> irac.Edge.ToID direction (Forward,
	// From -> To) or against it (Reverse, To -> From) to assemble a
	// human-meaningful chain — see doc.go for why EdgeSupports and
	// EdgeConcludesFrom are walked in Reverse.
	Reverse bool
}

// Path is one materialized set of NodeRef values discovered by walking a
// case's reasoning tree, plus the Hops describing how each non-root node
// was reached. Nodes[0] is always the root (see RootID). For a
// PathKindRuleGroupedIssues Path, every Hop originates from Nodes[0] (a
// simple one-level fan-out). For a PathKindReasoningChain Path, Hops forms
// a short tree rooted at Nodes[0] (Issue -> Rule -> Application, then
// Application fanning out to every supporting Fact and derived
// Conclusion) — see Hop.FromIndex. len(Hops) is always len(Nodes)-1: every
// non-root node was produced by exactly one hop.
type Path struct {
	// Kind classifies which materialization strategy produced this Path.
	Kind PathKind

	// CaseID identifies the case this Path belongs to.
	CaseID string

	// Nodes is the ordered set of node references making up this Path,
	// root first.
	Nodes []NodeRef

	// Hops describes the edge that produced each non-root element of
	// Nodes: Hops[i] produced Nodes[i+1]. len(Hops) == len(Nodes)-1.
	Hops []Hop
}

// RootID returns the ID of the first node in the path, or "" if the path
// has no nodes.
func (p Path) RootID() string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return p.Nodes[0].ID
}

// nodeDepths returns, for each index into p.Nodes, the number of hops
// from the root (index 0) required to reach it: depths[0] is always 0,
// and depths[i] for i > 0 is 1 + the depth of Hops[i-1].FromIndex. This
// walks Hops in order, which is always safe because every builder in this
// package appends a node's producing Hop before appending any node that
// descends from it (FromIndex never refers to a not-yet-populated index).
func (p Path) nodeDepths() []int {
	depths := make([]int, len(p.Nodes))
	for i, hop := range p.Hops {
		nodeIndex := i + 1
		depths[nodeIndex] = depths[hop.FromIndex] + 1
	}
	return depths
}

// Depth returns the maximum number of edge hops from the root (Nodes[0])
// to any other node in the path, or 0 for an empty or single-node path.
func (p Path) Depth() int {
	if len(p.Nodes) == 0 {
		return 0
	}
	max := 0
	for _, d := range p.nodeDepths() {
		if d > max {
			max = d
		}
	}
	return max
}

// Truncate returns a copy of p containing only the nodes reachable from
// the root within maxDepth hops (and the Hops connecting them), discarding
// everything deeper. maxDepth <= 0 returns p unchanged, mirroring
// graph.TraversalQuery.MaxDepth's "zero means unbounded" convention.
func (p Path) Truncate(maxDepth int) Path {
	if maxDepth <= 0 || p.Depth() <= maxDepth {
		return p
	}

	depths := p.nodeDepths()

	// oldToNew maps a surviving node's original index to its index in the
	// truncated output, so Hop.FromIndex can be rewritten consistently.
	oldToNew := make(map[int]int, len(p.Nodes))
	out := Path{
		Kind:   p.Kind,
		CaseID: p.CaseID,
		Nodes:  make([]NodeRef, 0, len(p.Nodes)),
		Hops:   make([]Hop, 0, len(p.Hops)),
	}

	for i, node := range p.Nodes {
		if depths[i] > maxDepth {
			continue
		}
		oldToNew[i] = len(out.Nodes)
		out.Nodes = append(out.Nodes, node)
	}
	for i, hop := range p.Hops {
		nodeIndex := i + 1
		if depths[nodeIndex] > maxDepth {
			continue
		}
		out.Hops = append(out.Hops, Hop{
			FromIndex: oldToNew[hop.FromIndex],
			EdgeType:  hop.EdgeType,
			Reverse:   hop.Reverse,
		})
	}

	return out
}
