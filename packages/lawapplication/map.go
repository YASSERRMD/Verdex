package lawapplication

import "sort"

// MapIssueToControllingRules returns the sorted, de-duplicated union of
// governingRuleIDs (from the case tree's Rule--governs-->Issue edges,
// see IssueInput.GoverningRuleIDs) and every RuleID any argument in args
// cites via SupportingRuleIDs for issueNodeID — the first step of law
// application per the plan's "map each issue to controlling rules"
// requirement: a rule counts as controlling for an issue either because
// the tree structurally links it there, or because at least one party
// actually invoked it in argument for that issue, even if the governs
// edge is missing (e.g. an upstream gap in tree assembly).
func MapIssueToControllingRules(issueNodeID string, governingRuleIDs []string, args []ArgumentRef) []string {
	seen := make(map[string]struct{}, len(governingRuleIDs))
	var out []string

	addRule := func(ruleID string) {
		if ruleID == "" {
			return
		}
		if _, ok := seen[ruleID]; ok {
			return
		}
		seen[ruleID] = struct{}{}
		out = append(out, ruleID)
	}

	for _, ruleID := range governingRuleIDs {
		addRule(ruleID)
	}

	for _, a := range args {
		if a.IssueNodeID != issueNodeID {
			continue
		}
		for _, ruleID := range a.SupportingRuleIDs {
			addRule(ruleID)
		}
	}

	sort.Strings(out)
	return out
}

// argumentsForIssueAndRule returns every ArgumentRef in args addressing
// issueNodeID whose SupportingRuleIDs includes ruleID.
func argumentsForIssueAndRule(issueNodeID, ruleID string, args []ArgumentRef) []ArgumentRef {
	var out []ArgumentRef
	for _, a := range args {
		if a.IssueNodeID != issueNodeID {
			continue
		}
		for _, rid := range a.SupportingRuleIDs {
			if rid == ruleID {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

// partiesInvokingRule returns the sorted, de-duplicated set of PartyIDs
// among args (already filtered to a single issue+rule via
// argumentsForIssueAndRule) that invoked the rule.
func partiesInvokingRule(args []ArgumentRef) []string {
	seen := make(map[string]struct{}, len(args))
	var out []string
	for _, a := range args {
		if a.PartyID == "" {
			continue
		}
		if _, ok := seen[a.PartyID]; ok {
			continue
		}
		seen[a.PartyID] = struct{}{}
		out = append(out, a.PartyID)
	}
	sort.Strings(out)
	return out
}
