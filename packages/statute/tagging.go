package statute

import "github.com/YASSERRMD/verdex/packages/jurisdiction"

// CategoryCode is a short machine-readable category identifier matching
// packages/category's CategoryCode convention (e.g. "civil", "criminal",
// "labor"). Defined locally as an opaque string type — mirroring
// packages/irac.RuleNode's own JurisdictionCode/LegalFamily fields,
// which are opaque strings rather than hard dependencies on
// packages/jurisdiction — so this package can tag rules by category
// without taking a hard module dependency on packages/category. Callers
// that want compile-time-checked category codes can convert from
// category.CategoryCode via string(code) at the call site.
type CategoryCode string

// TaggedRule bundles a BuiltRule with the category/jurisdiction tags
// applied to it, so downstream stages (amendment.go, xref.go, embed.go,
// persist.go) can read tags without re-deriving them.
type TaggedRule struct {
	BuiltRule

	// CategoryCode is the case-category taxonomy code this rule belongs
	// to (see packages/category.Taxonomy for the code space).
	CategoryCode CategoryCode
}

// TagOptions configures TagRules.
type TagOptions struct {
	// CategoryCode tags every rule with this category code.
	CategoryCode CategoryCode

	// JurisdictionCode overwrites every rule's irac.RuleNode.JurisdictionCode
	// field. If empty, each rule's existing JurisdictionCode (set at
	// build time via RuleBuildOptions) is left unchanged.
	JurisdictionCode string

	// LegalFamily overwrites every rule's irac.RuleNode.LegalFamily
	// field. If empty, each rule's existing LegalFamily is left
	// unchanged. This package imports packages/jurisdiction directly
	// (see go.mod's require+replace) so LegalFamily is typed as
	// jurisdiction.LegalFamily rather than a bare opaque string —
	// unlike JurisdictionCode/CategoryCode above, which stay opaque
	// strings because no shared enum exists for them. TagRules stores
	// the value on irac.RuleNode.LegalFamily (a plain string field) via
	// jurisdiction.LegalFamily's underlying string representation.
	LegalFamily jurisdiction.LegalFamily
}

// IsValidLegalFamily reports whether opts.LegalFamily is either unset
// (no override) or one of packages/jurisdiction's recognized LegalFamily
// constants. TagRules itself does not enforce this — callers that want a
// hard validation gate before tagging should check it explicitly.
func (opts TagOptions) IsValidLegalFamily() bool {
	return opts.LegalFamily == "" || opts.LegalFamily.IsValid()
}

// TagRules applies opts to every rule in rules, returning a new slice of
// TaggedRule (rules is not mutated in place). Rules with an empty
// JurisdictionCode/LegalFamily after tagging are left as-is: TagRules
// performs no validation against a live packages/jurisdiction or
// packages/category registry, mirroring this package's opaque-string
// tagging convention (see CategoryCode's doc comment).
func TagRules(rules []BuiltRule, opts TagOptions) []TaggedRule {
	tagged := make([]TaggedRule, 0, len(rules))
	for _, r := range rules {
		if opts.JurisdictionCode != "" {
			r.Node.JurisdictionCode = opts.JurisdictionCode
		}
		if opts.LegalFamily != "" {
			r.Node.LegalFamily = string(opts.LegalFamily)
		}
		tagged = append(tagged, TaggedRule{
			BuiltRule:    r,
			CategoryCode: opts.CategoryCode,
		})
	}
	return tagged
}
