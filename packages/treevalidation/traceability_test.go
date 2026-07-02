package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestCheckConclusionTraceability(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := CheckConclusionTraceability(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("conclusion missing fact chain is flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		// Remove the Application--applies_to-->Fact edge so the
		// conclusion's chain never reaches a fact.
		filtered := make([]irac.Edge, 0, len(tree.Edges))
		for _, e := range tree.Edges {
			if e.Type == irac.EdgeAppliesTo && e.ToID == "fact-1" {
				continue
			}
			filtered = append(filtered, e)
		}
		tree.Edges = filtered

		findings := CheckConclusionTraceability(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeConclusionNotTraceable {
			t.Errorf("expected code %q, got %q", CodeConclusionNotTraceable, findings[0].Code)
		}
		if findings[0].NodeID != "conclusion-1" {
			t.Errorf("expected NodeID conclusion-1, got %q", findings[0].NodeID)
		}
		if findings[0].Severity != SeverityCritical {
			t.Errorf("expected SeverityCritical, got %v", findings[0].Severity)
		}
	})

	t.Run("conclusion missing rule chain is flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		filtered := make([]irac.Edge, 0, len(tree.Edges))
		for _, e := range tree.Edges {
			if e.Type == irac.EdgeAppliesTo && e.ToID == "rule-1" {
				continue
			}
			filtered = append(filtered, e)
		}
		tree.Edges = filtered

		findings := CheckConclusionTraceability(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeConclusionNotTraceable {
			t.Errorf("expected code %q, got %q", CodeConclusionNotTraceable, findings[0].Code)
		}
	})

	t.Run("conclusion with no concludes_from edge at all is flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		filtered := make([]irac.Edge, 0, len(tree.Edges))
		for _, e := range tree.Edges {
			if e.Type == irac.EdgeConcludesFrom {
				continue
			}
			filtered = append(filtered, e)
		}
		tree.Edges = filtered

		findings := CheckConclusionTraceability(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("tree with no conclusions yields no findings", func(t *testing.T) {
		issue := irac.NewIssueNode("issue-1", "case-1", "text", now, 0.9, testProvenance())
		tree := treeassemblyTreeOf(issue)
		findings := CheckConclusionTraceability(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
}
