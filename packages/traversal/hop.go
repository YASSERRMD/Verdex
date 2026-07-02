package traversal

import "github.com/YASSERRMD/verdex/packages/irac"

// Direction states whether a HopSpec should be walked along an
// irac.Edge's declared FromID -> ToID direction, or against it.
//
// packages/treeindex's doc/tree-indexing.md documents that two of the
// four legal irac edge triples point from the derived node back to what
// it derives from (EdgeSupports is Fact --supports--> Application, and
// EdgeConcludesFrom is Conclusion --concludes_from--> Application), so a
// human-meaningful traversal (e.g. "Issue, then what governs it") must
// walk some edges in Reverse. This package makes that choice explicit
// and per-hop, rather than baking in one fixed direction per EdgeType.
type Direction int

const (
	// Forward walks an edge along its declared FromID -> ToID direction.
	Forward Direction = iota

	// Reverse walks an edge against its declared direction (ToID -> FromID).
	Reverse
)

// String returns "forward" or "reverse".
func (d Direction) String() string {
	if d == Reverse {
		return "reverse"
	}
	return "forward"
}

// HopKind names one step of a multi-hop traversal. A HopKind does not
// change how a hop is walked (that's EdgeType + Direction); it labels
// *why* the hop is being walked, so a rendered TraversalResult can explain
// itself (see Path.Explain) and so a caller building a Query with the
// named convenience methods (ViaGoverningRule, ViaControllingPrecedent,
// ViaDistinguishingFacts) gets a self-documenting Path back.
type HopKind string

const (
	// HopKindCustom labels a hop added via the general-purpose Via method:
	// an arbitrary (EdgeType, Direction, NodeType filter) step not
	// otherwise named below.
	HopKindCustom HopKind = "custom"

	// HopKindGoverningRule labels the "issue -> governing rule" hop: walk
	// irac.EdgeGoverns in Reverse from an IssueNode to reach the
	// RuleNode(s) that govern it (Rule --governs--> Issue is the edge's
	// declared direction, so reaching the rule from the issue is a
	// Reverse walk — consistent with packages/treeindex's chain.go).
	HopKindGoverningRule HopKind = "governing_rule"

	// HopKindControllingPrecedent labels the "rule -> controlling
	// precedent" hop: from a RuleNode, find precedent-origin RuleNode(s)
	// that control it. The IRAC schema has no dedicated "rule cites
	// precedent" edge, so this hop is resolved via a caller-supplied
	// PrecedentResolver rather than a literal graph edge — see
	// precedent.go for the resolution semantics and why this package does
	// not import packages/application, packages/statute, or
	// packages/precedent directly.
	HopKindControllingPrecedent HopKind = "controlling_precedent"

	// HopKindDistinguishingFacts labels the "precedent -> distinguishing
	// facts" hop: from a precedent-origin RuleNode, find FactNode(s) in
	// the current case whose text diverges from the precedent's typical
	// fact pattern. Like HopKindControllingPrecedent, this has no direct
	// graph edge and is resolved via a caller-supplied
	// DistinguishingFactResolver — see distinguish.go.
	HopKindDistinguishingFacts HopKind = "distinguishing_facts"
)

// HopSpec describes one step of a Query's hop sequence. Most HopSpecs
// walk a real irac.Edge (EdgeType set); HopKindControllingPrecedent and
// HopKindDistinguishingFacts instead invoke a caller-supplied resolver
// function (see precedent.go / distinguish.go) since the schema has no
// literal edge for either relationship.
type HopSpec struct {
	// Kind labels why this hop exists (see HopKind).
	Kind HopKind

	// EdgeType is the irac.EdgeType to walk. Ignored (may be empty) for
	// resolver-backed hops (HopKindControllingPrecedent,
	// HopKindDistinguishingFacts).
	EdgeType irac.EdgeType

	// Direction states whether EdgeType is walked Forward or Reverse.
	// Ignored for resolver-backed hops.
	Direction Direction

	// NodeTypeFilter, if non-empty, restricts this hop's resulting nodes
	// to the given irac.NodeType. Empty means "no filter".
	NodeTypeFilter irac.NodeType
}
