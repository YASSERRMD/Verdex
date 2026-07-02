package knowledgeisolation

import "github.com/YASSERRMD/verdex/packages/irac"

// CaseID identifies the case a node, edge, or retrieval query belongs to.
// Defined locally as a plain string (rather than imported from another
// package) to match every sibling retrieval-layer package's convention
// (e.g. hybridretrieval.HybridQuery.CaseID, traversal.Query.CaseID,
// irac.Node.CaseID) of keeping this identifier an opaque, dependency-free
// string type.
type CaseID = string

// NodeScope classifies whether an irac.Node's data is private to a single
// case or shared across every case in a tenant.
type NodeScope int

const (
	// ScopeCaseFacts marks a node as belonging to exactly one case's
	// reasoning tree (Issue, Fact, Application, Conclusion nodes). A
	// CaseScopedStore rejects any attempt to read or link such a node
	// from a different case's authorized scope.
	ScopeCaseFacts NodeScope = iota

	// ScopeSharedLaw marks a node as part of the shared statute/
	// precedent corpus (Rule nodes). Shared-law nodes carry a CaseID in
	// irac.Node.CaseID for provenance (which case first ingested or
	// referenced them), but that field is not an ownership boundary:
	// any case within the tenant may read a shared-law node.
	ScopeSharedLaw
)

// String returns a human-readable label for s, used in audit records and
// error messages.
func (s NodeScope) String() string {
	switch s {
	case ScopeCaseFacts:
		return "case_facts"
	case ScopeSharedLaw:
		return "shared_law"
	default:
		return "unknown"
	}
}

// ClassifyNodeType returns the NodeScope for a given irac.NodeType.
// irac.NodeRule is the shared statute/precedent corpus (ScopeSharedLaw);
// every other recognized NodeType (Issue, Fact, Application, Conclusion)
// is case-scoped (ScopeCaseFacts). An unrecognized NodeType is treated as
// case-scoped, the more restrictive default, so an isolation guard never
// accidentally widens access for a node type it does not understand.
func ClassifyNodeType(t irac.NodeType) NodeScope {
	if t == irac.NodeRule {
		return ScopeSharedLaw
	}
	return ScopeCaseFacts
}

// IsSharedLawNode reports whether node belongs to the shared statute/
// precedent corpus rather than to a single case's private facts. It is a
// thin convenience wrapper around ClassifyNodeType(node.Type).
func IsSharedLawNode(node irac.Node) bool {
	return ClassifyNodeType(node.Type) == ScopeSharedLaw
}
