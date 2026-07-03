package grounding

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// nowFunc is overridden in tests for deterministic Report.GeneratedAt
// values, mirroring packages/reasoningtrace's nowFunc convention.
var nowFunc = time.Now

// Check runs a full grounding verification pass over opinion: every
// SupportingFactIDs/SupportingRuleIDs reference is cross-checked against
// the case's actual tree (reference.go), every controlling rule's
// citation is independently re-verified via packages/citation
// (citations.go), and every numeric figure or date mentioned in a
// conclusion's Text is checked against that same conclusion's own
// supporting fact text (consistency.go). The result is a Report with one
// ConclusionResult per opinion.Conclusions entry, a flattened Findings
// list, and a per-opinion confidence score.
//
// It requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase (see RequireCheckPermission) — checked first, so
// an unauthorized caller never triggers a single store read. Returns
// ErrEmptyCaseID if caseID is empty, ErrNilGraphStore if store is nil,
// and ErrOpinionCaseMismatch if opinion.CaseID does not equal caseID.
//
// Check reads the case's entire tree once (a single
// graph.GraphStore.Traverse call scoped to caseID) rather than one
// GetNode per referenced ID, since a typical opinion references a large
// fraction of the case's own fact/rule nodes anyway.
func Check(ctx context.Context, caseID string, store graph.GraphStore, opinion synthesisagent.Opinion) (Report, error) {
	if err := RequireCheckPermission(ctx); err != nil {
		return Report{}, err
	}
	if caseID == "" {
		return Report{}, ErrEmptyCaseID
	}
	if store == nil {
		return Report{}, ErrNilGraphStore
	}
	if opinion.CaseID != "" && opinion.CaseID != caseID {
		return Report{}, fmt.Errorf("%w: opinion case %q, requested case %q", ErrOpinionCaseMismatch, opinion.CaseID, caseID)
	}

	treeNodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID})
	if err != nil {
		return Report{}, fmt.Errorf("grounding: failed to load tree for case %q: %w", caseID, err)
	}
	nodes := nodesByID(treeNodes)

	report := Report{
		CaseID:      caseID,
		Conclusions: make([]ConclusionResult, len(opinion.Conclusions)),
		GeneratedAt: nowFunc(),
	}

	for i, tc := range opinion.Conclusions {
		result, err := checkConclusion(ctx, store, caseID, tc, nodes)
		if err != nil {
			return Report{}, err
		}
		report.Conclusions[i] = result
		report.Findings = append(report.Findings, result.Findings...)
	}

	report.OpinionScore = scoreOpinion(report.Conclusions)
	return report, nil
}

// checkConclusion runs every grounding check against one
// TentativeConclusion and returns its ConclusionResult.
func checkConclusion(
	ctx context.Context,
	store graph.GraphStore,
	caseID string,
	tc synthesisagent.TentativeConclusion,
	nodes map[string]irac.Node,
) (ConclusionResult, error) {
	claims := ExtractClaims(tc)
	outcomes := make([]VerificationOutcome, len(claims))
	var findings []Finding

	supportingText := supportingFactText(tc.SupportingFactIDs, nodes)

	for i, claim := range claims {
		switch claim.Kind {
		case ClaimReference:
			outcome, finding := verifyReferenceClaim(claim, nodes)
			outcomes[i] = outcome
			if finding != nil {
				findings = append(findings, *finding)
			}
		case ClaimNumeric, ClaimDate:
			outcome, finding := verifyFigureClaim(claim, supportingText)
			outcomes[i] = outcome
			if finding != nil {
				findings = append(findings, *finding)
			} else if outcome == OutcomeUnverifiable {
				findings = append(findings, unverifiableFinding(claim))
			}
		default:
			outcomes[i] = OutcomeUnverifiable
			findings = append(findings, unverifiableFinding(claim))
		}
	}

	citationFindings, err := verifyCitations(ctx, store, caseID, tc.SupportingRuleIDs)
	if err != nil {
		return ConclusionResult{}, fmt.Errorf("grounding: citation verification failed for issue %q: %w", tc.IssueNodeID, err)
	}

	result := ConclusionResult{
		IssueNodeID:      tc.IssueNodeID,
		Claims:           claims,
		Outcomes:         outcomes,
		Findings:         findings,
		CitationFindings: citationFindings,
	}
	result.ConfidenceScore = scoreConclusion(outcomes, citationFindings)
	return result, nil
}
