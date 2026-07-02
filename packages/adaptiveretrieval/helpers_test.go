package adaptiveretrieval_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// testProvenance builds a minimal irac.Provenance, mirroring the
// convention used by packages/traversal's and packages/treeindex's own
// test helpers.
func testProvenance(upstream ...string) irac.Provenance {
	return irac.Provenance{
		GeneratedBy:     "adaptiveretrieval-test",
		GeneratedAt:     time.Now(),
		UpstreamNodeIDs: upstream,
	}
}

// testSpan returns a single valid irac.SourceSpan.
func testSpan() irac.SourceSpan {
	return irac.SourceSpan{Start: 0, End: 10}
}

// mustCreateNode persists node into store, failing the test on error.
func mustCreateNode(t *testing.T, store graph.GraphStore, node irac.Node) {
	t.Helper()
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode(%s): %v", node.ID, err)
	}
}

// mustCreateEdge persists edge into store, failing the test on error.
func mustCreateEdge(t *testing.T, store graph.GraphStore, edge irac.Edge) {
	t.Helper()
	if err := store.CreateEdge(context.Background(), edge); err != nil {
		t.Fatalf("CreateEdge(%s -> %s): %v", edge.FromID, edge.ToID, err)
	}
}

// seedCleanTree builds a minimal, fully connected IRAC reasoning tree in
// store for caseID: one Issue, one Rule governing it, one Fact, one
// Application applying the Rule to the Fact, and one Conclusion
// concluding from the Application. Mirrors packages/traversal's and
// packages/treeindex's seedCleanTree fixture. Returns the Issue's node ID,
// the only one of the tree's five nodes every adaptiveretrieval test
// anchors a build from.
func seedCleanTree(t *testing.T, store graph.GraphStore, caseID string) (issueID string) {
	t.Helper()
	now := time.Now()

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "Was the contract breached?", now, 0.9, testProvenance(), testSpan())
	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "A contract is breached when a party fails to perform.", "us-ny", "common_law", now, 0.9, testProvenance(issue.ID), testSpan())
	fact := irac.NewFactNode(caseID+"-fact-1", caseID, "The seller did not deliver the goods.", now, 0.9, testProvenance(), testSpan())
	app := irac.NewApplicationNode(caseID+"-app-1", caseID, "The seller's non-delivery satisfies the breach rule.", now, 0.9, testProvenance(rule.ID, fact.ID), testSpan())
	conclusion := irac.NewConclusionNode(caseID+"-conclusion-1", caseID, "The contract was likely breached.", now, 0.8, testProvenance(app.ID), testSpan())

	mustCreateNode(t, store, issue.Node)
	mustCreateNode(t, store, rule.Node)
	mustCreateNode(t, store, fact.Node)
	mustCreateNode(t, store, app.Node)
	mustCreateNode(t, store, conclusion.Node)

	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})
	mustCreateEdge(t, store, irac.Edge{FromID: app.ID, ToID: rule.ID, Type: irac.EdgeAppliesTo})
	mustCreateEdge(t, store, irac.Edge{FromID: app.ID, ToID: fact.ID, Type: irac.EdgeAppliesTo})
	mustCreateEdge(t, store, irac.Edge{FromID: fact.ID, ToID: app.ID, Type: irac.EdgeSupports})
	mustCreateEdge(t, store, irac.Edge{FromID: conclusion.ID, ToID: app.ID, Type: irac.EdgeConcludesFrom})

	return issue.ID
}

// seedLargeFanout builds one IssueNode governed by n distinct RuleNodes
// under caseID, for tests exercising BuildBudget.MaxNodes enforcement
// against a wide (rather than deep) tree. n governing rules are reachable
// in a single hop from issueID via ViaGoverningRule (Issue --reverse
// EdgeGoverns--> Rule), matching DefaultHopSequence's first hop. Returns
// the Issue's node ID, the anchor every caller builds from.
func seedLargeFanout(t *testing.T, store graph.GraphStore, caseID string, n int) (issueID string) {
	t.Helper()
	now := time.Now()

	issue := irac.NewIssueNode(caseID+"-issue-1", caseID, "A wide-fanout issue.", now, 0.9, testProvenance(), testSpan())
	mustCreateNode(t, store, issue.Node)

	for i := 0; i < n; i++ {
		rule := irac.NewRuleNode(caseID+"-rule-"+strconv.Itoa(i), caseID, "Fanout governing rule.", "us-ny", "common_law", now, 0.9, testProvenance(), testSpan())
		mustCreateNode(t, store, rule.Node)
		mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns})
	}
	return issue.ID
}

// newSeededStore returns an InMemoryGraphStore seeded with the standard
// clean tree under caseID "case-1".
func newSeededStore(t *testing.T) graph.GraphStore {
	t.Helper()
	store := graph.NewInMemoryGraphStore()
	seedCleanTree(t, store, "case-1")
	return store
}
