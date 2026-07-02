package evidence

import (
	"regexp"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// statuteCitationPattern matches common statute/case-law citation shapes,
// mirroring packages/segmentation's citationPattern:
//   - "12 U.S.C. § 1983", "42 USC 1983"
//   - "Section 302 IPC", "S. 420 IPC", "Article 21"
//   - "(2020) 3 SCC 45", "AIR 1978 SC 597"
//   - "Smith v. Jones", "State v. Doe"
var statuteCitationPattern = regexp.MustCompile(
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

// IsStatutoryCitation reports whether seg represents a statute or case-law
// citation, and a confidence score for that determination.
//
// A SegmentCitation segment (already boundary-tagged by
// packages/segmentation's exhibit.go) is treated as a statutory citation
// at high confidence. Any other segment whose text matches
// statuteCitationPattern is also recognized, at slightly reduced
// confidence since it was not already structurally boundary-tagged.
func IsStatutoryCitation(seg segmentation.Segment) (bool, float64) {
	if seg.Type == segmentation.SegmentCitation {
		return true, 0.95
	}
	if statuteCitationPattern.MatchString(seg.Text) {
		return true, 0.8
	}
	return false, 0
}
