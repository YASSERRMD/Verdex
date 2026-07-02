package treeindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// caseEdgeIndex indexes a case's edges by ToID for repeated reverse-hop
// lookups by the chain-assembly logic below. Building this once per
// RebuildCase call avoids an O(edges) scan for every hop of every issue's
// chain. Only reverse (ToID-keyed) lookups are needed: every hop
// buildReasoningChainPaths walks — EdgeGoverns, EdgeAppliesTo,
// EdgeSupports, EdgeConcludesFrom — is walked in reverse of its declared
// direction (see doc.go), so a forward (FromID-keyed) index is unused
// here.
type caseEdgeIndex struct {
	inbound map[string][]irac.Edge
}

// newCaseEdgeIndex indexes edges by their ToID endpoint.
func newCaseEdgeIndex(edges []irac.Edge) *caseEdgeIndex {
	idx := &caseEdgeIndex{
		inbound: make(map[string][]irac.Edge),
	}
	for _, e := range edges {
		idx.inbound[e.ToID] = append(idx.inbound[e.ToID], e)
	}
	return idx
}

// inEdgesOfType returns every edge of edgeType ending at nodeID, i.e.
// edges that must be walked in Reverse (To -> From) to reach nodeID's
// logical "successor" in a human-meaningful chain.
func (idx *caseEdgeIndex) inEdgesOfType(nodeID string, edgeType irac.EdgeType) []irac.Edge {
	var in []irac.Edge
	for _, e := range idx.inbound[nodeID] {
		if e.Type == edgeType {
			in = append(in, e)
		}
	}
	return in
}

// buildReasoningChainPaths materializes one PathKindReasoningChain Path
// per (IssueNode, governing RuleNode, applying ApplicationNode) triple
// found in caseID, each chain running:
//
//	Issue --governs(reverse)--> Rule --applies_to(reverse)--> Application
//	  --supports(reverse)--> Fact*
//	  --concludes_from(reverse)--> Conclusion*
//
// This is the concrete "rule -> application -> conclusion" path the phase
// plan calls for. Two of the four legal edge triples point from the
// derived node back to what it derives from rather than cause to effect
// (EdgeSupports is Fact --supports--> Application, and EdgeConcludesFrom
// is Conclusion --concludes_from--> Application), so this function walks
// those edges against their declared direction (Hop.Reverse = true) to
// assemble a human-meaningful Issue-to-outcome chain; EdgeGoverns
// (Rule --governs--> Issue) and EdgeAppliesTo (Application --applies_to-->
// Rule) are likewise walked in reverse since the chain starts from the
// Issue, not the Rule. See doc.go and doc/tree-indexing.md.
//
// A chain fans out at the Application step: one Application can support
// multiple Facts and be concluded-from by multiple Conclusions. Rather
// than materialize the combinatorial cross product, each chain includes
// every Fact and every Conclusion reachable from its Application as
// sibling tail nodes appended in a fixed order (Facts, then Conclusions);
// Hops records each one as a separate reverse hop from the Application.
func buildReasoningChainPaths(ctx context.Context, store graph.GraphStore, caseID string) ([]Path, error) {
	edges, err := loadEdges(ctx, store, caseID)
	if err != nil {
		return nil, fmt.Errorf("treeindex: build reasoning chain paths for case %q: %w", caseID, err)
	}
	edgeIdx := newCaseEdgeIndex(edges)

	issueNodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID, NodeType: irac.NodeIssue})
	if err != nil {
		return nil, fmt.Errorf("treeindex: build reasoning chain paths for case %q: traverse issues: %w", caseID, err)
	}

	var paths []Path
	for _, issue := range issueNodes {
		// Rule --governs--> Issue: walk in reverse from the Issue to find
		// its governing Rule(s).
		for _, governs := range edgeIdx.inEdgesOfType(issue.ID, irac.EdgeGoverns) {
			ruleNode, err := store.GetNode(ctx, governs.FromID)
			if err != nil {
				return nil, fmt.Errorf("treeindex: build reasoning chain paths for case %q: get rule node %q: %w", caseID, governs.FromID, err)
			}

			// Application --applies_to--> Rule: walk in reverse from the
			// Rule to find the Application(s) applying it.
			for _, appliesTo := range edgeIdx.inEdgesOfType(ruleNode.ID, irac.EdgeAppliesTo) {
				applicationNode, err := store.GetNode(ctx, appliesTo.FromID)
				if err != nil {
					return nil, fmt.Errorf("treeindex: build reasoning chain paths for case %q: get application node %q: %w", caseID, appliesTo.FromID, err)
				}

				path, err := assembleChainFromApplication(ctx, store, edgeIdx, caseID, nodeRefFromNode(issue), nodeRefFromNode(ruleNode), nodeRefFromNode(applicationNode))
				if err != nil {
					return nil, err
				}
				paths = append(paths, path)
			}
		}
	}

	return paths, nil
}

// assembleChainFromApplication appends the Fact and Conclusion tail nodes
// reachable from applicationRef to the Issue -> Rule -> Application head,
// producing one complete PathKindReasoningChain Path.
func assembleChainFromApplication(ctx context.Context, store graph.GraphStore, edgeIdx *caseEdgeIndex, caseID string, issueRef, ruleRef, applicationRef NodeRef) (Path, error) {
	const (
		issueIndex       = 0
		ruleIndex        = 1
		applicationIndex = 2
	)

	nodes := []NodeRef{issueRef, ruleRef, applicationRef}
	hops := []Hop{
		{FromIndex: issueIndex, EdgeType: irac.EdgeGoverns, Reverse: true},
		{FromIndex: ruleIndex, EdgeType: irac.EdgeAppliesTo, Reverse: true},
	}

	// Fact --supports--> Application: walk in reverse from the Application
	// to find the Fact(s) supporting it.
	for _, supports := range edgeIdx.inEdgesOfType(applicationRef.ID, irac.EdgeSupports) {
		factNode, err := store.GetNode(ctx, supports.FromID)
		if err != nil {
			return Path{}, fmt.Errorf("treeindex: build reasoning chain paths for case %q: get fact node %q: %w", caseID, supports.FromID, err)
		}
		nodes = append(nodes, nodeRefFromNode(factNode))
		hops = append(hops, Hop{FromIndex: applicationIndex, EdgeType: irac.EdgeSupports, Reverse: true})
	}

	// Conclusion --concludes_from--> Application: walk in reverse from the
	// Application to find the Conclusion(s) drawn from it.
	for _, concludesFrom := range edgeIdx.inEdgesOfType(applicationRef.ID, irac.EdgeConcludesFrom) {
		conclusionNode, err := store.GetNode(ctx, concludesFrom.FromID)
		if err != nil {
			return Path{}, fmt.Errorf("treeindex: build reasoning chain paths for case %q: get conclusion node %q: %w", caseID, concludesFrom.FromID, err)
		}
		nodes = append(nodes, nodeRefFromNode(conclusionNode))
		hops = append(hops, Hop{FromIndex: applicationIndex, EdgeType: irac.EdgeConcludesFrom, Reverse: true})
	}

	return Path{
		Kind:   PathKindReasoningChain,
		CaseID: caseID,
		Nodes:  nodes,
		Hops:   hops,
	}, nil
}
