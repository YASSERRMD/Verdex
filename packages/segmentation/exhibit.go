package segmentation

import (
	"regexp"
	"strings"
)

// exhibitPattern matches common exhibit markers: "Exhibit A", "Exhibit 12",
// "Ex. 3", "Ex 3", "Annexure B", "Schedule 1" — case-insensitive, matched
// anywhere a new exhibit reference begins a line or clause.
var exhibitPattern = regexp.MustCompile(
	`(?i)\b(exhibit|ex\.?|annexure|schedule)\s+([A-Z]|\d+[A-Z]?)\b`,
)

// citationPattern matches common statute/case-law citation shapes:
//   - "12 U.S.C. § 1983", "42 USC 1983"
//   - "Section 302 IPC", "S. 420 IPC", "Article 21"
//   - "(2020) 3 SCC 45", "AIR 1978 SC 597"
//   - "Smith v. Jones", "State v. Doe"
var citationPattern = regexp.MustCompile(
	`(?i)(` +
		`\d+\s+U\.?S\.?C\.?\s*§?\s*\d+` + // 12 U.S.C. § 1983
		`|§\s*\d+` + // § 1983
		`|\bsection\s+\d+[A-Za-z]*(\s+\S+)?` + // Section 302 IPC
		`|\bs\.\s*\d+[A-Za-z]*` + // S. 420
		`|\barticle\s+\d+[A-Za-z]*` + // Article 21
		`|\(\d{4}\)\s*\d+\s+[A-Z]{2,}\s+\d+` + // (2020) 3 SCC 45
		`|\bAIR\s+\d{4}\s+[A-Z]{2,}\s+\d+` + // AIR 1978 SC 597
		`|\b[A-Z][a-zA-Z.]*\s+v\.?\s+[A-Z][a-zA-Z.]*` + // Smith v. Jones
		`)`,
)

// IsExhibitMarker reports whether text contains an exhibit/annexure/schedule
// marker per exhibitPattern.
func IsExhibitMarker(text string) bool {
	return exhibitPattern.MatchString(text)
}

// IsCitationMarker reports whether text contains a statute or case-law
// citation shape per citationPattern.
func IsCitationMarker(text string) bool {
	return citationPattern.MatchString(text)
}

// TagExhibitsAndCitations scans segs and reclassifies segments whose Text
// contains an exhibit marker as SegmentExhibit, or a citation shape as
// SegmentCitation. Exhibit markers take precedence over citation shapes
// when a segment matches both (an exhibit line introducing a citation is
// still primarily an exhibit boundary). Segments that already carry a
// SegmentHeading type are left unchanged, since heading detection is more
// structurally specific than a lexical marker match.
//
// Only segments with Type == SegmentParagraph or SegmentStatement are
// eligible for reclassification, preserving any earlier, more specific
// tagging.
func TagExhibitsAndCitations(segs []Segment) []Segment {
	out := make([]Segment, len(segs))
	for i, s := range segs {
		switch s.Type {
		case SegmentParagraph, SegmentStatement:
			switch {
			case IsExhibitMarker(s.Text):
				s.Type = SegmentExhibit
			case IsCitationMarker(s.Text):
				s.Type = SegmentCitation
			}
		}
		out[i] = s
	}
	return out
}

// SplitOnExhibitBoundaries splits text into spans such that every exhibit
// marker (per exhibitPattern) begins a new span, ensuring exhibit
// references are not buried mid-paragraph but become their own segment
// boundary. Spans cover the full rune range of text with no gaps or
// overlaps, matching the invariant SplitSentences/SplitClauses provide.
//
// Returns a single span covering all of text if no exhibit marker is found.
func SplitOnExhibitBoundaries(text string) []Span {
	runes := []rune(text)
	if strings.TrimSpace(text) == "" {
		return []Span{}
	}

	locs := exhibitPattern.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return []Span{{Start: 0, End: len(runes), Text: strings.TrimSpace(text)}}
	}

	// Convert byte offsets (from FindAllStringIndex) to rune offsets.
	byteToRune := make(map[int]int, len(runes)+1)
	ri := 0
	bi := 0
	for _, r := range text {
		byteToRune[bi] = ri
		bi += len(string(r))
		ri++
	}
	byteToRune[bi] = ri

	boundaries := []int{0}
	for _, loc := range locs {
		startRune := byteToRune[loc[0]]
		if startRune != 0 && startRune != boundaries[len(boundaries)-1] {
			boundaries = append(boundaries, startRune)
		}
	}
	if boundaries[len(boundaries)-1] != len(runes) {
		boundaries = append(boundaries, len(runes))
	}

	spans := make([]Span, 0, len(boundaries)-1)
	for i := 0; i < len(boundaries)-1; i++ {
		start, end := boundaries[i], boundaries[i+1]
		spans = append(spans, Span{
			Start: start,
			End:   end,
			Text:  strings.TrimSpace(string(runes[start:end])),
		})
	}
	return spans
}
