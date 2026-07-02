package treeindex_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// TestIndexer_ReasoningChain_Correctness seeds the standard clean IRAC
// tree (issue -> rule -> application -> fact/conclusion) and asserts the
// materialized reasoning-chain Path contains every expected node, in the
// documented order, with the documented Hop directions.
func TestIndexer_ReasoningChain_Correctness(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	caseID := "case-chain"

	issueID, ruleID, factID, appID, conclusionID := seedCleanTree(t, store, caseID)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	paths, err := idx.LookupPaths(ctx, caseID, issueID, "")
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}

	var chain *treeindex.Path
	for i := range paths {
		if paths[i].Kind == treeindex.PathKindReasoningChain {
			chain = &paths[i]
			break
		}
	}
	if chain == nil {
		t.Fatalf("expected a reasoning-chain path rooted at the issue, got %+v", paths)
	}

	if got := len(chain.Nodes); got != 5 {
		t.Fatalf("expected 5 nodes (issue, rule, application, fact, conclusion), got %d: %+v", got, chain.Nodes)
	}

	ids := make(map[string]irac.NodeType, len(chain.Nodes))
	for _, n := range chain.Nodes {
		ids[n.ID] = n.Type
	}
	for _, want := range []string{issueID, ruleID, appID, factID, conclusionID} {
		if _, ok := ids[want]; !ok {
			t.Errorf("expected node %q in the reasoning chain, got nodes %+v", want, chain.Nodes)
		}
	}

	if chain.Nodes[0].ID != issueID {
		t.Errorf("expected the chain to be rooted at the issue, got %+v", chain.Nodes[0])
	}
	if chain.Nodes[1].ID != ruleID {
		t.Errorf("expected the rule as the second node, got %+v", chain.Nodes[1])
	}
	if chain.Nodes[2].ID != appID {
		t.Errorf("expected the application as the third node, got %+v", chain.Nodes[2])
	}

	// The first two hops (issue->rule, rule->application) must be recorded
	// as Reverse, since EdgeGoverns is Rule->Issue and EdgeAppliesTo is
	// Application->Rule.
	if !chain.Hops[0].Reverse || chain.Hops[0].EdgeType != irac.EdgeGoverns {
		t.Errorf("expected hop 0 to be a reversed EdgeGoverns hop, got %+v", chain.Hops[0])
	}
	if !chain.Hops[1].Reverse || chain.Hops[1].EdgeType != irac.EdgeAppliesTo {
		t.Errorf("expected hop 1 to be a reversed EdgeAppliesTo hop, got %+v", chain.Hops[1])
	}

	if got := chain.Depth(); got != 3 {
		t.Errorf("expected chain depth 3 (issue->rule->application->{fact,conclusion}), got %d", got)
	}
}

// TestIndexer_ReasoningChain_FanOut seeds an application supported by two
// facts and concluded-from by one conclusion, and asserts all three tail
// nodes appear in the chain, each hopping directly from the application.
func TestIndexer_ReasoningChain_FanOut(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	caseID := "case-fanout"

	issueID, _, _, appID, _ := seedCleanTree(t, store, caseID)

	extraFact := irac.NewFactNode(caseID+"-fact-2", caseID, "a second supporting fact", testProvenance().GeneratedAt, 0.9, testProvenance())
	mustCreateNode(t, store, extraFact.Node)
	mustCreateEdge(t, store, irac.Edge{FromID: extraFact.ID, ToID: appID, Type: irac.EdgeSupports})

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	paths, err := idx.LookupPaths(ctx, caseID, issueID, "")
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}
	var chain *treeindex.Path
	for i := range paths {
		if paths[i].Kind == treeindex.PathKindReasoningChain {
			chain = &paths[i]
		}
	}
	if chain == nil {
		t.Fatalf("expected a reasoning chain rooted at the issue")
	}

	if got := len(chain.Nodes); got != 6 {
		t.Fatalf("expected 6 nodes (issue, rule, app, 2 facts, 1 conclusion), got %d: %+v", got, chain.Nodes)
	}

	// Every fact/conclusion tail node's hop must originate from the
	// application, which sits at index 2 (issue=0, rule=1, app=2).
	factAndConclusionHops := chain.Hops[2:]
	for _, h := range factAndConclusionHops {
		if h.FromIndex != 2 {
			t.Errorf("expected fan-out hop to originate from the application (index 2), got FromIndex=%d", h.FromIndex)
		}
	}

}
