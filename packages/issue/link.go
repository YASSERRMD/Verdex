package issue

import (
	"sort"
	"strings"

	"github.com/YASSERRMD/verdex/packages/timeline"
)

// minLinkOverlap is the minimum token-overlap ratio (see keywordOverlap)
// required for a CandidateIssue to be linked to a timeline.Party or a
// related fact/segment.
const minLinkOverlap = 0.15

// IssueLink associates a CandidateIssue with the timeline.Party IDs and
// any related irac.FactNode/segment IDs it concerns, so downstream
// reasoning can answer "which parties and facts does this issue touch"
// without re-deriving the relationship from raw text each time.
type IssueLink struct {
	// IssueIndex is the index into the []CandidateIssue slice passed to
	// LinkIssues that this link describes.
	IssueIndex int

	// PartyIDs lists the timeline.Party.ID values this issue concerns.
	PartyIDs []string

	// FactIDs lists the related irac.FactNode/segment IDs this issue
	// concerns (e.g. segmentation.Segment.ID values, or persisted
	// irac.FactNode.ID values once facts have been extracted).
	FactIDs []string
}

// LinkIssues associates each CandidateIssue in issues with the
// timeline.Party IDs and fact/segment IDs it concerns.
//
// Party linkage uses a lightweight name-mention heuristic: a party is
// linked to an issue if the issue's Text mentions the party's Name
// (case-insensitively) or contains a token overlapping the party's Name
// tokens above minLinkOverlap. This mirrors packages/evidence's
// AttributeParty text-marker convention, adapted to per-issue rather than
// per-segment attribution.
//
// Fact linkage uses facts, a map from fact/segment ID to its text
// (typically sourced from the same segmentation.Segment batch used
// upstream), matched against each issue's Text via the same keywordOverlap
// heuristic used by claim mapping (see claim_map.go).
func LinkIssues(issues []CandidateIssue, parties []timeline.Party, facts map[string]string) []IssueLink {
	links := make([]IssueLink, len(issues))
	for i, iss := range issues {
		links[i] = IssueLink{IssueIndex: i}
		links[i].PartyIDs = linkedPartyIDs(iss, parties)
		links[i].FactIDs = linkedFactIDs(iss, facts)
	}
	return links
}

// linkedPartyIDs returns the IDs of every party in parties that iss's Text
// mentions by name or overlaps with above minLinkOverlap.
func linkedPartyIDs(iss CandidateIssue, parties []timeline.Party) []string {
	var out []string
	lowerText := strings.ToLower(iss.Text)
	for _, p := range parties {
		if p.Name == "" {
			continue
		}
		if strings.Contains(lowerText, strings.ToLower(p.Name)) {
			out = append(out, p.ID)
			continue
		}
		if keywordOverlap(iss.Text, p.Name) >= minLinkOverlap {
			out = append(out, p.ID)
		}
	}
	return out
}

// linkedFactIDs returns the IDs (map keys) of every fact/segment in facts
// whose text overlaps iss's Text above minLinkOverlap, in a stable order
// determined by iterating a sorted copy of the map's keys.
func linkedFactIDs(iss CandidateIssue, facts map[string]string) []string {
	if len(facts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(facts))
	for id := range facts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var out []string
	for _, id := range ids {
		text := facts[id]
		if keywordOverlap(text, iss.Text) >= minLinkOverlap {
			out = append(out, id)
		}
	}
	return out
}
