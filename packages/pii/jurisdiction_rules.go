package pii

import "strings"

// CategoryRule describes how a single PIICategory should be treated within
// a jurisdiction: how sensitive it is considered, and which RedactionMode
// must be applied (overriding whatever mode a caller otherwise configured)
// when text originates from, or is destined for, that jurisdiction.
type CategoryRule struct {
	// Sensitivity is a relative sensitivity score in the closed interval
	// [0, 1], where higher means more sensitive. Used for reporting and for
	// StorageGuard threshold checks (see policy.go); it does not by itself
	// change redaction behavior.
	Sensitivity float64

	// RequiredMode, when non-empty, is the RedactionMode that must be used
	// for matches of this category in this jurisdiction, overriding the
	// caller's configured default mode. For example, a jurisdiction with
	// strict financial-privacy law might require ModeIrreversibleRedact for
	// CategoryFinancial regardless of what the rest of the pipeline uses.
	RequiredMode RedactionMode
}

// JurisdictionPIIRules holds per-jurisdiction PII category overrides, keyed
// by jurisdiction code (matching packages/jurisdiction's CountryCode
// convention, e.g. "AE", "PK", "IN" — or a finer-grained code such as
// "AE-DIFC" for a specific court/jurisdiction record, at the caller's
// discretion).
//
// This lets Verdex express, e.g., "national-ID numbers are more sensitive
// and must always be irreversibly redacted for jurisdiction X" without
// changing the core detection/redaction pipeline.
type JurisdictionPIIRules struct {
	// DefaultRule is applied to any category not explicitly listed for a
	// jurisdiction (and to any lookup for a jurisdiction code with no rules
	// registered at all, when a Sensitivity/RequiredMode is still needed).
	DefaultRule CategoryRule

	rules map[string]map[PIICategory]CategoryRule
}

// NewJurisdictionPIIRules constructs an empty rule set. defaultRule is used
// for any (jurisdiction, category) pair without a specific override.
func NewJurisdictionPIIRules(defaultRule CategoryRule) *JurisdictionPIIRules {
	return &JurisdictionPIIRules{
		DefaultRule: defaultRule,
		rules:       make(map[string]map[PIICategory]CategoryRule),
	}
}

// normalizeCode canonicalizes a jurisdiction code for lookup
// (case-insensitive, trimmed), so "ae", "AE", and " AE " all resolve to the
// same rule set.
func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// SetRule registers (or replaces) the CategoryRule for category within the
// given jurisdiction code.
func (j *JurisdictionPIIRules) SetRule(jurisdictionCode string, category PIICategory, rule CategoryRule) {
	code := normalizeCode(jurisdictionCode)
	if j.rules[code] == nil {
		j.rules[code] = make(map[PIICategory]CategoryRule)
	}
	j.rules[code][category] = rule
}

// RuleFor returns the effective CategoryRule for category within
// jurisdictionCode: the specific override if one is registered, otherwise
// DefaultRule.
func (j *JurisdictionPIIRules) RuleFor(jurisdictionCode string, category PIICategory) CategoryRule {
	code := normalizeCode(jurisdictionCode)
	if perCategory, ok := j.rules[code]; ok {
		if rule, ok := perCategory[category]; ok {
			return rule
		}
	}
	return j.DefaultRule
}

// ApplyToMatches resolves the required RedactionMode for every match's
// category under jurisdictionCode and, when a rule specifies a
// RequiredMode, records it by returning a per-match mode override map
// suitable for merging into a Redactor's ModeByCategory-style handling.
// Matches whose category has no RequiredMode configured are omitted from
// the returned map, leaving the caller's own default mode in effect.
func (j *JurisdictionPIIRules) ApplyToMatches(jurisdictionCode string, matches []PIIMatch) map[PIICategory]RedactionMode {
	overrides := make(map[PIICategory]RedactionMode)
	for _, m := range matches {
		rule := j.RuleFor(jurisdictionCode, m.Category)
		if rule.RequiredMode != "" {
			overrides[m.Category] = rule.RequiredMode
		}
	}
	return overrides
}
