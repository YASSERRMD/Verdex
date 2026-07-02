package precedent

import (
	"math"
	"time"
)

// recencyHalfLifeYears controls how quickly a precedent's recency
// contribution to AuthorityScore decays: every recencyHalfLifeYears years
// since DecidedDate, the recency component halves. 15 years is used as a
// deliberately gentle decay — old-but-never-overruled precedent (e.g.
// foundational common-law authorities) should still carry meaningful
// weight rather than being driven to near zero within a few years.
const recencyHalfLifeYears = 15.0

// courtWeightFactor and recencyFactor control how much AuthorityScore
// weighs court-hierarchy strength versus recency. Court hierarchy
// dominates (binding authority is the primary legal signal) with recency
// as a secondary adjustment, mirroring how practitioners treat an old
// Supreme Court decision as still highly authoritative while a recent
// trial-court decision remains only persuasive.
const (
	courtWeightFactor = 0.7
	recencyFactor     = 0.3
)

// RecencyScore returns a decayed recency score in (0, 1] for a precedent
// decided at decidedDate, evaluated as of asOf. More recent decisions
// score closer to 1; the score halves every recencyHalfLifeYears years
// via exponential decay. A zero-value decidedDate (unknown decision date)
// returns 0.5 as a neutral midpoint, since an unknown date is neither
// evidence of freshness nor staleness.
func RecencyScore(decidedDate, asOf time.Time) float64 {
	if decidedDate.IsZero() {
		return 0.5
	}
	if asOf.IsZero() {
		asOf = time.Now()
	}
	years := asOf.Sub(decidedDate).Hours() / (24 * 365.25)
	if years < 0 {
		// A decision "in the future" relative to asOf is treated as
		// maximally recent rather than penalized.
		years = 0
	}
	return math.Exp(-math.Ln2 * years / recencyHalfLifeYears)
}

// AuthorityScore combines a precedent's CourtLevel.Weight() (binding
// authority strength) and its recency (via RecencyScore, evaluated as of
// now) into a single score in [0, 1], with higher courts and more recent
// decisions scoring higher.
//
// The two components are blended as a weighted sum
// (courtWeightFactor*courtWeight + recencyFactor*recencyScore) rather
// than multiplied, so an old-but-supreme decision is not driven toward
// zero purely by age — court hierarchy is the dominant legal signal, with
// recency as a secondary adjustment.
func AuthorityScore(precedent HierarchyRule) float64 {
	return AuthorityScoreAsOf(precedent, time.Now())
}

// AuthorityScoreAsOf is AuthorityScore with an explicit reference time,
// for deterministic testing (and for scoring a corpus as of a historical
// date rather than the current wall clock).
func AuthorityScoreAsOf(precedent HierarchyRule, asOf time.Time) float64 {
	courtWeight := precedent.CourtLevel.Weight()
	recency := RecencyScore(precedent.Source.DecidedDate, asOf)
	score := courtWeightFactor*courtWeight + recencyFactor*recency
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// ScoredPrecedent bundles an EmbeddedPrecedent with its computed
// AuthorityScore, so downstream stages (persist.go) and callers can read
// the score without recomputing it.
type ScoredPrecedent struct {
	EmbeddedPrecedent

	// Authority is this precedent's AuthorityScoreAsOf(asOf) value.
	Authority float64
}

// ScorePrecedents computes AuthorityScoreAsOf(asOf) for every rule in
// rules, returning a new slice of ScoredPrecedent (rules is not mutated
// in place). If asOf is zero, time.Now() is used.
func ScorePrecedents(rules []EmbeddedPrecedent, asOf time.Time) []ScoredPrecedent {
	if asOf.IsZero() {
		asOf = time.Now()
	}
	out := make([]ScoredPrecedent, 0, len(rules))
	for _, r := range rules {
		out = append(out, ScoredPrecedent{
			EmbeddedPrecedent: r,
			Authority:         AuthorityScoreAsOf(r.HierarchyRule, asOf),
		})
	}
	return out
}
