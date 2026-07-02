package multilingual

import "unicode"

// Bidi control characters used to wrap right-to-left runs so that
// downstream renderers/consumers preserve correct visual ordering when
// RTL text is embedded in a predominantly left-to-right document (or vice
// versa). These are the standard Unicode bidirectional formatting
// characters; Verdex does not implement its own bidi algorithm, it applies
// these markers around detected RTL runs.
const (
	// RLE (Right-to-Left Embedding) opens a right-to-left run.
	RLE = '‫'

	// LRE (Left-to-Right Embedding) opens a left-to-right run.
	LRE = '‪'

	// PDF (Pop Directional Formatting) closes the most recently opened
	// embedding.
	PDF = '‬'
)

// TextRun is a contiguous span of text that shares a single directionality.
type TextRun struct {
	// Text is the run's content, in logical (reading) order — i.e. the
	// order a human would type or read the characters, not the order
	// they are painted on screen for a right-to-left run.
	Text string

	// IsRTL is true when Text should be rendered right-to-left (Arabic or
	// Urdu script content).
	IsRTL bool
}

// rtlScripts is the set of scripts Verdex treats as right-to-left.
var rtlScripts = map[Script]bool{
	ScriptArabic: true,
}

// IsRTLScript reports whether s is a right-to-left script.
func IsRTLScript(s Script) bool {
	return rtlScripts[s]
}

// containsRTLRune reports whether r belongs to a Unicode range Verdex
// treats as right-to-left (currently: Arabic script, which covers both
// Arabic and Urdu text).
func containsRTLRune(r rune) bool {
	return unicode.In(r, unicode.Arabic)
}

// DetectRTLRuns splits text into a sequence of TextRuns, each tagged
// IsRTL according to whether its characters belong to a right-to-left
// script. Characters with no directionality of their own (whitespace,
// digits, punctuation) attach to whichever run they fall inside rather
// than starting a new run, so runs are not needlessly fragmented around
// spaces and punctuation. Text is preserved in logical order: DetectRTLRuns
// does not reorder characters, it only annotates spans, preserving the
// logical-vs-visual-order distinction (reordering for display, if ever
// required, is a rendering-layer concern outside this package).
//
// DetectRTLRuns returns nil for empty text.
func DetectRTLRuns(text string) []TextRun {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	var runs []TextRun
	var current []rune
	currentRTL := false
	haveCurrent := false

	flush := func() {
		if haveCurrent && len(current) > 0 {
			runs = append(runs, TextRun{Text: string(current), IsRTL: currentRTL})
		}
		current = nil
	}

	for _, r := range runes {
		isRTL := containsRTLRune(r)
		isNeutral := !isRTL && !unicode.In(r, unicode.Latin, unicode.Tamil)

		switch {
		case !haveCurrent:
			current = append(current, r)
			currentRTL = isRTL
			haveCurrent = true
		case isNeutral:
			// Neutral characters (spaces, digits, punctuation) join the
			// current run without changing its directionality.
			current = append(current, r)
		case isRTL == currentRTL:
			current = append(current, r)
		default:
			flush()
			current = append(current, r)
			currentRTL = isRTL
			haveCurrent = true
		}
	}
	flush()

	return runs
}

// WrapWithBidiControls wraps text with explicit Unicode bidi embedding
// controls (RLE/LRE ... PDF) matching isRTL, so that downstream consumers
// which render text directly (rather than re-running the bidi algorithm)
// preserve correct visual ordering. The underlying logical text is
// unchanged; only formatting control characters are added at the
// boundaries.
func WrapWithBidiControls(text string, isRTL bool) string {
	if text == "" {
		return text
	}
	if isRTL {
		return string(RLE) + text + string(PDF)
	}
	return string(LRE) + text + string(PDF)
}
