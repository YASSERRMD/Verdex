package issue

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/evidence"
)

// claimRelevantTypes is the set of evidence.EvidenceType values considered
// "claims" for the purposes of MapClaimsToIssues: argument (advocacy or
// contention text) and witness statements (first-person testimonial
// language), the two evidentiary roles most likely to state or bear on a
// disputed issue.
var claimRelevantTypes = map[evidence.EvidenceType]struct{}{
	evidence.TypeArgument:         {},
	evidence.TypeWitnessStatement: {},
}

// minClaimOverlap is the minimum token-overlap ratio (see keywordOverlap)
// required for a Classification to be considered related to a
// CandidateIssue.
const minClaimOverlap = 0.2

// ClaimLink associates one evidence.Classification with the CandidateIssue
// it relates to, together with the overlap score that produced the match.
type ClaimLink struct {
	// IssueIndex is the index into the []CandidateIssue slice passed to
	// MapClaimsToIssues that this classification relates to.
	IssueIndex int

	// SegmentID identifies the segmentation.Segment the classification
	// was derived from (see evidence.Classification.SegmentID).
	SegmentID string

	// Overlap is the keyword-overlap score (see keywordOverlap) that
	// produced this match, in the closed interval [0, 1].
	Overlap float64
}

// MapClaimsToIssues maps evidence.Classification entries of type
// TypeArgument or TypeWitnessStatement to the CandidateIssues they relate
// to via keyword/text overlap between the classification's segment text
// and each candidate issue's Text.
//
// segmentText supplies the normalized text for each Classification's
// SegmentID (typically sourced from the same segmentation.Segment batch
// passed to IssueIdentifier.Identify), since evidence.Classification
// itself carries only a SegmentID, not the segment text.
//
// A Classification may match zero, one, or multiple issues; every match
// scoring at or above minClaimOverlap is returned. Classifications whose
// Type is not in claimRelevantTypes are skipped.
func MapClaimsToIssues(classifications []evidence.Classification, issues []CandidateIssue, segmentText map[string]string) []ClaimLink {
	var links []ClaimLink

	for _, c := range classifications {
		if _, ok := claimRelevantTypes[c.Type]; !ok {
			continue
		}
		text := segmentText[c.SegmentID]
		if strings.TrimSpace(text) == "" {
			continue
		}

		for idx, iss := range issues {
			overlap := keywordOverlap(text, iss.Text)
			if overlap >= minClaimOverlap {
				links = append(links, ClaimLink{
					IssueIndex: idx,
					SegmentID:  c.SegmentID,
					Overlap:    overlap,
				})
			}
		}
	}

	return links
}

// keywordOverlap returns the fraction of normalized tokens in b that also
// appear in a's token set (a Jaccard-adjacent "coverage" ratio: |A ∩ B| /
// |B|), used as a lightweight text-relatedness signal between a
// classification's segment text and a candidate issue's text.
func keywordOverlap(a, b string) float64 {
	tokensA := tokenSet(a)
	tokensB := tokenSet(b)
	if len(tokensB) == 0 {
		return 0
	}

	intersection := 0
	for t := range tokensB {
		if _, ok := tokensA[t]; ok {
			intersection++
		}
	}
	return float64(intersection) / float64(len(tokensB))
}

// stopWords are common function words excluded from tokenSet so overlap
// scoring reflects meaningful content words rather than incidental
// grammatical overlap.
var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "of": {}, "to": {}, "and": {}, "or": {},
	"is": {}, "was": {}, "were": {}, "be": {}, "been": {}, "that": {},
	"this": {}, "it": {}, "in": {}, "on": {}, "for": {}, "with": {}, "as": {},
	"by": {}, "at": {}, "from": {}, "not": {},
}

// tokenSet normalizes text to a lowercase, punctuation-stripped set of
// non-stopword tokens.
func tokenSet(text string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	})
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		if _, stop := stopWords[f]; stop {
			continue
		}
		set[f] = struct{}{}
	}
	return set
}
