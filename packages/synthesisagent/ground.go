package synthesisagent

// groundConclusion cross-checks every ID in mc.SupportingFactIDs and
// mc.SupportingRuleIDs against allowed (the exact set of fact/rule IDs
// fetchSynthesisInputs resolved for this issue from the case's actual
// tree — see issueSynthesisInput.allowedNodeIDs). Any ID not present in
// allowed is a fabrication: the model referenced a node that either does
// not exist in the tree at all, or exists but was never offered as
// evidence for this issue. Fabricated IDs are stripped rather than
// trusted, and recorded in the returned TentativeConclusion's
// FabricatedNodeIDs for transparency — this is the same anti-fabrication
// guarantee packages/firstpartyagent and packages/secondpartyagent
// enforce, applied at the synthesis stage (see doc/synthesis-agent.md).
//
// groundConclusion never rejects a TentativeConclusion outright for
// having some fabricated IDs, as long as at least one real supporting
// fact or rule ID remains after stripping. A conclusion left with zero
// supporting IDs of either kind after stripping is considered
// ungroundable and (ok=false) should be dropped by the caller (see
// assemble.go), since a conclusion with no tree-backed evidence at all is
// indistinguishable from an unsupported assertion.
func groundConclusion(mc modelConclusion, allowed map[string]struct{}) (TentativeConclusion, bool) {
	factIDs, fabricatedFacts := partitionByMembership(mc.SupportingFactIDs, allowed)
	ruleIDs, fabricatedRules := partitionByMembership(mc.SupportingRuleIDs, allowed)

	fabricated := append(append([]string{}, fabricatedFacts...), fabricatedRules...)

	tc := TentativeConclusion{
		IssueNodeID:       mc.IssueNodeID,
		Text:              mc.Text,
		FavoredParty:      mc.FavoredParty,
		Confidence:        mc.Confidence,
		SupportingFactIDs: factIDs,
		SupportingRuleIDs: ruleIDs,
		Grounded:          len(fabricated) == 0,
		FabricatedNodeIDs: fabricated,
	}

	if len(factIDs) == 0 && len(ruleIDs) == 0 {
		return TentativeConclusion{}, false
	}
	return tc, true
}

// partitionByMembership splits ids into those present in allowed (kept,
// in original order) and those absent (fabricated), mirroring
// packages/firstpartyagent/ground.go's helper of the same name exactly.
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
