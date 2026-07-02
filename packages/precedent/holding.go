package precedent

import (
	"regexp"
	"strings"
)

// HoldingExtractionResult bundles the two pieces of text
// ExtractHoldingAndRatio pulls out of a precedent's FullText.
type HoldingExtractionResult struct {
	// Holding is the court's core determination (e.g. "the manufacturer
	// owes a duty of care to the ultimate consumer"), extracted from a
	// "HELD:"/"HOLDING:" marker section.
	Holding string

	// RatioDecidendi is the reasoning behind the holding, extracted from a
	// "RATIO:"/"REASONING:" marker section when present, falling back to
	// text following the holding section up to the next recognized
	// marker (or end of text) otherwise.
	RatioDecidendi string
}

// holdingMarkerRe matches a line introducing the holding section, e.g.
// "HELD:", "HOLDING:", "HELD -", case-insensitive.
var holdingMarkerRe = regexp.MustCompile(`(?im)^\s*(?:HELD|HOLDING)\s*[:\-]\s*(.*)$`)

// ratioMarkerRe matches a line introducing the ratio decidendi section,
// e.g. "RATIO:", "RATIO DECIDENDI:", "REASONING:".
var ratioMarkerRe = regexp.MustCompile(`(?im)^\s*(?:RATIO(?:\s+DECIDENDI)?|REASONING)\s*[:\-]\s*(.*)$`)

// sectionStopMarkerRe matches any other recognized section marker that
// should terminate a holding/ratio capture (e.g. a subsequent "FACTS:" or
// "DISPOSITION:" heading), so extraction does not run past the intended
// section into unrelated judgment text.
var sectionStopMarkerRe = regexp.MustCompile(`(?im)^\s*(?:HELD|HOLDING|RATIO(?:\s+DECIDENDI)?|REASONING|FACTS|DISPOSITION|ORDER)\s*[:\-]`)

// ExtractHoldingAndRatio is a deterministic heuristic extractor that pulls
// the "holding" (the court's core determination) and "ratio decidendi"
// (the reasoning behind it) from fullText.
//
// The starting heuristic locates a "HELD:" or "HOLDING:" marker line and
// captures text up to the next recognized section marker (or end of
// input) as the Holding. A separate "RATIO:"/"RATIO DECIDENDI:"/
// "REASONING:" marker is used for RatioDecidendi when present; if no such
// marker exists, the text immediately following the Holding section (up
// to the next marker or end of input) is used as a fallback so
// RatioDecidendi is rarely left empty when a holding was found.
//
// This is explicitly a pluggable extension point: ExtractorFunc lets
// callers substitute a different heuristic (or a model-backed one) without
// changing the rest of the pipeline (see service.go's
// PrecedentIngestionService.HoldingExtractor).
//
// Returns ErrHoldingNotFound (with a zero-value HoldingExtractionResult)
// when no "HELD:"/"HOLDING:" marker can be located.
func ExtractHoldingAndRatio(fullText string) (HoldingExtractionResult, error) {
	if strings.TrimSpace(fullText) == "" {
		return HoldingExtractionResult{}, ErrHoldingNotFound
	}

	holdingLoc := holdingMarkerRe.FindStringSubmatchIndex(fullText)
	if holdingLoc == nil {
		return HoldingExtractionResult{}, ErrHoldingNotFound
	}

	holdingStart := holdingLoc[2] // start of captured group 1 (text after the marker)
	holdingSectionEnd := nextMarkerOffset(fullText, holdingLoc[1])

	ratioLoc := ratioMarkerRe.FindStringSubmatchIndex(fullText)
	var holding, ratio string
	if ratioLoc != nil {
		holding = strings.TrimSpace(fullText[holdingStart:holdingSectionEnd])
		ratioStart := ratioLoc[2]
		ratioSectionEnd := nextMarkerOffset(fullText, ratioLoc[1])
		ratio = strings.TrimSpace(fullText[ratioStart:ratioSectionEnd])
	} else {
		// No explicit RATIO/REASONING marker: split the holding section on
		// its first sentence boundary (the first ". " or end of the first
		// line, whichever comes first). The first sentence becomes the
		// Holding; any remaining text within the same section (up to the
		// next marker or end of input) becomes the RatioDecidendi
		// fallback, so RatioDecidendi is rarely left empty when the
		// judgment text continues past the court's core determination.
		section := fullText[holdingStart:holdingSectionEnd]
		splitAt := firstSentenceBoundary(section)
		if splitAt < 0 {
			holding = strings.TrimSpace(section)
			ratio = ""
		} else {
			holding = strings.TrimSpace(section[:splitAt])
			ratio = strings.TrimSpace(section[splitAt:])
		}
	}

	return HoldingExtractionResult{
		Holding:        collapseWhitespace(holding),
		RatioDecidendi: collapseWhitespace(ratio),
	}, nil
}

// ExtractorFunc is the pluggable extension point for holding/ratio
// extraction. ExtractHoldingAndRatio satisfies this signature; callers
// (e.g. PrecedentIngestionService.HoldingExtractor) may substitute an
// alternative implementation without changing the rest of the ingestion
// pipeline.
type ExtractorFunc func(fullText string) (HoldingExtractionResult, error)

// nextMarkerOffset returns the byte offset of the next recognized section
// marker in text at or after from, or len(text) if none is found.
func nextMarkerOffset(text string, from int) int {
	if from >= len(text) {
		return len(text)
	}
	rest := text[from:]
	loc := sectionStopMarkerRe.FindStringIndex(rest)
	if loc == nil {
		return len(text)
	}
	return from + loc[0]
}

// collapseWhitespace normalizes runs of whitespace (including newlines)
// into single spaces, trimming the result, so extracted holding/ratio text
// reads as a single flowing paragraph regardless of the source judgment's
// line wrapping.
func collapseWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// firstSentenceBoundary returns the byte offset immediately after the
// first sentence-ending punctuation (". ", "! ", "? ") in s, or the
// offset of the first blank-line paragraph break, whichever comes first.
// Returns -1 if s contains neither (i.e. it is a single sentence/
// paragraph in its entirety).
func firstSentenceBoundary(s string) int {
	best := -1
	for _, sep := range []string{". ", "! ", "? ", ".\n", "!\n", "?\n"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			end := idx + 1 // position right after the punctuation
			if best == -1 || end < best {
				best = end
			}
		}
	}
	if idx := strings.Index(s, "\n\n"); idx >= 0 {
		if best == -1 || idx < best {
			best = idx
		}
	}
	return best
}
