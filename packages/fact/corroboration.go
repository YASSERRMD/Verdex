package fact

// minCorroborationOverlap is the minimum symmetric token-overlap ratio
// (see corroborationOverlap) between two facts' text required to treat
// them as candidate corroboration for the same underlying claim.
const minCorroborationOverlap = 0.4

// CorroborationLink records that two independent fact nodes support the
// same underlying claim, together with the text-overlap score that
// produced the match.
type CorroborationLink struct {
	// FactAID and FactBID are the irac.FactNode.ID values that
	// corroborate one another. FactAID is always the fact that appeared
	// first in the input slice passed to DetectCorroboration, for
	// deterministic, reproducible output.
	FactAID string

	// FactBID is the corroborating fact's ID.
	FactBID string

	// Overlap is the token-overlap score (see corroborationOverlap) that
	// produced this match, in the closed interval [0, 1].
	Overlap float64
}

// CorroborationCandidate is the minimal shape DetectCorroboration needs:
// a fact's ID, its assertion text, and (optionally) the party it is
// attributed to.
type CorroborationCandidate struct {
	ID      string
	Text    string
	PartyID string
}

// DetectCorroboration scans candidates pairwise and returns a
// CorroborationLink for every pair of facts attributed to different,
// non-empty parties whose text overlap (see corroborationOverlap) is at
// or above minCorroborationOverlap — independent corroboration is most
// meaningful when it comes from more than one source/party, so same-party
// pairs (which may just be the same source restating itself) are
// skipped. Facts with an empty PartyID are compared against every other
// candidate, since "independent" cannot be established either way for
// them, and excluding them entirely would silently drop legitimate
// corroboration signals for facts built without party attribution.
func DetectCorroboration(candidates []CorroborationCandidate) []CorroborationLink {
	var links []CorroborationLink
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			a, b := candidates[i], candidates[j]
			if a.PartyID != "" && b.PartyID != "" && a.PartyID == b.PartyID {
				continue
			}
			if overlap := corroborationOverlap(a.Text, b.Text); overlap >= minCorroborationOverlap {
				links = append(links, CorroborationLink{
					FactAID: a.ID,
					FactBID: b.ID,
					Overlap: overlap,
				})
			}
		}
	}
	return links
}

// corroborationOverlap returns a symmetric Jaccard token-overlap ratio
// (|A ∩ B| / |A ∪ B|) between a and b's normalized token sets, used as a
// lightweight subject-overlap heuristic to detect candidate corroboration
// pairs. Symmetric (unlike packages/issue's directional keywordOverlap)
// because corroboration between two facts is a mutual relationship, not a
// "does A cover B" containment check.
func corroborationOverlap(a, b string) float64 {
	tokensA := temporalTokenSet(a)
	tokensB := temporalTokenSet(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}

	intersection := 0
	for t := range tokensA {
		if _, ok := tokensB[t]; ok {
			intersection++
		}
	}
	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// CorroborationCount returns the number of CorroborationLinks in links
// that reference factID (as either FactAID or FactBID), used by
// reliability.go to fold corroboration strength into a fact's
// reliability score.
func CorroborationCount(factID string, links []CorroborationLink) int {
	count := 0
	for _, l := range links {
		if l.FactAID == factID || l.FactBID == factID {
			count++
		}
	}
	return count
}
