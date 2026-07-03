package grounding

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/graph"
)

// verifyCitations runs packages/citation's existence verification over
// every rule node ID in ruleIDs (typically a TentativeConclusion's
// SupportingRuleIDs), reusing citation.Verify/citation.FindingsFromVerification
// rather than reimplementing anti-hallucination citation checking — this
// package's job is to compose citation's verdict into a grounding Report,
// not to duplicate the check itself.
//
// For every ruleID, verifyCitations builds a minimal citation.CitedUnit
// (NodeID/CaseID only — no Resolver is run, since this package only needs
// to know whether the cited node exists, not to format its citation
// text), verifies it against store, and translates the result into
// citation.Finding values via citation.FindingsFromVerification. A rule
// node that resolves but carries no citation text is not this function's
// concern (that is packages/citation's own Resolver/FindingsFromUnit
// pipeline, which requires a Resolver this package does not have); a rule
// node that does not exist, or exists under the wrong case, is.
//
// Returns the first unexpected store error immediately (mirroring
// citation.VerifyAll), alongside whatever citation.Finding values were
// already collected.
func verifyCitations(ctx context.Context, store graph.GraphStore, caseID string, ruleIDs []string) ([]citation.Finding, error) {
	var findings []citation.Finding
	for _, ruleID := range ruleIDs {
		unit := citation.CitedUnit{NodeID: ruleID, CaseID: caseID}
		result, err := citation.Verify(ctx, store, unit)
		if err != nil {
			return findings, err
		}
		findings = append(findings, citation.FindingsFromVerification(result)...)
	}
	return findings, nil
}
