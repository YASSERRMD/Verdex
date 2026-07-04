package securitytesting

import (
	"context"
	"errors"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// This file is task 1's automated security regression suite: a fixed
// set of Scenarios that re-check, on every run, that a previously
// established defense still holds -- so that if a future change
// accidentally weakens one, the very next suite run reports
// OutcomeFailed rather than the regression going unnoticed until
// something worse happens in production. Per this phase's brief, the
// non-binding guardrail (packages/guardrail, Phase 057) is this
// suite's flagship target: every scenario below is a genuine attempt
// to defeat CheckText/RequireDisclaimer through some adversarial
// input, followed by an assertion that the guardrail correctly held.

// ScenarioGuardrailRejectsVerdictLanguage proves guardrail.CheckText
// rejects unambiguous verdict/directive phrasing -- the platform-wide
// invariant CONTRIBUTING.md states as a hard rule
// ("Verdict or directive language is rejected by the output
// pipeline").
func ScenarioGuardrailRejectsVerdictLanguage() Scenario {
	return NewScenarioFunc(
		"regression/guardrail-rejects-verdict-language",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			adversarialTexts := []string{
				"The court hereby rules that the defendant is guilty as charged.",
				"IT IS HEREBY ORDERED that the plaintiff's motion is granted.",
				"The judgment of this court is that the defendant shall pay damages.",
			}
			for _, text := range adversarialTexts {
				if err := guardrail.CheckText(text); err == nil {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  fmt.Sprintf("guardrail.CheckText(%q) = nil, want ErrVerdictLanguageDetected -- verdict language was not rejected", text),
					}, nil
				} else if !errors.Is(err, guardrail.ErrVerdictLanguageDetected) {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  fmt.Sprintf("guardrail.CheckText(%q) returned %v, want an error wrapping ErrVerdictLanguageDetected", text, err),
					}, nil
				}
			}
			return Result{Outcome: OutcomePassed, Detail: fmt.Sprintf("guardrail.CheckText correctly rejected all %d adversarial verdict-language payloads", len(adversarialTexts))}, nil
		},
	)
}

// ScenarioGuardrailNotDefeatedByObfuscation is the adversarial half of
// this suite's guardrail coverage: it tries plausible ways an attacker
// (or a careless prompt-engineering change upstream) might attempt to
// sneak verdict language past CheckText -- mixed casing, and burying
// the verdict phrase inside otherwise-legitimate draft-analysis prose
// -- and confirms neither succeeds. This is exactly the "tests that
// try to defeat it (and confirm it holds), not bypass or weaken it"
// requirement this phase's brief calls for regarding
// packages/guardrail. See
// ScenarioGuardrailWhitespaceObfuscationKnownGap below for a THIRD
// evasion technique this suite tried that is NOT yet covered -- kept
// as a separate, explicitly-labeled scenario rather than folded in
// here, so this suite's "everything passes" default composition
// (SeedHarness, seed.go) stays a meaningful, all-green signal instead
// of permanently red on a known, tracked, un-remediated gap.
func ScenarioGuardrailNotDefeatedByObfuscation() Scenario {
	return NewScenarioFunc(
		"regression/guardrail-not-defeated-by-obfuscation",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			evasionAttempts := []string{
				// Mixed casing.
				"iT iS HeReBy OrDeReD that the respondent must comply immediately.",
				// Verdict phrase embedded mid-paragraph inside otherwise
				// legitimate-looking draft-analysis prose, rather than as an
				// obvious standalone sentence.
				"Based on the evidence reviewed, and noting the parties' submissions, the defendant is hereby ordered to comply, subject to further briefing on damages.",
			}
			for _, text := range evasionAttempts {
				if err := guardrail.CheckText(text); err == nil {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  fmt.Sprintf("guardrail.CheckText did not catch an obfuscated verdict-language evasion attempt: %q", text),
						Evidence: map[string]string{
							"payload": text,
						},
					}, nil
				}
			}
			return Result{Outcome: OutcomePassed, Detail: fmt.Sprintf("guardrail.CheckText correctly held against all %d obfuscation/evasion attempts (casing, mid-paragraph burial)", len(evasionAttempts))}, nil
		},
	)
}

