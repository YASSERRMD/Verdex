package statute

import (
	"fmt"
	"regexp"
	"strings"
)

// CrossReference is a single citation-shaped reference detected within a
// rule's text (e.g. "see Section 12" or "subject to Section 5(a)"),
// together with its resolution status against the rules loaded from the
// same corpus.
type CrossReference struct {
	// SourceRuleID is the irac.RuleNode.ID whose text contained the
	// reference.
	SourceRuleID string `json:"source_rule_id"`

	// RawText is the exact matched reference text (e.g. "Section 12").
	RawText string `json:"raw_text"`

	// Section is the section number extracted from RawText.
	Section string `json:"section"`

	// Clause is the clause identifier extracted from RawText, if the
	// reference was clause-specific (e.g. "Section 5(a)"). Empty
	// otherwise.
	Clause string `json:"clause,omitempty"`

	// ResolvedRuleID is the irac.RuleNode.ID within the same corpus that
	// RawText resolves to, when resolution succeeds. Empty when
	// unresolved.
	ResolvedRuleID string `json:"resolved_rule_id,omitempty"`
}

// IsResolved reports whether the reference was successfully matched to
// a rule node within the same corpus.
func (c CrossReference) IsResolved() bool {
	return c.ResolvedRuleID != ""
}

// xrefRe matches citation-shaped references within free text, e.g.
// "see Section 12", "under Section 5(a)", "pursuant to section 3".
// Capture groups: (1) section number, (2) optional clause identifier.
var xrefRe = regexp.MustCompile(`(?i)\bSection\s+(\S+?)(?:\(([a-zA-Z0-9]+)\))?\b`)

// DetectCrossReferences scans text for citation-shaped references and
// returns one CrossReference per match, unresolved (ResolvedRuleID
// empty). ResolveCrossReferences fills in resolution against a corpus.
func DetectCrossReferences(sourceRuleID, text string) []CrossReference {
	matches := xrefRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]CrossReference, 0, len(matches))
	for _, m := range matches {
		section := strings.TrimSuffix(m[1], ".")
		refs = append(refs, CrossReference{
			SourceRuleID: sourceRuleID,
			RawText:      strings.TrimSpace(m[0]),
			Section:      section,
			Clause:       m[2],
		})
	}
	return refs
}

// ResolveCrossReferences resolves each CrossReference in refs against
// rules (typically every BuiltRule/TaggedRule loaded from the same
// corpus as the reference's source), by matching Section (and Clause,
// when present) against each rule's Citation. References with no
// matching rule are returned unchanged (ResolvedRuleID left empty,
// still present in the returned slice — callers that require strict
// resolution should check IsResolved() per entry and surface
// ErrUnresolvedCrossReference themselves; this function does not error,
// since a partially-resolved corpus is a valid intermediate state during
// ingestion).
func ResolveCrossReferences(refs []CrossReference, rules []BuiltRule) []CrossReference {
	// Index rules by "section" and "section(clause)" citation keys for
	// O(1) lookup.
	bySection := make(map[string]string)       // section -> rule id (section- or act-granularity rule)
	bySectionClause := make(map[string]string) // "section(clause)" -> rule id
	for _, r := range rules {
		if r.Citation.Section == "" {
			continue
		}
		if r.Citation.Clause == "" {
			bySection[r.Citation.Section] = r.Node.ID
			continue
		}
		key := fmt.Sprintf("%s(%s)", r.Citation.Section, r.Citation.Clause)
		bySectionClause[key] = r.Node.ID
		// A clause-granularity rule's enclosing section is also a valid
		// resolution target for a bare "Section N" reference (first
		// clause found for that section wins if no dedicated section
		// rule exists).
		if _, ok := bySection[r.Citation.Section]; !ok {
			bySection[r.Citation.Section] = r.Node.ID
		}
	}

	resolved := make([]CrossReference, len(refs))
	for i, ref := range refs {
		resolved[i] = ref
		if ref.Clause != "" {
			key := fmt.Sprintf("%s(%s)", ref.Section, ref.Clause)
			if id, ok := bySectionClause[key]; ok {
				resolved[i].ResolvedRuleID = id
				continue
			}
		}
		if id, ok := bySection[ref.Section]; ok {
			resolved[i].ResolvedRuleID = id
		}
	}
	return resolved
}

// DetectAndResolveAll runs DetectCrossReferences over every rule's text
// in rules and resolves the results against the same rules slice,
// returning all CrossReferences (resolved and unresolved) found across
// the whole corpus.
func DetectAndResolveAll(rules []BuiltRule) []CrossReference {
	var all []CrossReference
	for _, r := range rules {
		all = append(all, DetectCrossReferences(r.Node.ID, r.Node.Text)...)
	}
	return ResolveCrossReferences(all, rules)
}

// UnresolvedCrossReferences filters refs down to only those that failed
// to resolve.
func UnresolvedCrossReferences(refs []CrossReference) []CrossReference {
	var out []CrossReference
	for _, r := range refs {
		if !r.IsResolved() {
			out = append(out, r)
		}
	}
	return out
}
