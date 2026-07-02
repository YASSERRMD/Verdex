package guardrail_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// TestCheckTextRejectsVerdictLanguage is the adversarial policy test
// suite required by this phase: every one of these real-world
// verdict/directive phrases must be rejected by CheckText, no exceptions.
func TestCheckTextRejectsVerdictLanguage(t *testing.T) {
	adversarial := []string{
		"the defendant is guilty",
		"the defendant is guilty of all charges",
		"judgment for the plaintiff is entered",
		"the court finds the defendant liable",
		"it is hereby ordered that the defendant shall pay damages",
		"the defendant is hereby ordered to vacate the premises",
		"the accused is convicted on all counts",
		"the accused is acquitted",
		"the defendant is sentenced to five years",
		"defendant shall pay $10,000 in damages",
		"the court holds the defendant liable for breach of contract",
		"JUDGMENT FOR THE PLAINTIFF",
		"The Defendant Is Guilty",
	}

	for _, text := range adversarial {
		t.Run(text, func(t *testing.T) {
			err := guardrail.CheckText(text)
			if err == nil {
				t.Fatalf("CheckText(%q) = nil, want ErrVerdictLanguageDetected", text)
			}
			if !errors.Is(err, guardrail.ErrVerdictLanguageDetected) {
				t.Fatalf("CheckText(%q) error = %v, want errors.Is ErrVerdictLanguageDetected", text, err)
			}
		})
	}
}

// TestCheckTextAllowsNonBindingAnalysis asserts that legitimate,
// non-binding draft-analysis phrasing is never false-positively rejected.
func TestCheckTextAllowsNonBindingAnalysis(t *testing.T) {
	legitimate := []string{
		"the evidence suggests the defendant's position is stronger on this issue",
		"this draft analysis weighs the plaintiff's argument as more persuasive, but the outcome is not binding",
		"a reasonable factfinder could conclude the contract was breached, though this is not a determination of liability",
		"the weight of the cited precedent favors the plaintiff's interpretation of the statute",
		"further review by a qualified attorney is recommended before relying on this analysis",
		"the applicable rule, if applied to these facts, would likely favor an award of damages, subject to judicial determination",
		"this issue remains genuinely unresolved on the current record",
		"the confidence score for this conclusion is 0.62",
	}

	for _, text := range legitimate {
		t.Run(text, func(t *testing.T) {
			if err := guardrail.CheckText(text); err != nil {
				t.Fatalf("CheckText(%q) = %v, want nil (false positive)", text, err)
			}
		})
	}
}

func TestCheckTextErrorTruncatesLongText(t *testing.T) {
	long := "the defendant is guilty" + strings.Repeat(" and this is a very long sentence padding out the text", 20)
	err := guardrail.CheckText(long)
	if err == nil {
		t.Fatal("CheckText(long verdict text) = nil, want error")
	}
	if len(err.Error()) >= len(long) {
		t.Fatalf("CheckText error message not truncated: got %d chars for a %d-char input", len(err.Error()), len(long))
	}
}
