package pii

import (
	"context"
	"regexp"
	"strings"
)

// PIIMatch describes a single detected occurrence of personally identifiable
// information within a text.
//
// Offsets are rune offsets into the text passed to Detect, mirroring
// packages/segmentation's SourceSpan convention (Start inclusive, End
// exclusive) so PII matches can be located precisely within the source text
// without ambiguity across multi-byte runes.
type PIIMatch struct {
	// Start is the inclusive rune offset into the source text at which this
	// match begins.
	Start int

	// End is the exclusive rune offset into the source text at which this
	// match ends. End must be >= Start.
	End int

	// Text is the exact matched substring, provided for convenience so
	// callers do not need to re-slice the original text by rune offset.
	Text string

	// Category classifies the kind of PII this match represents. Populated
	// by classification (see category.go); a Detector may leave this at its
	// zero value and let the caller classify separately.
	Category PIICategory

	// Pattern names the specific rule or model that produced this match
	// (e.g. "email", "phone", "person_name_heuristic"). Useful for
	// diagnostics and recall tuning.
	Pattern string

	// Confidence is this match's confidence score in the closed interval
	// [0, 1]. Deterministic regex/pattern matches typically report 1.0;
	// heuristic matches (e.g. person-name detection) may report less.
	Confidence float64
}

// Len returns the rune length of the match (End - Start). Returns 0 if End
// <= Start.
func (m PIIMatch) Len() int {
	if m.End <= m.Start {
		return 0
	}
	return m.End - m.Start
}

// Detector finds personally identifiable information within a text.
//
// This interface is the pluggable extension point for PII detection: the
// default implementation in this file (RuleBasedDetector) is a deterministic
// regex/heuristic engine with no ML/NER dependency, mirroring
// packages/segmentation and packages/multilingual's "no ML models, rule
// based" design principle. A future phase can swap in a real NER model (a
// named-entity-recognition model trained to find PERSON/LOC/ORG spans, or a
// hosted PII-detection API) by implementing this same interface — no caller
// of Detector needs to change.
type Detector interface {
	// Detect scans text and returns every PIIMatch found, in ascending
	// Start order. Returns an empty (nil) slice, not an error, when no PII
	// is found. ctx allows implementations that call out to an external
	// model or service to respect cancellation/deadlines.
	Detect(ctx context.Context, text string) ([]PIIMatch, error)
}

// RuleBasedDetector is the default, deterministic Detector implementation.
// It uses regular expressions and lightweight heuristics to find common PII
// patterns: email addresses, phone numbers, national-ID-like number
// patterns, physical addresses, and person names. It performs no machine
// learning and calls out to no external service, so its output is fully
// reproducible given the same input text.
//
// RuleBasedDetector is intentionally conservative in favor of recall over
// precision: downstream redaction (see redact.go) is expected to over-redact
// rather than leak PII, so borderline matches are still reported.
type RuleBasedDetector struct{}

// NewRuleBasedDetector constructs a RuleBasedDetector. It has no
// configuration today; the constructor exists so call sites can be updated
// uniformly if configuration is added later.
func NewRuleBasedDetector() *RuleBasedDetector {
	return &RuleBasedDetector{}
}

var (
	emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// phonePattern matches common phone-number shapes: optional leading +,
	// optional parenthesized area code, and groups of digits separated by
	// spaces, dots, or hyphens, with at least 7 total digits.
	phonePattern = regexp.MustCompile(`\+?\(?\d{2,4}\)?[\s.\-]?\d{3,4}[\s.\-]\d{3,4}(?:[\s.\-]\d{2,4})?`)

	// nationalIDPattern matches national-ID-like number sequences: long runs
	// of digits (optionally hyphen/space-grouped) of a length typical of
	// passport, SSN, Emirates ID, or similar identifiers (9-18 digits).
	nationalIDPattern = regexp.MustCompile(`\b\d{3}[-\s]?\d{2,4}[-\s]?\d{4,7}\b|\b\d{9,18}\b`)

	// addressPattern matches a leading street-number-plus-street-name
	// fragment followed by a common street-type word, a light heuristic for
	// physical addresses (e.g. "221B Baker Street", "742 Evergreen Terrace
	// Ave").
	addressPattern = regexp.MustCompile(`\b\d{1,6}[A-Za-z]?\s+([A-Z][a-zA-Z'\-]*\s?){1,4}(Street|St\.?|Avenue|Ave\.?|Road|Rd\.?|Boulevard|Blvd\.?|Lane|Ln\.?|Drive|Dr\.?|Court|Ct\.?|Way|Terrace|Place|Pl\.?)\b`)

	// personNamePattern is a heuristic for "First Last" or "First Middle
	// Last" capitalized name sequences, optionally preceded by an honorific.
	// It intentionally over-matches (e.g. proper nouns generally) since
	// downstream redaction favors recall.
	personNamePattern = regexp.MustCompile(`\b(?:Mr\.|Mrs\.|Ms\.|Dr\.|Judge|Justice)\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+){0,2}\b|\b[A-Z][a-z]+\s+[A-Z][a-z]+\b`)
)

