package treeassembly

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestDetectGaps_NilTree(t *testing.T) {
	gaps := DetectGaps(nil)
	if len(gaps) != 0 {
		t.Fatalf("expected no gaps for nil tree, got %v", gaps)
	}
}

func TestDetectGaps_NoGapsForFullyAddressedTree(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gaps := DetectGaps(tree)
	if len(gaps) != 0 {
		t.Fatalf("expected no gaps, got %v", gaps)
	}
}

func TestDetectGaps_UnaddressedIssue(t *testing.T) {
	now := time.Now()
	caseID := "case-1"

	addressedIssue := irac.NewIssueNode("issue-addressed", caseID, "Was there a breach?", now, 0.9, testProvenance())
	unaddressedIssue := irac.NewIssueNode("issue-unaddressed", caseID, "Was there damages?", now, 0.9, testProvenance())
	rule := irac.NewRuleNode("rule-1", caseID, "rule text", "us-ny", "common_law", now, 0.9, testProvenance(addressedIssue.ID))
	fact := irac.NewFactNode("fact-1", caseID, "fact text", now, 0.9, testProvenance())
	app := irac.NewApplicationNode("app-1", caseID, "application text", now, 0.9, testProvenance(rule.ID, fact.ID))

	input := AssemblyInput{
		CaseID:       caseID,
		Issues:       []irac.IssueNode{addressedIssue, unaddressedIssue},
		Rules:        []irac.RuleNode{rule},
		Facts:        []irac.FactNode{fact},
		Applications: []irac.ApplicationNode{app},
	}

	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gaps := DetectGaps(tree)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d: %v", len(gaps), gaps)
	}
	if gaps[0].Kind != GapUnaddressedIssue {
		t.Errorf("expected GapUnaddressedIssue, got %v", gaps[0].Kind)
	}
	if gaps[0].NodeID != unaddressedIssue.ID {
		t.Errorf("expected gap on %q, got %q", unaddressedIssue.ID, gaps[0].NodeID)
	}
}

func TestDetectGaps_UnresolvedApplication_OnlyWhenConclusionsPresent(t *testing.T) {
	input := syntheticInput("case-1")

	// No conclusions at all: an unresolved application should NOT be
	// flagged, since conclusion synthesis is out of scope (Phase 055).
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gaps := DetectGaps(tree)
	for _, g := range gaps {
		if g.Kind == GapUnresolvedApplication {
			t.Fatalf("did not expect unresolved-application gap with zero conclusions: %v", g)
		}
	}

	// With a conclusion present but not resolving this application, the
	// gap should be flagged. Use a second, unrelated application.
	otherApp := irac.NewApplicationNode("app-2", input.CaseID, "unrelated application", time.Now(), 0.9, testProvenance())
	conclusion := irac.NewConclusionNode("conclusion-1", input.CaseID, "Draft analysis text.", time.Now(), 0.8, testProvenance(input.Applications[0].ID))

	input2 := input
	input2.Applications = append([]irac.ApplicationNode{}, input.Applications...)
	input2.Applications = append(input2.Applications, otherApp)

	provider := fixedConclusionProvider{conclusions: []irac.ConclusionNode{conclusion}}
	tree2, err := ComposeTree(context.Background(), input2, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gaps2 := DetectGaps(tree2)
	found := false
	for _, g := range gaps2 {
		if g.Kind == GapUnresolvedApplication && g.NodeID == otherApp.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unresolved-application gap for %q, got %v", otherApp.ID, gaps2)
	}
}
