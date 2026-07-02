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

// Hop describes one edge traversed between two consecutive NodeRef values
// in a Path. Direction records whether the edge was walked in its
// declared irac.Edge direction (Forward, From -> To) or against it
// (Reverse, To -> From) to assemble a human-meaningful chain — see doc.go
// for why EdgeSupports and EdgeConcludesFrom are walked in Reverse.
type Hop struct {
	// EdgeType is the irac.EdgeType connecting the two hops.
	EdgeType irac.EdgeType

	// Reverse reports whether this hop was walked against the edge's
	// declared irac.Edge.FromID -> irac.Edge.ToID direction.
	Reverse bool
}

// Path is one materialized, ordered chain of NodeRef values discovered by
// walking a case's reasoning tree, plus the Hops connecting each
// consecutive pair. len(Hops) is always len(Nodes)-1.
type Path struct {
	// Kind classifies which materialization strategy produced this Path.
	Kind PathKind

	// CaseID identifies the case this Path belongs to.
	CaseID string

	// Nodes is the ordered chain of node references making up this Path,
	// root first.
	Nodes []NodeRef

	// Hops describes the edge connecting each consecutive pair of Nodes.
	// len(Hops) == len(Nodes)-1.
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

// Depth returns the number of edge hops in the path (len(Nodes)-1), or 0
// for an empty or single-node path.
func (p Path) Depth() int {
	if len(p.Nodes) == 0 {
		return 0
	}
	return len(p.Nodes) - 1
}

// Truncate returns a copy of p containing only the first maxDepth hops
// (i.e. at most maxDepth+1 nodes). maxDepth <= 0 returns p unchanged,
// mirroring graph.TraversalQuery.MaxDepth's "zero means unbounded"
// convention.
func (p Path) Truncate(maxDepth int) Path {
	if maxDepth <= 0 || p.Depth() <= maxDepth {
		return p
	}
	out := Path{
		Kind:   p.Kind,
		CaseID: p.CaseID,
		Nodes:  make([]NodeRef, maxDepth+1),
		Hops:   make([]Hop, maxDepth),
	}
	copy(out.Nodes, p.Nodes[:maxDepth+1])
	copy(out.Hops, p.Hops[:maxDepth])
	return out
}
