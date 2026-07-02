package statute

import "time"

// Amendment records a single historical change to a rule's text: the
// prior text it replaced, the date the change took effect, and an
// optional human-readable description of the amending instrument (e.g.
// "Amendment Act No. 7 of 2019").
type Amendment struct {
	// PriorText is the rule text as it read before this amendment took
	// effect.
	PriorText string `json:"prior_text"`

	// EffectiveDate is the date this amendment took effect. Required.
	EffectiveDate time.Time `json:"effective_date"`

	// Description is a short human-readable note about the amending
	// instrument (e.g. its own act/section citation). Optional.
	Description string `json:"description,omitempty"`
}

// AmendmentRecord bundles a rule's amendment history and current
// lifecycle state with the rule's own ID, letting AmendedRule/
// ApplyAmendments carry this alongside — but not embedded directly on —
// irac.RuleNode, since irac.RuleNode's schema is owned by
// packages/irac and this phase must not modify it.
type AmendmentRecord struct {
	// RuleID is the irac.RuleNode.ID this record describes.
	RuleID string `json:"rule_id"`

	// EffectiveDate is the date the rule's *current* text took effect.
	// Nil means unknown/unspecified (the rule is treated as always having
	// been in force in its current form).
	EffectiveDate *time.Time `json:"effective_date,omitempty"`

	// History lists this rule's amendments in chronological order
	// (oldest first).
	History []Amendment `json:"history,omitempty"`

	// SupersededBy holds the ID of the irac.RuleNode that replaced this
	// one, when this rule has itself been superseded by a later
	// amendment that was significant enough to be modeled as a new rule
	// node rather than an in-place text change. Nil means this rule is
	// not superseded (it is either current or was amended in place,
	// tracked via History instead).
	SupersededBy *string `json:"superseded_by,omitempty"`
}

// AmendedRule bundles a TaggedRule with its AmendmentRecord.
type AmendedRule struct {
	TaggedRule
	Amendment AmendmentRecord
}

// NewAmendmentRecord constructs an AmendmentRecord for ruleID with no
// history and no effective date set.
func NewAmendmentRecord(ruleID string) AmendmentRecord {
	return AmendmentRecord{RuleID: ruleID}
}

// WithEffectiveDate returns a copy of r with EffectiveDate set to date.
func (r AmendmentRecord) WithEffectiveDate(date time.Time) AmendmentRecord {
	r.EffectiveDate = &date
	return r
}

// AddAmendment returns a copy of r with amendment appended to History.
// History is kept in the order amendments are added; callers supplying
// amendments out of chronological order should sort before calling, or
// use SortHistory afterward.
func (r AmendmentRecord) AddAmendment(amendment Amendment) AmendmentRecord {
	history := make([]Amendment, len(r.History), len(r.History)+1)
	copy(history, r.History)
	r.History = append(history, amendment)
	return r
}

// SortHistory returns a copy of r with History sorted chronologically by
// EffectiveDate (oldest first).
func (r AmendmentRecord) SortHistory() AmendmentRecord {
	sorted := make([]Amendment, len(r.History))
	copy(sorted, r.History)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].EffectiveDate.Before(sorted[j-1].EffectiveDate); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	r.History = sorted
	return r
}

// SupersedeBy returns a copy of r with SupersededBy set to replacementID.
func (r AmendmentRecord) SupersedeBy(replacementID string) AmendmentRecord {
	r.SupersededBy = &replacementID
	return r
}

// IsSuperseded reports whether r has been superseded by another rule
// node.
func (r AmendmentRecord) IsSuperseded() bool {
	return r.SupersededBy != nil && *r.SupersededBy != ""
}

// SupersessionChain walks records (keyed by RuleID) starting at startID,
// following each record's SupersededBy link, and returns the ordered
// chain of rule IDs from startID to the final (non-superseded, or
// unresolvable) rule. The chain always includes startID as its first
// element. A cycle in the data (a rule that eventually supersedes
// itself) is detected and breaks the walk rather than looping forever;
// the returned bool reports whether a cycle was detected.
func SupersessionChain(records map[string]AmendmentRecord, startID string) ([]string, bool) {
	chain := []string{startID}
	seen := map[string]struct{}{startID: {}}

	current := startID
	for {
		rec, ok := records[current]
		if !ok || !rec.IsSuperseded() {
			return chain, false
		}
		next := *rec.SupersededBy
		if _, cyc := seen[next]; cyc {
			return chain, true
		}
		chain = append(chain, next)
		seen[next] = struct{}{}
		current = next
	}
}

// ApplyAmendments zips rules with their corresponding AmendmentRecord
// (looked up by irac.RuleNode.ID in records) into AmendedRules. Rules
// with no matching record get a fresh empty AmendmentRecord (via
// NewAmendmentRecord), so every rule always has an AmendedRule entry.
func ApplyAmendments(rules []TaggedRule, records map[string]AmendmentRecord) []AmendedRule {
	out := make([]AmendedRule, 0, len(rules))
	for _, r := range rules {
		rec, ok := records[r.Node.ID]
		if !ok {
			rec = NewAmendmentRecord(r.Node.ID)
		}
		out = append(out, AmendedRule{TaggedRule: r, Amendment: rec})
	}
	return out
}
