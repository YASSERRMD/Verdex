package treeassembly

import "github.com/YASSERRMD/verdex/packages/irac"

// AssemblyInput bundles the previously extracted IRAC components this
// package composes into one reasoning tree. Every field is produced by
// an upstream phase's service: Issues by packages/issue's
// IssueExtractionService (Phase 033), Facts by packages/fact's
// FactConstructionService (Phase 034), Applications and (transitively)
// the Rules they reference by packages/application's ApplicationService
// (Phases 035-037). This package does not extract or construct any of
// these itself — it only assembles what already exists into a single,
// validated, versioned tree.
type AssemblyInput struct {
	// CaseID identifies the case whose tree is being assembled.
	// Required.
	CaseID string

	// Issues is the case's extracted irac.IssueNodes (Phase 033).
	Issues []irac.IssueNode

	// Rules is the case's linked irac.RuleNodes (Phases 035/036).
	Rules []irac.RuleNode

	// Facts is the case's constructed irac.FactNodes (Phase 034).
	Facts []irac.FactNode

	// Applications is the case's built irac.ApplicationNodes
	// (Phase 037).
	Applications []irac.ApplicationNode
}

// Tree is the assembled IRAC reasoning tree for one case at one
// revision: every node gathered from AssemblyInput (plus any
// ConclusionNodes supplied by a ConclusionProvider — see compose.go),
// every edge connecting them, and the irac.TreeRevision stamping this
// assembly as a specific, versioned snapshot (see revision.go and
// packages/irac/version.go).
type Tree struct {
	// Nodes is every node in the assembled tree, heterogeneous by
	// irac.NodeType.
	Nodes []irac.NodeLike

	// Edges is every edge connecting Nodes, constrained to the legal
	// (FromNodeType, EdgeType, ToNodeType) triples declared in
	// packages/irac/edge.go.
	Edges []irac.Edge

	// Revision identifies this Tree as a specific, versioned snapshot
	// of its case's reasoning tree (see packages/irac/version.go).
	Revision irac.TreeRevision
}
