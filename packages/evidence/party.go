package evidence

import (
	"regexp"
	"strings"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// PartyRole identifies which side of a case a Classification's segment is
// attributed to.
type PartyRole string

const (
	// PartyFirst is the first-named/moving party: plaintiff, prosecution,
	// petitioner, or appellant, depending on proceeding type.
	PartyFirst PartyRole = "first_party"

	// PartySecond is the second-named/responding party: defendant,
	// respondent, or appellee, depending on proceeding type.
	PartySecond PartyRole = "second_party"

	// PartyUnattributed means no party attribution heuristic matched; the
	// segment's speaker or side could not be determined.
	PartyUnattributed PartyRole = "unattributed"
)

// firstPartyMarkers are speaker labels or explicit textual markers
// indicating the first-named/moving party (plaintiff, prosecution,
// petitioner, appellant).
var firstPartyMarkers = []string{
	"plaintiff", "prosecution", "petitioner", "appellant", "complainant", "claimant",
}

// secondPartyMarkers are speaker labels or explicit textual markers
// indicating the second-named/responding party (defendant, respondent,
// appellee).
var secondPartyMarkers = []string{
	"defendant", "respondent", "appellee", "accused",
}

// explicitMarkerPattern matches an explicit "on behalf of the plaintiff",
// "for the defendant", "counsel for the respondent" style attribution
// phrase, capturing the party-role word.
var explicitMarkerPattern = regexp.MustCompile(
	`(?i)\b(?:on\s+behalf\s+of|for|counsel\s+for)\s+the\s+(plaintiff|prosecution|petitioner|appellant|complainant|claimant|defendant|respondent|appellee|accused)\b`,
)

// AttributeParty determines the PartyRole attributed to seg, based first on
// its SpeakerLabel (when set, per packages/segmentation's speaker
// attribution), then on an explicit textual marker within its Text.
// Returns PartyUnattributed when neither signal identifies a side.
func AttributeParty(seg segmentation.Segment) PartyRole {
	if role, ok := partyFromLabel(string(seg.SpeakerLabel)); ok {
		return role
	}
	if role, ok := partyFromExplicitMarker(seg.Text); ok {
		return role
	}
	if role, ok := partyFromLabel(seg.Text); ok {
		return role
	}
	return PartyUnattributed
}

// partyFromLabel checks label (a SpeakerLabel or free text) for a
// first-party or second-party marker word, case-insensitively.
func partyFromLabel(label string) (PartyRole, bool) {
	lower := strings.ToLower(label)
	if lower == "" {
		return "", false
	}
	for _, m := range firstPartyMarkers {
		if strings.Contains(lower, m) {
			return PartyFirst, true
		}
	}
	for _, m := range secondPartyMarkers {
		if strings.Contains(lower, m) {
			return PartySecond, true
		}
	}
	return "", false
}

// partyFromExplicitMarker checks text for an explicit "on behalf of the
// X"/"counsel for the X" attribution phrase.
func partyFromExplicitMarker(text string) (PartyRole, bool) {
	m := explicitMarkerPattern.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return partyFromLabel(m[1])
}
