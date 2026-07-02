package firstpartyagent

import (
	"context"
	"fmt"
	"sort"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// maxTreeNodesFetched bounds a single GetTree page, mirroring
// packages/issueagent/fetch.go's own constant and rationale: a case's
// reasoning tree at Verdex's target scale is expected to comfortably fit
// within this page.
const maxTreeNodesFetched = 5000

// maxFactsPerIssue and maxRulesPerIssue bound how much evidence is
// rendered into a single issue's prompt block, keeping the rendered
// prompt within the argument-construction template's issues_block MaxLen
// and giving the model a focused, ranked evidence set rather than an
// unbounded dump of every fact/rule the tree happens to contain.
const (
	maxFactsPerIssue = 20
	maxRulesPerIssue = 10
)

// issueEvidence is one FramedIssue paired with the FactNodes/RuleNodes
// from the case's tree available as evidence for constructing an
// argument on it.
type issueEvidence struct {
	Issue issueagent.FramedIssue
	Facts []knowledgeapi.NodeDTO
	Rules []knowledgeapi.NodeDTO
}

// allowedNodeIDs returns the set of every FactNode/RuleNode ID present in
// ev, the exact set an Argument on this issue is permitted to cite. Used
// both to render the prompt's evidence list and, downstream, as the
// ground-truth set ground.go validates the model's response against.
func (ev issueEvidence) allowedNodeIDs() map[string]struct{} {
	out := make(map[string]struct{}, len(ev.Facts)+len(ev.Rules))
	for _, f := range ev.Facts {
		out[f.ID] = struct{}{}
	}
	for _, r := range ev.Rules {
		out[r.ID] = struct{}{}
	}
	return out
}

// fetchIssueEvidence pulls the case's full tree once via api.GetTree and,
// for every issue in issues, resolves:
//   - governing RuleNodes (Rule --governs--> Issue, matching
//     packages/issueagent's own resolution of this edge direction), and
//   - FactNodes reachable from those rules through the tree's
//     Application layer (Rule <--applies_to-- Application --applies_to--> Fact
//     is not itself a legal edge triple; instead Application applies_to
//     both Fact and Rule independently, and Fact supports Application —
//     see irac/edge.go's legalEdgeTriples). An ApplicationNode that
//     applies_to a given governing rule is treated as reasoning "about"
//     that rule, and every FactNode supporting that Application is
//     treated as evidence for the issue the rule governs.
//
// When an issue has no governing rules (and therefore no
// Application-linked facts can be resolved structurally), every FactNode
// in the case's tree is offered as a fallback candidate, ranked by
// confidence and capped at maxFactsPerIssue, so an issue with thin rule
// linkage still gets some evidence to argue from rather than none.
//
// This never mutates the tree and never talks to packages/graph,
// packages/treeindex, or packages/traversal directly — knowledgeapi is
// the only access path, matching packages/issueagent's own convention.
func fetchIssueEvidence(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string, issues []issueagent.FramedIssue) ([]issueEvidence, error) {
	tree, err := api.GetTree(ctx, knowledgeapi.GetTreeRequest{
		CaseID: caseID,
		Page:   knowledgeapi.PageRequest{Page: 1, PerPage: maxTreeNodesFetched},
	})
	if err != nil {
		return nil, fmt.Errorf("firstpartyagent: fetch case tree: %w", err)
	}

	idx := buildTreeIndex(tree)

	out := make([]issueEvidence, 0, len(issues))
	for _, issue := range issues {
		rules := idx.governingRules(issue.SourceIssueNodeID)
		facts := idx.factsForRules(rules)
		if len(facts) == 0 {
			facts = idx.allFactsFallback()
		}

		sortNodesByConfidenceDesc(facts)
		sortNodesByConfidenceDesc(rules)
		if len(facts) > maxFactsPerIssue {
			facts = facts[:maxFactsPerIssue]
		}
		if len(rules) > maxRulesPerIssue {
			rules = rules[:maxRulesPerIssue]
		}

		out = append(out, issueEvidence{Issue: issue, Facts: facts, Rules: rules})
	}
	return out, nil
}

// treeIndex is an in-memory index over one GetTree response, built once
// per fetchIssueEvidence call and reused across every issue.
type treeIndex struct {
	nodesByID            map[string]knowledgeapi.NodeDTO
	rulesGoverningIssue  map[string][]string // issueID -> ruleIDs
	applicationsForRule  map[string][]string // ruleID -> applicationIDs (Application --applies_to--> Rule)
	factsSupportingAppln map[string][]string // applicationID -> factIDs (Fact --supports--> Application)
	allFacts             []knowledgeapi.NodeDTO
}

func buildTreeIndex(tree knowledgeapi.GetTreeResponse) *treeIndex {
	idx := &treeIndex{
		nodesByID:            make(map[string]knowledgeapi.NodeDTO, len(tree.Nodes)),
		rulesGoverningIssue:  make(map[string][]string),
		applicationsForRule:  make(map[string][]string),
		factsSupportingAppln: make(map[string][]string),
	}
	for _, n := range tree.Nodes {
		idx.nodesByID[n.ID] = n
		if n.Type == string(irac.NodeFact) {
			idx.allFacts = append(idx.allFacts, n)
		}
	}
	for _, e := range tree.Edges {
		switch irac.EdgeType(e.Type) {
		case irac.EdgeGoverns:
			// Rule --governs--> Issue
			idx.rulesGoverningIssue[e.ToID] = append(idx.rulesGoverningIssue[e.ToID], e.FromID)
		case irac.EdgeAppliesTo:
			// Application --applies_to--> Rule (target side only; Fact
			// targets are irrelevant to this index, applications are
			// linked to facts via EdgeSupports instead).
			if target, ok := idx.nodesByID[e.ToID]; ok && target.Type == string(irac.NodeRule) {
				idx.applicationsForRule[e.ToID] = append(idx.applicationsForRule[e.ToID], e.FromID)
			}
		case irac.EdgeSupports:
			// Fact --supports--> Application
			idx.factsSupportingAppln[e.ToID] = append(idx.factsSupportingAppln[e.ToID], e.FromID)
		case irac.EdgeConcludesFrom:
			// Not relevant to evidence gathering.
		}
	}
	return idx
}

// governingRules returns every RuleNode governing issueID, resolved via
// the tree's own EdgeGoverns edges.
func (idx *treeIndex) governingRules(issueID string) []knowledgeapi.NodeDTO {
	var out []knowledgeapi.NodeDTO
	for _, ruleID := range idx.rulesGoverningIssue[issueID] {
		if n, ok := idx.nodesByID[ruleID]; ok && n.Type == string(irac.NodeRule) {
			out = append(out, n)
		}
	}
	return out
}

// factsForRules returns every FactNode supporting an ApplicationNode that
// applies one of rules, deduplicated by node ID.
func (idx *treeIndex) factsForRules(rules []knowledgeapi.NodeDTO) []knowledgeapi.NodeDTO {
	seen := make(map[string]struct{})
	var out []knowledgeapi.NodeDTO
	for _, rule := range rules {
		for _, applicationID := range idx.applicationsForRule[rule.ID] {
			for _, factID := range idx.factsSupportingAppln[applicationID] {
				if _, ok := seen[factID]; ok {
					continue
				}
				n, ok := idx.nodesByID[factID]
				if !ok || n.Type != string(irac.NodeFact) {
					continue
				}
				seen[factID] = struct{}{}
				out = append(out, n)
			}
		}
	}
	return out
}

// allFactsFallback returns every FactNode in the tree, for an issue whose
// rule linkage is too thin to resolve any Application-linked facts
// structurally.
func (idx *treeIndex) allFactsFallback() []knowledgeapi.NodeDTO {
	out := make([]knowledgeapi.NodeDTO, len(idx.allFacts))
	copy(out, idx.allFacts)
	return out
}

// sortNodesByConfidenceDesc sorts nodes by Confidence descending,
// breaking ties by ID for deterministic output across repeated runs over
// identical input.
func sortNodesByConfidenceDesc(nodes []knowledgeapi.NodeDTO) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Confidence != nodes[j].Confidence {
			return nodes[i].Confidence > nodes[j].Confidence
		}
		return nodes[i].ID < nodes[j].ID
	})
}
