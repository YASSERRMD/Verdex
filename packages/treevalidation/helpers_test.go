package treevalidation

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// testProvenance builds a minimal valid irac.Provenance referencing
// upstream, mirroring packages/treeassembly's own test helper
// convention.
func testProvenance(upstream ...string) irac.Provenance {
	return irac.Provenance{
		GeneratedBy:     "treevalidation-test",
		GeneratedAt:     time.Now(),
		UpstreamNodeIDs: upstream,
	}
}

// testSpan returns a single valid irac.SourceSpan, for nodes that must
// carry source-span backing to pass FlagUnsupportedClaims.
func testSpan() irac.SourceSpan {
	return irac.SourceSpan{Start: 0, End: 10}
}

// cleanTree builds a minimal, fully valid IRAC reasoning tree for caseID
// in jurisdictionCode: one Issue, one Rule governing it, one Fact, one
// Application applying the Rule to the Fact, and one Conclusion
// concluding from the Application. Every node carries source spans and
// confidence 0.9. This is the baseline "nothing wrong" tree every check
// in this package must NOT flag.
func cleanTree(caseID, jurisdictionCode string) treeassembly.Tree {
	now := time.Now()

	issue := irac.NewIssueNode("issue-1", caseID, "Was the contract breached?", now, 0.9, testProvenance(), testSpan())
	rule := irac.NewRuleNode("rule-1", caseID, "A contract is breached when a party fails to perform.", jurisdictionCode, "common_law", now, 0.9, testProvenance(issue.ID), testSpan())
	fact := irac.NewFactNode("fact-1", caseID, "The seller did not deliver the goods.", now, 0.9, testProvenance(), testSpan())
	app := irac.NewApplicationNode("app-1", caseID, "The seller's non-delivery satisfies the breach rule.", now, 0.9, testProvenance(rule.ID, fact.ID), testSpan())
	conclusion := irac.NewConclusionNode("conclusion-1", caseID, "The contract was likely breached.", now, 0.8, testProvenance(app.ID), testSpan())

	nodes := []irac.NodeLike{issue, rule, fact, app, conclusion}
	edges := []irac.Edge{
		{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns},
		{FromID: app.ID, ToID: rule.ID, Type: irac.EdgeAppliesTo},
		{FromID: app.ID, ToID: fact.ID, Type: irac.EdgeAppliesTo},
		{FromID: fact.ID, ToID: app.ID, Type: irac.EdgeSupports},
		{FromID: conclusion.ID, ToID: app.ID, Type: irac.EdgeConcludesFrom},
	}

	return treeassembly.Tree{
		Nodes:    nodes,
		Edges:    edges,
		Revision: irac.NewInitialRevision(caseID, now),
	}
}

// treeassemblyTreeOf wraps an arbitrary set of nodes (and, optionally,
// edges) into a treeassembly.Tree, for tests that need a tree smaller or
// differently shaped than cleanTree's baseline.
func treeassemblyTreeOf(nodes ...irac.NodeLike) treeassembly.Tree {
	return treeassembly.Tree{Nodes: nodes}
}

// treeWithEdges wraps nodes and edges into a treeassembly.Tree.
func treeWithEdges(nodes []irac.NodeLike, edges []irac.Edge) treeassembly.Tree {
	return treeassembly.Tree{Nodes: nodes, Edges: edges}
}
