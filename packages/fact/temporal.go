package fact

import (
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/timeline"
)

// TemporalAnchor links a fact node to a timeline.Event it concerns, when
// the fact describes a dated occurrence, carrying the resolved date
// forward as OccurredAt (mirroring timeline.Event.OccurredAt's own
// "nil means unknown/unresolved" convention).
type TemporalAnchor struct {
	// FactID is the irac.FactNode.ID this anchor belongs to.
	FactID string

	// EventID is the timeline.Event.ID this fact is anchored to, when a
	// matching event was found. Empty when the fact could not be
	// anchored to any supplied event.
	EventID string

	// OccurredAt is the date the fact's underlying occurrence took
	// place, when known. Nil when the fact could not be anchored to a
	// dated event.
	OccurredAt *time.Time
}

// AnchorToEvent links a fact (identified by factID, with text
// factText) to the timeline.Event among events whose SegmentID matches
// segmentID — the same segment the fact was built from (see
// build.go's BuildFactNode) — treating that as the strongest possible
// anchor since the fact and event share a common originating segment.
//
// If no event shares segmentID (or segmentID is empty), AnchorToEvent
// falls back to a token-overlap match against event.Description,
// choosing the highest-scoring event at or above minTemporalOverlap. If
// no event of either kind matches, AnchorToEvent returns a TemporalAnchor
// with an empty EventID and nil OccurredAt.
func AnchorToEvent(factID, factText, segmentID string, events []timeline.Event) TemporalAnchor {
	anchor := TemporalAnchor{FactID: factID}

	if segmentID != "" {
		for _, ev := range events {
			if ev.SegmentID == segmentID {
				anchor.EventID = ev.ID
				anchor.OccurredAt = ev.OccurredAt
				return anchor
			}
		}
	}

	bestScore := 0.0
	var best *timeline.Event
	for i, ev := range events {
		score := temporalTokenOverlap(factText, ev.Description)
		if score > bestScore {
			bestScore = score
			best = &events[i]
		}
	}
	if best != nil && bestScore >= minTemporalOverlap {
		anchor.EventID = best.ID
		anchor.OccurredAt = best.OccurredAt
	}
	return anchor
}

// minTemporalOverlap is the minimum token-overlap ratio (see
// temporalTokenOverlap) required to anchor a fact to an event via
// description-text similarity, when no shared SegmentID is available.
const minTemporalOverlap = 0.3

// temporalTokenOverlap returns the fraction of normalized tokens in b
// that also appear in a's token set, mirroring packages/issue's
// keywordOverlap convention (see packages/issue/claim_map.go) but
// re-implemented locally to avoid a hard dependency on that package.
func temporalTokenOverlap(a, b string) float64 {
	tokensA := temporalTokenSet(a)
	tokensB := temporalTokenSet(b)
	if len(tokensB) == 0 {
		return 0
	}
	intersection := 0
	for t := range tokensB {
		if _, ok := tokensA[t]; ok {
			intersection++
		}
	}
	return float64(intersection) / float64(len(tokensB))
}

// temporalTokenSet normalizes text to a lowercase, punctuation-stripped
// set of tokens.
func temporalTokenSet(text string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	})
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if f != "" {
			set[f] = struct{}{}
		}
	}
	return set
}
