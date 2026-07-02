package vectorindex_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func TestIsLeafNodeType(t *testing.T) {
	cases := []struct {
		nodeType irac.NodeType
		want     bool
	}{
		{irac.NodeIssue, false},
		{irac.NodeRule, true},
		{irac.NodeFact, true},
		{irac.NodeApplication, false},
		{irac.NodeConclusion, true},
	}
	for _, c := range cases {
		if got := vectorindex.IsLeafNodeType(c.nodeType); got != c.want {
			t.Errorf("IsLeafNodeType(%s) = %v, want %v", c.nodeType, got, c.want)
		}
	}
}

func TestProjectLeaves_NilStore(t *testing.T) {
	_, err := vectorindex.ProjectLeaves(context.Background(), nil, "case-1", vectorindex.ProjectionOptions{})
	if err != vectorindex.ErrNilGraphStore {
		t.Fatalf("expected ErrNilGraphStore, got %v", err)
	}
}

func TestProjectLeaves_EmptyCaseID(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	_, err := vectorindex.ProjectLeaves(context.Background(), store, "", vectorindex.ProjectionOptions{})
	if err != vectorindex.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestProjectLeaves_FiltersStructuralNodes(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	now := time.Now()

	mustCreateNode(t, store, irac.Node{ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-1", Text: "Is there a breach?", CreatedAt: now})
	mustCreateNode(t, store, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-1", Text: "A contract requires offer and acceptance.", CreatedAt: now})
	mustCreateNode(t, store, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-1", Text: "The parties signed a written agreement.", CreatedAt: now})
	mustCreateNode(t, store, irac.Node{ID: "app-1", Type: irac.NodeApplication, CaseID: "case-1", Text: "Applying the rule to the fact.", CreatedAt: now})
	mustCreateNode(t, store, irac.Node{ID: "conclusion-1", Type: irac.NodeConclusion, CaseID: "case-1", Text: "The contract is likely valid.", CreatedAt: now})

	leaves, err := vectorindex.ProjectLeaves(ctx, store, "case-1", vectorindex.ProjectionOptions{
		CategoryCode:     vectorindex.CategoryCode("contract"),
		JurisdictionCode: vectorindex.JurisdictionCode("us-ny"),
		PartyID:          vectorindex.PartyID("plaintiff"),
	})
	if err != nil {
		t.Fatalf("ProjectLeaves: %v", err)
	}
	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves (rule, fact, conclusion), got %d", len(leaves))
	}

	seen := make(map[string]vectorindex.IndexableLeaf, len(leaves))
	for _, l := range leaves {
		seen[l.ID] = l
	}
	for _, id := range []string{"rule-1", "fact-1", "conclusion-1"} {
		if _, ok := seen[id]; !ok {
			t.Errorf("expected leaf %q to be projected, was excluded", id)
		}
	}
	for _, id := range []string{"issue-1", "app-1"} {
		if _, ok := seen[id]; ok {
			t.Errorf("expected structural node %q to be excluded, was projected", id)
		}
	}

	for _, l := range leaves {
		if l.CategoryCode != "contract" {
			t.Errorf("leaf %q: CategoryCode = %q, want %q", l.ID, l.CategoryCode, "contract")
		}
		if l.JurisdictionCode != "us-ny" {
			t.Errorf("leaf %q: JurisdictionCode = %q, want %q", l.ID, l.JurisdictionCode, "us-ny")
		}
		if l.PartyID != "plaintiff" {
			t.Errorf("leaf %q: PartyID = %q, want %q", l.ID, l.PartyID, "plaintiff")
		}
	}
}

func TestProjectLeavesFromNodes_RetainsSpansAndJurisdiction(t *testing.T) {
	now := time.Now()
	span := irac.SourceSpan{Start: 0, End: 10}

	issue := irac.NewIssueNode("issue-1", "case-1", "Is there a breach?", now, 0.9, irac.Provenance{})
	rule := irac.NewRuleNode("rule-1", "case-1", "A contract requires offer and acceptance.", "us-ca", "common_law", now, 0.9, irac.Provenance{}, span)
	fact := irac.NewFactNode("fact-1", "case-1", "The parties signed a written agreement.", now, 0.9, irac.Provenance{}, span)
	conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "The contract is likely valid.", now, 0.9, irac.Provenance{}, span)

	nodes := []irac.NodeLike{issue, rule, fact, conclusion}

	leaves := vectorindex.ProjectLeavesFromNodes(nodes, vectorindex.ProjectionOptions{
		JurisdictionCode: vectorindex.JurisdictionCode("us-ny"), // fallback only
		CategoryCode:     vectorindex.CategoryCode("contract"),
	})

	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves, got %d", len(leaves))
	}

	byID := make(map[string]vectorindex.IndexableLeaf, len(leaves))
	for _, l := range leaves {
		byID[l.ID] = l
	}

	ruleLeaf, ok := byID["rule-1"]
	if !ok {
		t.Fatalf("expected rule-1 to be projected")
	}
	if ruleLeaf.JurisdictionCode != "us-ca" {
		t.Errorf("rule leaf JurisdictionCode = %q, want the RuleNode's own %q (not the ProjectionOptions fallback)", ruleLeaf.JurisdictionCode, "us-ca")
	}
	if len(ruleLeaf.SourceSpans) != 1 || ruleLeaf.SourceSpans[0] != span {
		t.Errorf("rule leaf SourceSpans = %v, want [%v]", ruleLeaf.SourceSpans, span)
	}

	factLeaf, ok := byID["fact-1"]
	if !ok {
		t.Fatalf("expected fact-1 to be projected")
	}
	if factLeaf.JurisdictionCode != "us-ny" {
		t.Errorf("fact leaf JurisdictionCode = %q, want the ProjectionOptions fallback %q", factLeaf.JurisdictionCode, "us-ny")
	}
	if len(factLeaf.SourceSpans) != 1 {
		t.Errorf("fact leaf SourceSpans = %v, want 1 span", factLeaf.SourceSpans)
	}

	if _, ok := byID["issue-1"]; ok {
		t.Errorf("expected issue-1 to be excluded from projection")
	}
}
