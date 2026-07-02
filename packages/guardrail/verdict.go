package guardrail

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// CheckText is the hard gate against verdict or directive phrasing in any
// reasoning-output text. It wraps irac.ContainsVerdictLanguage — which
// returns a bare bool, a shape any caller can accidentally ignore — and
// instead returns a typed, errors.Is-compatible error
// (ErrVerdictLanguageDetected) so a caller must handle rejection
// explicitly. There is no companion bool-returning variant: an
// error-returning function is the only way this package exposes this
// check, by design (see doc.go's override-prevention discussion and
// doc/guardrail-policy.md).
//
// CheckText is a strict superset of what
// packages/synthesisagent.Provider checked inline before this package
// existed (a direct irac.ContainsVerdictLanguage(tc.Text) call): the
// underlying wordlist and matching logic are identical, so any text
// rejected by the old inline check is still rejected here, and vice
// versa.
func CheckText(s string) error {
	if irac.ContainsVerdictLanguage(s) {
		return fmt.Errorf("%w: %q", ErrVerdictLanguageDetected, truncateForError(s))
	}
	return nil
}

// truncateForError bounds how much of a rejected text is echoed back in
// an error message, so a very long reasoning-output string does not blow
// up log lines or error-wrapping chains.
func truncateForError(s string) string {
	const maxLen = 120
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
