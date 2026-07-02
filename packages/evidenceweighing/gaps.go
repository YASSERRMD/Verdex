package evidenceweighing

// DetectGaps surfaces defects in the evidentiary record from arguments
// and facts: any SupportingFactID that does not resolve to a known
// FactRef (GapKindMissingFact), and any issue among issueNodeIDs for
// which no argument cites at least one fact (GapKindUncitedIssue).
//
// issueNodeIDs is supplied explicitly (rather than inferred purely from
// arguments) so a caller can surface an issue that received arguments
// with zero SupportingFactIDs at all (an issue argued on no evidence,
// which would otherwise not appear in arguments' fact-citation data in
// any form) as well as an issue that received no arguments whatsoever.
//
// A missing-fact gap should already be prevented by each argument agent's
// own anti-fabrication grounding step (firstpartyagent/ground.go,
// secondpartyagent/ground.go), but this package checks it defensively
// rather than trusting that invariant, per the plan's explicit
// requirement to surface gaps in the evidentiary record.
func DetectGaps(arguments []CitingArgument, facts []FactRef, issueNodeIDs []string) []Gap {
	knownFacts := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		knownFacts[f.ID] = struct{}{}
	}

	var gaps []Gap

	citedIssues := make(map[string]struct{}, len(issueNodeIDs))
	for _, arg := range arguments {
		hasFact := false
		for _, factID := range arg.SupportingFactIDs {
			if factID == "" {
				continue
			}
			if _, ok := knownFacts[factID]; !ok {
				gaps = append(gaps, Gap{
					Kind:        GapKindMissingFact,
					IssueNodeID: arg.IssueNodeID,
					ArgumentID:  arg.ArgumentID,
					FactNodeID:  factID,
					Description: "argument " + arg.ArgumentID + " cites fact " + factID + " which does not exist in the case's tree",
				})
				continue
			}
			hasFact = true
		}
		if hasFact && arg.IssueNodeID != "" {
			citedIssues[arg.IssueNodeID] = struct{}{}
		}
	}

	for _, issueID := range issueNodeIDs {
		if issueID == "" {
			continue
		}
		if _, ok := citedIssues[issueID]; !ok {
			gaps = append(gaps, Gap{
				Kind:        GapKindUncitedIssue,
				IssueNodeID: issueID,
				Description: "issue " + issueID + " has no argument from either party citing any fact",
			})
		}
	}

	return gaps
}
