package evidenceweighing

// DetectContradictions scans arguments for facts cited by two arguments
// that share an IssueNodeID but come from different, non-empty PartyID
// values, and returns one Contradiction per such pair. PartyID is the
// only stance proxy available at this reasoning stage — see
// types.go's CitingArgument doc comment — so this necessarily treats
// every same-issue, opposing-party citation of the same fact as a
// contradiction, even in the (legally ordinary) case where both parties
// cite an undisputed fact but draw different legal conclusions from it.
// This is a conservative over-detection by design: Phase 055's synthesis
// agent is expected to review flagged Contradictions with the fuller
// context of each argument's Claim, not treat every flagged pair as
// necessarily an actual factual dispute. See
// doc/evidence-weighing.md's "Contradiction detection" section.
//
// Arguments attributed to an empty PartyID are skipped entirely (neither
// side of a contradiction can be established for them), mirroring
// packages/fact's DetectCorroboration handling of same-party pairs but
// applied to the opposite condition here (opposing parties, not matching
// ones).
//
// Results are deterministic: arguments are compared in input order, and
// ArgumentAID is always the argument that appeared first in arguments.
func DetectContradictions(arguments []CitingArgument) []Contradiction {
	var out []Contradiction
	for i := 0; i < len(arguments); i++ {
		a := arguments[i]
		if a.PartyID == "" {
			continue
		}
		for j := i + 1; j < len(arguments); j++ {
			b := arguments[j]
			if b.PartyID == "" || b.PartyID == a.PartyID {
				continue
			}
			if a.IssueNodeID == "" || a.IssueNodeID != b.IssueNodeID {
				continue
			}
			for _, sharedFact := range sharedFactIDs(a.SupportingFactIDs, b.SupportingFactIDs) {
				out = append(out, Contradiction{
					FactNodeID:  sharedFact,
					IssueNodeID: a.IssueNodeID,
					ArgumentAID: a.ArgumentID,
					ArgumentBID: b.ArgumentID,
					PartyAID:    a.PartyID,
					PartyBID:    b.PartyID,
				})
			}
		}
	}
	return out
}

// sharedFactIDs returns every fact ID present in both a and b, in the
// order it appears in a, skipping duplicates within a itself.
func sharedFactIDs(a, b []string) []string {
	inB := make(map[string]struct{}, len(b))
	for _, id := range b {
		inB[id] = struct{}{}
	}

	seen := make(map[string]struct{}, len(a))
	var shared []string
	for _, id := range a {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := inB[id]; ok {
			shared = append(shared, id)
		}
	}
	return shared
}

// factsInContradiction returns the set of fact IDs appearing in at least
// one Contradiction.
func factsInContradiction(contradictions []Contradiction) map[string]struct{} {
	out := make(map[string]struct{}, len(contradictions))
	for _, c := range contradictions {
		out[c.FactNodeID] = struct{}{}
	}
	return out
}

// CorroborationCounts returns, for every fact ID cited by at least one
// argument, the number of distinct arguments (across both parties) whose
// SupportingFactIDs include it. Unlike DetectContradictions, this counts
// every citing argument regardless of party — a fact repeatedly relied
// upon (even by the same party across multiple arguments, or especially
// by both parties for compatible claims) is corroborated evidence in the
// sense that matters for reliability scoring: it is not resting on a
// single, isolated assertion.
func CorroborationCounts(arguments []CitingArgument) map[string]int {
	counts := make(map[string]int)
	for _, arg := range arguments {
		seen := make(map[string]struct{}, len(arg.SupportingFactIDs))
		for _, factID := range arg.SupportingFactIDs {
			if factID == "" {
				continue
			}
			if _, ok := seen[factID]; ok {
				continue
			}
			seen[factID] = struct{}{}
			counts[factID]++
		}
	}
	return counts
}
