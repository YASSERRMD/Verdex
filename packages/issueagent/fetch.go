package issueagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// maxIssuesPerAnalysis bounds how many IssueNodes a single run frames, to
// keep the rendered prompt within the framing template's issues_block
// MaxLen and avoid an unbounded model call for a pathologically large
// tree.
const maxIssuesPerAnalysis = 100

// issueContext is one IssueNode plus the governing RuleNodes already
// linked to it in the case's reasoning tree (Rule --governs--> Issue, see
// irac.EdgeGoverns), gathered from knowledgeapi ahead of the model call.
type issueContext struct {
	Node          knowledgeapi.NodeDTO
	GoverningRule []knowledgeapi.NodeDTO
}

// fetchIssueContexts pulls the full case tree via api.GetTree (nodes and
// edges together, in one call) and locally resolves, for every IssueNode,
// the RuleNodes governing it. It never mutates the tree — read-only,
// exactly the access pattern packages/knowledgeapi's GetTree method is
// meant for.
//
// Governing rules are resolved from the tree's own Edges rather than via
// api.LookupPaths, because irac.EdgeGoverns edges point Rule -> Issue
// (see irac/edge.go's legalEdgeTriples): LookupPaths only walks forward
// from a given start node, so looking "from" the issue node would never
// find the rule that governs it. Fetching the whole tree once and
// filtering edges locally is also cheaper than one LookupPaths round trip
// per issue.
//
// Returns ErrNoIssueNodes if the tree has no nodes of type irac.NodeIssue.
func fetchIssueContexts(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string) ([]issueContext, error) {
	tree, err := api.GetTree(ctx, knowledgeapi.GetTreeRequest{
		CaseID: caseID,
		Page:   knowledgeapi.PageRequest{Page: 1, PerPage: maxTreeNodesFetched},
	})
	if err != nil {
		return nil, fmt.Errorf("issueagent: fetch case tree: %w", err)
	}

	nodesByID := make(map[string]knowledgeapi.NodeDTO, len(tree.Nodes))
	var issues []knowledgeapi.NodeDTO
	for _, n := range tree.Nodes {
		nodesByID[n.ID] = n
		if n.Type == string(irac.NodeIssue) {
			issues = append(issues, n)
		}
	}
	if len(issues) == 0 {
		return nil, ErrNoIssueNodes
	}
	if len(issues) > maxIssuesPerAnalysis {
		issues = issues[:maxIssuesPerAnalysis]
	}

	// governingRulesByIssue maps an IssueNode ID to every RuleNode ID with
	// a governs edge pointing at it (Rule --governs--> Issue).
	governingRulesByIssue := make(map[string][]string)
	for _, e := range tree.Edges {
		if e.Type != string(irac.EdgeGoverns) {
			continue
		}
		governingRulesByIssue[e.ToID] = append(governingRulesByIssue[e.ToID], e.FromID)
	}

	out := make([]issueContext, 0, len(issues))
	for _, issueNode := range issues {
		ruleIDs := governingRulesByIssue[issueNode.ID]
		rules := make([]knowledgeapi.NodeDTO, 0, len(ruleIDs))
		for _, ruleID := range ruleIDs {
			if rule, ok := nodesByID[ruleID]; ok && rule.Type == string(irac.NodeRule) {
				rules = append(rules, rule)
			}
		}
		out = append(out, issueContext{Node: issueNode, GoverningRule: rules})
	}
	return out, nil
}

// maxTreeNodesFetched bounds a single GetTree page. A case's reasoning
// tree in Verdex's target scale is expected to comfortably fit within
// this page; a future phase can add cursor-following if that assumption
// stops holding.
const maxTreeNodesFetched = 5000
