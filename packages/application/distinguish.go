package application

import (
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// DistinguishingFact records a fact from the current case that differs
// from the typical fact pattern the precedent was decided on — the
// classic common-law "distinguishing" move, where a party argues a cited
// precedent should not control because the present facts diverge from
// it in some legally significant way. DistinguishingFact is only
// meaningful when the linked rule's Origin is OriginPrecedent: a statute
// has no "typical fact pattern" to diverge from, so
// NewDistinguishingFact rejects OriginStatute rules.
type DistinguishingFact struct {
	// Fact is the current case's fact node that diverges from the
	// precedent's fact pattern.
	Fact irac.FactNode

	// Rule is the precedent-origin OriginatedRule being distinguished.
	Rule OriginatedRule

	// Rationale is a free-text explanation of how/why Fact differs from
	// the precedent's typical fact pattern (e.g. "the precedent involved
	// a written contract; here the agreement was oral").
	Rationale string

	// NotedAt is when this distinguishing fact was recorded.
	NotedAt time.Time
}

// NewDistinguishingFact constructs a DistinguishingFact for fact and
// rule. Returns ErrEmptyInput if rationale is blank, and
// errNotPrecedentOrigin if rule.Origin is not OriginPrecedent.
func NewDistinguishingFact(fact irac.FactNode, rule OriginatedRule, rationale string, notedAt time.Time) (DistinguishingFact, error) {
	if strings.TrimSpace(rationale) == "" {
		return DistinguishingFact{}, ErrEmptyInput
	}
	if rule.Origin != OriginPrecedent {
		return DistinguishingFact{}, errNotPrecedentOrigin
	}
	if notedAt.IsZero() {
		notedAt = time.Now()
	}
	return DistinguishingFact{
		Fact:      fact,
		Rule:      rule,
		Rationale: rationale,
		NotedAt:   notedAt,
	}, nil
}
