package treeindex

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// buildRuleGroupedIssuePaths materializes one PathKindRuleGroupedIssues
// Path per RuleNode in caseID, each grouping that rule with every
// IssueNode it governs (irac.EdgeGoverns, Rule --governs--> Issue).
//
// This is treeindex's stand-in for "issue -> sub-issue" relatedness: the
// IRAC schema has no literal parent-issue/sub-issue edge (irac.IssueNode
// carries no ParentIssueID — that concept lives only on packages/issue's
// pre-tree CandidateIssue), but issues that share a governing rule are a
// legitimate, schema-faithful notion of "related issues" that IS directly
// backed by a real edge. A RuleNode governing zero issues produces no
// Path (there is nothing to group).
func buildRuleGroupedIssuePaths(ctx context.Context, store graph.GraphStore, caseID string) ([]Path, error) {
	edges, err := loadEdges(ctx, store, caseID)
	if err != nil {
		return nil, fmt.Errorf("treeindex: build rule-grouped issue paths for case %q: %w", caseID, err)
	}

	// governedBy maps a rule node id to every issue node id it governs, in
	// the order the edges were returned.
	governedBy := make(map[string][]string)
	var ruleOrder []string
	for _, e := range edges {
		if e.Type != irac.EdgeGoverns {
			continue
		}
		if _, ok := governedBy[e.FromID]; !ok {
			ruleOrder = append(ruleOrder, e.FromID)
		}
		governedBy[e.FromID] = append(governedBy[e.FromID], e.ToID)
	}

	paths := make([]Path, 0, len(ruleOrder))
	for _, ruleID := range ruleOrder {
		ruleNode, err := store.GetNode(ctx, ruleID)
		if err != nil {
			return nil, fmt.Errorf("treeindex: build rule-grouped issue paths for case %q: get rule node %q: %w", caseID, ruleID, err)
		}

		nodes := make([]NodeRef, 0, len(governedBy[ruleID])+1)
		hops := make([]Hop, 0, len(governedBy[ruleID]))
		nodes = append(nodes, nodeRefFromNode(ruleNode))

		for _, issueID := range governedBy[ruleID] {
			issueNode, err := store.GetNode(ctx, issueID)
			if err != nil {
				return nil, fmt.Errorf("treeindex: build rule-grouped issue paths for case %q: get issue node %q: %w", caseID, issueID, err)
			}
			nodes = append(nodes, nodeRefFromNode(issueNode))
			hops = append(hops, Hop{EdgeType: irac.EdgeGoverns, Reverse: false})
		}

		paths = append(paths, Path{
			Kind:   PathKindRuleGroupedIssues,
			CaseID: caseID,
			Nodes:  nodes,
			Hops:   hops,
		})
	}

	return paths, nil
}
