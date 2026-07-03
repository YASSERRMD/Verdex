package casesearch

import (
	"context"
	"strings"
)

// PartyLookup resolves the parties involved in a case, so Search can
// apply Filter.PartyName. Typically backed by packages/timeline's Party
// records for the case. casesearch does not depend on packages/timeline
// directly (it has no case-scoped construction pattern to compose the
// way knowledgeapi/hybridretrieval do), so this is a narrow function seam
// instead — a caller with a timeline.Timeline (or any other party
// source) per case adapts it into this shape.
//
// Returning an error for one case is treated as "this case has no
// resolvable party data" (excluded from a party-filtered search) rather
// than failing the whole request, mirroring CaseSearcherResolver's
// per-case error tolerance.
type PartyLookup func(ctx context.Context, caseID string) ([]string, error)

// matchesPartyName reports whether any name in names contains needle as a
// case-insensitive substring.
func matchesPartyName(names []string, needle string) bool {
	if needle == "" {
		return true
	}
	lowerNeedle := strings.ToLower(needle)
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), lowerNeedle) {
			return true
		}
	}
	return false
}
