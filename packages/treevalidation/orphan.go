package treevalidation

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeOrphanNode flags a node with zero incoming and zero outgoing edges
// of any kind.
const CodeOrphanNode = "orphan_node"

// DetectOrphans returns a Finding for every node in tree that has neither
// an incoming nor an outgoing edge. This is distinct from
// CheckConclusionTraceability: a node can have edges (so it is not an
// orphan) and still fail traceability (e.g. a conclusion that concludes
// from an application which only applies a rule, never a fact). Orphan
// detection instead catches nodes that are wholly disconnected from the
// rest of the tree — e.g. a FactNode nobody's application ever cites, or
// an IssueNode no rule governs.
//
// A nil tree yields no findings. Order is deterministic: input order of
// tree.Nodes.
func DetectOrphans(tree treeassembly.Tree) []Finding {
	findings := make([]Finding, 0)

	connected := make(map[string]struct{}, len(tree.Nodes))
	for _, e := range tree.Edges {
		connected[e.FromID] = struct{}{}
		connected[e.ToID] = struct{}{}
	}

	for _, n := range tree.Nodes {
		if _, ok := connected[n.GetID()]; ok {
			continue
		}
		findings = append(findings, Finding{
			Severity: SeverityCritical,
			Code:     CodeOrphanNode,
			Message:  fmt.Sprintf("node %q (%s) has no incoming or outgoing edges", n.GetID(), n.GetType()),
			NodeID:   n.GetID(),
		})
	}

	return findings
}
