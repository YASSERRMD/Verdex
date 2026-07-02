package timeline

import "strings"

// Claim links a Party to the Event and/or PartyFact entries it relies on
// to support a single assertion (e.g. "the tenant breached the lease"),
// so downstream IRAC reasoning can trace a claim back to its supporting
// timeline evidence.
type Claim struct {
	// ID uniquely identifies this claim within its case.
	ID string

	// PartyID is the Party.ID making this claim.
	PartyID string

	// Description is a short human-readable statement of the claim.
	Description string

	// EventIDs lists the Event.ID values this claim relies on. May be
	// empty if the claim is supported purely by PartyFact entries.
	EventIDs []string

	// FactIDs lists the PartyFact.ID values this claim relies on. May be
	// empty if the claim is supported purely by Event entries.
	FactIDs []string
}

// Validate checks that c has a non-empty ID, PartyID, Description, and at
// least one supporting Event or PartyFact reference. Returns
// ErrInvalidClaim if any check fails.
func (c Claim) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return ErrInvalidClaim
	}
	if strings.TrimSpace(c.PartyID) == "" {
		return ErrInvalidClaim
	}
	if strings.TrimSpace(c.Description) == "" {
		return ErrInvalidClaim
	}
	if len(c.EventIDs) == 0 && len(c.FactIDs) == 0 {
		return ErrInvalidClaim
	}
	return nil
}

// SupportCount returns the total number of supporting references (Event
// plus PartyFact) this claim relies on.
func (c Claim) SupportCount() int {
	return len(c.EventIDs) + len(c.FactIDs)
}

// ValidateClaimLinkage checks that every EventIDs/FactIDs reference on
// claim resolves within the given known ID sets, returning
// ErrEventNotFound or ErrPartyNotFound-adjacent ErrEmptyInput style errors
// as appropriate. This is a pure integrity check separate from
// Claim.Validate's shape check, so a store or service can validate
// linkage against its actual persisted Event/PartyFact records before
// accepting a Claim.
func ValidateClaimLinkage(claim Claim, knownEventIDs, knownFactIDs map[string]bool) error {
	for _, id := range claim.EventIDs {
		if !knownEventIDs[id] {
			return ErrEventNotFound
		}
	}
	for _, id := range claim.FactIDs {
		if !knownFactIDs[id] {
			return ErrEmptyInput
		}
	}
	return nil
}