// Detect implements Detector using the package-level regex patterns. Matches
// from different patterns that overlap are de-duplicated, preferring the
// earliest-starting, then longest, match.
func (d *RuleBasedDetector) Detect(_ context.Context, text string) ([]PIIMatch, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	runes := []rune(text)

	var matches []PIIMatch
	matches = append(matches, findMatches(runes, text, emailPattern, "email")...)
	matches = append(matches, findMatches(runes, text, phonePattern, "phone")...)
	matches = append(matches, findMatches(runes, text, nationalIDPattern, "national_id")...)
	matches = append(matches, findMatches(runes, text, addressPattern, "address")...)
	matches = append(matches, findMatches(runes, text, personNamePattern, "person_name_heuristic")...)

	matches = dedupeOverlapping(matches)
	return matches, nil
}

// findMatches runs pattern over text (using byte offsets from regexp) and
// converts each match to rune offsets, tagging it with patternName.
func findMatches(runes []rune, text string, pattern *regexp.Regexp, patternName string) []PIIMatch {
	locs := pattern.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return nil
	}

	// Build a byte-offset -> rune-offset lookup by walking runes and
	// accumulating each rune's UTF-8 byte length.
	byteToRune := make(map[int]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		byteToRune[b] = i
		b += len(string(r))
	}
	byteToRune[b] = len(runes) // end-of-string sentinel

	confidence := 1.0
	if patternName == "person_name_heuristic" {
		confidence = 0.6
	}

	out := make([]PIIMatch, 0, len(locs))
	for _, loc := range locs {
		startRune, ok := byteToRune[loc[0]]
		if !ok {
			startRune = nearestRuneOffset(byteToRune, loc[0])
		}
		endRune, ok := byteToRune[loc[1]]
		if !ok {
			endRune = nearestRuneOffset(byteToRune, loc[1])
		}
		out = append(out, PIIMatch{
			Start:      startRune,
			End:        endRune,
			Text:       string(runes[startRune:endRune]),
			Pattern:    patternName,
			Confidence: confidence,
		})
	}
	return out
}

// nearestRuneOffset finds the rune offset for a byte offset not present in
// the map (should not normally happen given full population above; kept as
// a defensive fallback).
func nearestRuneOffset(byteToRune map[int]int, byteOffset int) int {
	best := 0
	bestByte := -1
	for b, r := range byteToRune {
		if b <= byteOffset && b > bestByte {
			bestByte = b
			best = r
		}
	}
	return best
}

// dedupeOverlapping sorts matches by Start then removes matches that overlap
// an already-accepted match, preferring the earliest-starting, then longest
// match at each position. This avoids e.g. a person-name heuristic match
// re-flagging text already captured by a more specific email/phone pattern.
func dedupeOverlapping(matches []PIIMatch) []PIIMatch {
	if len(matches) == 0 {
		return nil
	}

	sortMatches(matches)

	var out []PIIMatch
	for _, m := range matches {
		overlaps := false
		for _, existing := range out {
			if m.Start < existing.End && existing.Start < m.End {
				overlaps = true
				break
			}
		}
		if !overlaps {
			out = append(out, m)
		}
	}
	return out
}

// sortMatches sorts matches by Start ascending, then by descending length
// (longest first) so dedupeOverlapping prefers the most specific/longest
// match at a given starting position.
func sortMatches(matches []PIIMatch) {
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0; j-- {
			a, b := matches[j-1], matches[j]
			if a.Start > b.Start || (a.Start == b.Start && a.Len() < b.Len()) {
				matches[j-1], matches[j] = matches[j], matches[j-1]
			} else {
				break
			}
		}
	}
}
