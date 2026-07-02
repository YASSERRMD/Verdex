package synthesisagent

import (
	"encoding/json"
	"sort"
	"time"
)

// assembleOpinion grounds every modelConclusion the model proposed
// against inputs (the exact per-issue evidence fetchSynthesisInputs
// resolved from the case's tree), derives each surviving conclusion's
// weakest link, and returns the final Opinion.
//
// An issue for which every proposed conclusion fails grounding
// contributes no TentativeConclusion and is recorded in
// Opinion.SkippedIssueNodeIDs rather than causing the whole run to fail —
// mirroring firstpartyagent.assembleArgumentSet's per-issue,
// non-fatal-skip convention exactly.
func assembleOpinion(caseID string, inputs []issueSynthesisInput, modelResp modelSynthesisResponse) Opinion {
	byIssue := modelResp.byIssueNodeID()

	var conclusions []TentativeConclusion
	var skipped []string

	for _, in := range inputs {
		allowed := in.allowedNodeIDs()

		proposals := byIssue[in.Issue.SourceIssueNodeID]
		var groundedForIssue []TentativeConclusion
		for _, mc := range proposals {
			tc, ok := groundConclusion(mc, allowed)
			if !ok {
				continue
			}
			tc.WeakestLink = deriveWeakestLink(tc, in, mc.WeakestLink)
			groundedForIssue = append(groundedForIssue, tc)
		}

		if len(groundedForIssue) == 0 {
			skipped = append(skipped, in.Issue.SourceIssueNodeID)
			continue
		}
		conclusions = append(conclusions, groundedForIssue...)
	}

	sort.Strings(skipped)

	return Opinion{
		CaseID:              caseID,
		Conclusions:         conclusions,
		SkippedIssueNodeIDs: skipped,
		GeneratedAt:         time.Now().UTC(),
	}
}

// encodeResult JSON-encodes opinion for use as Decision.FinalText.
func encodeResult(opinion Opinion) (string, error) {
	b, err := json.Marshal(opinion)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeResult unmarshals a JSON-encoded Opinion, as produced by
// encodeResult and carried in agentframework.Result.FinalText. Exposed so
// a caller using agentframework.Runner directly (rather than this
// package's Synthesize convenience function) can still recover a typed
// result.
func DecodeResult(finalText string) (Opinion, error) {
	var opinion Opinion
	if err := json.Unmarshal([]byte(finalText), &opinion); err != nil {
		return Opinion{}, err
	}
	return opinion, nil
}
