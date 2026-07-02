package issueagent

import (
	"encoding/json"
	"time"
)

// assembleResult combines each issueContext with the model's (possibly
// partial) framing response into the final, ranked IssueAnalysisResult.
// An issue the model's response did not cover still gets a FramedIssue —
// built entirely from heuristic signals — rather than silently vanishing
// from the result.
func assembleResult(caseID, jurisdictionCode, legalFamily string, contexts []issueContext, modelResp modelFramingResponse) IssueAnalysisResult {
	byID := modelResp.byIssueNodeID()

	issues := make([]FramedIssue, 0, len(contexts))
	for _, ic := range contexts {
		issues = append(issues, framIssue(ic, byID[ic.Node.ID]))
	}
	rankIssues(issues)

	return IssueAnalysisResult{
		CaseID:           caseID,
		JurisdictionCode: jurisdictionCode,
		LegalFamily:      legalFamily,
		Issues:           issues,
		GeneratedAt:      time.Now().UTC(),
	}
}

// framIssue builds one FramedIssue from an issueContext and the matching
// (possibly zero-value, if the model omitted this issue)
// modelFramedIssue.
func framIssue(ic issueContext, m modelFramedIssue) FramedIssue {
	_, modelScoreOK := scoreOK(m)
	heuristicScore := heuristicMaterialityScore(ic)
	materiality := blendMateriality(heuristicScore, m.MaterialityScore, modelScoreOK)

	ruleLinkageSignal := ruleLinkageConfidenceSignal(len(ic.GoverningRule))
	_, modelConfidenceOK := confidenceOK(m)
	confidence := blendConfidence(ruleLinkageSignal, m.Confidence, modelConfidenceOK)

	ambiguities := append([]string{}, heuristicAmbiguities(ic)...)
	ambiguities = append(ambiguities, m.Ambiguities...)

	questions := m.GoverningQuestions
	if len(questions) == 0 {
		questions = fallbackGoverningQuestions(ic)
	}

	return FramedIssue{
		SourceIssueNodeID:  ic.Node.ID,
		Question:           ic.Node.Text,
		MaterialityScore:   materiality,
		GoverningQuestions: questions,
		Ambiguities:        ambiguities,
		Confidence:         confidence,
		RuleLinkageCount:   len(ic.GoverningRule),
	}
}

// scoreOK reports whether m carries a usable (non-default-zero,
// present-in-response) materiality score. A model that genuinely scores
// an issue at exactly 0.0 is indistinguishable from an omitted field
// here; this is an accepted approximation documented on FramedIssue's
// MaterialityScore field; SourceIssueNodeID presence is what actually
// distinguishes "covered by the model" from "not covered", so callers
// wanting to special-case a true zero score should also check whether the
// issue's SourceIssueNodeID appears in the model's raw response.
func scoreOK(m modelFramedIssue) (float64, bool) {
	if m.SourceIssueNodeID == "" {
		return 0, false
	}
	return m.MaterialityScore, true
}

// confidenceOK mirrors scoreOK for the Confidence field.
func confidenceOK(m modelFramedIssue) (float64, bool) {
	if m.SourceIssueNodeID == "" {
		return 0, false
	}
	return m.Confidence, true
}

// fallbackGoverningQuestions derives a governing question per linked
// RuleNode directly from its text when the model's response omitted this
// issue (or omitted GoverningQuestions for it), so a FramedIssue never
// has an empty GoverningQuestions purely due to a partial model response
// when rule linkage exists to derive one from.
func fallbackGoverningQuestions(ic issueContext) []string {
	if len(ic.GoverningRule) == 0 {
		return nil
	}
	out := make([]string, 0, len(ic.GoverningRule))
	for _, r := range ic.GoverningRule {
		out = append(out, r.Text)
	}
	return out
}

// encodeResult JSON-encodes result for use as Decision.FinalText.
func encodeResult(result IssueAnalysisResult) (string, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeResult unmarshals a JSON-encoded IssueAnalysisResult, as produced
// by encodeResult and carried in agentframework.Result.FinalText. Exposed
// so a caller using agentframework.Runner directly (rather than this
// package's Analyze convenience function) can still recover a typed
// result.
func DecodeResult(finalText string) (IssueAnalysisResult, error) {
	var result IssueAnalysisResult
	if err := json.Unmarshal([]byte(finalText), &result); err != nil {
		return IssueAnalysisResult{}, err
	}
	return result, nil
}
