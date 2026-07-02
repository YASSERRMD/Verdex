package timeline

import "strings"

// PartyRole identifies which side of a case a Party represents.
//
// This is a local equivalent of packages/evidence.PartyRole rather than a
// hard cross-module dependency: packages/timeline models the two (or,
// occasionally, more) parties and their case-facing relationships, which is
// a distinct concern from packages/evidence's per-segment attribution
// heuristic, even though the concept and naming intentionally mirror it.
type PartyRole string

const (
	// PartyFirst is the first-named/moving party: plaintiff, prosecution,
	// petitioner, or appellant, depending on proceeding type.
	PartyFirst PartyRole = "first_party"

	// PartySecond is the second-named/responding party: defendant,
	// respondent, or appellee, depending on proceeding type.
	PartySecond PartyRole = "second_party"

	// PartyThird is a third party joined to the case (an intervenor,
	// co-defendant, or other non-primary participant) who is neither the
	// first- nor second-named party.
	PartyThird PartyRole = "third_party"
)

// Valid reports whether r is one of the known PartyRole constants.
func (r PartyRole) Valid() bool {
	switch r {
	case PartyFirst, PartySecond, PartyThird:
		return true
	default:
		return false
	}
}

// Party is one participant in a case: their case role (first/second/third
// party), display name, and optional counsel of record.
type Party struct {
	// ID uniquely identifies this party within its case.
	ID string

	// Role classifies this party's side of the case (see PartyRole).
	Role PartyRole

	// Name is the party's display name (e.g. "Jane Doe" or "Acme Corp").
	Name string

	// Counsel is the name of the party's counsel of record, when known.
	// Nil means no counsel has been recorded for this party.
	Counsel *string
}

// Validate checks that p has a non-empty ID, non-empty Name, and a
// recognized Role. Returns ErrInvalidParty if any check fails.
func (p Party) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return ErrInvalidParty
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidParty
	}
	if !p.Role.Valid() {
		return ErrInvalidParty
	}
	return nil
}

// HasCounsel reports whether p has a non-empty Counsel recorded.
func (p Party) HasCounsel() bool {
	return p.Counsel != nil && strings.TrimSpace(*p.Counsel) != ""
}
