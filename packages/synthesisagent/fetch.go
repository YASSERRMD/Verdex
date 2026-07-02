package synthesisagent

import (
	"context"
	"fmt"
	"sort"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// maxTreeNodesFetched bounds a single GetTree page, mirroring
// packages/firstpartyagent/fetch.go's own constant and rationale.
const maxTreeNodesFetched = 5000

// issueSynthesisInput bundles one FramedIssue with everything this
// package needs to render its synthesis prompt section and ground the
// model's response for it: both parties' arguments on the issue, the
// issue's IssueApplication (controlling rules/citations/conflicts), the
// evidence-weighing FactWeights for facts either party cited, and the
// exact set of fact/rule node IDs the model is permitted to cite.
type issueSynthesisInput struct {
	Issue           issueagent.FramedIssue
	FirstArguments  []firstpartyagent.Argument
	SecondArguments []secondpartyagent.Argument
	Application     lawapplication.IssueApplication
	HasApplication  bool
	FactWeights     map[string]evidenceweighing.FactWeight
	Facts           []knowledgeapi.NodeDTO
	Rules           []knowledgeapi.NodeDTO
}

// allowedNodeIDs returns the set of every FactNode/RuleNode ID present in
// in_, the exact set a TentativeConclusion on this issue is permitted to
// cite. Used both to render the prompt's evidence list and, downstream,
// as the ground-truth set ground.go validates the model's response
// against.
func (in issueSynthesisInput) allowedNodeIDs() map[string]struct{} {
	out := make(map[string]struct{}, len(in.Facts)+len(in.Rules))
	for _, f := range in.Facts {
		out[f.ID] = struct{}{}
	}
	for _, r := range in.Rules {
		out[r.ID] = struct{}{}
	}
	return out
}

// fetchSynthesisInputs pulls the case's full tree once via api.GetTree
// and, for every issue in issues, resolves the tree-backed facts/rules
// available as citable evidence, then pairs that with the corresponding
// arguments (by IssueNodeID and PartyID), IssueApplication (by
// IssueNodeID), and FactWeights (by FactNodeID) already computed
// upstream.
//
// Facts/rules offered per issue are the union of: every fact/rule either
// party's arguments on this issue actually cited (SupportingFactIDs/
// SupportingRuleIDs), plus every controlling rule the law-application
// stage found for this issue. This keeps the model's citable universe
// grounded in what upstream agents already validated, rather than
// re-deriving governs/applies_to graph structure independently.
func fetchSynthesisInputs(
	ctx context.Context,
	api *knowledgeapi.KnowledgeAPI,
	caseID string,
	issues []issueagent.FramedIssue,
	firstParty firstpartyagent.ArgumentSet,
	secondParty secondpartyagent.ArgumentSet,
	evidence evidenceweighing.Result,
	lawApp lawapplication.Result,
) ([]issueSynthesisInput, error) {
	tree, err := api.GetTree(ctx, knowledgeapi.GetTreeRequest{
		CaseID: caseID,
		Page:   knowledgeapi.PageRequest{Page: 1, PerPage: maxTreeNodesFetched},
	})
	if err != nil {
		return nil, fmt.Errorf("synthesisagent: fetch case tree: %w", err)
	}
	nodesByID := make(map[string]knowledgeapi.NodeDTO, len(tree.Nodes))
	for _, n := range tree.Nodes {
		nodesByID[n.ID] = n
	}

	firstByIssue := groupFirstPartyByIssue(firstParty)
	secondByIssue := groupSecondPartyByIssue(secondParty)
	appByIssue := groupApplicationsByIssue(lawApp)
	weightByFact := groupFactWeights(evidence)

	out := make([]issueSynthesisInput, 0, len(issues))
	for _, issue := range issues {
		firstArgs := firstByIssue[issue.SourceIssueNodeID]
		secondArgs := secondByIssue[issue.SourceIssueNodeID]
		app, hasApp := appByIssue[issue.SourceIssueNodeID]

		factIDs := make(map[string]struct{})
		ruleIDs := make(map[string]struct{})
		for _, a := range firstArgs {
			addAll(factIDs, a.SupportingFactIDs)
			addAll(ruleIDs, a.SupportingRuleIDs)
		}
		for _, a := range secondArgs {
			addAll(factIDs, a.SupportingFactIDs)
			addAll(ruleIDs, a.SupportingRuleIDs)
		}
		if hasApp {
			addAll(ruleIDs, app.ControllingRuleIDs)
		}

		facts := resolveNodes(nodesByID, factIDs, irac.NodeFact)
		rules := resolveNodes(nodesByID, ruleIDs, irac.NodeRule)
		sortNodesByID(facts)
		sortNodesByID(rules)

		factWeights := make(map[string]evidenceweighing.FactWeight, len(facts))
		for _, f := range facts {
			if fw, ok := weightByFact[f.ID]; ok {
				factWeights[f.ID] = fw
			}
		}

		out = append(out, issueSynthesisInput{
			Issue:           issue,
			FirstArguments:  firstArgs,
			SecondArguments: secondArgs,
			Application:     app,
			HasApplication:  hasApp,
			FactWeights:     factWeights,
			Facts:           facts,
			Rules:           rules,
		})
	}
	return out, nil
}

// groupFirstPartyByIssue groups set's Arguments by IssueNodeID.
func groupFirstPartyByIssue(set firstpartyagent.ArgumentSet) map[string][]firstpartyagent.Argument {
	out := make(map[string][]firstpartyagent.Argument)
	for _, a := range set.Arguments {
		out[a.IssueNodeID] = append(out[a.IssueNodeID], a)
	}
	return out
}

// groupSecondPartyByIssue groups set's Arguments by IssueNodeID.
func groupSecondPartyByIssue(set secondpartyagent.ArgumentSet) map[string][]secondpartyagent.Argument {
	out := make(map[string][]secondpartyagent.Argument)
	for _, a := range set.Arguments {
		out[a.IssueNodeID] = append(out[a.IssueNodeID], a)
	}
	return out
}

// groupApplicationsByIssue indexes result's IssueApplications by
// IssueNodeID.
func groupApplicationsByIssue(result lawapplication.Result) map[string]lawapplication.IssueApplication {
	out := make(map[string]lawapplication.IssueApplication, len(result.IssueApplications))
	for _, app := range result.IssueApplications {
		out[app.IssueNodeID] = app
	}
	return out
}

// groupFactWeights indexes result's FactWeights by FactNodeID.
func groupFactWeights(result evidenceweighing.Result) map[string]evidenceweighing.FactWeight {
	out := make(map[string]evidenceweighing.FactWeight, len(result.FactWeights))
	for _, fw := range result.FactWeights {
		out[fw.FactNodeID] = fw
	}
	return out
}

// addAll inserts every id in ids into set.
func addAll(set map[string]struct{}, ids []string) {
	for _, id := range ids {
		set[id] = struct{}{}
	}
}

// resolveNodes returns every NodeDTO in ids present in nodesByID with the
// given wantType, skipping IDs that either do not exist in the tree or
// exist as a different node type.
func resolveNodes(nodesByID map[string]knowledgeapi.NodeDTO, ids map[string]struct{}, wantType irac.NodeType) []knowledgeapi.NodeDTO {
	out := make([]knowledgeapi.NodeDTO, 0, len(ids))
	for id := range ids {
		n, ok := nodesByID[id]
		if !ok || n.Type != string(wantType) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// sortNodesByID sorts nodes by ID ascending, for deterministic prompt
// rendering across repeated runs over identical input.
func sortNodesByID(nodes []knowledgeapi.NodeDTO) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}
