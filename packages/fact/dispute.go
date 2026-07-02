package fact

import "strings"

// DisputeStatus classifies whether a fact's assertion is contested
// between the case's parties.
type DisputeStatus string

const (
	// Undisputed means no contradictory assertion from another party was
	// found for this fact.
	Undisputed DisputeStatus = "undisputed"

	// Disputed means both parties assert contradictory versions of the
	// same underlying claim.
	Disputed DisputeStatus = "disputed"

	// Unknown means dispute status could not be determined (e.g. no
	// party attribution, or nothing to compare the fact against).
	Unknown DisputeStatus = "unknown"
)

// contradictionPairs lists lexical keyword pairs treated as contradictory
// when they each appear in one of two facts attributed to different
// parties. This re-implements, locally and independently, the
// contradiction-heuristic idea from packages/timeline/conflict.go's
// contradictionPairs — deliberately without a hard dependency on that
// file's internals (DetectConflicts, PartyFact, etc.), since this
// package's dispute flagging operates over irac.FactNode text rather than
// timeline.PartyFact. The heuristic is intentionally simple, rule-based,
// and deterministic (no ML), mirroring packages/evidence and
// packages/timeline's shared "no ML models" design principle.
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
	{"false", "true"},
	{"disputes", "confirms"},
}

// hasKeyword reports whether text contains keyword as a case-insensitive
// substring.
func hasKeyword(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

// keywordWithoutNegation reports whether text contains pos in a way that
// is not solely explained by text also containing the longer negated
// phrase neg (e.g. pos="did", neg="did not": a text containing only "did
// not" should not count as asserting the bare positive "did").
func keywordWithoutNegation(text, pos, neg string) bool {
	if !hasKeyword(text, pos) {
		return false
	}
	if pos == neg {
		return false
	}
	if strings.Contains(neg, pos) && hasKeyword(text, neg) {
		return false
	}
	return true
}

// contradicts reports whether textA and textB contain an opposing
// keyword pair from contradictionPairs, in either assignment, along with
// a human-readable reason when true.
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

// FactWithParty is the minimal shape DetermineDisputeStatus needs to
// compare a candidate fact's text and party attribution against its
// peers: an ID, the fact's Text, and the PartyID it was attributed to
// (e.g. via AttributeParty). Kept minimal and local, rather than
// requiring a full irac.FactNode plus PartyAttribution pair, so this
// package's own tests and callers can construct fixtures cheaply.
type FactWithParty struct {
	// ID uniquely identifies the fact.
	ID string

	// Text is the fact's assertion text.
	Text string

	// PartyID is the party this fact is attributed to. Facts with an
	// empty PartyID are never compared against one another (there is
	// nothing to determine "different parties" from).
	PartyID string
}

// DetermineDisputeStatus compares candidate against every entry in peers
// and returns Disputed (with the ID of the first contradicting peer) as
// soon as a peer attributed to a different, non-empty PartyID contains a
// contradictory keyword pair (see contradicts). Returns Unknown if
// candidate.PartyID is empty (no basis for a same-subject/different-party
// comparison) or peers is empty, and Undisputed otherwise.
func DetermineDisputeStatus(candidate FactWithParty, peers []FactWithParty) (DisputeStatus, string) {
	if strings.TrimSpace(candidate.PartyID) == "" || len(peers) == 0 {
		return Unknown, ""
	}

	for _, peer := range peers {
		if peer.ID == candidate.ID {
			continue
		}
		if peer.PartyID == "" || peer.PartyID == candidate.PartyID {
			continue
		}
		if _, ok := contradicts(candidate.Text, peer.Text); ok {
			return Disputed, peer.ID
		}
	}
	return Undisputed, ""
}
