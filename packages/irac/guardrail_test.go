package irac

import (
	"testing"
	"time"
)

// TestDraftAnalysisLabel_NoVerdictLanguage proves the guardrail label
// constant itself never contains verdict- or directive-sounding language
// from the non-binding-guardrail word list (CONTRIBUTING.md: "Verdict or
// directive language is rejected").
func TestDraftAnalysisLabel_NoVerdictLanguage(t *testing.T) {
	if ContainsVerdictLanguage(DraftAnalysisLabel) {
		t.Fatalf("DraftAnalysisLabel %q contains verdict/directive language", DraftAnalysisLabel)
	}
	if DraftAnalysisLabel != "draft_analysis" {
		t.Fatalf("DraftAnalysisLabel = %q, want %q", DraftAnalysisLabel, "draft_analysis")
	}
}

// TestContainsVerdictLanguage exercises the verdict-language word list
// directly, proving each listed word/phrase is detected and that benign
// analytical text is not flagged.
func TestContainsVerdictLanguage(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"guilty", "the defendant is guilty", true},
		{"liable", "the defendant is liable for damages", true},
		{"shall pay", "the defendant shall pay $500", true},
		{"is ordered", "the defendant is ordered to comply", true},
		{"is hereby ordered", "it is hereby ordered that...", true},
		{"judgment for", "judgment for the plaintiff", true},
		{"convicted", "the defendant was convicted", true},
		{"acquitted", "the defendant was acquitted", true},
		{"sentenced", "the defendant was sentenced", true},
		{"case-insensitive", "The Defendant Is GUILTY", true},

		{"draft analysis label itself", DraftAnalysisLabel, false},
		{"benign analysis", "the elements of the claim appear satisfied on this record", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsVerdictLanguage(tt.text); got != tt.want {
				t.Errorf("ContainsVerdictLanguage(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestNewConclusionNode_AlwaysAttachesGuardrailLabel proves
// NewConclusionNode is the only exported constructor for ConclusionNode
// and it unconditionally sets Label to DraftAnalysisLabel, regardless of
// the conclusion's own Text content (even if that text itself sounds like
// a verdict — this package does not censor node Text, but it always
// attaches the non-binding label alongside it).
func TestNewConclusionNode_AlwaysAttachesGuardrailLabel(t *testing.T) {
	now := time.Now().UTC()
	prov := Provenance{GeneratedBy: "test", GeneratedAt: now}

	tests := []string{
		"the elements appear satisfied on this record",
		"",
		"the defendant is guilty", // even verdict-sounding Text still gets the label
	}

	for _, text := range tests {
		c := NewConclusionNode("c1", "case-1", text, now, 0.5, prov)
		if c.Label != DraftAnalysisLabel {
			t.Errorf("NewConclusionNode(text=%q).Label = %q, want %q", text, c.Label, DraftAnalysisLabel)
		}
		if !c.HasGuardrailLabel() {
			t.Errorf("NewConclusionNode(text=%q).HasGuardrailLabel() = false, want true", text)
		}
	}
}

// TestConclusionNode_ZeroValueHasNoGuardrailLabel documents that a bare
// struct literal (bypassing NewConclusionNode) does NOT carry the
// guardrail label — this is exactly the gap ValidateTree, MarshalTree,
// and UnmarshalTree all defensively check for.
func TestConclusionNode_ZeroValueHasNoGuardrailLabel(t *testing.T) {
	var c ConclusionNode
	if c.HasGuardrailLabel() {
		t.Fatalf("zero-value ConclusionNode unexpectedly has a guardrail label: %q", c.Label)
	}
}
