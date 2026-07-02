package evidence

import (
	"regexp"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// documentReferencePattern matches common document/exhibit reference
// phrases: "Exhibit A", "Ex. 3", "Annexure B", "Schedule 1" (mirroring
// packages/segmentation's exhibitPattern), plus generic document-noun
// references such as "the contract", "the agreement", "the letter dated",
// "the report", "the invoice", "the email from".
var documentReferencePattern = regexp.MustCompile(
	`(?i)\b(exhibit|ex\.?|annexure|schedule)\s+([A-Z]|\d+[A-Z]?)\b|` +
		`\b(the\s+)?(contract|agreement|invoice|receipt|report|letter|memorandum|memo|email|deed|certificate|ledger|statement\s+of\s+account)\b.*\b(dated|attached|annexed|marked)\b|` +
		`\b(contract|agreement|invoice|receipt|deed|certificate)\s+(dated|no\.?|number)\b`,
)

// physicalExhibitPattern matches references to tangible, non-documentary
// physical evidence: weapons, clothing, samples, and similar items
// typically introduced as "Exhibit" markers but describing a physical
// object rather than paper.
var physicalExhibitPattern = regexp.MustCompile(
	`(?i)\b(the\s+)?(weapon|knife|firearm|gun|bullet|casing|garment|clothing|blood\s+sample|dna\s+sample|fingerprint|footprint|tissue\s+sample|physical\s+evidence)\b`,
)

// IsDocumentaryEvidence reports whether seg represents documentary
// evidence, and a confidence score for that determination.
//
// A SegmentExhibit segment (already boundary-tagged by
// packages/segmentation's exhibit.go) is treated as documentary evidence
// at high confidence unless it more specifically matches the physical
// exhibit pattern (see documentary.go's physicalExhibitPattern and
// party.go's IsPhysicalExhibit). Any segment whose text matches a
// document-reference phrase is also recognized, at a confidence reflecting
// that it was not already structurally boundary-tagged.
func IsDocumentaryEvidence(seg segmentation.Segment) (bool, float64) {
	if seg.Type == segmentation.SegmentExhibit {
		if physicalExhibitPattern.MatchString(seg.Text) {
			return false, 0 // let IsPhysicalExhibit claim it instead
		}
		return true, 0.9
	}

	if documentReferencePattern.MatchString(seg.Text) {
		return true, 0.65
	}

	return false, 0
}

// IsPhysicalExhibit reports whether seg references tangible,
// non-documentary physical evidence, and a confidence score for that
// determination.
func IsPhysicalExhibit(seg segmentation.Segment) (bool, float64) {
	if !physicalExhibitPattern.MatchString(seg.Text) {
		return false, 0
	}
	if seg.Type == segmentation.SegmentExhibit {
		return true, 0.85
	}
	return true, 0.6
}
