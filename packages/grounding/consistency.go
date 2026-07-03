package grounding

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// verifyFigureClaim checks a ClaimNumeric or ClaimDate Claim's Value
// against supportingText, the concatenated Text of every fact node the
// owning conclusion actually cites (its verified SupportingFactIDs,
// resolved to irac.Node.Text — see check.go). A figure or date the
// conclusion's prose asserts must appear, verbatim, in at least one of
// its own supporting facts to count as grounded: a conclusion is free to
// draw an inference from its facts, but a bare number or date it states
// as if it were itself a fact must actually be traceable to one.
//
// Returns:
//   - OutcomeUnverifiable (no Finding) when the conclusion has no
//     supporting fact text at all to check against — this is a coverage
//     gap, not a confirmed mismatch, and CodeUnverifiableClaim is raised
//     as a SeverityWarning Finding by the caller (check.go) rather than
//     here, to keep this function pure and side-effect-free per claim.
//   - OutcomeGrounded (no Finding) when claim.Value is found verbatim in
//     supportingText.
//   - OutcomeUngrounded plus a SeverityCritical Finding (CodeNumericMismatch
//     or CodeDateMismatch depending on claim.Kind) when supportingText is
//     non-empty but does not contain claim.Value — the conclusion's prose
//     asserts a figure or date its own cited facts do not support.
func verifyFigureClaim(claim Claim, supportingText string) (VerificationOutcome, *Finding) {
	if strings.TrimSpace(supportingText) == "" {
		return OutcomeUnverifiable, nil
	}
	if strings.Contains(supportingText, claim.Value) {
		return OutcomeGrounded, nil
	}

	code := CodeNumericMismatch
	label := "numeric figure"
	if claim.Kind == ClaimDate {
		code = CodeDateMismatch
		label = "date"
	}
	return OutcomeUngrounded, &Finding{
		Severity:    SeverityCritical,
		Code:        code,
		Message:     "conclusion for issue " + claim.IssueNodeID + " states " + label + " \"" + claim.Value + "\" which does not appear in any of its supporting facts",
		IssueNodeID: claim.IssueNodeID,
		Claim:       claim,
	}
}

// unverifiableFinding builds the SeverityWarning Finding raised for a
// ClaimNumeric/ClaimDate claim whose owning conclusion has no supporting
// fact text at all to check it against.
func unverifiableFinding(claim Claim) Finding {
	return Finding{
		Severity:    SeverityWarning,
		Code:        CodeUnverifiableClaim,
		Message:     "conclusion for issue " + claim.IssueNodeID + " has no supporting facts to verify its stated figures/dates against",
		IssueNodeID: claim.IssueNodeID,
		Claim:       claim,
	}
}

// supportingFactText concatenates the Text of every node in factIDs found
// in nodes, space-joined, for use as verifyFigureClaim's supportingText
// argument. Node IDs absent from nodes (already flagged separately by
// verifyReferenceClaim) are simply skipped rather than causing an error.
func supportingFactText(factIDs []string, nodes map[string]irac.Node) string {
	var parts []string
	for _, id := range factIDs {
		if n, ok := nodes[id]; ok {
			parts = append(parts, n.Text)
		}
	}
	return strings.Join(parts, " ")
}
