package lawapplication

// ruleRefIndex indexes rules by ID for O(1) lookup during citation
// attachment and confidence scoring.
func ruleRefIndex(rules []RuleRef) map[string]RuleRef {
	idx := make(map[string]RuleRef, len(rules))
	for _, r := range rules {
		idx[r.ID] = r
	}
	return idx
}

// AttachCitations resolves one AppliedCitation per entry in
// controllingRuleIDs, via lookup (typically backed by
// knowledgeapi.ResolveCitation). Every controlling rule gets an
// AppliedCitation entry regardless of lookup outcome — an unresolved or
// unverified citation is recorded with Resolved/Verified set
// accordingly rather than silently dropped, per the plan's "cite every
// applied authority... track unresolved/unverified citations as a
// quality signal" requirement. A nil lookup (no citation resolver
// configured) records every entry as unresolved.
func AttachCitations(controllingRuleIDs []string, rules []RuleRef, lookup CitationLookupFunc) []AppliedCitation {
	idx := ruleRefIndex(rules)

	out := make([]AppliedCitation, 0, len(controllingRuleIDs))
	for _, ruleID := range controllingRuleIDs {
		origin := InferOrigin(idx[ruleID])

		if lookup == nil {
			out = append(out, AppliedCitation{
				RuleID:   ruleID,
				Origin:   origin,
				Resolved: false,
			})
			continue
		}

		citationText, resolvedOrigin, verified, status, err := lookup(ruleID)
		if err != nil {
			out = append(out, AppliedCitation{
				RuleID:   ruleID,
				Origin:   origin,
				Resolved: false,
			})
			continue
		}

		if resolvedOrigin.IsValid() && resolvedOrigin != OriginUnknown {
			origin = resolvedOrigin
		}

		out = append(out, AppliedCitation{
			RuleID:             ruleID,
			Citation:           citationText,
			Origin:             origin,
			Resolved:           true,
			Verified:           verified,
			VerificationStatus: status,
		})
	}

	return out
}
