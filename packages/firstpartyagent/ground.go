package firstpartyagent

// groundArgument cross-checks every ID in ma.SupportingFactIDs and
// ma.SupportingRuleIDs against allowed (the exact set of fact/rule IDs
// fetchIssueEvidence resolved for this issue from the case's actual
// tree — see issueEvidence.allowedNodeIDs). Any ID not present in allowed
// is a fabrication: the model referenced a node that either does not
// exist in the tree at all, or exists but was never offered as evidence
// for this issue. Fabricated IDs are stripped rather than trusted, and
// recorded in the returned Argument's FabricatedNodeIDs for transparency
// — this is the anti-fabrication guarantee packages/firstpartyagent
// exists to enforce (see doc/first-party-agent.md).
//
// groundArgument never rejects an Argument outright for having some
// fabricated IDs, as long as at least one real supporting fact or rule
// ID remains after stripping — an argument grounded in partially real
// evidence is still useful, with Grounded=false and FabricatedNodeIDs
// flagging exactly what was removed for a caller/reviewer to audit. An
// Argument left with zero supporting IDs of either kind after stripping
// is considered ungroundable and (ok=false) should be dropped by the
// caller (see assemble.go), since a "claim" with no tree-backed evidence
// at all is indistinguishable from an unsupported assertion.
func groundArgument(ma modelArgument, allowed map[string]struct{}) (Argument, bool) {
	factIDs, fabricatedFacts := partitionByMembership(ma.SupportingFactIDs, allowed)
	ruleIDs, fabricatedRules := partitionByMembership(ma.SupportingRuleIDs, allowed)

	fabricated := append(append([]string{}, fabricatedFacts...), fabricatedRules...)

	arg := Argument{
		IssueNodeID:       ma.IssueNodeID,
		Claim:             ma.Claim,
		SupportingFactIDs: factIDs,
		SupportingRuleIDs: ruleIDs,
		Counterarguments:  ma.Counterarguments,
		Grounded:          len(fabricated) == 0,
		FabricatedNodeIDs: fabricated,
	}

	if len(factIDs) == 0 && len(ruleIDs) == 0 {
		return Argument{}, false
	}
	return arg, true
}

// partitionByMembership splits ids into those present in allowed (kept,
// in original order) and those absent (fabricated).
func partitionByMembership(ids []string, allowed map[string]struct{}) (kept, fabricated []string) {
	for _, id := range ids {
		if _, ok := allowed[id]; ok {
			kept = append(kept, id)
		} else {
			fabricated = append(fabricated, id)
		}
	}
	return kept, fabricated
}
