package segmentation

import (
	"strings"
	"unicode"
)

// abbreviations lists common English abbreviations whose trailing period
// must NOT be treated as a sentence boundary. Matching is case-insensitive
// and anchored to the token immediately preceding the period.
//
// This mirrors packages/multilingual's rule-based, no-ML design principle:
// sentence splitting here is a deterministic function of punctuation and a
// small, well-known abbreviation list — never a statistical or ML model.
var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"sr": true, "jr": true, "st": true, "vs": true, "etc": true,
	"e.g": true, "i.e": true, "no": true, "art": true, "sec": true,
	"para": true, "fig": true, "vol": true, "ed": true, "inc": true,
	"ltd": true, "co": true, "corp": true, "jan": true, "feb": true,
	"mar": true, "apr": true, "jun": true, "jul": true, "aug": true,
	"sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
	"hon": true, "govt": true, "u.s": true, "u.k": true,
}

// sentenceTerminators are the runes that can end a sentence.
var sentenceTerminators = map[rune]bool{
	'.': true, '!': true, '?': true,
	'۔': true, // ARABIC FULL STOP
	'।': true, // DEVANAGARI DANDA (used in some South Asian scripts)
}

// clauseSeparators are the runes that mark a clause boundary within a
// sentence (weaker than a sentence terminator).
var clauseSeparators = map[rune]bool{
	',': true, ';': true, ':': true,
	'،': true, // ARABIC COMMA
	'؛': true, // ARABIC SEMICOLON
}

// Span is a rune-offset range [Start, End) into the text passed to
// SplitSentences/SplitClauses, paired with the substring it denotes.
type Span struct {
	// Start is the inclusive rune offset at which Text begins.
	Start int
	// End is the exclusive rune offset at which Text ends.
	End int
	// Text is the trimmed substring text[Start:End] would denote before
	// trimming; Text itself has surrounding whitespace trimmed but Start/End
	// still refer to the untrimmed boundaries used for span coverage.
	Text string
}

// SplitSentences splits text into sentence-level Spans using deterministic,
// rule-based punctuation matching. It is abbreviation-aware: a period
// immediately following a known abbreviation (see abbreviations) is not
// treated as a sentence boundary.
//
// The returned spans, in order, cover the full rune range of text with no
// gaps and no overlaps (ValidateSpanCoverage over the returned Start/End
// values with totalRunes = len([]rune(text)) will succeed), so callers can
// reconstruct exact source offsets for downstream Segment.Span population.
//
// SplitSentences returns an empty (non-nil) slice for empty/whitespace-only
// text.
func SplitSentences(text string) []Span {
	return splitByTerminators(text, sentenceTerminators, isAbbreviationBoundary)
}

// SplitClauses splits text into clause-level Spans, breaking on comma,
// semicolon, colon, and script-specific clause punctuation in addition to
// sentence terminators. Clauses are always at least as fine-grained as
// sentences: every sentence boundary is also a clause boundary.
//
// Like SplitSentences, the returned spans cover the full rune range of text
// with no gaps and no overlaps.
func SplitClauses(text string) []Span {
	combined := map[rune]bool{}
	for r := range sentenceTerminators {
		combined[r] = true
	}
	for r := range clauseSeparators {
		combined[r] = true
	}
	return splitByTerminators(text, combined, isAbbreviationBoundary)
}

// splitByTerminators is the shared implementation behind SplitSentences and
// SplitClauses. terminators is the set of runes that can end a unit;
// suppress(runes, i) reports whether the terminator at rune index i should
// NOT be treated as a boundary (e.g. because it follows an abbreviation).
func splitByTerminators(text string, terminators map[rune]bool, suppress func(runes []rune, i int) bool) []Span {
	runes := []rune(text)
	if len(strings.TrimSpace(text)) == 0 {
		return []Span{}
	}

	spans := []Span{}
	unitStart := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if !terminators[r] {
			continue
		}
		if suppress != nil && suppress(runes, i) {
			continue
		}
		// Extend over any immediately-following terminators/quotes/spaces
		// that belong to the same boundary (e.g. "?!", closing quote).
		end := i + 1
		for end < len(runes) && (terminators[runes[end]] || isClosingQuote(runes[end])) {
			end++
		}
		spans = append(spans, Span{
			Start: unitStart,
			End:   end,
			Text:  strings.TrimSpace(string(runes[unitStart:end])),
		})
		unitStart = end
	}
	if unitStart < len(runes) {
		spans = append(spans, Span{
			Start: unitStart,
			End:   len(runes),
			Text:  strings.TrimSpace(string(runes[unitStart:])),
		})
	}

	return spans
}

// isClosingQuote reports whether r is a closing quote/bracket rune commonly
// found immediately after sentence-terminating punctuation.
func isClosingQuote(r rune) bool {
	switch r {
	case '"', '\'', '”', '’', ')', ']':
		return true
	default:
		return false
	}
}

// isAbbreviationBoundary reports whether the terminator at runes[i] (which
// must be '.') is preceded by a known abbreviation and should therefore NOT
// be treated as a sentence/clause boundary. Only '.' is ever suppressed;
// '!' and '?' always terminate.
func isAbbreviationBoundary(runes []rune, i int) bool {
	if runes[i] != '.' {
		return false
	}
	// Walk backward to collect the word immediately preceding the period.
	j := i
	for j > 0 && !unicode.IsSpace(runes[j-1]) && runes[j-1] != '.' {
		j--
	}
	word := strings.ToLower(string(runes[j:i]))
	if word == "" {
		return false
	}
	return abbreviations[word]
}
