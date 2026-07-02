package reasoningorchestration

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

// treeSnapshot is this package's own narrow view of a case's tree,
// fetched once per run via knowledgeapi.GetTree and reused across
// StageEvidenceWeighing and StageLawApplication: every FactNode (as
// evidenceweighing.FactRef), every RuleNode (as lawapplication.RuleRef),
// and the Rule--governs-->Issue edges (for lawapplication.IssueInput.
// GoverningRuleIDs).
type treeSnapshot struct {
	facts            []evidenceweighing.FactRef
	rules            []lawapplication.RuleRef
	governingByIssue map[string][]string
	allIssueNodeIDs  []string
}

// fetchTreeSnapshot loads caseID's full tree (unfiltered, so both fact
// and rule nodes and every governs edge come back in one call) and
// derives a treeSnapshot from it.
func fetchTreeSnapshot(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string) (treeSnapshot, error) {
	resp, err := api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: caseID})
	if err != nil {
		return treeSnapshot{}, fmt.Errorf("reasoningorchestration: fetch tree: %w", err)
	}

	snap := treeSnapshot{governingByIssue: make(map[string][]string)}

	for _, n := range resp.Nodes {
		switch irac.NodeType(n.Type) {
		case irac.NodeFact:
			snap.facts = append(snap.facts, evidenceweighing.FactRef{
				ID: n.ID, Text: n.Text, Confidence: n.Confidence,
			})
		case irac.NodeRule:
			snap.rules = append(snap.rules, lawapplication.RuleRef{ID: n.ID, Text: n.Text})
		case irac.NodeIssue:
			snap.allIssueNodeIDs = append(snap.allIssueNodeIDs, n.ID)
		}
	}

	for _, e := range resp.Edges {
		if irac.EdgeType(e.Type) == irac.EdgeGoverns {
			snap.governingByIssue[e.ToID] = append(snap.governingByIssue[e.ToID], e.FromID)
		}
	}

	return snap, nil
}

// issueInputs converts every FramedIssue in issues into a
// lawapplication.IssueInput, attaching each issue's governing rule IDs
// from the snapshot.
func (s treeSnapshot) issueInputs(issues []issueagent.FramedIssue) []lawapplication.IssueInput {
	out := make([]lawapplication.IssueInput, 0, len(issues))
	for _, fi := range issues {
		out = append(out, lawapplication.IssueInput{
			Issue:            fi,
			GoverningRuleIDs: s.governingByIssue[fi.SourceIssueNodeID],
		})
	}
	return out
}

// citationLookup adapts api.ResolveCitation to lawapplication.CitationLookupFunc.
func citationLookup(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string) lawapplication.CitationLookupFunc {
	return func(ruleID string) (string, lawapplication.Origin, bool, string, error) {
		resp, err := api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: caseID, NodeID: ruleID})
		if err != nil {
			return "", "", false, "", err
		}
		origin := lawapplication.Origin(resp.Citation.Origin)
		return resp.Citation.Citation, origin, resp.Citation.Verified, resp.Citation.VerificationStatus, nil
	}
}
