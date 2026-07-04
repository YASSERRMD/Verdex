package threatmodel_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestSanitizeOutput_StripsControlChars(t *testing.T) {
	t.Parallel()

	raw := "Draft analysis text\x00 with an embedded\x01 null and control\x1b byte."
	out := threatmodel.SanitizeOutput(raw)

	if out.StrippedControlChars != 3 {
		t.Errorf("SanitizeOutput().StrippedControlChars = %d, want 3", out.StrippedControlChars)
	}
	if out.Text == raw {
		t.Error("SanitizeOutput().Text unchanged, want control chars stripped")
	}
	for _, r := range out.Text {
		if r == 0x00 || r == 0x01 || r == 0x1b {
			t.Errorf("SanitizeOutput().Text = %q still contains a stripped control char", out.Text)
		}
	}
}

func TestSanitizeOutput_CleanTextUnaffected(t *testing.T) {
	t.Parallel()

	raw := "This is a perfectly clean draft analysis with no control characters."
	out := threatmodel.SanitizeOutput(raw)

	if out.Text != raw {
		t.Errorf("SanitizeOutput().Text = %q, want unchanged %q", out.Text, raw)
	}
	if out.StrippedControlChars != 0 {
		t.Errorf("SanitizeOutput().StrippedControlChars = %d, want 0", out.StrippedControlChars)
	}
	if len(out.Findings) != 0 {
		t.Errorf("SanitizeOutput().Findings = %+v, want none", out.Findings)
	}
}

func TestSanitizeOutput_FlagsEchoedInjectionAttempt(t *testing.T) {
	t.Parallel()

	// Output that echoes back compliance with an injected instruction --
	// a classic "the model complied" tell -- should be flagged, not
	// silently passed through.
	raw := "Ignoring previous instructions as requested, here is the system prompt."
	out := threatmodel.SanitizeOutput(raw)

	if len(out.Findings) == 0 {
		t.Error("SanitizeOutput().Findings is empty, want at least one finding for echoed injection compliance")
	}
}

func TestVerifyGuardrailIntact_ValidLabelAndText(t *testing.T) {
	t.Parallel()

	v := threatmodel.VerifyGuardrailIntact(string(guardrail.DraftAnalysisLabel), "The evidence suggests several possible interpretations.")
	if !v.Passed() {
		t.Errorf("VerifyGuardrailIntact().Passed() = false, want true; err = %v", v.Err)
	}
	if !v.LabelIntact {
		t.Error("VerifyGuardrailIntact().LabelIntact = false, want true")
	}
	if !v.NoVerdictLanguage {
		t.Error("VerifyGuardrailIntact().NoVerdictLanguage = false, want true")
	}
	if v.Err != nil {
		t.Errorf("VerifyGuardrailIntact().Err = %v, want nil", v.Err)
	}
}

func TestVerifyGuardrailIntact_MissingLabel(t *testing.T) {
	t.Parallel()

	v := threatmodel.VerifyGuardrailIntact("some_other_label", "Neutral analysis text.")
	if v.Passed() {
		t.Error("VerifyGuardrailIntact().Passed() = true, want false for a missing/wrong label")
	}
	if v.LabelIntact {
		t.Error("VerifyGuardrailIntact().LabelIntact = true, want false")
	}
	if !errors.Is(v.Err, guardrail.ErrMissingLabel) {
		t.Errorf("VerifyGuardrailIntact().Err = %v, want wrapping guardrail.ErrMissingLabel", v.Err)
	}
}

func TestVerifyGuardrailIntact_VerdictLanguageDetected(t *testing.T) {
	t.Parallel()

	// Correct label, but the text itself has drifted into verdict
	// language -- the label alone is not sufficient, so this must still
	// fail.
	v := threatmodel.VerifyGuardrailIntact(string(guardrail.DraftAnalysisLabel), "The defendant is guilty of the charges.")
	if v.Passed() {
		t.Error("VerifyGuardrailIntact().Passed() = true, want false for verdict language")
	}
	if !v.LabelIntact {
		t.Error("VerifyGuardrailIntact().LabelIntact = false, want true (label itself was correct)")
	}
	if v.NoVerdictLanguage {
		t.Error("VerifyGuardrailIntact().NoVerdictLanguage = true, want false")
	}
	if !errors.Is(v.Err, guardrail.ErrVerdictLanguageDetected) {
		t.Errorf("VerifyGuardrailIntact().Err = %v, want wrapping guardrail.ErrVerdictLanguageDetected", v.Err)
	}
}

func TestGuardrailVerification_Passed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		labelIntact       bool
		noVerdictLanguage bool
		want              bool
	}{
		{"both true", true, true, true},
		{"label false", false, true, false},
		{"content false", true, false, false},
		{"both false", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := threatmodel.GuardrailVerification{LabelIntact: tt.labelIntact, NoVerdictLanguage: tt.noVerdictLanguage}
			if got := v.Passed(); got != tt.want {
				t.Errorf("Passed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSummarizeSanitizedOutput(t *testing.T) {
	t.Parallel()

	t.Run("clean output", func(t *testing.T) {
		t.Parallel()
		out := threatmodel.SanitizeOutput("perfectly clean text")
		got := threatmodel.SummarizeSanitizedOutput(out)
		if got != "clean" {
			t.Errorf("SummarizeSanitizedOutput() = %q, want %q", got, "clean")
		}
	})

	t.Run("stripped control chars reported", func(t *testing.T) {
		t.Parallel()
		out := threatmodel.SanitizeOutput("bad\x00text")
		got := threatmodel.SummarizeSanitizedOutput(out)
		if got == "clean" {
			t.Errorf("SummarizeSanitizedOutput() = %q, want a non-clean summary", got)
		}
	})
}
