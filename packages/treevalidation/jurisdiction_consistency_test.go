package treevalidation

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestCheckJurisdictionConsistency(t *testing.T) {
	now := time.Now()

	t.Run("clean tree has no findings", func(t *testing.T) {
		tree := cleanTree("case-1", "us-ny")
		findings := CheckJurisdictionConsistency(tree, "us-ny")
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("mismatched rule jurisdiction is flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "uk-eng")
		findings := CheckJurisdictionConsistency(tree, "us-ny")
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].Code != CodeJurisdictionMismatch {
			t.Errorf("expected code %q, got %q", CodeJurisdictionMismatch, findings[0].Code)
		}
		if findings[0].NodeID != "rule-1" {
			t.Errorf("expected NodeID rule-1, got %q", findings[0].NodeID)
		}
	})

	t.Run("mismatched jurisdiction in override list is not flagged", func(t *testing.T) {
		tree := cleanTree("case-1", "uk-eng")
		findings := CheckJurisdictionConsistency(tree, "us-ny", "uk-eng")
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("blank case jurisdiction disables the check", func(t *testing.T) {
		tree := cleanTree("case-1", "uk-eng")
		findings := CheckJurisdictionConsistency(tree, "")
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})

	t.Run("multiple rules with one mismatch flags only the mismatched one", func(t *testing.T) {
		ruleOK := irac.NewRuleNode("rule-ok", "case-1", "text", "us-ny", "common_law", now, 0.9, testProvenance())
		ruleBad := irac.NewRuleNode("rule-bad", "case-1", "text", "uk-eng", "common_law", now, 0.9, testProvenance())
		tree := treeassemblyTreeOf(ruleOK, ruleBad)

		findings := CheckJurisdictionConsistency(tree, "us-ny")
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if findings[0].NodeID != "rule-bad" {
			t.Errorf("expected NodeID rule-bad, got %q", findings[0].NodeID)
		}
	})
}
