package firstpartyagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// assembleArgumentSet grounds and scores every modelArgument the model
// proposed against evidence (the exact per-issue evidence
// fetchIssueEvidence resolved from the case's tree), resolves citations
// for each surviving argument's supporting rules via resolveCitations,
// and returns the final ArgumentSet.
//
// An issue for which every proposed argument fails grounding (see
// ground.go) contributes no Argument and is recorded in
// ArgumentSet.SkippedIssueNodeIDs rather than causing the whole run to
// fail — a caller inspecting SkippedIssueNodeIDs can distinguish "this
// issue's arguments were all fabricated" from "this issue was never
// framed to begin with".
func assembleArgumentSet(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string, partyID PartyID, evidence []issueEvidence, modelResp modelArgumentResponse) (ArgumentSet, error) {
	byIssue := modelResp.byIssueNodeID()

	var arguments []Argument
	var skipped []string

	for _, ev := range evidence {
		allowed := ev.allowedNodeIDs()
		factsByID := indexNodesByID(ev.Facts)

		proposals := byIssue[ev.Issue.SourceIssueNodeID]
		var groundedForIssue []Argument
		for i, ma := range proposals {
			arg, ok := groundArgument(ma, allowed)
			if !ok {
				continue
			}
			arg.ID = fmt.Sprintf("%s-arg-%d", ev.Issue.SourceIssueNodeID, i)
			arg.PartyID = partyID

			citations, err := resolveCitations(ctx, api, caseID, arg.SupportingRuleIDs)
			if err != nil {
				return ArgumentSet{}, err
			}
			arg.Citations = citations
			arg.Strength = strengthScore(arg, factsByID)

			groundedForIssue = append(groundedForIssue, arg)
		}

		if len(groundedForIssue) == 0 {
			skipped = append(skipped, ev.Issue.SourceIssueNodeID)
			continue
		}
		arguments = append(arguments, groundedForIssue...)
	}

	sort.Strings(skipped)

	return ArgumentSet{
		CaseID:              caseID,
		PartyID:             partyID,
		Arguments:           arguments,
		SkippedIssueNodeIDs: skipped,
		GeneratedAt:         time.Now().UTC(),
	}, nil
}

// resolveCitations resolves a CitationRef for every ruleID via
// api.ResolveCitation, skipping (rather than failing the whole run for) a
// rule whose citation cannot be resolved — an unresolvable citation is a
// legitimate, common outcome (e.g. no citation.Resolver configured, or
// the rule genuinely has no external citation), not a fabrication signal;
// fabrication is already handled by ground.go before this function is
// ever reached.
func resolveCitations(ctx context.Context, api *knowledgeapi.KnowledgeAPI, caseID string, ruleIDs []string) ([]CitationRef, error) {
	var out []CitationRef
	for _, ruleID := range ruleIDs {
		resp, err := api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: caseID, NodeID: ruleID})
		if err != nil {
			continue
		}
		out = append(out, CitationRef{
			NodeID:             resp.Citation.NodeID,
			Citation:           resp.Citation.Citation,
			VerificationStatus: resp.Citation.VerificationStatus,
			Verified:           resp.Citation.Verified,
			ConfidenceScore:    resp.Citation.ConfidenceScore,
		})
	}
	return out, nil
}

// indexNodesByID indexes nodes by ID for O(1) lookup during scoring.
func indexNodesByID(nodes []knowledgeapi.NodeDTO) map[string]knowledgeapi.NodeDTO {
	out := make(map[string]knowledgeapi.NodeDTO, len(nodes))
	for _, n := range nodes {
		out[n.ID] = n
	}
	return out
}

// encodeResult JSON-encodes result for use as Decision.FinalText.
func encodeResult(result ArgumentSet) (string, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeResult unmarshals a JSON-encoded ArgumentSet, as produced by
// encodeResult and carried in agentframework.Result.FinalText. Exposed so
// a caller using agentframework.Runner directly (rather than this
// package's Argue convenience function) can still recover a typed
// result.
func DecodeResult(finalText string) (ArgumentSet, error) {
	var result ArgumentSet
	if err := json.Unmarshal([]byte(finalText), &result); err != nil {
		return ArgumentSet{}, err
	}
	return result, nil
}
