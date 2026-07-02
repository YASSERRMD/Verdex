package treeindex_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

func TestIndexer_RebuildAndLookup_ReasoningChain(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	caseID := "case-1"
	issueID, ruleID, factID, appID, conclusionID := seedCleanTree(t, store, caseID)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	paths, err := idx.LookupPaths(ctx, caseID, issueID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 reasoning-chain path rooted at the issue, got %d", len(paths))
	}

	p := paths[0]
	if p.Kind != treeindex.PathKindReasoningChain {
		t.Errorf("Kind = %q, want %q", p.Kind, treeindex.PathKindReasoningChain)
	}
	if got := p.RootID(); got != issueID {
		t.Errorf("RootID() = %q, want %q", got, issueID)
	}

	wantIDs := map[string]bool{issueID: false, ruleID: false, appID: false, factID: false, conclusionID: false}
	for _, n := range p.Nodes {
		if _, ok := wantIDs[n.ID]; ok {
			wantIDs[n.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("expected chain to include node %q, nodes were %+v", id, p.Nodes)
		}
	}
}

func TestIndexer_RebuildAndLookup_RuleGroupedIssues(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	caseID := "case-2"
	now := time.Now()

	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "A duty of care arises in foreseeable-harm situations.", "US-CA", "common_law", now, 0.9, testProvenance(), testSpan())
	issueA := irac.NewIssueNode(caseID+"-issue-a", caseID, "Did the defendant owe a duty of care?", now, 0.9, testProvenance(), testSpan())
	issueB := irac.NewIssueNode(caseID+"-issue-b", caseID, "Was the duty of care breached?", now, 0.9, testProvenance(), testSpan())

	for _, n := range []irac.Node{rule.Node, issueA.Node, issueB.Node} {
		if err := store.CreateNode(ctx, n); err != nil {
			t.Fatalf("CreateNode(%s): %v", n.ID, err)
		}
	}
	for _, e := range []irac.Edge{
		{FromID: rule.ID, ToID: issueA.ID, Type: irac.EdgeGoverns},
		{FromID: rule.ID, ToID: issueB.ID, Type: irac.EdgeGoverns},
	} {
		if err := store.CreateEdge(ctx, e); err != nil {
			t.Fatalf("CreateEdge(%+v): %v", e, err)
		}
	}

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	paths, err := idx.LookupPaths(ctx, caseID, rule.ID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 rule-grouped-issues path, got %d", len(paths))
	}
	p := paths[0]
	if p.Kind != treeindex.PathKindRuleGroupedIssues {
		t.Errorf("Kind = %q, want %q", p.Kind, treeindex.PathKindRuleGroupedIssues)
	}
	if len(p.Nodes) != 3 {
		t.Fatalf("expected rule + 2 issues = 3 nodes, got %d: %+v", len(p.Nodes), p.Nodes)
	}
}

func TestIndexer_LookupPaths_DepthBounding(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	caseID := "case-3"
	issueID, _, _, _, _ := seedCleanTree(t, store, caseID)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	full, err := idx.LookupPathsWithDepth(ctx, caseID, issueID, irac.EdgeGoverns, 0)
	if err != nil {
		t.Fatalf("LookupPathsWithDepth(unbounded): %v", err)
	}
	if len(full) != 1 || full[0].Depth() < 2 {
		t.Fatalf("expected an unbounded chain of depth >= 2, got %+v", full)
	}

	bounded, err := idx.LookupPathsWithDepth(ctx, caseID, issueID, irac.EdgeGoverns, 1)
	if err != nil {
		t.Fatalf("LookupPathsWithDepth(1): %v", err)
	}
	if len(bounded) != 1 {
		t.Fatalf("expected 1 path, got %d", len(bounded))
	}
	if got := bounded[0].Depth(); got != 1 {
		t.Errorf("Depth() with maxDepth=1 = %d, want 1", got)
	}
	if len(bounded[0].Nodes) != 2 {
		t.Errorf("expected truncation to 2 nodes (issue + rule), got %d: %+v", len(bounded[0].Nodes), bounded[0].Nodes)
	}
}

