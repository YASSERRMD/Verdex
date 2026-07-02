package treeassembly

import "github.com/YASSERRMD/verdex/packages/irac"

// IncrementalResult bundles the outcome of ReassembleIncremental: the
// updated Tree, plus the validation issues and gaps recomputed over just
// the delta (prev's existing nodes/edges are assumed to have already
// been validated when prev was produced, so they are not re-checked
// here — see ReassembleIncremental's doc comment).
type IncrementalResult struct {
	Tree             *Tree
	ValidationIssues []irac.ValidationIssue
	Gaps             []Gap
}

// ReassembleIncremental extends prev with newFacts and newApplications
// without rebuilding the whole tree from scratch: it appends only the
// new nodes and the edges newly implied by their provenance, bumps the
// revision once (via NextRevision), and runs validation/gap-detection
// only over the affected subset — the new nodes/edges plus any existing
// node they connect to — rather than the full tree.
//
// This mirrors the incremental-evidence reality of a live case: as new
// facts or applications arrive (e.g. a new document is ingested, or
// packages/application re-runs for a newly linked rule), the tree should
// grow rather than being recomputed wholesale each time.
//
// Returns ErrNilPrevTree if prev is nil. If both newFacts and
// newApplications are empty, prev is still revisioned (a new revision
// with no content change), matching NextRevision's "called whenever
// ComposeTree runs again" contract.
func ReassembleIncremental(prev *Tree, newFacts []irac.FactNode, newApplications []irac.ApplicationNode) (*IncrementalResult, error) {
	if prev == nil {
		return nil, ErrNilPrevTree
	}

	existingByID := make(map[string]irac.NodeLike, len(prev.Nodes))
	for _, n := range prev.Nodes {
		existingByID[n.GetID()] = n
	}

	nodes := make([]irac.NodeLike, len(prev.Nodes), len(prev.Nodes)+len(newFacts)+len(newApplications))
	copy(nodes, prev.Nodes)

	edges := make([]irac.Edge, len(prev.Edges))
	copy(edges, prev.Edges)

	deltaNodes := make([]irac.NodeLike, 0, len(newFacts)+len(newApplications))
	deltaEdges := make([]irac.Edge, 0)

	factByID := make(map[string]irac.FactNode)
	for _, n := range prev.Nodes {
		if f, ok := n.(irac.FactNode); ok {
			factByID[f.ID] = f
		}
	}
	for _, f := range newFacts {
		if _, exists := existingByID[f.ID]; exists {
			continue
		}
		nodes = append(nodes, f)
		deltaNodes = append(deltaNodes, f)
		existingByID[f.ID] = f
		factByID[f.ID] = f
	}

	for _, a := range newApplications {
		if _, exists := existingByID[a.ID]; exists {
			continue
		}
		nodes = append(nodes, a)
		deltaNodes = append(deltaNodes, a)
		existingByID[a.ID] = a

		for _, upID := range a.Provenance.UpstreamNodeIDs {
			target, ok := existingByID[upID]
			if !ok {
				continue
			}
			switch target.GetType() {
			case irac.NodeRule:
				e := irac.Edge{FromID: a.ID, ToID: upID, Type: irac.EdgeAppliesTo}
				edges = append(edges, e)
				deltaEdges = append(deltaEdges, e)
			case irac.NodeFact:
				e1 := irac.Edge{FromID: a.ID, ToID: upID, Type: irac.EdgeAppliesTo}
				e2 := irac.Edge{FromID: upID, ToID: a.ID, Type: irac.EdgeSupports}
				edges = append(edges, e1, e2)
				deltaEdges = append(deltaEdges, e1, e2)
			}
		}
	}
	// Also connect any newly added fact to applications already present
	// in prev whose provenance references it (covers facts arriving
	// after the application that will consume them).
	for _, f := range newFacts {
		for _, n := range prev.Nodes {
			app, ok := n.(irac.ApplicationNode)
			if !ok {
				continue
			}
			for _, upID := range app.Provenance.UpstreamNodeIDs {
				if upID != f.ID {
					continue
				}
				e1 := irac.Edge{FromID: app.ID, ToID: f.ID, Type: irac.EdgeAppliesTo}
				e2 := irac.Edge{FromID: f.ID, ToID: app.ID, Type: irac.EdgeSupports}
				edges = append(edges, e1, e2)
				deltaEdges = append(deltaEdges, e1, e2)
			}
		}
	}

	updated := &Tree{Nodes: nodes, Edges: edges, Revision: prev.Revision}
	updated.Revision = NextRevision(updated)

	// Validate/detect gaps only over the delta subgraph: the new nodes
	// plus the edges touching them, rather than the whole tree, per this
	// function's "incremental" contract.
	deltaIssues := irac.ValidateTree(deltaNodes, deltaEdges)

	deltaTree := &Tree{Nodes: deltaNodes, Edges: deltaEdges, Revision: updated.Revision}
	deltaGaps := DetectGaps(deltaTree)

	return &IncrementalResult{
		Tree:             updated,
		ValidationIssues: deltaIssues,
		Gaps:             deltaGaps,
	}, nil
}
