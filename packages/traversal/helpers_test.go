package traversal_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// testProvenance builds a minimal irac.Provenance, mirroring the
// convention used by packages/treeindex's own test helpers.
func testProvenance(upstream ...string) irac.Provenance {
	return irac.Provenance{
		GeneratedBy:     "traversal-test",
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
// concluding from the Application. Mirrors packages/treeindex's
// seedCleanTree fixture.
func seedCleanTree(t *testing.T, store graph.GraphStore, caseID string) (issueID, ruleID, factID, appID, conclusionID string) {
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

	return issue.ID, rule.ID, fact.ID, app.ID, conclusion.ID
}

// seedTwoIssuesOneRule builds two IssueNodes governed by the same
// RuleNode, for tests exercising fan-out at the governing-rule hop.
func seedTwoIssuesOneRule(t *testing.T, store graph.GraphStore, caseID string) (issue1ID, issue2ID, ruleID string) {
	t.Helper()
	now := time.Now()

	issue1 := irac.NewIssueNode(caseID+"-issue-1", caseID, "Was notice given?", now, 0.9, testProvenance(), testSpan())
	issue2 := irac.NewIssueNode(caseID+"-issue-2", caseID, "Was notice timely?", now, 0.9, testProvenance(), testSpan())
	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "Notice must be given within 30 days.", "us-ny", "common_law", now, 0.9, testProvenance(), testSpan())

	mustCreateNode(t, store, issue1.Node)
	mustCreateNode(t, store, issue2.Node)
	mustCreateNode(t, store, rule.Node)

	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue1.ID, Type: irac.EdgeGoverns})
	mustCreateEdge(t, store, irac.Edge{FromID: rule.ID, ToID: issue2.ID, Type: irac.EdgeGoverns})

	return issue1.ID, issue2.ID, rule.ID
}

// newSeededStore returns an InMemoryGraphStore seeded with the standard
// clean tree under caseID "case-1", for tests that only need a valid
// Query to execute against (e.g. validation error-path tests) and don't
// care about the tree's exact shape.
func newSeededStore(t *testing.T) graph.GraphStore {
	t.Helper()
	store := graph.NewInMemoryGraphStore()
	seedCleanTree(t, store, "case-1")
	return store
}

// ctxBackground returns context.Background(), exposed as a package-level
// helper purely to keep test call sites terse.
func ctxBackground() context.Context {
	return context.Background()
}

// timeNow returns the current time, exposed as a package-level helper
// purely to keep test call sites terse.
func timeNow() time.Time {
	return time.Now()
}
