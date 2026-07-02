package timeline

import "strings"

// Well-known Relationship.Kind values. This is not an exhaustive enum —
// Kind is deliberately a free-form string so a case's relationship can be
// described even when it doesn't fit a predefined category — but these
// constants cover the common cases and give callers a canonical spelling
// to prefer when one applies.
const (
	KindLandlordTenant   = "landlord-tenant"
	KindEmployerEmployee = "employer-employee"
	KindContractual      = "contractual"
	KindFamilial         = "familial"
	KindNeighbor         = "neighbor"
	KindOther            = "other"
)

// Relationship describes how two parties in a case relate to one another
// (e.g. landlord-tenant, employer-employee), independent of the case's
// legal category — see packages/category for the case-level
// classification this complements.
type Relationship struct {
	// ID uniquely identifies this relationship within its case.
	ID string

	// PartyAID and PartyBID are the Party.ID values this relationship
	// connects. Order is not significant for symmetric relationship kinds
	// (e.g. "neighbor"), but is meaningful for directional ones (e.g.
	// PartyAID is the landlord and PartyBID is the tenant in a
	// "landlord-tenant" relationship, per Description).
	PartyAID string
	PartyBID string

	// Kind is a short, free-form label for the relationship type (e.g.
	// KindLandlordTenant, KindEmployerEmployee, or any custom string a
	// jurisdiction-specific case calls for).
	Kind string

	// Description is a human-readable elaboration of the relationship
	// (e.g. "PartyA has leased unit 4B to PartyB since 2021").
	Description string
}

// Validate checks that r has a non-empty ID, two distinct non-empty party
// IDs, and a non-empty Kind. Returns ErrInvalidParty if any check fails.
func (r Relationship) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrInvalidParty
	}
	if strings.TrimSpace(r.PartyAID) == "" || strings.TrimSpace(r.PartyBID) == "" {
		return ErrInvalidParty
	}
	if r.PartyAID == r.PartyBID {
		return ErrInvalidParty
	}
	if strings.TrimSpace(r.Kind) == "" {
		return ErrInvalidParty
	}
	return nil
}

// Involves reports whether partyID is either PartyAID or PartyBID on r.
func (r Relationship) Involves(partyID string) bool {
	return r.PartyAID == partyID || r.PartyBID == partyID
}
