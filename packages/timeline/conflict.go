package timeline

import (
	"strings"
	"time"
)

// Conflict flags that two PartyFact entries from different parties assert
// incompatible claims about the same subject.
type Conflict struct {
	// ID uniquely identifies this conflict within its case.
	ID string

	// FactAID and FactBID are the PartyFact.ID values in conflict. FactAID
	// is always the fact that appeared first in the input slice passed to
	// DetectConflicts, for deterministic, reproducible output.
	FactAID string
	FactBID string

	// Subject is the shared PartyFact.Subject the two facts both address.
	Subject string

	// Reason is a short human-readable explanation of why these two facts
	// were flagged (e.g. which contradictory keyword pair matched).
	Reason string
}

// contradictionPairs lists lexical keyword pairs treated as contradictory
// when they each appear in one of two same-subject facts from different
// parties. This is a starting heuristic, not an exhaustive or
// semantically-grounded contradiction detector: it is deliberately simple,
// deterministic, and rule-based (no ML), mirroring
// packages/evidence/party.go's lexical-marker approach. Real natural
// language contradiction detection is a substantially harder problem that
// a future phase may address with a pluggable Detector interface, the same
// way packages/evidence.Classifier is pluggable.
var contradictionPairs = [][2]string{
	{"did not", "did"},
	{"never", "always"},
	{"denied", "admitted"},
	{"refused", "agreed"},
	{"failed to", "successfully"},
	{"breached", "complied"},
	{"absent", "present"},
	{"late", "on time"},
	{"did not pay", "paid"},
	{"no notice", "gave notice"},
}

// hasKeyword reports whether text contains keyword as a case-insensitive
// substring.
func hasKeyword(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

// contradicts reports whether textA and textB contain an opposing pair
// from contradictionPairs, in either assignment (A has side 0 and B has
// side 1, or vice versa), along with a human-readable reason when true.
//
// "did not"/"did" is checked with care: because "did not" contains "did"
// as a substring, a fact containing "did not X" would otherwise trivially
// self-match against a positive "did" fact that is really just describing
// the same "did not" sentence reflected back. Checking the negative-form
// keyword ("did not") before the bare positive keyword, and requiring the
// positive side to NOT also contain the negative phrase, avoids that false
// positive.
func contradicts(textA, textB string) (string, bool) {
	for _, pair := range contradictionPairs {
		neg, pos := pair[0], pair[1]
		aNeg, aPos := hasKeyword(textA, neg), keywordWithoutNegation(textA, pos, neg)
		bNeg, bPos := hasKeyword(textB, neg), keywordWithoutNegation(textB, pos, neg)
		if aNeg && bPos {
			return "contradictory keywords: \"" + neg + "\" vs \"" + pos + "\"", true
		}
		if aPos && bNeg {
			return "contradictory keywords: \"" + pos + "\" vs \"" + neg + "\"", true
		}
	}
	return "", false
}

// keywordWithoutNegation reports whether text contains pos but that match
// is not solely an artifact of text also containing neg (e.g. pos="did",
// neg="did not" — a text containing only "did not" should not count as
// containing the positive "did" keyword for contradiction purposes).
func keywordWithoutNegation(text, pos, neg string) bool {
	if !hasKeyword(text, pos) {
		return false
	}
	if pos == neg {
		return false
	}
	// If every occurrence-worthy signal for pos is explained by neg being
	// a superstring containing pos (e.g. neg="did not" contains pos="did"),
	// and text's only match of pos is via neg, treat it as not a genuine
	// standalone positive assertion.
	if strings.Contains(neg, pos) && hasKeyword(text, neg) {
		return false
	}
	return true
}

// DetectConflicts scans facts pairwise and flags a Conflict for every pair
// from different parties that share a non-empty Subject and whose Text
// contains a contradictory keyword pair (see contradictionPairs).
//
// This heuristic does not consider event dates directly on PartyFact
// (PartyFact has no OccurredAt of its own); when the caller has separately
// established that two facts' underlying events fall on the same or
// overlapping date, pass that context via sameOrOverlappingDate so it can
// gate the check — facts about the same subject on unrelated dates are not
// flagged. Pass a nil sameOrOverlappingDate to skip the date gate
// entirely (treat every same-subject pair as eligible).
func DetectConflicts(facts []PartyFact, sameOrOverlappingDate func(a, b PartyFact) bool) []Conflict {
	var conflicts []Conflict
	for i := 0; i < len(facts); i++ {
		for j := i + 1; j < len(facts); j++ {
			a, b := facts[i], facts[j]
			if a.PartyID == b.PartyID {
				continue
			}
			if a.Subject == "" || b.Subject == "" || a.Subject != b.Subject {
				continue
			}
			if sameOrOverlappingDate != nil && !sameOrOverlappingDate(a, b) {
				continue
			}
			if reason, ok := contradicts(a.Text, b.Text); ok {
				conflicts = append(conflicts, Conflict{
					ID:      a.ID + "|" + b.ID,
					FactAID: a.ID,
					FactBID: b.ID,
					Subject: a.Subject,
					Reason:  reason,
				})
			}
		}
	}
	return conflicts
}

// EventsSameOrOverlappingDate returns a sameOrOverlappingDate predicate
// (for use with DetectConflicts) backed by a segmentID -> *time.Time
// lookup, treating two facts as date-eligible when both segments resolve
// to a known date and those dates are equal.
func EventsSameOrOverlappingDate(dateForSegment map[string]*time.Time) func(a, b PartyFact) bool {
	return func(a, b PartyFact) bool {
		da, okA := dateForSegment[a.SegmentID]
		db, okB := dateForSegment[b.SegmentID]
		if !okA || !okB || da == nil || db == nil {
			return false
		}
		return da.Equal(*db)
	}
}
