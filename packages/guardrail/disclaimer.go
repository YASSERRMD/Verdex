package guardrail

import "strings"

// outputDisclaimer is the mandatory disclaimer text appended to
// reasoning-output surfaces (a rendered Opinion, an exported report,
// etc.). It mirrors the spirit and wording of
// packages/prompts's nonBindingDisclaimer, but is a separate constant
// deliberately: packages/prompts's disclaimer is injected into
// LLM-facing INPUT (a prompt body, via PromptTemplate.NonBindingLabel),
// while outputDisclaimer is injected into human-facing OUTPUT — a
// different surface with a different audience, even though the message
// is the same in substance.
const outputDisclaimer = `

---
DRAFT ANALYSIS — NON-BINDING: This output is an AI-generated draft legal
analysis produced by the Verdex platform. It is not a verdict, ruling, or
judgment, and it does not constitute legal advice. It must not be treated
as, cited as, or relied upon in place of a determination by a qualified
legal professional or a court. The analysis may contain errors, omissions,
or inaccuracies.`

// RequireDisclaimer appends the mandatory non-binding disclaimer to text
// and returns the result. It is idempotent: if text already ends with
// outputDisclaimer, it is returned unchanged rather than appending a
// second copy. Unlike CheckText/RequireLabel, this function cannot fail —
// it always returns a disclaimed string — because there is no
// counterfactual "reject" outcome for a missing disclaimer; the correct
// remediation is simply to add it, which this function always does.
func RequireDisclaimer(text string) string {
	if strings.HasSuffix(text, outputDisclaimer) {
		return text
	}
	return text + outputDisclaimer
}

// EnsureDisclaimer is an alias for RequireDisclaimer, provided under the
// alternate name the platform's other packages use for
// already-satisfied-is-a-no-op operations (e.g.
// packages/treeassembly.ReassembleIncremental's "extend if needed"
// convention), so callers can use whichever name reads better at the
// call site. Both names apply the exact same idempotent behavior.
func EnsureDisclaimer(text string) string {
	return RequireDisclaimer(text)
}

// HasDisclaimer reports whether text already carries the mandatory
// non-binding disclaimer, without modifying it.
func HasDisclaimer(text string) bool {
	return strings.HasSuffix(text, outputDisclaimer)
}
