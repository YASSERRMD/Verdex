package irac

// This file documents and validates the jurisdiction tagging carried on
// RuleNode (JurisdictionCode, LegalFamily — see node.go). Rule nodes are
// jurisdiction-aware because a legal rule's authority is inherently scoped
// to the jurisdiction (and legal tradition) it derives from: the same
// Issue litigated in two jurisdictions may be governed by different Rule
// nodes.
//
// JurisdictionCode and LegalFamily are deliberately opaque strings here
// rather than packages/jurisdiction.CountryCode / packages/jurisdiction.
// LegalFamily types: this phase is a pure schema/domain-model phase and
// must not take a hard module dependency on packages/jurisdiction. A
// later integration phase can validate a RuleNode's JurisdictionCode
// against packages/jurisdiction's seeded jurisdiction records without any
// change to this package's exported shape.

// HasJurisdictionTag reports whether r has been tagged with a non-empty
// JurisdictionCode. RuleNode.JurisdictionCode is not enforced as
// mandatory at construction time (a rule sourced from a not-yet-resolved
// jurisdiction is still a valid intermediate state during extraction),
// but downstream jurisdiction-aware reasoning phases should check this
// before relying on the tag.
func (r RuleNode) HasJurisdictionTag() bool {
	return r.JurisdictionCode != ""
}

// HasLegalFamilyTag reports whether r has been tagged with a non-empty
// LegalFamily.
func (r RuleNode) HasLegalFamilyTag() bool {
	return r.LegalFamily != ""
}

// IsFullyJurisdictionTagged reports whether r carries both a
// JurisdictionCode and a LegalFamily tag.
func (r RuleNode) IsFullyJurisdictionTagged() bool {
	return r.HasJurisdictionTag() && r.HasLegalFamilyTag()
}
