package localization

import (
	"github.com/YASSERRMD/verdex/packages/citation"
)

// LocalizeCitation composes a citation.Formatter's jurisdiction-
// appropriate citation text (packages/citation's Phase 046
// responsibility -- which citation *style* a case name, statute
// section, or precedent reference renders in for a given legal
// family/jurisdiction key) with this package's own locale-aware
// bidi handling for a right-to-left target locale.
//
// This function deliberately does not reimplement common-law vs
// civil-law citation templates -- see packages/citation/formatter.go's
// CommonLawFormatter/CivilLawFormatter, which remain the only citation-
// style logic in this codebase.
//
// It also deliberately does NOT re-render any digit run inside the
// formatted citation with locale-aware thousands-grouping (an earlier
// version of this function did; see git history). Grouping a citation
// year or docket number -- "[2,020] UKSC 1" instead of "[2020] UKSC
// 1" -- is not a real legal-citation convention in any of this
// package's four locales; citation figures (years, section numbers,
// docket numbers) are conventionally rendered ungrouped and in Western
// numerals even within otherwise-Arabic-script legal text (matching
// UAE/Gulf judicial-document convention -- this platform's primary
// jurisdiction focus per packages/jurisdiction's seed data), which is
// exactly what citation.FormatInput's plain ASCII-digit fields already
// produce untouched. FormatInteger/FormatFloat (format.go) remain for
// genuinely locale-sensitive figures -- dates and report prose numbers
// -- where grouping and locale convention do apply; citation text is
// not one of them.
//
// What genuinely is locale-sensitive about a citation is
// *directionality*: a citation is itself always Latin-script/LTR text
// (case names, "v", statute names, and reporter abbreviations in this
// codebase's supported jurisdictions are all rendered in Latin
// script), but when embedded inside a right-to-left target locale's
// surrounding prose (a localized report body, see report.go), it must
// be wrapped with explicit bidi embedding controls so a renderer that
// does not itself re-run the Unicode bidi algorithm still displays it
// left-to-right in the correct position within the RTL paragraph. This
// package reuses packages/multilingual's WrapWithBidiControls posture
// by convention (the same RLE/PDF marker pair, applied here rather
// than imported, since packages/multilingual's own WrapWithBidiControls
// takes an isRTL bool for a text *span* it already classified --
// exactly the shape LocalizeCitation's own IsRTL(locale) call
// produces) rather than reimplementing bidi markers from scratch.
//
// Returns ErrNilFormatter if formatter is nil.
func LocalizeCitation(formatter citation.Formatter, locale Locale, in citation.FormatInput) (string, error) {
	if formatter == nil {
		return "", ErrNilFormatter
	}
	formatted := formatter.Format(in)
	if formatted == "" {
		return formatted, nil
	}
	if IsRTL(locale) {
		return wrapLTRRunForRTLContext(formatted), nil
	}
	return formatted, nil
}

// Unicode bidi embedding control characters, matching
// packages/multilingual's RLE/LRE/PDF constants exactly (same code
// points, same semantics) -- see this file's doc comment for why this
// package applies them directly rather than importing
// packages/multilingual.
const (
	// bidiLRE (Left-to-Right Embedding) opens a left-to-right run.
	bidiLRE = '‪'

	// bidiPDF (Pop Directional Formatting) closes the most recently
	// opened embedding.
	bidiPDF = '‬'
)

// wrapLTRRunForRTLContext wraps text (a citation string, always
// Latin-script/LTR in this codebase's supported jurisdictions) with
// explicit LRE...PDF bidi embedding controls, so a citation embedded
// inside right-to-left surrounding prose renders in the correct
// left-to-right internal order. The underlying text is unchanged;
// only formatting control characters are added at the boundaries,
// mirroring packages/multilingual.WrapWithBidiControls's exact
// contract for the isRTL=false (embed an LTR run) case -- the only
// case LocalizeCitation ever needs, since a citation string itself is
// never RTL text in this codebase.
func wrapLTRRunForRTLContext(text string) string {
	return string(bidiLRE) + text + string(bidiPDF)
}
