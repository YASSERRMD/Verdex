package vectorindex

import (
	"github.com/YASSERRMD/verdex/packages/irac"
)

// JurisdictionCode identifies the jurisdiction an IndexableLeaf derives its
// authority or context from. Opaque string here (no hard dependency on
// packages/jurisdiction), mirroring packages/irac/jurisdiction.go's
// JurisdictionCode/LegalFamily convention: this package is infrastructure
// over packages/irac and packages/graph and must not take on a jurisdiction
// schema dependency just to carry a metadata filter value.
type JurisdictionCode string

// CategoryCode identifies the legal category (e.g. "contract", "tort") an
// IndexableLeaf belongs to. Opaque string here for the same reason as
// JurisdictionCode — no hard dependency on packages/category.
type CategoryCode string

// PartyID identifies a party (e.g. plaintiff, defendant) an IndexableLeaf
// relates to, when known. Opaque string; Verdex has no single canonical
// "party" package this phase depends on.
type PartyID string

// IndexableLeaf is the projection of one content-bearing irac.Node into the
// shape this package embeds and indexes. See ProjectLeaves for the
// projection rule (which NodeTypes qualify and why) and doc/vector-index.md
// for the full rationale.
type IndexableLeaf struct {
	// ID is the underlying irac.Node's ID. Also used as the VectorStore
	// record ID (see store.go), so a leaf's vector record can always be
	// traced back to the exact tree node it was derived from.
	ID string

	// NodeType is the underlying irac.Node's NodeType. Always one of
	// irac.NodeFact, irac.NodeRule, or irac.NodeConclusion — see
	// ProjectLeaves.
	NodeType irac.NodeType

	// CaseID identifies the case this leaf's node belongs to.
	CaseID string

	// JurisdictionCode is the jurisdiction metadata filter value for this
	// leaf. Populated from irac.RuleNode.JurisdictionCode for rule leaves;
	// empty for fact/conclusion leaves unless a caller enriches it (see
	// ProjectionOptions).
	JurisdictionCode JurisdictionCode

	// CategoryCode is the category metadata filter value for this leaf.
	// irac.Node carries no native category field, so this is always
	// populated by the caller via ProjectionOptions rather than derived
	// from the node itself.
	CategoryCode CategoryCode

	// PartyID is the party metadata filter value for this leaf. irac.Node
	// carries no native party field, so this is always populated by the
	// caller via ProjectionOptions rather than derived from the node
	// itself.
	PartyID PartyID

	// Text is the node's human-readable content — the exact text that gets
	// embedded.
	Text string

	// SourceSpans traces this leaf's text back to the ingested source
	// document(s) it was drawn from.
	SourceSpans []irac.SourceSpan
}

// IsLeafNodeType reports whether t is a NodeType this package projects for
// semantic indexing: irac.NodeFact, irac.NodeRule, or irac.NodeConclusion.
//
// The projection rule: IssueNode and ApplicationNode are structural
// scaffolding — they exist to connect a reasoning tree together rather than
// to assert new content — while FactNode, RuleNode, and ConclusionNode are
// the tree's content-bearing leaves: factual assertions, legal rule/
// precedent text, and reasoned outcomes. A semantic-recall query ("find
// similar facts", "find similar rules", "find similar conclusions") is
// looking for one of these three, never a bare issue framing or an
// application's intermediate reasoning step. See doc/vector-index.md.
func IsLeafNodeType(t irac.NodeType) bool {
	switch t {
	case irac.NodeFact, irac.NodeRule, irac.NodeConclusion:
		return true
	case irac.NodeIssue, irac.NodeApplication:
		return false
	default:
		return false
	}
}
