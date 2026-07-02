package synthesisagent

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

// deriveWeakestLink identifies the single supporting element that most
// threatens tc's reliability, preferring a deterministically-computed
// signal over the model's own self-reported weakestLink text (modelText):
//
//  1. A fabricated (stripped) reference is the strongest possible
//     weak-link signal — the model cited something that turned out not to
//     exist, so that is surfaced first.
//  2. Otherwise, the lowest-weight or contradicted fact among in.Facts
//     that this conclusion actually relies on (SupportingFactIDs) is
//     surfaced, since a contradicted or thin fact is a concrete,
//     tree-verifiable weakness.
//  3. Otherwise, a lawapplication.ConflictingAuthority affecting the
//     issue's controlling rules is surfaced.
//  4. Otherwise, the model's own self-reported modelText is used verbatim
//     (it is not a fabrication signal in itself, just unverified
//     free-text commentary).
//  5. If nothing else is available (no supporting facts, no conflicts,
//     and the model reported no weakest-link text), the result is empty.
func deriveWeakestLink(tc TentativeConclusion, in issueSynthesisInput, modelText string) string {
	if len(tc.FabricatedNodeIDs) > 0 {
		return fmt.Sprintf("cited node(s) %v do not exist in the case tree (or were not offered as evidence for this issue) and were removed", tc.FabricatedNodeIDs)
	}

	if weakest, ok := weakestFact(tc.SupportingFactIDs, in.FactWeights); ok {
		if weakest.Contradicted {
			return fmt.Sprintf("supporting fact %q is contradicted by opposing evidence", weakest.FactNodeID)
		}
		return fmt.Sprintf("supporting fact %q has the lowest evidentiary weight (%.2f) among this conclusion's cited facts", weakest.FactNodeID, weakest.Weight)
	}

	if in.HasApplication && len(in.Application.Conflicts) > 0 {
		c := in.Application.Conflicts[0]
		return fmt.Sprintf("controlling rules %q and %q conflict: %s", c.FirstRuleID, c.SecondRuleID, c.Rationale)
	}

	return modelText
}

// weakestFact returns the evidenceweighing.FactWeight with the lowest
// Weight among factIDs present in weights (preferring any Contradicted
// fact over a merely low-weight one), and whether any such fact was
// found.
func weakestFact(factIDs []string, weights map[string]evidenceweighing.FactWeight) (evidenceweighing.FactWeight, bool) {
	var (
		best   evidenceweighing.FactWeight
		found  bool
		bestOK bool
	)
	for _, id := range factIDs {
		fw, ok := weights[id]
		if !ok {
			continue
		}
		if !found {
			best, found, bestOK = fw, true, fw.Contradicted
			continue
		}
		// A contradicted fact always outranks a merely low-weight one as
		// the "weakest" signal, since contradiction is a stronger,
		// concrete reliability threat than a low-but-uncontested weight.
		if fw.Contradicted && !bestOK {
			best, bestOK = fw, true
			continue
		}
		if fw.Contradicted == bestOK && fw.Weight < best.Weight {
			best = fw
		}
	}
	return best, found
}
