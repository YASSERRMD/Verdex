package lawapplication

import (
	"sort"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

// BuildElementFactMap produces the element-to-fact bookkeeping for a
// single controlling rule within an issue: for every argument (across
// both parties) that addresses issueNodeID and cites ruleID in its
// SupportingRuleIDs, every fact in that argument's SupportingFactIDs is
// recorded as backing ruleID's application, annotated with its
// evidenceweighing.FactWeight (weight/contradiction) when available and
// which parties cited it.
//
// This is deterministic bookkeeping/aggregation, not natural-language
// generation, per the plan's "apply legal tests/elements to facts"
// requirement: it structures which weighed facts back the application of
// which rule, without synthesizing prose about how the fact satisfies a
// legal element (that judgment belongs to Phase 055's synthesis agent).
// A fact cited without a matching FactWeight in evidence (e.g. the
// evidenceweighing run predates a later-added argument) is still
// recorded, with FactWeight 0 and Contradicted false — a caller can
// detect this defensively via evidence.FactWeights coverage, but this
// function does not fail or drop the citation.
func BuildElementFactMap(issueNodeID, ruleID string, args []ArgumentRef, evidence evidenceweighing.Result) []ElementFactEntry {
	weights := make(map[string]evidenceweighing.FactWeight, len(evidence.FactWeights))
	for _, fw := range evidence.FactWeights {
		weights[fw.FactNodeID] = fw
	}

	citingArgs := argumentsForIssueAndRule(issueNodeID, ruleID, args)

	type accum struct {
		parties map[string]struct{}
	}
	factOrder := make([]string, 0, len(citingArgs))
	factAccum := make(map[string]*accum, len(citingArgs))

	for _, a := range citingArgs {
		for _, factID := range a.SupportingFactIDs {
			if factID == "" {
				continue
			}
			acc, ok := factAccum[factID]
			if !ok {
				acc = &accum{parties: make(map[string]struct{})}
				factAccum[factID] = acc
				factOrder = append(factOrder, factID)
			}
			if a.PartyID != "" {
				acc.parties[a.PartyID] = struct{}{}
			}
		}
	}

	sort.Strings(factOrder)

	out := make([]ElementFactEntry, 0, len(factOrder))
	for _, factID := range factOrder {
		acc := factAccum[factID]
		parties := make([]string, 0, len(acc.parties))
		for p := range acc.parties {
			parties = append(parties, p)
		}
		sort.Strings(parties)

		fw := weights[factID]
		out = append(out, ElementFactEntry{
			RuleID:         ruleID,
			FactNodeID:     factID,
			FactWeight:     fw.Weight,
			Contradicted:   fw.Contradicted,
			CitingPartyIDs: parties,
		})
	}

	return out
}
