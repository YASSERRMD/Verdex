package grounding

import "github.com/YASSERRMD/verdex/packages/synthesisagent"

// ExtractClaims extracts every Claim from a single
// synthesisagent.TentativeConclusion: one ClaimReference per
// SupportingFactIDs/SupportingRuleIDs entry (what the conclusion says it
// relies on), plus every numeric figure (ClaimNumeric) and calendar date
// (ClaimDate) mentioned in the conclusion's Text (what the conclusion's
// prose actually asserts). ClaimCitation is never produced by
// ExtractClaims: citation-existence verification (citations.go) runs
// directly over a conclusion's SupportingRuleIDs against a
// graph.GraphStore via packages/citation, which needs a store to verify
// against and therefore cannot be part of this pure extraction step; its
// results are carried on ConclusionResult.CitationFindings instead of
// being folded back into a Claim/Finding pair the way the other three
// kinds are.
//
// ExtractClaims is a pure function: it never consults the case's tree, so
// it can never itself determine whether a claim is grounded — that is
// reference.go and consistency.go's job.
func ExtractClaims(tc synthesisagent.TentativeConclusion) []Claim {
	var claims []Claim

	for _, id := range tc.SupportingFactIDs {
		claims = append(claims, Claim{
			IssueNodeID: tc.IssueNodeID,
			Kind:        ClaimReference,
			Value:       id,
			SourceText:  tc.Text,
		})
	}
	for _, id := range tc.SupportingRuleIDs {
		claims = append(claims, Claim{
			IssueNodeID: tc.IssueNodeID,
			Kind:        ClaimReference,
			Value:       id,
			SourceText:  tc.Text,
		})
	}

	for _, n := range extractNumerics(tc.Text) {
		claims = append(claims, Claim{
			IssueNodeID: tc.IssueNodeID,
			Kind:        ClaimNumeric,
			Value:       n,
			SourceText:  tc.Text,
		})
	}
	for _, d := range extractDates(tc.Text) {
		claims = append(claims, Claim{
			IssueNodeID: tc.IssueNodeID,
			Kind:        ClaimDate,
			Value:       d,
			SourceText:  tc.Text,
		})
	}

	return claims
}

// ExtractOpinionClaims runs ExtractClaims over every conclusion in
// opinion, returning one []Claim slice per conclusion, in the same order
// as opinion.Conclusions.
func ExtractOpinionClaims(opinion synthesisagent.Opinion) [][]Claim {
	out := make([][]Claim, len(opinion.Conclusions))
	for i, tc := range opinion.Conclusions {
		out[i] = ExtractClaims(tc)
	}
	return out
}
