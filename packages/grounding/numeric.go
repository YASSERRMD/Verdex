package grounding

import "regexp"

// numericPattern matches a standalone numeral in prose: an integer or
// decimal, optionally with thousands separators, an optional currency
// symbol prefix, and an optional trailing percent sign (e.g. "1,250",
// "$4,500.00", "37.5%"). Deliberately does not match bare single digits
// that are almost always part of ordinary prose punctuation (list
// markers, footnote references) rather than a factual figure — this
// package cares about amounts, counts, and percentages a conclusion
// asserts as evidence, not every digit that appears in text.
var numericPattern = regexp.MustCompile(`[$€£]?\b\d{1,3}(?:,\d{3})*(?:\.\d+)?%?\b`)

// datePatterns are deterministic, regex-based date extractors, checked in
// order, mirroring packages/timeline/event.go's "no ML models, rule
// based" extraction idiom exactly (redeclared locally rather than
// imported, since packages/timeline is not otherwise a dependency of this
// package and pulling it in solely for three regexes would be a heavier
// coupling than the alternative of a ~20-line local copy).
var datePatterns = []*regexp.Regexp{
	// ISO-8601-shaped dates: 2024-03-15.
	regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`),
	// "March 15, 2024" / "March 15 2024" style dates.
	regexp.MustCompile(`(?i)\b(?:January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+\d{4}\b`),
	// "03/15/2024" (month/day/year) style dates.
	regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{4}\b`),
}

// minSignificantDigits is the fewest digit characters a numericPattern
// match must contain (ignoring separators/symbols) to be treated as a
// meaningful figure worth grounding-checking, filtering out incidental
// single-digit or two-digit numbers (list markers, minor cross-references)
// that are rarely the kind of "numeric assertion" a grounding check should
// flag.
const minSignificantDigits = 3

// extractNumerics returns every distinct numericPattern match in text with
// at least minSignificantDigits digit characters, in first-appearance
// order, with duplicates removed.
func extractNumerics(text string) []string {
	matches := numericPattern.FindAllString(text, -1)
	seen := make(map[string]struct{}, len(matches))
	var out []string
	for _, m := range matches {
		if countDigits(m) < minSignificantDigits {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

// extractDates returns every distinct date substring in text matched by
// any of datePatterns, in pattern-then-appearance order, with duplicates
// removed.
func extractDates(text string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, pattern := range datePatterns {
		for _, m := range pattern.FindAllString(text, -1) {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

// countDigits returns the number of ASCII digit characters in s.
func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}