func TestIndexer_LookupPaths_CacheHitsAndMisses(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	caseID := "case-4"
	issueID, _, _, _, _ := seedCleanTree(t, store, caseID)

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	if _, err := idx.LookupPaths(ctx, caseID, issueID, irac.EdgeGoverns); err != nil {
		t.Fatalf("first LookupPaths: %v", err)
	}
	afterFirst := idx.Stats()
	if afterFirst.CacheMisses != 1 {
		t.Errorf("CacheMisses after first lookup = %d, want 1", afterFirst.CacheMisses)
	}
	if afterFirst.CacheHits != 0 {
		t.Errorf("CacheHits after first lookup = %d, want 0", afterFirst.CacheHits)
	}

	if _, err := idx.LookupPaths(ctx, caseID, issueID, irac.EdgeGoverns); err != nil {
		t.Fatalf("second LookupPaths: %v", err)
	}
	afterSecond := idx.Stats()
	if afterSecond.CacheHits != 1 {
		t.Errorf("CacheHits after second (repeat) lookup = %d, want 1", afterSecond.CacheHits)
	}
	if afterSecond.CacheMisses != 1 {
		t.Errorf("CacheMisses after second (repeat) lookup = %d, want still 1", afterSecond.CacheMisses)
	}

	// A rebuild must purge the cache: the very next lookup for the same key
	// should be a fresh miss, not another hit against stale cached data.
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase (second time): %v", err)
	}
	if _, err := idx.LookupPaths(ctx, caseID, issueID, irac.EdgeGoverns); err != nil {
		t.Fatalf("LookupPaths after rebuild: %v", err)
	}
	afterRebuild := idx.Stats()
	if afterRebuild.CacheMisses != 2 {
		t.Errorf("CacheMisses after post-rebuild lookup = %d, want 2 (rebuild must purge the cache)", afterRebuild.CacheMisses)
	}
}

func TestIndexer_LookupPaths_UnknownCase(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	_, err = idx.LookupPaths(context.Background(), "never-indexed", "some-node", irac.EdgeGoverns)
	if err != treeindex.ErrCaseNotIndexed {
		t.Errorf("LookupPaths on an unbuilt case: got %v, want ErrCaseNotIndexed", err)
	}
}

func TestIndexer_RebuildCase_PicksUpNewNodesAndEdges(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	caseID := "case-5"
	now := time.Now()

	rule := irac.NewRuleNode(caseID+"-rule-1", caseID, "A duty of care arises in foreseeable-harm situations.", "US-CA", "common_law", now, 0.9, testProvenance(), testSpan())
	issueA := irac.NewIssueNode(caseID+"-issue-a", caseID, "Did the defendant owe a duty of care?", now, 0.9, testProvenance(), testSpan())
	if err := store.CreateNode(ctx, rule.Node); err != nil {
		t.Fatalf("CreateNode(rule): %v", err)
	}
	if err := store.CreateNode(ctx, issueA.Node); err != nil {
		t.Fatalf("CreateNode(issueA): %v", err)
	}
	if err := store.CreateEdge(ctx, irac.Edge{FromID: rule.ID, ToID: issueA.ID, Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	if err := idx.RebuildCase(ctx, caseID); err != nil {
		t.Fatalf("RebuildCase: %v", err)
	}

	before, err := idx.LookupPaths(ctx, caseID, rule.ID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths (before): %v", err)
	}
	if len(before) != 1 || len(before[0].Nodes) != 2 {
		t.Fatalf("expected rule + 1 issue before adding issueB, got %+v", before)
	}

	// Add a second issue governed by the same rule after the initial build,
	// then reindex via ReindexOnRevision (the tree-revision-driven hook)
	// rather than calling RebuildCase directly, to exercise that entry
	// point too.
	issueB := irac.NewIssueNode(caseID+"-issue-b", caseID, "Was the duty of care breached?", now, 0.9, testProvenance(), testSpan())
	if err := store.CreateNode(ctx, issueB.Node); err != nil {
		t.Fatalf("CreateNode(issueB): %v", err)
	}
	if err := store.CreateEdge(ctx, irac.Edge{FromID: rule.ID, ToID: issueB.ID, Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	revision := irac.NewInitialRevision(caseID, now)
	revision = irac.NextRevision(revision, time.Now())
	if err := treeindex.ReindexOnRevision(ctx, idx, revision); err != nil {
		t.Fatalf("ReindexOnRevision: %v", err)
	}

	after, err := idx.LookupPaths(ctx, caseID, rule.ID, irac.EdgeGoverns)
	if err != nil {
		t.Fatalf("LookupPaths (after): %v", err)
	}
	if len(after) != 1 || len(after[0].Nodes) != 3 {
		t.Fatalf("expected rule + 2 issues after ReindexOnRevision, got %+v", after)
	}
}

func TestNewIndexer_NilStore(t *testing.T) {
	if _, err := treeindex.NewIndexer(nil, treeindex.IndexerOptions{}); err != treeindex.ErrNilGraphStore {
		t.Errorf("NewIndexer(nil, ...): got %v, want ErrNilGraphStore", err)
	}
}

func TestReindexOnRevision_EmptyCaseID(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	idx, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	err = treeindex.ReindexOnRevision(context.Background(), idx, irac.TreeRevision{})
	if err != treeindex.ErrEmptyCaseID {
		t.Errorf("ReindexOnRevision with empty CaseID: got %v, want ErrEmptyCaseID", err)
	}
}
