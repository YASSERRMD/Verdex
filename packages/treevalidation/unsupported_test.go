package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestFlagUnsupportedClaims(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings at default threshold", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := FlagUnsupportedClaims(tree, 0.5)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("node with no spans is flagged", func(t *testing.T) {
		fact := irac.NewFactNode("fact-1", "case-1", "text", now, 0.9, testProvenance()) // no spans
		tree := treeassemblyTreeOf(fact)

		findings := FlagUnsupportedClaims(tree, 0.5)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeMissingSourceSpans {
			t.Errorf("expected code %q, got %q", CodeMissingSourceSpans, findings[0].Code)
		}
	})

	t.Run("node below confidence threshold is flagged", func(t *testing.T) {
		fact := irac.NewFactNode("fact-1", "case-1", "text", now, 0.2, testProvenance(), testSpan())
		tree := treeassemblyTreeOf(fact)

		findings := FlagUnsupportedClaims(tree, 0.5)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeLowConfidence {
			t.Errorf("expected code %q, got %q", CodeLowConfidence, findings[0].Code)
		}
	})

	t.Run("node both missing spans and below threshold gets two findings", func(t *testing.T) {
		fact := irac.NewFactNode("fact-1", "case-1", "text", now, 0.1, testProvenance())
		tree := treeassemblyTreeOf(fact)

		findings := FlagUnsupportedClaims(tree, 0.5)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("confidence exactly at threshold is not flagged", func(t *testing.T) {
		fact := irac.NewFactNode("fact-1", "case-1", "text", now, 0.5, testProvenance(), testSpan())
		tree := treeassemblyTreeOf(fact)

		findings := FlagUnsupportedClaims(tree, 0.5)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
}
