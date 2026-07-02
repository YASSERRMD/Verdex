package secondpartyagent

// groundArgument cross-checks every ID in ma.SupportingFactIDs and
// ma.SupportingRuleIDs against allowedNodes (the exact set of fact/rule
// IDs fetchIssueEvidence resolved for this issue from the case's actual
// tree — see issueEvidence.allowedNodeIDs), and every ID in
// ma.RebutsArgumentIDs against allowedArgumentIDs (the exact set of
// firstpartyagent.Argument.ID values present in the first-party
// ArgumentSet supplied to New). Any ID not present in its respective
// allowed set is a fabrication: the model referenced a node or an
// opposing argument that does not actually exist. Fabricated IDs are
// stripped rather than trusted, and recorded in the returned Argument's
// FabricatedNodeIDs / FabricatedRebuttalIDs for transparency — this is
// the anti-fabrication guarantee packages/secondpartyagent inherits from
// packages/firstpartyagent and extends to rebuttal linkage (see
// doc/second-party-agent.md).
//
// groundArgument never rejects an Argument outright for having some
// fabricated supporting-node IDs, as long as at least one real
// supporting fact or rule ID remains after stripping — an argument
// grounded in partially real evidence is still useful, with
// Grounded=false and FabricatedNodeIDs flagging exactly what was removed
// for a caller/reviewer to audit. An Argument left with zero supporting
// IDs of either kind after stripping is considered ungroundable and
// (ok=false) should be dropped by the caller (see assemble.go), since a
// "claim" with no tree-backed evidence at all is indistinguishable from
// an unsupported assertion.
//
// A fabricated RebuttalArgumentID, by contrast, never causes the
// Argument itself to be dropped — an argument can stand on its own
// supporting evidence even if one of its claimed rebuttal targets turns
// out to be invalid. The fabricated ID is simply stripped from
// RebutsArgumentIDs and recorded in FabricatedRebuttalIDs.
func groundArgument(ma modelArgument, allowedNodes map[string]struct{}, allowedArgumentIDs map[string]struct{}) (Argument, bool) {
	factIDs, fabricatedFacts := partitionByMembership(ma.SupportingFactIDs, allowedNodes)
	ruleIDs, fabricatedRules := partitionByMembership(ma.SupportingRuleIDs, allowedNodes)
	rebutIDs, fabricatedRebuts := partitionByMembership(ma.RebutsArgumentIDs, allowedArgumentIDs)

	fabricated := append(append([]string{}, fabricatedFacts...), fabricatedRules...)

	arg := Argument{
		IssueNodeID:           ma.IssueNodeID,
		Claim:                 ma.Claim,
		SupportingFactIDs:     factIDs,
		SupportingRuleIDs:     ruleIDs,
		RebutsArgumentIDs:     rebutIDs,
		Counterarguments:      ma.Counterarguments,
		Grounded:              len(fabricated) == 0,
		FabricatedNodeIDs:     fabricated,
		FabricatedRebuttalIDs: fabricatedRebuts,
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
