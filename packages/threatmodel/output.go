package threatmodel

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// This file is task 4's output-handling safeguards: validation applied
// to model output before it is surfaced to a human or persisted.
// SanitizeOutput strips/flags unexpected control sequences;
// VerifyGuardrailIntact confirms the mandatory non-binding guardrail
// label and verdict-language check are still intact. Both call
// through to packages/guardrail (Phase 057) -- the only non-binding-
// guardrail enforcement mechanism in this codebase -- rather than
// reimplementing any label or verdict-language check; see doc.go.

// SanitizedOutput is the result of running SanitizeOutput against a
// piece of model output: the cleaned text plus a record of what, if
// anything, was stripped or flagged.
type SanitizedOutput struct {
	// Text is the output after control-character stripping.
	Text string `json:"text"`

	// StrippedControlChars is how many disallowed control characters
	// were removed from the original text -- non-zero here is worth
	// logging/alerting on: legitimate model output should essentially
	// never contain raw control bytes, so a non-zero count may indicate
	// an injection attempt that survived into the output stage, a
	// serialization bug, or a compromised provider response.
	StrippedControlChars int `json:"stripped_control_chars"`

	// Findings lists any prompt-injection-style patterns
	// DetectInjectionAttempt located within the output text itself --
	// output that echoes injected instructions back (a common
	// jailbreak "compliance" tell) is itself worth flagging, not just
	// ingestion-time input.
	Findings []Finding `json:"findings,omitempty"`
}

// SanitizeOutput strips disallowed control characters from raw model
// output (via SanitizeControlChars) and additionally screens the
// cleaned text for injection-style patterns (via
// DetectInjectionAttempt) that may indicate the model echoed back or
// complied with an injected instruction. Unlike the ingestion-side
// DetectInjectionAttempt call, a positive Finding here does not by
// itself mean the output must be discarded -- it means the output
// should be flagged for review; callers that want a hard rejection
// gate should inspect the returned SanitizedOutput.Findings themselves
// and decide, since what counts as an acceptable output varies by
// caller (e.g. a moderation dashboard wants to surface every finding
// for human review, while a fully automated pipeline may want to
// reject outright).
func SanitizeOutput(raw string) SanitizedOutput {
	cleaned := SanitizeControlChars(raw)
	stripped := countControlChars(raw) - countControlChars(cleaned)
	if stripped < 0 {
		stripped = 0
	}
	_, findings := DetectInjectionAttempt(cleaned)
	return SanitizedOutput{
		Text:                 cleaned,
		StrippedControlChars: stripped,
		Findings:             findings,
	}
}

// countControlChars counts disallowed control characters (the same
// set SanitizeControlChars strips) in s, used by SanitizeOutput to
// report how many were actually removed.
func countControlChars(s string) int {
	n := 0
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			n++
		}
	}
	return n
}

// GuardrailVerification is the result of VerifyGuardrailIntact: a
// structured record of whether model output still carries the
// mandatory non-binding guardrail label and passes the verdict-
// language content check, suitable for logging or an audit Detail
// string.
type GuardrailVerification struct {
	// LabelIntact reports whether label still equals
	// guardrail.DraftAnalysisLabel.
	LabelIntact bool `json:"label_intact"`

	// NoVerdictLanguage reports whether text passed
	// guardrail.CheckText (i.e. contains no detected verdict/directive
	// language).
	NoVerdictLanguage bool `json:"no_verdict_language"`

	// Err is the first error encountered (label check takes precedence
	// over the content check), nil if both checks passed.
	Err error `json:"-"`
}

// Passed reports whether both the label and content checks succeeded.
func (v GuardrailVerification) Passed() bool {
	return v.LabelIntact && v.NoVerdictLanguage
}

// VerifyGuardrailIntact is a defensive re-verification of the two
// guarantees packages/guardrail's own pipeline-stage check already
// enforces once, applied again immediately before output is surfaced
// or persisted -- independent of whether that earlier check passed,
// so a mutation occurring after the pipeline's own check (e.g. a
// lossy serialization round-trip, a downstream transform) cannot
// silently drop the label or reintroduce verdict language without
// this second check catching it. It calls guardrail.RequireLabel and
// guardrail.CheckText directly rather than reimplementing either
// check.
func VerifyGuardrailIntact(label string, text string) GuardrailVerification {
	v := GuardrailVerification{}

	if err := guardrail.RequireLabel(label); err != nil {
		v.Err = wrapf("VerifyGuardrailIntact", err)
		return v
	}
	v.LabelIntact = true

	if err := guardrail.CheckText(text); err != nil {
		v.Err = wrapf("VerifyGuardrailIntact", err)
		return v
	}
	v.NoVerdictLanguage = true

	return v
}

// SummarizeSanitizedOutput renders a short, human-readable one-line
// summary of a SanitizedOutput, e.g. "3 control chars stripped, 1
// finding" -- used by audit Detail strings and log lines.
func SummarizeSanitizedOutput(out SanitizedOutput) string {
	parts := make([]string, 0, 2)
	if out.StrippedControlChars > 0 {
		parts = append(parts, strconv.Itoa(out.StrippedControlChars)+" control chars stripped")
	}
	if len(out.Findings) > 0 {
		parts = append(parts, SummarizeFindings(out.Findings))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, "; ")
}
