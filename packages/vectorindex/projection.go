package vectorindex

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// ProjectionOptions supplies the metadata-filter values ProjectLeaves and
// ProjectLeavesFromNodes cannot derive from an irac.Node alone.
// irac.Node (the shape packages/graph.GraphStore actually persists — see
// packages/graph/backup.go's toNode/toNodeLike round-trip) carries no
// native category or party field, and loses a RuleNode's JurisdictionCode
// once flattened to the base Node shape. Callers that know these values
// (e.g. from the case record, or from re-deriving a RuleNode) supply them
// here so IndexableLeaf.CategoryCode / .PartyID / .JurisdictionCode can be
// populated for metadata filtering at query time (see store.go).
type ProjectionOptions struct {
	// CategoryCode is applied to every projected leaf for the case.
	CategoryCode CategoryCode

	// JurisdictionCode is applied to every projected leaf for the case
	// that does not already carry a more specific jurisdiction tag (see
	// ProjectLeavesFromNodes, which prefers a RuleNode's own
	// JurisdictionCode when the richer typed node is available).
	JurisdictionCode JurisdictionCode

	// PartyID is applied to every projected leaf for the case.
	PartyID PartyID
}

// ProjectLeaves reads every node belonging to caseID from store via
// GraphStore.Traverse, and returns the subset that qualifies as an
// indexable leaf (see IsLeafNodeType) projected into IndexableLeaf values.
//
// Because GraphStore persists only the base irac.Node shape (Spans,
// JurisdictionCode, and LegalFamily are not part of that shape — see
// packages/graph/backup.go), leaves projected this way carry no
// SourceSpans and take their JurisdictionCode/CategoryCode/PartyID
// entirely from opts. Callers that have the original typed tree
// (irac.RuleNode, irac.FactNode, irac.ConclusionNode values, e.g. from
// packages/treeassembly's Tree.Nodes before it was persisted) should use
// ProjectLeavesFromNodes instead to retain Spans and a RuleNode's own
// JurisdictionCode.
func ProjectLeaves(ctx context.Context, store graph.GraphStore, caseID string, opts ProjectionOptions) ([]IndexableLeaf, error) {
	if store == nil {
		return nil, ErrNilGraphStore
	}
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}

	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID})
	if err != nil {
		return nil, err
	}

	leaves := make([]IndexableLeaf, 0, len(nodes))
	for _, n := range nodes {
		if !IsLeafNodeType(n.Type) {
			continue
		}
		leaves = append(leaves, IndexableLeaf{
			ID:               n.ID,
			NodeType:         n.Type,
			CaseID:           n.CaseID,
			JurisdictionCode: opts.JurisdictionCode,
			CategoryCode:     opts.CategoryCode,
			PartyID:          opts.PartyID,
			Text:             n.Text,
		})
	}
	return leaves, nil
}

// ProjectLeavesFromNodes projects the indexable leaves out of a slice of
// typed irac.NodeLike values (e.g. packages/treeassembly's Tree.Nodes)
// directly, without going through a GraphStore round-trip. This retains
// each leaf's SourceSpans and, for RuleNode, its own JurisdictionCode —
// both of which are lost once a node has been flattened to the base
// irac.Node shape GraphStore persists (see ProjectLeaves).
//
// opts.JurisdictionCode is used as a fallback for leaves that do not carry
// their own jurisdiction tag (fact and conclusion leaves, and rule leaves
// with an empty JurisdictionCode); a RuleNode's own non-empty
// JurisdictionCode always takes precedence. opts.CategoryCode and
// opts.PartyID are applied to every leaf, since no typed node carries
// either field natively.
func ProjectLeavesFromNodes(nodes []irac.NodeLike, opts ProjectionOptions) []IndexableLeaf {
	leaves := make([]IndexableLeaf, 0, len(nodes))
	for _, n := range nodes {
		if !IsLeafNodeType(n.GetType()) {
			continue
		}
		leaves = append(leaves, leafFromNodeLike(n, opts))
	}
	return leaves
}

// leafFromNodeLike projects a single typed irac.NodeLike into an
// IndexableLeaf, type-switching to extract Spans and (for RuleNode)
// JurisdictionCode — mirroring packages/treevalidation's caseIDOf /
// nodeSpans helper convention of type-switching over the concrete wrapper
// types.
func leafFromNodeLike(n irac.NodeLike, opts ProjectionOptions) IndexableLeaf {
	leaf := IndexableLeaf{
		ID:               n.GetID(),
		NodeType:         n.GetType(),
		JurisdictionCode: opts.JurisdictionCode,
		CategoryCode:     opts.CategoryCode,
		PartyID:          opts.PartyID,
	}

	switch v := n.(type) {
	case irac.RuleNode:
		leaf.CaseID = v.CaseID
		leaf.Text = v.Text
		leaf.SourceSpans = v.Spans
		if v.JurisdictionCode != "" {
			leaf.JurisdictionCode = JurisdictionCode(v.JurisdictionCode)
		}
	case irac.FactNode:
		leaf.CaseID = v.CaseID
		leaf.Text = v.Text
		leaf.SourceSpans = v.Spans
	case irac.ConclusionNode:
		leaf.CaseID = v.CaseID
		leaf.Text = v.Text
		leaf.SourceSpans = v.Spans
	}

	return leaf
}
