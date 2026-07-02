package segmentation

import (
	"regexp"
	"strings"
	"unicode"
)

// headingNumberingPattern matches common legal/document numbering prefixes
// that introduce a heading or section, e.g. "1.", "1.1", "I.", "A.",
// "(a)", "Section 3", "Article 12", "Chapter IV".
var headingNumberingPattern = regexp.MustCompile(
	`(?i)^\s*(` +
		`(\d+(\.\d+)*\.?)|` + // 1  1.  1.1  1.2.3
		`([IVXLCDM]+\.)|` + // Roman numerals: I.  IV.  XII.
		`(\([a-zA-Z0-9]+\))|` + // (a) (1) (iv)
		`([A-Z]\.)|` + // A.  B.
		`(section\s+\S+)|` +
		`(article\s+\S+)|` +
		`(chapter\s+\S+)|` +
		`(part\s+\S+)` +
		`)\s+\S`,
)

// maxHeadingWords bounds how many words a short, punctuation-free,
// all-caps/title-case line may contain before it's no longer considered a
// plausible heading (avoids misclassifying long all-caps quoted statements).
const maxHeadingWords = 12

// IsHeadingLine reports whether line is structurally a heading/section
// title using deterministic heuristics — no ML model:
//
//  1. Numbering patterns: lines beginning with "1.", "1.1", "I.", "(a)",
//     "Section 3", "Article 12", "Chapter IV", etc.
//  2. Short ALL-CAPS lines: entirely upper-case (ignoring digits,
//     punctuation, and whitespace), at least one letter, at most
//     maxHeadingWords words, and not ending in a sentence terminator.
//  3. Short Title-Case lines with no terminal punctuation: every word
//     capitalized, at most maxHeadingWords words, no trailing sentence
//     terminator — e.g. "Statement Of Facts".
//
// Empty or whitespace-only lines are never headings.
func IsHeadingLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	if headingNumberingPattern.MatchString(trimmed) {
		return true
	}

	words := strings.Fields(trimmed)
	if len(words) == 0 || len(words) > maxHeadingWords {
		return false
	}

	lastRune := []rune(trimmed)[len([]rune(trimmed))-1]
	if sentenceTerminators[lastRune] {
		return false
	}

	if isAllCapsLine(trimmed) {
		return true
	}

	return isTitleCaseLine(words)
}

// isAllCapsLine reports whether trimmed contains at least one letter and no
// lower-case letters.
func isAllCapsLine(trimmed string) bool {
	hasLetter := false
	for _, r := range trimmed {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsUpper(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

// isTitleCaseLine reports whether every word starts with an upper-case
// letter (allowing short connector words like "of", "the", "and", "a", "in"
// to be lower-case, as is conventional in title case).
func isTitleCaseLine(words []string) bool {
	connectors := map[string]bool{
		"of": true, "the": true, "and": true, "a": true, "an": true,
		"in": true, "on": true, "for": true, "to": true, "or": true,
	}
	capitalized := 0
	for _, w := range words {
		runes := []rune(w)
		first := runes[0]
		if !unicode.IsLetter(first) {
			continue
		}
		if unicode.IsUpper(first) {
			capitalized++
			continue
		}
		if connectors[strings.ToLower(w)] {
			continue
		}
		return false
	}
	// Require at least one genuinely capitalized word so a fully-lowercase
	// connector-only line (unlikely in practice) isn't misclassified.
	return capitalized > 0
}

// TagHeadings scans segs and sets Type = SegmentHeading on every segment
// whose Text satisfies IsHeadingLine, returning the updated slice. Segments
// that already have a non-paragraph Type are left unchanged, so heading
// detection never overrides a more specific classification (e.g. exhibit or
// citation) made by an earlier pass.
func TagHeadings(segs []Segment) []Segment {
	out := make([]Segment, len(segs))
	for i, s := range segs {
		if s.Type == SegmentParagraph && IsHeadingLine(s.Text) {
			s.Type = SegmentHeading
		}
		out[i] = s
	}
	return out
}
