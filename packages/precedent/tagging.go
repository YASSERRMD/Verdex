package precedent

import (
	"regexp"
	"strings"
)

// CategoryCode is a short machine-readable category identifier matching
// packages/category's CategoryCode convention (e.g. "civil", "criminal",
// "tort"). Defined locally as an opaque string type — mirroring
// packages/statute's own CategoryCode and irac.RuleNode's
// JurisdictionCode/LegalFamily fields, which are opaque strings rather
// than hard dependencies on packages/category — so this package can tag
// rules by category without taking a hard module dependency on
// packages/category. Callers that want compile-time-checked category
// codes can convert from category.CategoryCode via string(code) at the
// call site.
type CategoryCode string

// TaggedPrecedent bundles a PrecedentRule with the category/issue tags
// applied to it, so downstream stages (hierarchy.go, embed.go,
// authority.go, persist.go) can read tags without re-deriving them.
type TaggedPrecedent struct {
	PrecedentRule

	// CategoryCode is the case-category taxonomy code this precedent
	// belongs to (see packages/category.Taxonomy for the code space).
	CategoryCode CategoryCode

	// IssueKeywords are candidate keywords drawn from the precedent's
	// Holding/RatioDecidendi text, for later issue-matching (see
	// packages/issue). This is a lightweight lexical heuristic, not a
	// full topic model.
	IssueKeywords []string
}

// TagOptions configures TagPrecedents.
type TagOptions struct {
	// CategoryCode tags every precedent with this category code.
	CategoryCode CategoryCode

	// JurisdictionCode overwrites every precedent's
	// irac.RuleNode.JurisdictionCode field. If empty, each precedent's
	// existing JurisdictionCode (set at build time via RuleBuildOptions)
	// is left unchanged.
	JurisdictionCode string

	// LegalFamily overwrites every precedent's irac.RuleNode.LegalFamily
	// field. If empty, each precedent's existing LegalFamily is left
	// unchanged.
	LegalFamily string

	// MaxKeywords caps how many IssueKeywords are extracted per
	// precedent. If zero or negative, defaultMaxKeywords is used.
	MaxKeywords int
}

// defaultMaxKeywords is the default cap on extracted IssueKeywords per
// precedent when TagOptions.MaxKeywords is unset.
const defaultMaxKeywords = 8

// TagPrecedents applies opts to every rule in rules, returning a new slice
// of TaggedPrecedent (rules is not mutated in place). Rules with an empty
// JurisdictionCode/LegalFamily after tagging are left as-is: TagPrecedents
// performs no validation against a live packages/jurisdiction or
// packages/category registry, mirroring packages/statute's opaque-string
// tagging convention.
func TagPrecedents(rules []PrecedentRule, opts TagOptions) []TaggedPrecedent {
	maxKeywords := opts.MaxKeywords
	if maxKeywords <= 0 {
		maxKeywords = defaultMaxKeywords
	}

	tagged := make([]TaggedPrecedent, 0, len(rules))
	for _, r := range rules {
		if opts.JurisdictionCode != "" {
			r.JurisdictionCode = opts.JurisdictionCode
		}
		if opts.LegalFamily != "" {
			r.LegalFamily = opts.LegalFamily
		}
		tagged = append(tagged, TaggedPrecedent{
			PrecedentRule: r,
			CategoryCode:  opts.CategoryCode,
			IssueKeywords: ExtractIssueKeywords(r.Holding+" "+r.RatioDecidendi, maxKeywords),
		})
	}
	return tagged
}

// wordRe matches a single alphabetic "word" token for keyword extraction.
var wordRe = regexp.MustCompile(`[A-Za-z][A-Za-z'-]*`)

// stopWords lists common English function words excluded from candidate
// issue keywords, since they carry no discriminating legal-issue signal.
var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"been": {}, "but": {}, "by": {}, "case": {}, "court": {}, "did": {},
	"do": {}, "does": {}, "for": {}, "from": {}, "had": {}, "has": {},
	"have": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"of": {}, "on": {}, "or": {}, "that": {}, "the": {}, "their": {},
	"there": {}, "this": {}, "to": {}, "was": {}, "were": {}, "which": {},
	"who": {}, "with": {},
}

// ExtractIssueKeywords is a deterministic lexical heuristic that pulls
// candidate issue keywords out of text: it lowercases, tokenizes on
// alphabetic runs, drops stop words and single-character tokens, then
// returns up to max unique tokens in order of first appearance.
//
// This is intentionally simple — a frequency/stopword filter, not a topic
// model — mirroring this package's broader "deterministic heuristic, no
// ML model at runtime" design principle (see doc.go).
func ExtractIssueKeywords(text string, max int) []string {
	if max <= 0 {
		max = defaultMaxKeywords
	}
	seen := make(map[string]struct{})
	var out []string
	for _, m := range wordRe.FindAllString(text, -1) {
		lower := strings.ToLower(m)
		if len(lower) < 3 {
			continue
		}
		if _, stop := stopWords[lower]; stop {
			continue
		}
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, lower)
		if len(out) >= max {
			break
		}
	}
	return out
}
