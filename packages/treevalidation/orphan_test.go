package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestDetectOrphans(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := DetectOrphans(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("node with zero edges is flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		orphanFact := irac.NewFactNode("fact-orphan", "case-1", "An unrelated, unreferenced fact.", now, 0.9, testProvenance(), testSpan())
		tree.Nodes = append(tree.Nodes, orphanFact)

		findings := DetectOrphans(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeOrphanNode {
			t.Errorf("expected code %q, got %q", CodeOrphanNode, findings[0].Code)
		}
		if findings[0].NodeID != "fact-orphan" {
			t.Errorf("expected NodeID fact-orphan, got %q", findings[0].NodeID)
		}
	})

	t.Run("node with only an incoming edge is not an orphan", func(t *testing.T) {
		issue := irac.NewIssueNode("issue-1", "case-1", "text", now, 0.9, testProvenance())
		rule := irac.NewRuleNode("rule-1", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		edges := []irac.Edge{{FromID: rule.ID, ToID: issue.ID, Type: irac.EdgeGoverns}}
		tree := treeWithEdges([]irac.NodeLike{issue, rule}, edges)

		findings := DetectOrphans(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("empty tree yields no findings", func(t *testing.T) {
		findings := DetectOrphans(treeWithEdges(nil, nil))
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
}