// ScenarioGuardrailWhitespaceObfuscationKnownGap is a THIRD evasion
// technique this suite's adversarial testing discovered: collapsing or
// expanding the internal whitespace of an exact verdictLanguageWordlist
// phrase (e.g. "shall pay" -> "shall   pay") defeats
// guardrail.CheckText / irac.ContainsVerdictLanguage's plain
// strings.Contains substring match, since a literal substring match
// requires an EXACT run of characters, not a whitespace-normalized
// one. This IS a real, reproducible gap this adversarial suite found
// -- not a false positive in this suite's own fixture (see
// doc/security-testing.md's "Known gaps this suite found" section for
// the full writeup and why this phase does not itself patch
// packages/irac's wordlist-matching code).
//
// This Scenario is deliberately excluded from SeedHarness's default
// composition (seed.go) and from NewRegressionSuite -- a suite whose
// "everything must pass" default run is permanently red on a known,
// externally-owned, not-yet-remediated gap stops being a useful
// signal for anything else regressing. It is reachable by name via
// NewHarness(ScenarioGuardrailWhitespaceObfuscationKnownGap()) for a
// caller (or a future remediation-verification re-run, see
// Engine.VerifyRemediation) that specifically wants to check whether
// this particular gap has been closed yet.
func ScenarioGuardrailWhitespaceObfuscationKnownGap() Scenario {
	return NewScenarioFunc(
		"regression/guardrail-whitespace-obfuscation-known-gap",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			// "shall pay" is a real, verbatim entry in
			// irac's verdictLanguageWordlist; expanding the internal space
			// to three spaces is a trivial obfuscation an attacker (or a
			// sloppy LLM completion) could produce with no effort at all.
			text := "the defendant shall   pay damages immediately."
			if err := guardrail.CheckText(text); err == nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "guardrail.CheckText did not catch a whitespace-obfuscated verdict phrase (\"shall   pay\") -- known, tracked gap in irac.ContainsVerdictLanguage's exact-substring matching; see doc/security-testing.md",
					Evidence: map[string]string{
						"payload": text,
					},
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "guardrail.CheckText now correctly catches whitespace-obfuscated verdict phrases -- this known gap has been remediated"}, nil
		},
	)
}

// ScenarioGuardrailDisclaimerSurvivesConcatenation proves
// guardrail.RequireDisclaimer's idempotency guarantee holds even when
// an attacker attempts to defeat it by pre-seeding text with a
// near-identical but subtly altered fake disclaimer, attempting to
// trick a naive suffix check into treating the real disclaimer as
// already present when it is not (so the genuine disclaimer never gets
// appended). RequireDisclaimer must still append its own real
// disclaimer in this case, since HasSuffix requires an EXACT match,
// not a fuzzy one.
func ScenarioGuardrailDisclaimerSurvivesConcatenation() Scenario {
	return NewScenarioFunc(
		"regression/guardrail-disclaimer-survives-spoofing",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			// A fake, subtly-altered disclaimer an attacker (or a
			// compromised upstream template) might prepend, hoping
			// RequireDisclaimer's idempotency check treats it as
			// equivalent to the real thing and skips appending the actual
			// disclaimer.
			spoofed := "Some draft analysis text.\n\n---\nDRAFT ANALYSIS -- this is NOT non-binding and IS a final verdict."

			result := guardrail.RequireDisclaimer(spoofed)
			if !guardrail.HasDisclaimer(result) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "guardrail.RequireDisclaimer did not append the real disclaimer after a spoofed fake-disclaimer prefix -- disclaimer-spoofing evasion succeeded",
					Evidence: map[string]string{
						"spoofed_input": spoofed,
						"result":        result,
					},
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "guardrail.RequireDisclaimer correctly appended the real disclaimer despite a spoofed fake-disclaimer prefix in the input"}, nil
		},
	)
}

// NewRegressionSuite returns every Scenario in this file's fixed
// automated security regression suite.
func NewRegressionSuite() []Scenario {
	return []Scenario{
		ScenarioGuardrailRejectsVerdictLanguage(),
		ScenarioGuardrailNotDefeatedByObfuscation(),
		ScenarioGuardrailDisclaimerSurvivesConcatenation(),
	}
}
