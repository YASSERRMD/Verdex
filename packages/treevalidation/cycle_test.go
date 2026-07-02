package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestDetectCycles(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := DetectCycles(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("full-graph cycle spanning multiple node types is detected", func(t *testing.T) {
		// Build a synthetic (structurally illegal, but that's not this
		// check's concern) cycle: A -> B -> C -> A, using arbitrary node
		// IDs and non-EdgeSupports edge types (EdgeSupports is
		// deliberately excluded from the adjacency graph — see
		// DetectCycles's doc comment — since it is the intentional
		// reverse-direction half of the Application<->Fact pair, not
		// independent cycle-forming information). DetectCycles operates
		// over the raw adjacency of tree.Edges regardless of triple
		// legality, so it still must catch this.
		a := irac.NewIssueNode("a", "case-1", "text", now, 0.9, testProvenance())
		b := irac.NewRuleNode("b", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		c := irac.NewApplicationNode("c", "case-1", "text", now, 0.9, testProvenance())

		edges := []irac.Edge{
			{FromID: "a", ToID: "b", Type: irac.EdgeGoverns},
			{FromID: "b", ToID: "c", Type: irac.EdgeAppliesTo},
			{FromID: "c", ToID: "a", Type: irac.EdgeConcludesFrom},
		}
		tree := treeWithEdges([]irac.NodeLike{a, b, c}, edges)

		findings := DetectCycles(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeGraphCycle {
			t.Errorf("expected code %q, got %q", CodeGraphCycle, findings[0].Code)
		}
	})

	t.Run("acyclic diamond shape is not flagged", func(t *testing.T) {
		a := irac.NewIssueNode("a", "case-1", "text", now, 0.9, testProvenance())
		b := irac.NewRuleNode("b", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		c := irac.NewRuleNode("c", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		d := irac.NewFactNode("d", "case-1", "text", now, 0.9, testProvenance())

		edges := []irac.Edge{
			{FromID: "b", ToID: "a", Type: irac.EdgeGoverns},
			{FromID: "c", ToID: "a", Type: irac.EdgeGoverns},
		}
		tree := treeWithEdges([]irac.NodeLike{a, b, c, d}, edges)

		findings := DetectCycles(tree)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("self-loop is detected as a cycle", func(t *testing.T) {
		a := irac.NewIssueNode("a", "case-1", "text", now, 0.9, testProvenance())
		edges := []irac.Edge{{FromID: "a", ToID: "a", Type: irac.EdgeGoverns}}
		tree := treeWithEdges([]irac.NodeLike{a}, edges)

		findings := DetectCycles(tree)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
}
