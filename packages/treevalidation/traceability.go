package treevalidation

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeConclusionNotTraceable flags a ConclusionNode that does not walk,
// via the tree's edges, to at least one irac.FactNode and at least one
// irac.RuleNode.
const CodeConclusionNotTraceable = "conclusion_not_traceable"

// CheckConclusionTraceability enforces the rule that every ConclusionNode
// in tree must trace — by walking edges Conclusion --concludes_from-->
// Application --applies_to--> Fact/Rule (see packages/irac/edge.go's
// legal-triple table) — to at least one irac.FactNode AND at least one
// irac.RuleNode. This is deeper than generic orphan detection (orphan.go):
// a conclusion can have edges and still fail to reach both a fact and a
// rule, e.g. if it only concludes from an application that applies a rule
// but cites no supporting fact.
//
// A nil tree yields no findings. Order is deterministic: input order of
// tree.Nodes, restricted to ConclusionNodes.
func CheckConclusionTraceability(tree treeassembly.Tree) []Finding {
	findings := make([]Finding, 0)

	// applicationsFromConclusion[conclusionID] = set of application IDs
	// reached via Conclusion --concludes_from--> Application.
	applicationsFromConclusion := make(map[string]map[string]struct{})
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeConcludesFrom {
			continue
		}
		if applicationsFromConclusion[e.FromID] == nil {
			applicationsFromConclusion[e.FromID] = make(map[string]struct{})
		}
		applicationsFromConclusion[e.FromID][e.ToID] = struct{}{}
	}

	// factsAndRulesFromApplication[applicationID] = (reachesFact,
	// reachesRule), via Application --applies_to--> Fact/Rule.
	nodeTypeByID := make(map[string]irac.NodeType, len(tree.Nodes))
	for _, n := range tree.Nodes {
		nodeTypeByID[n.GetID()] = n.GetType()
	}

	appliesFromApplication := make(map[string]map[string]struct{})
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeAppliesTo {
			continue
		}
		if appliesFromApplication[e.FromID] == nil {
			appliesFromApplication[e.FromID] = make(map[string]struct{})
		}
		appliesFromApplication[e.FromID][e.ToID] = struct{}{}
	}

	for _, n := range tree.Nodes {
		conclusion, ok := n.(irac.ConclusionNode)
		if !ok {
			continue
		}

		reachesFact := false
		reachesRule := false

		for appID := range applicationsFromConclusion[conclusion.ID] {
			for targetID := range appliesFromApplication[appID] {
				switch nodeTypeByID[targetID] {
				case irac.NodeFact:
					reachesFact = true
				case irac.NodeRule:
					reachesRule = true
				}
			}
		}

		if !reachesFact || !reachesRule {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				Code:     CodeConclusionNotTraceable,
				Message: fmt.Sprintf(
					"conclusion %q does not trace to both a fact and a rule (reaches_fact=%t, reaches_rule=%t)",
					conclusion.ID, reachesFact, reachesRule,
				),
				NodeID: conclusion.ID,
			})
		}
	}

	return findings
}
