package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestCheckConfidencePropagation(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := CheckConfidencePropagation(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("conclusion confidence exceeding chain minimum is flagged", func(t *testing.T) {
		issue := irac.NewIssueNode("issue-1", "case-1", "text", now, 0.9, testProvenance())
		rule := irac.NewRuleNode("rule-1", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		fact := irac.NewFactNode("fact-1", "case-1", "text", now, 0.3, testProvenance()) // weak link
		app := irac.NewApplicationNode("app-1", "case-1", "text", now, 0.9, testProvenance())
		// Conclusion claims higher confidence than the weakest link (fact
		// at 0.3).
		conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "text", now, 0.8, testProvenance())

		edges := []irac.Edge{
			{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns},
			{FromID: app.ID, ToID: rule.ID, Type: irac.EdgeAppliesTo},
			{FromID: app.ID, ToID: fact.ID, Type: irac.EdgeAppliesTo},
			{FromID: conclusion.ID, ToID: app.ID, Type: irac.EdgeConcludesFrom},
		}
		tree := treeWithEdges([]irac.NodeLike{issue, rule, fact, app, conclusion}, edges)

		findings := CheckConfidencePropagation(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeConfidenceExceedsChain {
			t.Errorf("expected code %q, got %q", CodeConfidenceExceedsChain, findings[0].Code)
		}
		if findings[0].NodeID != "conclusion-1" {
			t.Errorf("expected NodeID conclusion-1, got %q", findings[0].NodeID)
		}
	})

	t.Run("conclusion confidence equal to chain minimum is not flagged", func(t *testing.T) {
		app := irac.NewApplicationNode("app-1", "case-1", "text", now, 0.5, testProvenance())
		conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "text", now, 0.5, testProvenance())
		edges := []irac.Edge{
			{FromID: conclusion.ID, ToID: app.ID, Type: irac.EdgeConcludesFrom},
		}
		tree := treeWithEdges([]irac.NodeLike{app, conclusion}, edges)

		findings := CheckConfidencePropagation(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("conclusion with no resolvable chain is skipped (traceability's job)", func(t *testing.T) {
		conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "text", now, 0.9, testProvenance())
		tree := treeassemblyTreeOf(conclusion)

		findings := CheckConfidencePropagation(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
}
