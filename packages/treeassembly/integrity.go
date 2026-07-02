package treeassembly

import "github.com/YASSERRMD/verdex/packages/irac"

// ValidateIntegrity wraps irac.ValidateTree over an assembled Tree's
// nodes and edges, returning every structural integrity problem found:
// dangling edges, illegal edge triples, self-loops, unknown node/edge
// types, and ConclusionNodes missing their mandatory draft_analysis
// guardrail label (see packages/irac/validate.go). It does not fail
// fast — every issue across the whole tree is collected and returned in
// one pass, mirroring ValidateTree's own contract.
//
// A nil tree is treated as having no nodes or edges, and so validates
// with no issues (an empty tree is trivially structurally valid; callers
// wanting to reject empty trees should check ErrEmptyInput at
// composition time instead).
func ValidateIntegrity(tree *Tree) []irac.ValidationIssue {
	if tree == nil {
		return irac.ValidateTree(nil, nil)
	}
	return irac.ValidateTree(tree.Nodes, tree.Edges)
}

// HasCriticalIntegrityFailure reports whether issues contains at least
// one ValidationIssue that should block persistence of the tree it was
// computed from. Every irac.ValidationIssue kind is currently treated as
// critical: a dangling edge, illegal edge triple, self-loop, unknown
// node/edge type, or missing guardrail label all indicate the tree
// cannot be trusted as an accurate representation of the case's
// reasoning, per CONTRIBUTING.md's guardrail spirit.
func HasCriticalIntegrityFailure(issues []irac.ValidationIssue) bool {
	return len(issues) > 0
}
