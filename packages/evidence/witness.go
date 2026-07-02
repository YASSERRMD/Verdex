package evidence

import (
	"regexp"
	"strings"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// firstPersonTestimonyPattern matches common first-person testimonial verbs
// and phrases typical of sworn statements and depositions: "I saw", "I
// witnessed", "I heard", "I observed", "I testify", "to the best of my
// knowledge", "I recall", "I was present", etc.
var firstPersonTestimonyPattern = regexp.MustCompile(
	`(?i)\bI\s+(saw|witnessed|heard|observed|testify|testified|recall|remember|was\s+present|stated|state|confirm|can\s+confirm)\b|` +
		`to\s+the\s+best\s+of\s+my\s+knowledge|` +
		`\bI\s+swear\b|\bunder\s+oath\b`,
)

// depositionMarkerPattern matches document-header phrases that introduce a
// deposition or sworn statement (e.g. "Deposition of John Doe", "Witness
// Statement", "Affidavit of Jane Roe").
var depositionMarkerPattern = regexp.MustCompile(
	`(?i)\b(deposition|witness\s+statement|affidavit|sworn\s+statement)\s+of\b|` +
		`(?i)\b(deposition|witness\s+statement|affidavit)\b`,
)

// IsWitnessStatement reports whether seg represents witness testimony, and
// a confidence score for that determination.
//
// A SegmentStatement segment (speaker-attributed, per
// packages/segmentation's speaker.go) carrying first-person testimonial
// language is treated as high-confidence witness testimony. A
// SegmentStatement with a non-empty SpeakerLabel but no explicit
// first-person marker is still treated as testimony, at lower confidence,
// since speaker attribution alone is a meaningful signal. A
// SegmentParagraph containing an explicit deposition/affidavit marker or
// first-person testimonial language is also recognized, again at reduced
// confidence relative to a directly speaker-attributed statement.
func IsWitnessStatement(seg segmentation.Segment) (bool, float64) {
	text := seg.Text
	hasFirstPerson := firstPersonTestimonyPattern.MatchString(text)
	hasDepositionMarker := depositionMarkerPattern.MatchString(text)

	switch seg.Type {
	case segmentation.SegmentStatement:
		if hasFirstPerson {
			return true, 0.95
		}
		if strings.TrimSpace(string(seg.SpeakerLabel)) != "" {
			return true, 0.75
		}
		if hasDepositionMarker {
			return true, 0.8
		}
		return true, 0.6 // any speaker-attributed statement is presumptively testimony
	default:
		if hasDepositionMarker {
			return true, 0.7
		}
		if hasFirstPerson {
			return true, 0.6
		}
		return false, 0
	}
}
