package grounding

import "github.com/YASSERRMD/verdex/packages/irac"

// verifyReferenceClaim checks a ClaimReference Claim's Value (a node ID)
// against nodes, the exact set of irac.Node IDs that actually exist in
// the case's tree (typically loaded via one graph.GraphStore.Traverse
// call scoped to the case — see check.go). This mirrors
// packages/synthesisagent/ground.go's groundConclusion/
// partitionByMembership check exactly, but runs as a second, independent
// pass over the already-assembled Opinion rather than trusting that the
// synthesis stage's own grounding pass is still valid once the opinion
// has been composed, exported, or otherwise handled by later code.
//
// Returns OutcomeGrounded when claim.Value is a key in nodes, and a
// SeverityCritical CodeFabricatedReference Finding plus OutcomeUngrounded
// otherwise — a conclusion asserting it relies on a node that does not
// exist in the case's own tree is exactly the hallucination this package
// exists to catch.
func verifyReferenceClaim(claim Claim, nodes map[string]irac.Node) (VerificationOutcome, *Finding) {
	if _, ok := nodes[claim.Value]; ok {
		return OutcomeGrounded, nil
	}
	return OutcomeUngrounded, &Finding{
		Severity:    SeverityCritical,
		Code:        CodeFabricatedReference,
		Message:     "conclusion for issue " + claim.IssueNodeID + " references node " + claim.Value + " which does not exist in the case's tree",
		IssueNodeID: claim.IssueNodeID,
		Claim:       claim,
	}
}

// nodesByID indexes nodes by their irac.Node.ID for O(1) membership
// lookups by verifyReferenceClaim and numeric/date checks.
func nodesByID(nodes []irac.Node) map[string]irac.Node {
	out := make(map[string]irac.Node, len(nodes))
	for _, n := range nodes {
		out[n.ID] = n
	}
	return out
}
