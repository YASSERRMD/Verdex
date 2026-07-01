package prompts

import (
	"fmt"
	"strings"
	"unicode"
)

// SanitizeValue cleans a template variable value before injection:
//
//  1. Strips ASCII/Unicode control characters (except ordinary whitespace such
//     as space, tab, newline, and carriage return).
//  2. Returns ErrInjectionAttempt if the value contains "{{" or "}}" — the
//     delimiters of Go text/template actions — which could allow a caller to
//     inject additional template directives into the rendered output.
//  3. Returns ErrVariableTooLong if maxLen > 0 and len([]rune(v)) > maxLen.
//     Truncation is not applied automatically; the caller must handle it.
//
// The cleaned string is returned together with any error.
func SanitizeValue(v string, maxLen int) (string, error) {
	// 1. Strip control characters, keeping ordinary whitespace.
	var b strings.Builder
	b.Grow(len(v))
	for _, r := range v {
		if r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			// Drop the control character; don't surface an error for this —
			// it is silently sanitised rather than rejected.
			continue
		}
		b.WriteRune(r)
	}
	cleaned := b.String()

	// 2. Prompt-injection guard: reject values that contain template delimiters.
	if strings.Contains(cleaned, "{{") || strings.Contains(cleaned, "}}") {
		return cleaned, fmt.Errorf("%w: value contains template delimiters", ErrInjectionAttempt)
	}

	// 3. Length check (rune-based, not byte-based).
	if maxLen > 0 && len([]rune(cleaned)) > maxLen {
		return cleaned, fmt.Errorf("%w: length %d exceeds limit %d",
			ErrVariableTooLong, len([]rune(cleaned)), maxLen)
	}

	return cleaned, nil
}
