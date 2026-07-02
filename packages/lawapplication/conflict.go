package lawapplication

import (
	"fmt"
	"sort"
)

// DetectConflictingAuthority flags every pair of distinct controlling
// rules for issueNodeID that were each invoked exclusively by different
// parties: if every argument citing ruleA's PartyID differs from every
// party citing ruleB (and both sets are non-empty), the two rules are
// treated as conflicting authority — one side's case rests on ruleA,
// the other's on ruleB, for the same issue. This package does not
// attempt to decide which rule prevails; it surfaces the pair as a
// ConflictingAuthority finding, per the plan's explicit "handle
// conflicting authority... rather than silently picking one"
// requirement.
//
// This is a deliberately conservative, party-based proxy for conflict,
// mirroring evidenceweighing.DetectContradictions' identical tradeoff:
// two rules invoked by opposing parties for the same issue are not
// necessarily legally inconsistent (a party may cite a rule merely for
// background context, or two rules may be complementary rather than
// competing), but without semantic comparison of how each argument's
// Claim actually uses the rule, opposing-party citation is the only
// stance proxy available at this reasoning stage. See
// doc/law-application.md.
func DetectConflictingAuthority(issueNodeID string, controllingRuleIDs []string, args []ArgumentRef) []ConflictingAuthority {
	ruleParties := make(map[string][]string, len(controllingRuleIDs))
	for _, ruleID := range controllingRuleIDs {
		citing := argumentsForIssueAndRule(issueNodeID, ruleID, args)
		ruleParties[ruleID] = partiesInvokingRule(citing)
	}

	sorted := make([]string, len(controllingRuleIDs))
	copy(sorted, controllingRuleIDs)
	sort.Strings(sorted)

	var out []ConflictingAuthority
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			ruleA, ruleB := sorted[i], sorted[j]
			partiesA := ruleParties[ruleA]
			partiesB := ruleParties[ruleB]
			if len(partiesA) == 0 || len(partiesB) == 0 {
				continue
			}
			if !disjoint(partiesA, partiesB) {
				continue
			}

			out = append(out, ConflictingAuthority{
				IssueNodeID:   issueNodeID,
				FirstRuleID:   ruleA,
				SecondRuleID:  ruleB,
				FirstPartyID:  partiesA[0],
				SecondPartyID: partiesB[0],
				Rationale: fmt.Sprintf(
					"rule %q (invoked by %v) and rule %q (invoked by %v) were cited by disjoint, opposing parties for issue %q; treated as conflicting authority pending synthesis review",
					ruleA, partiesA, ruleB, partiesB, issueNodeID,
				),
			})
		}
	}

	return out
}

// disjoint reports whether a and b (each already sorted, per
// partiesInvokingRule) share no common element.
func disjoint(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return false
		}
	}
	return true
}
