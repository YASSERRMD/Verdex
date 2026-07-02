package multilingual

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeForm identifies which Unicode normalization form was applied to a
// piece of text.
type NormalizeForm string

const (
	// FormNFC is Unicode Normalization Form C (canonical composition).
	// Verdex uses NFC as the default normalization target: it keeps
	// precomposed characters (e.g. Arabic presentation forms folded to
	// their base + combining-mark decomposition, then recomposed) while
	// remaining byte-for-byte comparable across sources that encode the
	// same visible text differently.
	FormNFC NormalizeForm = "NFC"

	// FormNFKC is Unicode Normalization Form KC (compatibility
	// composition). NFKC additionally folds compatibility variants (e.g.
	// Arabic presentation-form ligatures, full-width Latin letters) to
	// their canonical equivalents, which is useful for legal-term
	// matching where visually distinct but semantically identical glyphs
	// must compare equal.
	FormNFKC NormalizeForm = "NFKC"
)

// NormalizeUnicode transforms text into the requested Unicode normalization
// form and strips control characters that carry no semantic meaning for
// downstream text processing (segmentation, tokenization, reasoning).
//
// Whitespace cleanup rules:
//   - Any run of Unicode whitespace (including non-breaking spaces, tabs,
//     and mixed newline styles) collapses to a single ASCII space.
//   - Leading and trailing whitespace is trimmed.
//   - C0/C1 control characters other than whitespace are dropped entirely,
//     since they cannot be rendered or reasoned over meaningfully.
//
// NormalizeUnicode is idempotent: normalizing already-normalized text with
// the same form returns identical output (see unicode_test.go).
func NormalizeUnicode(text string, form NormalizeForm) string {
	cleaned := stripControlChars(text)
	collapsed := collapseWhitespace(cleaned)

	switch form {
	case FormNFKC:
		return norm.NFKC.String(collapsed)
	case FormNFC:
		fallthrough
	default:
		return norm.NFC.String(collapsed)
	}
}

// stripControlChars removes C0/C1 control characters (category Cc) other
// than the whitespace characters \t, \n, \r, which collapseWhitespace
// handles separately.
func stripControlChars(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		if unicode.Is(unicode.Cc, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// collapseWhitespace collapses any run of Unicode whitespace to a single
// ASCII space and trims leading/trailing whitespace.
func collapseWhitespace(text string) string {
	fields := strings.FieldsFunc(text, unicode.IsSpace)
	return strings.Join(fields, " ")
}
